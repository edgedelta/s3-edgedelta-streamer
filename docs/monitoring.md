# Monitoring & Metrics

The streamer emits rich telemetry through OpenTelemetry and the EdgeDelta agent. Use this guide to wire dashboards and alerts.

## Metrics Overview

| Category | Metric | Description |
| --- | --- | --- |
| S3 Workers | `s3_files_processed_total` | Count of files processed successfully |
|  | `s3_bytes_processed_total` | Bytes downloaded and streamed |
|  | `s3_files_errored_total` | Failures while reading from S3 |
|  | `s3_processing_latency_seconds` | Time spent per file |
| HTTP Sender | `http_batches_sent_total` | Batches delivered to EdgeDelta |
|  | `http_lines_sent_total` | Total log lines pushed |
|  | `http_bytes_sent_total` | Payload volume |
|  | `http_errors_total` | Non-successful HTTP responses |
|  | `http_buffer_drops_total` | Lines discarded due to buffer pressure |
| Processing | `processing_lag_seconds` | Difference between file timestamps and now |

> **Warning:** `http_buffer_drops_total` should remain at zero outside of backlog catch-up windows. Trigger alerts if it trends upward.

## OTLP Configuration

```yaml
otlp:
  enabled: true
  endpoint: "localhost:4317"
  export_interval: 10s
  service_name: "s3-edgedelta-streamer"
  service_version: "1.0.0"
  insecure: true
```

The built-in exporter supports any OTLP collector. For EdgeDelta, ensure ports `4317` and `8080-8081` remain reachable from the streamer host.

## Dashboards

- **EdgeDelta Dashboard Template**: See `dashboard-header.md` for layout, widgets, and copy.
- **Key Widgets**:
  - S3 throughput (bytes/min) vs HTTP throughput.
  - Processing lag with alert thresholds at 60s.
  - Error counters plotted as **rate of change** rather than absolute totals.

Include the performance hero chart (`assets/dashboard.jpg`) to showcase historic stability.

## Alerting Suggestions

| Symptom | Metric Trigger | Suggested Action |
| --- | --- | --- |
| Sustained lag | `processing_lag_seconds > 60` for 5 min | Scale HTTP endpoints or add workers |
| Buffer drops | `http_buffer_drops_total` increases during steady state | Increase buffer size or reduce S3 workers |
| HTTP failures | `http_errors_total` rate > 0.05 | Inspect EdgeDelta agent health |
| S3 failures | `s3_files_errored_total` rate > 0.02 | Validate IAM permissions and bucket region |

## Logging

Application logs live at `/opt/edgedelta/s3-streamer/logs/streamer.log`. EdgeDelta agent logs: `/var/log/edgedelta/edgedelta.log`.

> **Tip:** Forward both log streams to central observability tooling to correlate S3 errors with downstream processing glitches.
