package worker

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/edgedelta/s3-edgedelta-streamer/internal/logging"
	"github.com/edgedelta/s3-edgedelta-streamer/internal/scanner"
	"github.com/edgedelta/s3-edgedelta-streamer/internal/state"
	"github.com/edgedelta/s3-edgedelta-streamer/internal/tcppool"
)

// Pool manages a pool of workers
type Pool struct {
	s3Client       *s3.Client
	tcpPool        *tcppool.Pool
	stateManager   *state.Manager
	bucket         string
	workerCount    int
	jobQueue       chan scanner.FileJob
	wg             sync.WaitGroup
	stopCh         chan struct{}
	filesProcessed atomic.Int64
	bytesProcessed atomic.Int64
	errors         atomic.Int64
}

// NewPool creates a new worker pool
func NewPool(
	s3Client *s3.Client,
	tcpPool *tcppool.Pool,
	stateManager *state.Manager,
	bucket string,
	workerCount int,
	queueSize int,
) *Pool {
	// Strip s3:// prefix from bucket name
	bucket = strings.TrimPrefix(bucket, "s3://")

	return &Pool{
		s3Client:     s3Client,
		tcpPool:      tcpPool,
		stateManager: stateManager,
		bucket:       bucket,
		workerCount:  workerCount,
		jobQueue:     make(chan scanner.FileJob, queueSize),
		stopCh:       make(chan struct{}),
	}
}

// Start starts all workers
func (p *Pool) Start() {
	for i := 0; i < p.workerCount; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}
}

// Stop stops all workers gracefully
func (p *Pool) Stop() {
	close(p.stopCh)
	close(p.jobQueue)
	p.wg.Wait()
}

// Submit submits a job to the worker pool
func (p *Pool) Submit(job scanner.FileJob) bool {
	select {
	case p.jobQueue <- job:
		return true
	case <-p.stopCh:
		return false
	default:
		// Queue is full
		return false
	}
}

// GetMetricsCounters returns pointers to the metrics counters
func (p *Pool) GetMetricsCounters() (*atomic.Int64, *atomic.Int64, *atomic.Int64) {
	return &p.filesProcessed, &p.bytesProcessed, &p.errors
}

// worker processes jobs from the queue
func (p *Pool) worker(id int) {
	defer p.wg.Done()

	for {
		select {
		case job, ok := <-p.jobQueue:
			if !ok {
				return // Channel closed
			}
			if err := p.processJob(job); err != nil {
				logging.GetDefaultLogger().Error("Worker failed to process job",
					"worker_id", id,
					"s3_key", job.S3Key,
					"error", err)
				p.errors.Add(1)
			} else {
				p.filesProcessed.Add(1)
			}
		case <-p.stopCh:
			return
		}
	}
}

// processJob downloads, decompresses, and streams a file to Edge Delta
func (p *Pool) processJob(job scanner.FileJob) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Download from S3
	result, err := p.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(p.bucket),
		Key:    aws.String(job.S3Key),
	})
	if err != nil {
		return fmt.Errorf("failed to get S3 object: %w", err)
	}
	defer result.Body.Close()

	// Decompress (all files are gzipped)
	gzReader, err := gzip.NewReader(result.Body)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzReader.Close()

	// Create a fresh TCP connection for each file (avoid Edge Delta connection timeouts)
	addr := fmt.Sprintf("%s:%d", p.tcpPool.GetHost(), p.tcpPool.GetPort())
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", addr, err)
	}
	defer conn.Close()

	// Stream decompressed data to TCP connection
	written, err := io.Copy(conn, gzReader)
	if err != nil {
		return fmt.Errorf("failed to stream to TCP: %w", err)
	}

	// Update state
	p.bytesProcessed.Add(written)
	p.stateManager.UpdateProgress(job.Timestamp, job.S3Key, written)

	return nil
}

// QueueDepth returns the current queue depth
func (p *Pool) QueueDepth() int {
	return len(p.jobQueue)
}
