package worker

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/edgedelta/s3-edgedelta-streamer/internal/scanner"
	"github.com/edgedelta/s3-edgedelta-streamer/internal/state"
	"github.com/edgedelta/s3-edgedelta-streamer/internal/tcppool"
)

func TestNewPool(t *testing.T) {
	// Create mock dependencies
	s3Client := &s3.Client{}
	tcpPool := &tcppool.Pool{}
	stateManager := &state.Manager{}
	bucket := "s3://test-bucket"
	workerCount := 4
	queueSize := 25

	pool := NewPool(s3Client, tcpPool, stateManager, bucket, workerCount, queueSize)

	if pool == nil {
		t.Fatal("NewPool returned nil")
	}

	// Bucket should have s3:// prefix stripped
	expectedBucket := "test-bucket"
	if pool.bucket != expectedBucket {
		t.Errorf("Expected bucket %s, got %s", expectedBucket, pool.bucket)
	}

	if pool.workerCount != workerCount {
		t.Errorf("Expected workerCount %d, got %d", workerCount, pool.workerCount)
	}

	if cap(pool.jobQueue) != queueSize {
		t.Errorf("Expected queue size %d, got %d", queueSize, cap(pool.jobQueue))
	}

	if pool.s3Client != s3Client {
		t.Error("s3Client not set correctly")
	}

	if pool.tcpPool != tcpPool {
		t.Error("tcpPool not set correctly")
	}

	if pool.stateManager != stateManager {
		t.Error("stateManager not set correctly")
	}
}

func TestPool_StartStop(t *testing.T) {
	// Create mock dependencies
	s3Client := &s3.Client{}
	tcpPool := &tcppool.Pool{}
	stateManager := &state.Manager{}
	bucket := "test-bucket"
	workerCount := 2
	queueSize := 10

	pool := NewPool(s3Client, tcpPool, stateManager, bucket, workerCount, queueSize)

	// Start the pool
	pool.Start()

	// Give it a moment to start
	time.Sleep(50 * time.Millisecond)

	// Stop the pool
	pool.Stop()

	// Wait for all workers to finish
	pool.wg.Wait()
}

func TestPool_Submit(t *testing.T) {
	// Create mock dependencies
	s3Client := &s3.Client{}
	tcpPool := &tcppool.Pool{}
	stateManager := &state.Manager{}
	bucket := "test-bucket"
	workerCount := 2
	queueSize := 10

	pool := NewPool(s3Client, tcpPool, stateManager, bucket, workerCount, queueSize)

	job := scanner.FileJob{
		S3Key:     "test-key",
		Size:      1024,
		Timestamp: time.Now().Unix(),
	}

	// Submit a job without starting the pool
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

func TestPool_GetMetricsCounters(t *testing.T) {
	// Create mock dependencies
	s3Client := &s3.Client{}
	tcpPool := &tcppool.Pool{}
	stateManager := &state.Manager{}
	bucket := "test-bucket"
	workerCount := 2
	queueSize := 10

	pool := NewPool(s3Client, tcpPool, stateManager, bucket, workerCount, queueSize)

	filesProcessed, bytesProcessed, errors := pool.GetMetricsCounters()

	if filesProcessed == nil {
		t.Error("filesProcessed counter should not be nil")
	}

	if bytesProcessed == nil {
		t.Error("bytesProcessed counter should not be nil")
	}

	if errors == nil {
		t.Error("errors counter should not be nil")
	}

	// Test that counters start at 0
	if filesProcessed.Load() != 0 {
		t.Errorf("Expected initial filesProcessed 0, got %d", filesProcessed.Load())
	}

	if bytesProcessed.Load() != 0 {
		t.Errorf("Expected initial bytesProcessed 0, got %d", bytesProcessed.Load())
	}

	if errors.Load() != 0 {
		t.Errorf("Expected initial errors 0, got %d", errors.Load())
	}
}
