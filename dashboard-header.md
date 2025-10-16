<div style="display: flex; align-items: center; gap: 20px; margin-bottom: 20px;">
  <img src="https://upload.wikimedia.org/wikipedia/commons/b/bc/Amazon-S3-Logo.svg" alt="S3" style="height: 48px;">
  <svg width="30" height="30" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
    <polyline points="16 18 22 12 16 6"></polyline>
    <polyline points="8 6 2 12 8 18"></polyline>
  </svg>
  <img src="https://edgedelta.com/brand-assets/edge-delta-logos/gradient/white-transparent.svg" alt="Edge Delta" style="height: 48px;">
</div>

## S3 Streaming Pipeline Monitor

This dashboard tracks the S3-to-EdgeDelta streaming pipeline, which processes Zscaler web logs from AWS S3 and delivers them to EdgeDelta via dual HTTP endpoints.

### Architecture
- **15 S3 workers** download and decompress gzipped log files
- **10 HTTP workers** batch and send logs across 2 endpoints (ports 8080, 8081)
- **Target processing lag:** < 60 seconds from file timestamp to ingestion

### Key Metrics
- **Throughput:** Files/bytes processed, batches sent, processing latency
- **Reliability:** HTTP errors, buffer drops, file processing errors
- **Performance:** Processing lag, batch efficiency, worker utilization

Data flows continuously with 15-second scan intervals. Buffer drops indicate transient overload conditions during startup or burst traffic.
