package worker

import (
	"bufio"
	"compress/gzip"
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/edgedelta/s3-edgedelta-streamer/internal/scanner"
	"github.com/edgedelta/s3-edgedelta-streamer/internal/state"
	"gopkg.in/natefinch/lumberjack.v2"
)

// FilePool manages a pool of workers that write to rotating log files
type FilePool struct {
	s3Client       *s3.Client
	fileWriter     *lumberjack.Logger
	outputFilePath string
	stateManager   state.StateManager
	bucket         string
	workerCount    int
	jobQueue       chan scanner.FileJob
	wg             sync.WaitGroup
	stopCh         chan struct{}
	filesProcessed atomic.Int64
	bytesProcessed atomic.Int64
	errors         atomic.Int64
	activeWorkers  atomic.Int64 // Track actively processing workers
	writeMutex     sync.Mutex   // Protect concurrent writes to file
}

// NewFilePool creates a new file-based worker pool
func NewFilePool(
	s3Client *s3.Client,
	outputFilePath string,
	maxSizeMB int,
	maxBackups int,
	stateManager state.StateManager,
	bucket string,
	workerCount int,
	queueSize int,
) *FilePool {
	// Strip s3:// prefix from bucket name
	bucket = strings.TrimPrefix(bucket, "s3://")

	// Create lumberjack rotating file writer
	fileWriter := &lumberjack.Logger{
		Filename:   outputFilePath,
		MaxSize:    maxSizeMB,  // megabytes
		MaxBackups: maxBackups, // keep N old files
		Compress:   true,       // compress rotated files
		LocalTime:  true,       // use local time for filenames
	}

	return &FilePool{
		s3Client:       s3Client,
		fileWriter:     fileWriter,
		outputFilePath: outputFilePath,
		stateManager:   stateManager,
		bucket:         bucket,
		workerCount:    workerCount,
		jobQueue:       make(chan scanner.FileJob, queueSize),
		stopCh:         make(chan struct{}),
	}
}

// Start starts all workers
func (p *FilePool) Start() {
	for i := 0; i < p.workerCount; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}
}

// Stop stops all workers gracefully
func (p *FilePool) Stop() {
	close(p.stopCh)
	close(p.jobQueue)
	p.wg.Wait()
	p.fileWriter.Close()
}

// Submit submits a job to the worker pool
func (p *FilePool) Submit(job scanner.FileJob) bool {
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
func (p *FilePool) GetMetricsCounters() (*atomic.Int64, *atomic.Int64, *atomic.Int64) {
	return &p.filesProcessed, &p.bytesProcessed, &p.errors
}

// worker processes jobs from the queue
func (p *FilePool) worker(id int) {
	defer p.wg.Done()

	for {
		select {
		case job, ok := <-p.jobQueue:
			if !ok {
				return // Channel closed
			}
			// Track that this worker is actively processing
			p.activeWorkers.Add(1)
			if err := p.processJob(job); err != nil {
				fmt.Printf("Worker %d: Error processing %s: %v\n", id, job.S3Key, err)
				p.errors.Add(1)
			} else {
				p.filesProcessed.Add(1)
			}
			// Done processing, decrement active counter
			p.activeWorkers.Add(-1)
		case <-p.stopCh:
			return
		}
	}
}

// processJob downloads, decompresses, and writes file to rotating log
func (p *FilePool) processJob(job scanner.FileJob) error {
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

	// Process file line by line
	scanner := bufio.NewScanner(gzReader)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024) // 1MB initial, 10MB max buffer

	var totalBytes int64
	lineCount := 0

	// Lock for writing to ensure thread safety
	p.writeMutex.Lock()
	defer p.writeMutex.Unlock()

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		// Strip leading comma if present (some S3 data has it)
		if len(line) > 0 && line[0] == ',' {
			line = line[1:]
		}

		// Skip array bracket lines (not valid JSONL)
		trimmed := strings.TrimSpace(string(line))
		if len(trimmed) == 1 && (trimmed[0] == '[' || trimmed[0] == ']') {
			continue
		}

		// Write line to file (preserve JSONL format)
		n, err := p.fileWriter.Write(line)
		if err != nil {
			return fmt.Errorf("failed to write line to file: %w", err)
		}
		totalBytes += int64(n)

		// Write newline
		n, err = p.fileWriter.Write([]byte("\n"))
		if err != nil {
			return fmt.Errorf("failed to write newline to file: %w", err)
		}
		totalBytes += int64(n)

		lineCount++
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to scan file: %w", err)
	}

	// Update state
	p.bytesProcessed.Add(totalBytes)
	p.stateManager.UpdateProgress(job.Timestamp, job.S3Key, totalBytes)

	fmt.Printf("Processed %s: %d lines, %d bytes (written to file)\n", job.S3Key, lineCount, totalBytes)

	return nil
}

