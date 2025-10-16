package output

import (
	"testing"
	"time"
)

func TestNewHTTPSender(t *testing.T) {
	endpoints := []string{"http://localhost:8080"}
	batchLines := 1000
	batchBytes := 1024 * 1024
	flushInterval := time.Second
	workers := 5
	bufferSize := 10000
	timeout := 30 * time.Second
	maxIdleConns := 100
	idleConnTimeout := 90 * time.Second
	tlsHandshakeTimeout := 10 * time.Second
	responseHeaderTimeout := 10 * time.Second
	expectContinueTimeout := time.Second

	sender := NewHTTPSender(
		endpoints,
		batchLines,
		batchBytes,
		flushInterval,
		workers,
		bufferSize,
		timeout,
		maxIdleConns,
		idleConnTimeout,
		tlsHandshakeTimeout,
		responseHeaderTimeout,
		expectContinueTimeout,
		nil, // metrics client
	)

	if sender == nil {
		t.Fatal("NewHTTPSender returned nil")
	}

	if len(sender.endpoints) != 1 {
		t.Errorf("Expected 1 endpoint, got %d", len(sender.endpoints))
	}

	if sender.batchLines != batchLines {
		t.Errorf("Expected batchLines %d, got %d", batchLines, sender.batchLines)
	}

	if sender.bufferSize != bufferSize {
		t.Errorf("Expected bufferSize %d, got %d", bufferSize, sender.bufferSize)
	}

	if sender.client == nil {
		t.Error("HTTP client should not be nil")
	}

	if sender.client.Timeout != timeout {
		t.Errorf("Expected client timeout %v, got %v", timeout, sender.client.Timeout)
	}
}

func TestHTTPSender_SendLine(t *testing.T) {
	sender := NewHTTPSender(
		[]string{"http://localhost:8080"},
		1000, 1024*1024, time.Second, 1, 1000,
		30*time.Second, 100, 90*time.Second,
		10*time.Second, 10*time.Second, time.Second,
		nil,
	)

	// Test that SendLine can queue lines without blocking (buffer has space)
	testLine := []byte("test log line")

	// Send line in a goroutine in case it would block
	done := make(chan bool, 1)
	go func() {
		sender.SendLine(testLine)
		done <- true
	}()

	// Wait a short time to see if it completes (should complete since buffer has space)
	select {
	case <-done:
		// Good, it didn't block
	case <-time.After(100 * time.Millisecond):
		t.Error("SendLine blocked unexpectedly with available buffer space")
	}

	// Check that the line was queued
	select {
	case line := <-sender.lineChan:
		if string(line) != string(testLine) {
			t.Errorf("Expected line %q, got %q", testLine, line)
		}
	default:
		t.Error("Line was not queued")
	}
}

func TestHTTPSender_BufferFull(t *testing.T) {
	// Create sender with very small buffer
	sender := NewHTTPSender(
		[]string{"http://localhost:8080"},
		1000, 1024*1024, time.Second, 1, 1, // bufferSize = 1
		30*time.Second, 100, 90*time.Second,
		10*time.Second, 10*time.Second, time.Second,
		nil,
	)

	// Fill the buffer
	sender.SendLine([]byte("line 1"))

	// This send should block since buffer is full
	done := make(chan bool, 1)
	go func() {
		sender.SendLine([]byte("line 2"))
		done <- true
	}()

	// It should not complete immediately since buffer is full
	select {
	case <-done:
		t.Error("SendLine should have blocked with full buffer")
	case <-time.After(100 * time.Millisecond):
		// Good, it blocked as expected
	}

	// Verify buffer size
	if sender.bufferSize != 1 {
		t.Errorf("Expected bufferSize 1, got %d", sender.bufferSize)
	}
}

func TestHTTPSender_GetMetrics(t *testing.T) {
	sender := NewHTTPSender(
		[]string{"http://localhost:8080"},
		1000, 1024*1024, time.Second, 1, 1000,
		30*time.Second, 100, 90*time.Second,
		10*time.Second, 10*time.Second, time.Second,
		nil,
	)

	lines, bytes, batches, errors := sender.GetMetrics()

	// Initially all should be 0
	if lines != 0 {
		t.Errorf("Expected initial lines 0, got %d", lines)
	}
	if bytes != 0 {
		t.Errorf("Expected initial bytes 0, got %d", bytes)
	}
	if batches != 0 {
		t.Errorf("Expected initial batches 0, got %d", batches)
	}
	if errors != 0 {
		t.Errorf("Expected initial errors 0, got %d", errors)
	}
}

func TestHTTPSender_MultipleEndpoints(t *testing.T) {
	endpoints := []string{
		"http://localhost:8080",
		"http://localhost:8081",
		"http://localhost:8082",
	}

	sender := NewHTTPSender(
		endpoints,
		1000, 1024*1024, time.Second, 1, 1000,
		30*time.Second, 100, 90*time.Second,
		10*time.Second, 10*time.Second, time.Second,
		nil,
	)

	if len(sender.endpoints) != 3 {
		t.Errorf("Expected 3 endpoints, got %d", len(sender.endpoints))
	}

	for i, expected := range endpoints {
		if sender.endpoints[i] != expected {
			t.Errorf("Expected endpoint[%d] %s, got %s", i, expected, sender.endpoints[i])
		}
	}
}

func TestBatch_NewBatch(t *testing.T) {
	batch := &Batch{
		Lines: [][]byte{
			[]byte("line 1"),
			[]byte("line 2"),
			[]byte("line 3"),
		},
		Size: 17, // "line 1line 2line 3" = 17 bytes
	}

	if len(batch.Lines) != 3 {
		t.Errorf("Expected 3 lines, got %d", len(batch.Lines))
	}

	if batch.Size != 17 {
		t.Errorf("Expected size 17, got %d", batch.Size)
	}
}
