## S3 to EdgeDelta Streamer

<p align="center">
  <img src="https://edgedelta.com/brand-assets/edge-delta-logos/gradient/black-transparent.svg" alt="Edge Delta logo" width="320" />
</p>

High-performance pipeline that streams S3 log files to EdgeDelta via HTTP in near real time.

**Highlights**
- Handles hundreds of thousands of gzipped files per day with sub-second lag
- Pluggable log-format registry (Zscaler, Cisco Umbrella, AWS services, and custom patterns)
- Optional Redis-backed state for safe horizontal scaling
- First-class observability: OTLP metrics, health endpoints, and dashboards
- Automated installer, systemd integration, and container images

## Table of Contents
- [S3 to EdgeDelta Streamer](#s3-to-edgedelta-streamer)
- [Table of Contents](#table-of-contents)
- [Quick Start](#quick-start)
- [Deployment Options](#deployment-options)
- [Configuration Basics](#configuration-basics)
- [Operations \& Monitoring](#operations--monitoring)
- [Troubleshooting](#troubleshooting)
- [Architecture \& Scaling](#architecture--scaling)
- [Documentation Map](#documentation-map)
- [Support](#support)

## Quick Start

> **Prerequisites:** EdgeDelta agent running, AWS credentials with `s3:GetObject` and `s3:ListBucket`, sudo access, and (optionally) Redis for distributed deployments.

```bash
git clone https://github.com/daniel-edgedelta/s3-edgedelta-streamer.git
cd s3-edgedelta-streamer
sudo ./install.sh
```

The installer validates prerequisites, prompts for configuration, encrypts credentials, and registers a `systemd` service that tracks the EdgeDelta agent lifecycle.

Verify the deployment:

```bash
sudo systemctl status s3-streamer
sudo journalctl -fu s3-streamer
```

> **Tip:** Deploy your EdgeDelta pipeline before installing so ports `8080`, `8081`, and `4317` are available.

## Deployment Options

<details>
  <summary><strong>Systemd (recommended)</strong></summary>

The installation script places binaries under `/opt/edgedelta/s3-streamer/`, stores encrypted credentials in `/etc/systemd/creds/s3-streamer/`, and persists state in `/var/lib/s3-streamer/state.json`. Start/stop the service with `systemctl` or re-run `install.sh` to rotate credentials.

</details>

<details>
  <summary><strong>Docker</strong></summary>

```bash
docker build -t s3-edgedelta-streamer .
docker run -p 8080:8080 \
  -e AWS_ACCESS_KEY_ID=your_key \
  -e AWS_SECRET_ACCESS_KEY=your_secret \
  -e AWS_REGION=us-east-1 \
  s3-edgedelta-streamer
```

</details>

<details>
  <summary><strong>Docker Compose</strong></summary>

```bash
docker-compose up -d
docker-compose logs -f s3-edgedelta-streamer
docker-compose down
```

</details>

## Configuration Basics

| Category | Minimal Settings | Notes |
| --- | --- | --- |
| **S3** | `bucket`, `prefix`, `region` | Remove any `s3://` prefix from the bucket name. |
| **HTTP sender** | `endpoints`, `batch_lines`, `batch_bytes`, `flush_interval`, `workers` | Maintain a ~1.5:1 ratio between S3 and HTTP workers. |
| **Processing** | `worker_count`, `queue_size`, `scan_interval`, `delay_window` | Increase `delay_window` to ensure files are complete before processing. |
| **State** | `file_path`, `save_interval` | Default persistence uses the local filesystem. |
| **Redis (optional)** | `host`, `port`, `password`, `database`, `key_prefix` | Required when multiple streamer instances share state. |
| **OTLP metrics** | `enabled`, `endpoint`, `service_name` | Streams telemetry to the EdgeDelta collector (4317/tcp). |

> **Tip:** Keep `default_format: "auto"` to enable automatic log-format detection. Custom recipes live in [`docs/log-formats.md`](docs/log-formats.md).

Minimal snippet for the state block:

```yaml
state:
  file_path: "/var/lib/s3-streamer/state.json"
  save_interval: 30s
  redis:
    enabled: false
```

## Operations & Monitoring

- Day-to-day commands, health endpoints, and migration flows: [`docs/operations.md`](docs/operations.md)
- Full metric catalog, dashboard ideas, and alerting strategies: [`docs/monitoring.md`](docs/monitoring.md)

> **Warning:** When Redis is enabled the streamer falls back to file storage if the cache is unavailable. Monitor logs for `Redis unavailable, falling back to file storage` to catch infrastructure issues early.

## Troubleshooting

| Symptom | Quick Fix | Deeper Dive |
| --- | --- | --- |
| Rising `http_buffer_drops_total` | Increase `http.buffer_size` or lower `processing.worker_count`. | Drops are normal during backlog catch-up; investigate only if they persist. |
| `processing_lag_seconds > 60` | Add HTTP endpoints or workers; confirm EdgeDelta capacity. | Scaling tips in [`docs/performance.md`](docs/performance.md). |
| S3 errors (`InvalidBucketName`, `AccessDenied`) | Remove `s3://` prefix, verify IAM permissions and region. | Re-run installer to regenerate credentials if needed. |
| HTTP 4xx/5xx spikes | Check EdgeDelta agent status and port availability. | Restart the agent (`systemctl restart edgedelta`). |
| Redis fallback warnings | Validate Redis availability with `redis-cli ping`. | Run `./s3-edgedelta-streamer --migrate-state` after Redis recovers. |

> **Need to rewind state?** Stop the service, edit `/var/lib/s3-streamer/state.json`, and restart. Delete the file to process everything from scratch.

## Architecture & Scaling

```
S3 (gzipped JSONL)
  ↓
15 S3 workers → 10k line buffer → 10 HTTP workers
  ↓
EdgeDelta HTTP inputs (8080/8081) → EdgeDelta backend
```

- HTTP streaming avoids temporary files and keeps latency low.
- Workers load-balance across endpoints using round robin.
- Redis-backed state unlocks multi-instance deployments.
- Real-world performance data and tuning levers live in [`docs/performance.md`](docs/performance.md).

## Documentation Map

- [`docs/log-formats.md`](docs/log-formats.md) – Complete log-format reference and regex tips
- [`docs/operations.md`](docs/operations.md) – Systemd, Docker, health endpoints, migrations
- [`docs/monitoring.md`](docs/monitoring.md) – Metrics catalog, dashboards, alert playbooks
- [`docs/performance.md`](docs/performance.md) – Throughput snapshots, scaling heuristics, data layout
- [`dashboard-header.md`](dashboard-header.md) – Ready-to-use EdgeDelta dashboard header copy

## Support

1. Inspect local logs: `/opt/edgedelta/s3-streamer/logs/streamer.log`
2. Check the EdgeDelta agent logs: `/var/log/edgedelta/edgedelta.log`
3. Review the troubleshooting table above
4. Escalate to EdgeDelta support with recent logs, metrics, and config snapshots