// QueueDepth returns the current queue depth
func (p *FilePool) QueueDepth() int {
	return len(p.jobQueue)
}

// WaitForIdle waits for all workers to finish processing (queue empty AND no active workers)
func (p *FilePool) WaitForIdle() {
	for {
		queueDepth := len(p.jobQueue)
		activeCount := p.activeWorkers.Load()

		if queueDepth == 0 && activeCount == 0 {
			return
		}

		time.Sleep(100 * time.Millisecond)
	}
}

// InjectMarker writes a special marker JSON line to the log file for tracking
func (p *FilePool) InjectMarker(markerID string, injectTime time.Time, markerType string) error {
	hostname, _ := os.Hostname()

	// Format marker to match the exact format of normal logs with spaces
	// Normal format: { "sourcetype" : "zscalernss-web", "event" : {"key":"value"}}
	markerJSON := fmt.Sprintf(
		`{ "sourcetype" : "edgedelta_marker", "event" : {"marker_id":"%s","inject_time":%.9f,"type":"%s","hostname":"%s"}}`,
		markerID,
		float64(injectTime.UnixNano())/1e9,
		markerType,
		hostname,
	)

	fmt.Printf("DEBUG: Marker JSON: %s\n", markerJSON)

	// Lock for writing to ensure thread safety
	p.writeMutex.Lock()
	defer p.writeMutex.Unlock()

	// Write marker JSON
	n, err := p.fileWriter.Write([]byte(markerJSON))
	if err != nil {
		return fmt.Errorf("failed to write marker to file: %w", err)
	}
	fmt.Printf("DEBUG: Wrote %d bytes of marker JSON\n", n)

	// Write newline
	n2, err := p.fileWriter.Write([]byte("\n"))
	if err != nil {
		return fmt.Errorf("failed to write newline after marker: %w", err)
	}
	fmt.Printf("DEBUG: Wrote %d bytes of newline, total marker size: %d bytes\n", n2, n+n2)

	// CRITICAL: Flush to disk so EdgeDelta can immediately see the marker
	// lumberjack doesn't have a Flush method, but we can sync the file manually
	if file, err := os.OpenFile(p.outputFilePath, os.O_WRONLY, 0); err == nil {
		if syncErr := file.Sync(); syncErr != nil {
			fmt.Printf("DEBUG: Failed to sync file: %v\n", syncErr)
		} else {
			fmt.Printf("DEBUG: Synced file to disk\n")
		}
		file.Close()
	}

	return nil
}

// GetCurrentFileSize returns the current size of the active log file in bytes
func (p *FilePool) GetCurrentFileSize() (int64, error) {
	fileInfo, err := os.Stat(p.outputFilePath)
	if err != nil {
		return 0, fmt.Errorf("failed to stat file: %w", err)
	}
	return fileInfo.Size(), nil
}

// RotateFile manually rotates the log file (closes current, starts new)
func (p *FilePool) RotateFile() error {
	p.writeMutex.Lock()
	defer p.writeMutex.Unlock()

	return p.fileWriter.Rotate()
}
