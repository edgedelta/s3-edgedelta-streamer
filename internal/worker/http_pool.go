package worker

import (
	"bufio"
	"compress/gzip"
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/edgedelta/s3-edgedelta-streamer/internal/formats"
	"github.com/edgedelta/s3-edgedelta-streamer/internal/logging"
	"github.com/edgedelta/s3-edgedelta-streamer/internal/metrics"
	"github.com/edgedelta/s3-edgedelta-streamer/internal/output"
	"github.com/edgedelta/s3-edgedelta-streamer/internal/scanner"
	"github.com/edgedelta/s3-edgedelta-streamer/internal/state"
)

// HTTPPool processes S3 files and sends lines via HTTP to EdgeDelta
type HTTPPool struct {
	s3Client     *s3.Client
	stateManager state.StateManager
	httpSender   *output.HTTPSender
	bucket       string
	workerCount  int
	jobQueue     chan scanner.FileJob
	wg           sync.WaitGroup
	stopChan     chan struct{}
	stopped      atomic.Bool

	// Metrics (local counters)
	filesProcessed atomic.Int64
	bytesProcessed atomic.Int64
	errors         atomic.Int64

	// OTLP metrics client
	metricsClient *metrics.Metrics

	// Log format for content processing
	logFormat formats.LogFormat
}

// NewHTTPPool creates a new HTTP worker pool
func NewHTTPPool(
	s3Client *s3.Client,
	httpSender *output.HTTPSender,
	stateManager state.StateManager,
	bucket string,
	workerCount int,
	queueSize int,
	metricsClient *metrics.Metrics,
	logFormat formats.LogFormat,
) *HTTPPool {
	return &HTTPPool{
		s3Client:      s3Client,
		httpSender:    httpSender,
		stateManager:  stateManager,
		bucket:        bucket,
		workerCount:   workerCount,
		jobQueue:      make(chan scanner.FileJob, queueSize),
		stopChan:      make(chan struct{}),
		metricsClient: metricsClient,
		logFormat:     logFormat,
	}
}

// Start starts the worker pool
func (hp *HTTPPool) Start() {
	for i := 0; i < hp.workerCount; i++ {
		hp.wg.Add(1)
		go hp.worker(i)
	}
}

// Stop gracefully stops the worker pool
func (hp *HTTPPool) Stop() {
	if hp.stopped.CompareAndSwap(false, true) {
		close(hp.stopChan)
		close(hp.jobQueue)
		hp.wg.Wait()
	}
}

// Submit submits a job to the worker pool
func (hp *HTTPPool) Submit(job scanner.FileJob) bool {
	select {
	case hp.jobQueue <- job:
		return true
	case <-hp.stopChan:
		return false
	default:
		return false
	}
}

// WaitForIdle waits until all jobs are processed
func (hp *HTTPPool) WaitForIdle() {
	for {
		if len(hp.jobQueue) == 0 {
			return
		}
	}
}

// worker processes jobs from the queue
func (hp *HTTPPool) worker(id int) {
	defer hp.wg.Done()

	for job := range hp.jobQueue {
		if err := hp.processFile(job); err != nil {
			logging.GetDefaultLogger().Error("Worker failed to process file",
				"worker_id", id,
				"s3_key", job.S3Key,
				"error", err)
			hp.errors.Add(1)
			if hp.metricsClient != nil {
				hp.metricsClient.RecordFileError(context.Background())
			}
		} else {
			hp.filesProcessed.Add(1)
			// State updates happen in main loop after batch completion
		}
	}
}

// processFile downloads and processes a single S3 file
func (hp *HTTPPool) processFile(job scanner.FileJob) error {
	startTime := time.Now()

	// Download from S3
	result, err := hp.s3Client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String(hp.bucket),
		Key:    aws.String(job.S3Key),
	})
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}
	defer result.Body.Close()

	// Decompress (all files are gzipped)
	gzReader, err := gzip.NewReader(result.Body)
	if err != nil {
		// Try reading as plain text if gzip fails (unlikely but handle it)
		return fmt.Errorf("failed to decompress (all files should be gzipped): %w", err)
	}
	defer gzReader.Close()

	// Read and send lines
	scanner := bufio.NewScanner(gzReader)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024) // 1MB max line size

	lineCount := 0
	byteCount := 0
	isFirstLine := true

	for scanner.Scan() {
		line := scanner.Bytes()
		lineCount++

		// Apply format-specific content processing
		processedLine, err := hp.logFormat.ProcessContent(line, isFirstLine)
		if err != nil {
			return fmt.Errorf("failed to process line %d: %w", lineCount, err)
		}
		isFirstLine = false

		// Skip lines that should be filtered out (e.g., headers)
		if processedLine == nil {
			continue
		}

		byteCount += len(processedLine)

		// Send processed line to HTTP sender
		lineCopy := make([]byte, len(processedLine))
		copy(lineCopy, processedLine)
		hp.httpSender.SendLine(lineCopy)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to scan: %w", err)
	}

	hp.bytesProcessed.Add(int64(byteCount))
	logging.GetDefaultLogger().Info("Processed file successfully",
		"s3_key", job.S3Key,
		"lines", lineCount,
		"bytes", byteCount,
		"destination", "http")

	// Record metrics
	if hp.metricsClient != nil {
		latency := time.Since(startTime)
		hp.metricsClient.RecordFileProcessed(context.Background(), int64(byteCount), latency)
	}

	return nil
}

// GetMetrics returns current metrics
func (hp *HTTPPool) GetMetrics() (files, bytes, errors int64) {
	return hp.filesProcessed.Load(), hp.bytesProcessed.Load(), hp.errors.Load()
}

// GetMetricsCounters returns atomic counters for metrics (for compatibility)
func (hp *HTTPPool) GetMetricsCounters() (*atomic.Int64, *atomic.Int64, *atomic.Int64) {
	return &hp.filesProcessed, &hp.bytesProcessed, &hp.errors
}
