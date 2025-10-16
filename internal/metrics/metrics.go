package metrics

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"google.golang.org/grpc/credentials/insecure"
)

// Metrics holds all application metrics
type Metrics struct {
	// S3 Worker metrics
	FilesProcessed    metric.Int64Counter
	BytesProcessed    metric.Int64Counter
	FilesErrored      metric.Int64Counter
	ProcessingLatency metric.Float64Histogram

	// HTTP Sender metrics
	HTTPBatchesSent       metric.Int64Counter
	HTTPLinesSent         metric.Int64Counter
	HTTPBytesSent         metric.Int64Counter
	HTTPErrors            metric.Int64Counter
	HTTPNetworkErrors     metric.Int64Counter
	HTTPTimeoutErrors     metric.Int64Counter
	HTTPServerErrors      metric.Int64Counter
	HTTPBufferDrops       metric.Int64Counter
	HTTPBufferUtilization metric.Float64Gauge
	HTTPActiveConnections metric.Int64Gauge
	HTTPIdleConnections   metric.Int64Gauge
	HTTPRequestLatency    metric.Float64Histogram

	// Processing lag metrics
	ProcessingLag metric.Float64Gauge

	meterProvider *sdkmetric.MeterProvider
}

// InitMetrics initializes OpenTelemetry metrics with OTLP exporter
func InitMetrics(ctx context.Context, endpoint string, serviceName string, serviceVersion string, exportInterval time.Duration, useInsecure bool) (*Metrics, error) {
	// Create resource with service information
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(serviceVersion),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create OTLP gRPC exporter
	var opts []otlpmetricgrpc.Option
	opts = append(opts, otlpmetricgrpc.WithEndpoint(endpoint))

	if useInsecure {
		opts = append(opts, otlpmetricgrpc.WithTLSCredentials(insecure.NewCredentials()))
	}

	exporter, err := otlpmetricgrpc.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	// Create meter provider with periodic reader
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(
			sdkmetric.NewPeriodicReader(exporter,
				sdkmetric.WithInterval(exportInterval),
			),
		),
	)

	// Set global meter provider
	otel.SetMeterProvider(meterProvider)

	// Get meter
	meter := meterProvider.Meter("s3-edgedelta-streamer")

	// Create metrics
	m := &Metrics{
		meterProvider: meterProvider,
	}

	// S3 Worker metrics
	m.FilesProcessed, err = meter.Int64Counter(
		"s3_files_processed_total",
		metric.WithDescription("Total number of S3 files processed"),
		metric.WithUnit("{file}"),
	)
	if err != nil {
		return nil, err
	}

	m.BytesProcessed, err = meter.Int64Counter(
		"s3_bytes_processed_total",
		metric.WithDescription("Total bytes processed from S3"),
		metric.WithUnit("By"),
	)
	if err != nil {
		return nil, err
	}

	m.FilesErrored, err = meter.Int64Counter(
		"s3_files_errored_total",
		metric.WithDescription("Total number of S3 file processing errors"),
		metric.WithUnit("{error}"),
	)
	if err != nil {
		return nil, err
	}

	m.ProcessingLatency, err = meter.Float64Histogram(
		"s3_processing_latency_seconds",
		metric.WithDescription("Time to process each S3 file"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}

	// HTTP Sender metrics
	m.HTTPBatchesSent, err = meter.Int64Counter(
		"http_batches_sent_total",
		metric.WithDescription("Total number of HTTP batches sent to EdgeDelta"),
		metric.WithUnit("{batch}"),
	)
	if err != nil {
		return nil, err
	}

	m.HTTPLinesSent, err = meter.Int64Counter(
		"http_lines_sent_total",
		metric.WithDescription("Total number of log lines sent via HTTP"),
		metric.WithUnit("{line}"),
	)
	if err != nil {
		return nil, err
	}

	m.HTTPBytesSent, err = meter.Int64Counter(
		"http_bytes_sent_total",
		metric.WithDescription("Total bytes sent via HTTP"),
		metric.WithUnit("By"),
	)
	if err != nil {
		return nil, err
	}

	m.HTTPErrors, err = meter.Int64Counter(
		"http_errors_total",
		metric.WithDescription("Total HTTP send errors"),
		metric.WithUnit("{error}"),
	)
	if err != nil {
		return nil, err
	}

	m.HTTPNetworkErrors, err = meter.Int64Counter(
		"http_network_errors_total",
		metric.WithDescription("Total HTTP network errors"),
		metric.WithUnit("{error}"),
	)
	if err != nil {
		return nil, err
	}

	m.HTTPTimeoutErrors, err = meter.Int64Counter(
		"http_timeout_errors_total",
		metric.WithDescription("Total HTTP timeout errors"),
		metric.WithUnit("{error}"),
	)
	if err != nil {
		return nil, err
	}

	m.HTTPServerErrors, err = meter.Int64Counter(
		"http_server_errors_total",
		metric.WithDescription("Total HTTP server errors (5xx)"),
		metric.WithUnit("{error}"),
	)
	if err != nil {
		return nil, err
	}

	m.HTTPBufferDrops, err = meter.Int64Counter(
		"http_buffer_drops_total",
		metric.WithDescription("Total lines dropped due to buffer overflow"),
		metric.WithUnit("{line}"),
	)
	if err != nil {
		return nil, err
	}

	m.HTTPBufferUtilization, err = meter.Float64Gauge(
		"http_buffer_utilization_ratio",
		metric.WithDescription("Current buffer utilization (0.0 to 1.0)"),
		metric.WithUnit("{ratio}"),
	)
	if err != nil {
		return nil, err
	}

	m.HTTPActiveConnections, err = meter.Int64Gauge(
		"http_active_connections",
		metric.WithDescription("Number of active HTTP connections"),
		metric.WithUnit("{connection}"),
	)
	if err != nil {
		return nil, err
	}

	m.HTTPIdleConnections, err = meter.Int64Gauge(
		"http_idle_connections",
		metric.WithDescription("Number of idle HTTP connections"),
		metric.WithUnit("{connection}"),
	)
	if err != nil {
		return nil, err
	}

	m.HTTPRequestLatency, err = meter.Float64Histogram(
		"http_request_duration_seconds",
		metric.WithDescription("HTTP request latency in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}

	// Processing lag gauge
	m.ProcessingLag, err = meter.Float64Gauge(
		"processing_lag_seconds",
		metric.WithDescription("Time lag between file timestamp and current time"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}

	return m, nil
}

// Shutdown gracefully shuts down the meter provider
func (m *Metrics) Shutdown(ctx context.Context) error {
	if m.meterProvider != nil {
		return m.meterProvider.Shutdown(ctx)
	}
	return nil
}

// RecordFileProcessed records a successfully processed file
func (m *Metrics) RecordFileProcessed(ctx context.Context, bytes int64, latency time.Duration) {
	m.FilesProcessed.Add(ctx, 1)
	m.BytesProcessed.Add(ctx, bytes)
	m.ProcessingLatency.Record(ctx, latency.Seconds())
}

// RecordFileError records a file processing error
func (m *Metrics) RecordFileError(ctx context.Context) {
	m.FilesErrored.Add(ctx, 1)
}

// RecordHTTPBatch records an HTTP batch sent
func (m *Metrics) RecordHTTPBatch(ctx context.Context, lines, bytes int64) {
	m.HTTPBatchesSent.Add(ctx, 1)
	m.HTTPLinesSent.Add(ctx, lines)
	m.HTTPBytesSent.Add(ctx, bytes)
}

// RecordHTTPError records an HTTP error
func (m *Metrics) RecordHTTPError(ctx context.Context) {
	m.HTTPErrors.Add(ctx, 1)
}

// RecordHTTPNetworkError records an HTTP network error
func (m *Metrics) RecordHTTPNetworkError(ctx context.Context) {
	m.HTTPErrors.Add(ctx, 1)
	m.HTTPNetworkErrors.Add(ctx, 1)
}

// RecordHTTPTimeoutError records an HTTP timeout error
func (m *Metrics) RecordHTTPTimeoutError(ctx context.Context) {
	m.HTTPErrors.Add(ctx, 1)
	m.HTTPTimeoutErrors.Add(ctx, 1)
}

// RecordHTTPServerError records an HTTP server error (5xx)
func (m *Metrics) RecordHTTPServerError(ctx context.Context) {
	m.HTTPErrors.Add(ctx, 1)
	m.HTTPServerErrors.Add(ctx, 1)
}

// RecordBufferDrop records lines dropped due to buffer overflow
func (m *Metrics) RecordBufferDrop(ctx context.Context, lines int64) {
	m.HTTPBufferDrops.Add(ctx, lines)
}

// UpdateBufferUtilization updates the buffer utilization gauge
func (m *Metrics) UpdateBufferUtilization(ctx context.Context, utilization float64) {
	m.HTTPBufferUtilization.Record(ctx, utilization, metric.WithAttributes(
		attribute.String("component", "http_sender"),
	))
}

// UpdateHTTPConnections updates the HTTP connection pool gauges
func (m *Metrics) UpdateHTTPConnections(ctx context.Context, active, idle int64) {
	m.HTTPActiveConnections.Record(ctx, active, metric.WithAttributes(
		attribute.String("component", "http_sender"),
	))
	m.HTTPIdleConnections.Record(ctx, idle, metric.WithAttributes(
		attribute.String("component", "http_sender"),
	))
}

// RecordHTTPRequestLatency records HTTP request latency
func (m *Metrics) RecordHTTPRequestLatency(ctx context.Context, durationSeconds float64) {
	m.HTTPRequestLatency.Record(ctx, durationSeconds, metric.WithAttributes(
		attribute.String("component", "http_sender"),
	))
}

// UpdateProcessingLag updates the processing lag gauge
func (m *Metrics) UpdateProcessingLag(ctx context.Context, lagSeconds float64) {
	m.ProcessingLag.Record(ctx, lagSeconds, metric.WithAttributes(
		attribute.String("component", "scanner"),
	))
}
