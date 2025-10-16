package worker

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/edgedelta/s3-edgedelta-streamer/internal/metrics"
	"github.com/edgedelta/s3-edgedelta-streamer/internal/output"
	"github.com/edgedelta/s3-edgedelta-streamer/internal/scanner"
	"github.com/edgedelta/s3-edgedelta-streamer/internal/state"
)

func TestNewHTTPPool(t *testing.T) {
	// Create mock dependencies
	s3Client := &s3.Client{}
	var stateManager state.StateManager = &state.Manager{}
	httpSender := &output.HTTPSender{}
	bucket := "test-bucket"
	workerCount := 5
	queueSize := 100
	metricsClient := &metrics.Metrics{}

	pool := NewHTTPPool(s3Client, httpSender, stateManager, bucket, workerCount, queueSize, metricsClient, nil)

	if pool == nil {
		t.Fatal("NewHTTPPool returned nil")
	}

	if pool.bucket != bucket {
		t.Errorf("Expected bucket %s, got %s", bucket, pool.bucket)
	}

	if pool.workerCount != workerCount {
		t.Errorf("Expected workerCount %d, got %d", workerCount, pool.workerCount)
	}

	if cap(pool.jobQueue) != queueSize {
		t.Errorf("Expected queue size %d, got %d", queueSize, cap(pool.jobQueue))
	}
}

func TestHTTPPool_StartStop(t *testing.T) {
	// Create mock dependencies
	s3Client := &s3.Client{}
	var stateManager state.StateManager = &state.Manager{}
	httpSender := &output.HTTPSender{}
	bucket := "test-bucket"
	workerCount := 2
	queueSize := 10
	metricsClient := &metrics.Metrics{}

	pool := NewHTTPPool(s3Client, httpSender, stateManager, bucket, workerCount, queueSize, metricsClient, nil)

	// Start the pool
	pool.Start()

	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	// Check that it's not stopped
	if pool.stopped.Load() {
		t.Error("Pool should not be stopped after starting")
	}

	// Stop the pool
	pool.Stop()

	// Give it a moment to stop
	time.Sleep(100 * time.Millisecond)

	// Check that it's stopped
	if !pool.stopped.Load() {
		t.Error("Pool should be stopped after stopping")
	}
}

func TestHTTPPool_GetMetrics(t *testing.T) {
	// Create mock dependencies
	s3Client := &s3.Client{}
	var stateManager state.StateManager = &state.Manager{}
	httpSender := &output.HTTPSender{}
	bucket := "test-bucket"
	workerCount := 2
	queueSize := 10
	metricsClient := &metrics.Metrics{}

	pool := NewHTTPPool(s3Client, httpSender, stateManager, bucket, workerCount, queueSize, metricsClient, nil)

	files, bytes, errors := pool.GetMetrics()

	// Initially all should be 0
	if files != 0 {
		t.Errorf("Expected initial files 0, got %d", files)
	}
	if bytes != 0 {
		t.Errorf("Expected initial bytes 0, got %d", bytes)
	}
	if errors != 0 {
		t.Errorf("Expected initial errors 0, got %d", errors)
	}
}

func TestHTTPPool_EnqueueJob(t *testing.T) {
	// Create mock dependencies
	s3Client := &s3.Client{}
	var stateManager state.StateManager = &state.Manager{}
	httpSender := &output.HTTPSender{}
	bucket := "test-bucket"
	workerCount := 2
	queueSize := 10
	metricsClient := &metrics.Metrics{}

	pool := NewHTTPPool(s3Client, httpSender, stateManager, bucket, workerCount, queueSize, metricsClient, nil)

	job := scanner.FileJob{
		S3Key:     "test-key",
		Size:      1024,
		Timestamp: time.Now().Unix(),
	}

	// Submit a job without starting the pool (to avoid processing)
	submitted := pool.Submit(job)

	if !submitted {
		t.Error("Job should have been submitted successfully")
	}

	// Check that job was queued
	select {
	case queuedJob := <-pool.jobQueue:
		if queuedJob.S3Key != job.S3Key {
			t.Errorf("Expected job key %s, got %s", job.S3Key, queuedJob.S3Key)
		}
		if queuedJob.Size != job.Size {
			t.Errorf("Expected job size %d, got %d", job.Size, queuedJob.Size)
		}
	default:
		t.Error("Job should have been queued")
	}
}
