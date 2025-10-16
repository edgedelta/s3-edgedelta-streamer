# Performance & Scaling

This document summarizes real-world throughput, tuning levers, and data characteristics for the streamer.

## Production Snapshot

![Performance Dashboard](../assets/dashboard.jpg)

**Observed Metrics**

- 315k+ files processed, 2.88 TB streamed
- ~60 ms average processing latency
- ~493 ms end-to-end lag (well under the 60 s SLO)
- 228 MB/min sustained throughput (≈13.7 GB/hour)
- Zero steady-state data loss across 24/7 operation

## Scaling Guidelines

> **Tip:** Horizontal scaling with multiple streamer instances **requires Redis state storage**.

### Scaling Up

1. Add additional HTTP endpoints (8082, 8083, …).
2. Increase `http.workers` proportionally.
3. Monitor CPU, memory, and network limits on EdgeDelta.
4. Introduce more streamer instances once Redis is in place.

### Scaling Down

1. Reduce `processing.worker_count`.
2. Increase `processing.scan_interval` for less frequent S3 polling.
3. Increase `processing.delay_window` to process older files, smoothing bursts.

## Performance Tuning Checklist

| Symptom | Tuning Lever | Notes |
| --- | --- | --- |
| High buffer drops | Increase `http.buffer_size` or lower S3 workers | Drops during backlog replay are acceptable |
| Sustained lag | Add HTTP endpoints, tune workers | Ensure load balancer distributes evenly |
| Redis spikes | Adjust `state.save_interval` | Longer intervals lower write pressure |
| S3 throttling | Backoff `scan_interval`, enable S3 request metrics | Consider AWS support for high-volume buckets |

## Data Format Reference

- **Files**: gzip-compressed JSONL
- **Typical size**: ~650 KB compressed (~10 MB uncompressed)
- **Lines per file**: ≈6,500
- **Partitioning**: Hive-style `year=YYYY/month=M/day=D/`
- **Naming**: `<unix_timestamp>_<id>_<id>_<seq>[.gz]`

Example filename: `1760305292_56442_130_1.gz → 2025-10-12 21:41:32 UTC`

## EdgeDelta Pipeline Snapshot

![Pipeline Status](../assets/pipeline-status.jpg)

- 513 GB processed (24.86% growth)
- 508 MB recent activity (+204%)
- 3 active agents delivering real-time data
