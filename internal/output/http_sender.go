package output

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/edgedelta/s3-edgedelta-streamer/internal/logging"
	"github.com/edgedelta/s3-edgedelta-streamer/internal/metrics"
)

// HTTPSender batches log lines and sends them via HTTP to EdgeDelta
type HTTPSender struct {
	endpoints     []string
	client        *http.Client
	batchLines    int
	batchBytes    int
	flushInterval time.Duration
	workers       int
	bufferSize    int

	lineChan  chan []byte
	batchChan chan *Batch
	doneChan  chan struct{}
	wg        sync.WaitGroup

	ctx    context.Context
	cancel context.CancelFunc

	// Metrics (local counters)
	sentLines   atomic.Int64
	sentBytes   atomic.Int64
	sentBatches atomic.Int64
	errors      atomic.Int64

	// OTLP metrics client
	metricsClient *metrics.Metrics
}

// Batch represents a batch of log lines ready to send
type Batch struct {
	Lines [][]byte
	Size  int
}

// NewHTTPSender creates a new HTTP sender
func NewHTTPSender(endpoints []string, batchLines, batchBytes int, flushInterval time.Duration, workers int, bufferSize int, timeout time.Duration, maxIdleConns int, idleConnTimeout time.Duration, tlsHandshakeTimeout, responseHeaderTimeout, expectContinueTimeout time.Duration, metricsClient *metrics.Metrics) *HTTPSender {
	transport := &http.Transport{
		MaxIdleConns:          maxIdleConns,
		MaxIdleConnsPerHost:   maxIdleConns,
		IdleConnTimeout:       idleConnTimeout,
		TLSHandshakeTimeout:   tlsHandshakeTimeout,
		ResponseHeaderTimeout: responseHeaderTimeout,
		ExpectContinueTimeout: expectContinueTimeout,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}

	// Create cancellable context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())

	return &HTTPSender{
		endpoints:     endpoints,
		client:        client,
		batchLines:    batchLines,
		batchBytes:    batchBytes,
		flushInterval: flushInterval,
		workers:       workers,
		bufferSize:    bufferSize,
		lineChan:      make(chan []byte, bufferSize), // Configurable buffer for incoming lines
		batchChan:     make(chan *Batch, workers*2),
		doneChan:      make(chan struct{}),
		metricsClient: metricsClient,
		ctx:           ctx,
		cancel:        cancel,
	}
}

// Start starts the HTTP sender (batcher + workers)
func (hs *HTTPSender) Start() {
	// Start batcher
	hs.wg.Add(1)
	go hs.batcher()

	// Start HTTP sender workers
	for i := 0; i < hs.workers; i++ {
		hs.wg.Add(1)
		go hs.sender(i)
	}
}

// Stop gracefully stops the HTTP sender
func (hs *HTTPSender) Stop() {
	// Cancel context to signal shutdown
	hs.cancel()

	// Close channels
	close(hs.lineChan)
	hs.wg.Wait()
	close(hs.batchChan)
	close(hs.doneChan)
}

// SendLine queues a log line for sending, blocking if buffer is full
func (hs *HTTPSender) SendLine(line []byte) {
	hs.lineChan <- line
}

// batcher accumulates lines into batches and flushes periodically
func (hs *HTTPSender) batcher() {
	defer hs.wg.Done()

	currentBatch := &Batch{
		Lines: make([][]byte, 0, hs.batchLines),
		Size:  0,
	}

	flushTicker := time.NewTicker(hs.flushInterval)
	defer flushTicker.Stop()

	// Buffer utilization monitoring (every 5 seconds)
	bufferMonitorTicker := time.NewTicker(5 * time.Second)
	defer bufferMonitorTicker.Stop()

	flushBatch := func() {
		if len(currentBatch.Lines) > 0 {
			// Send batch to senders
			select {
			case hs.batchChan <- currentBatch:
				// Create new batch
				currentBatch = &Batch{
					Lines: make([][]byte, 0, hs.batchLines),
					Size:  0,
				}
			case <-hs.doneChan:
				return
			}
		}
	}

	for {
		select {
		case line, ok := <-hs.lineChan:
			if !ok {
				// Channel closed, flush and exit
				flushBatch()
				return
			}

			// Add line to batch
			currentBatch.Lines = append(currentBatch.Lines, line)
			currentBatch.Size += len(line) + 1 // +1 for newline

			// Flush if batch is full
			if len(currentBatch.Lines) >= hs.batchLines || currentBatch.Size >= hs.batchBytes {
				flushBatch()
			}

		case <-flushTicker.C:
			// Periodic flush (even if batch not full)
			flushBatch()

		case <-bufferMonitorTicker.C:
			// Update buffer utilization metric
			if hs.metricsClient != nil {
				utilization := float64(len(hs.lineChan)) / float64(hs.bufferSize)
				hs.metricsClient.UpdateBufferUtilization(context.Background(), utilization)
			}

		case <-hs.doneChan:
			flushBatch()
			return
		}
	}
}

// sender reads batches and sends them via HTTP POST
func (hs *HTTPSender) sender(workerID int) {
	defer hs.wg.Done()

	// Select endpoint for this worker (round-robin distribution)
	endpoint := hs.endpoints[workerID%len(hs.endpoints)]

	for batch := range hs.batchChan {
		if err := hs.sendBatch(batch, endpoint); err != nil {
			logging.GetDefaultLogger().Error("HTTP worker failed to send batch",
				"worker_id", workerID,
				"endpoint", endpoint,
				"batch_lines", len(batch.Lines),
				"error", err)
			hs.errors.Add(1)
			if hs.metricsClient != nil {
				// Categorize error type
				errStr := err.Error()
				if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline exceeded") {
					hs.metricsClient.RecordHTTPTimeoutError(context.Background())
				} else if strings.Contains(errStr, "connection refused") || strings.Contains(errStr, "network") || strings.Contains(errStr, "dial") {
					hs.metricsClient.RecordHTTPNetworkError(context.Background())
				} else if strings.Contains(errStr, "HTTP 5") {
					hs.metricsClient.RecordHTTPServerError(context.Background())
				} else {
					hs.metricsClient.RecordHTTPError(context.Background())
				}
			}
		} else {
			hs.sentBatches.Add(1)
			hs.sentLines.Add(int64(len(batch.Lines)))
			hs.sentBytes.Add(int64(batch.Size))
			if hs.metricsClient != nil {
				hs.metricsClient.RecordHTTPBatch(context.Background(), int64(len(batch.Lines)), int64(batch.Size))
			}
		}
	}
}

// sendBatch sends a batch via HTTP POST
func (hs *HTTPSender) sendBatch(batch *Batch, endpoint string) error {
	// Build request body (newline-delimited JSON)
	var buf bytes.Buffer
	for _, line := range batch.Lines {
		buf.Write(line)
		buf.WriteByte('\n')
	}

	// Create request with context for cancellation
	req, err := http.NewRequestWithContext(hs.ctx, "POST", endpoint, &buf)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-ndjson")

	// Send request with timing
	start := time.Now()
	resp, err := hs.client.Do(req)
	duration := time.Since(start).Seconds()

	// Record latency metric
	if hs.metricsClient != nil {
		hs.metricsClient.RecordHTTPRequestLatency(context.Background(), duration)
	}

	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	// Drain response body
	_, _ = io.Copy(io.Discard, resp.Body)

	return nil
}

// GetMetrics returns current metrics
func (hs *HTTPSender) GetMetrics() (lines, bytes, batches, errors int64) {
	return hs.sentLines.Load(), hs.sentBytes.Load(), hs.sentBatches.Load(), hs.errors.Load()
}
