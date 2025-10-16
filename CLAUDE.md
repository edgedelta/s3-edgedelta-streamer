# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This project streams Zscaler web logs from AWS S3 to Edge Delta for real-time log processing and analysis. The implementation uses HTTP streaming with dual endpoints to achieve high throughput (228 MB/minute) while maintaining low processing lag (< 60 seconds).

## Current Architecture

**Selected Method: HTTP Streaming with Dual Endpoints**

The final implementation uses direct HTTP streaming to Edge Delta with load balancing across two HTTP endpoints:

```
S3 → 15 S3 Workers (download & decompress) →
    10,000-line buffer →
    10 HTTP Workers (batch & send) →
    Edge Delta HTTP Inputs (ports 8080, 8081) →
    Edge Delta Processing
```

### Why HTTP Streaming?

After testing, HTTP streaming was chosen because:
- **Simplicity**: No file I/O, markers, or Redis dependencies
- **Performance**: Eliminates disk bottlenecks
- **Backpressure**: HTTP responses provide natural flow control
- **Reliability**: Stateless design, easy to restart
- **Scalability**: Can add more HTTP endpoints to distribute load

### Dual Endpoint Strategy

The system uses 2 HTTP endpoints (ports 8080, 8081) to:
- Distribute EdgeDelta ingestion load across multiple HTTP inputs
- Reduce memory pressure on EdgeDelta (50% memory reduction observed)
- Maintain stable throughput during burst traffic
- The 10 HTTP workers round-robin across both endpoints (5 workers per endpoint)

## System Components

### S3-to-EdgeDelta Streamer

**Location**: `/root/s3_work/s3-edgedelta-streamer/`

**Binary**: `s3-edgedelta-streamer-http` (29 MB)

**Configuration**: `config.yaml`

**Key Components**:
- **S3 Scanner** (`internal/scanner/`): Discovers new S3 objects within delay window
- **State Manager** (`internal/state/`): Tracks last processed timestamp to resume correctly
- **HTTP Worker Pool** (`internal/worker/http_pool.go`): 15 workers download and decompress files
- **HTTP Sender** (`internal/output/http_sender.go`): Batches lines and sends via HTTP POST
- **Metrics Exporter** (`internal/metrics/`): Exports OTLP metrics to EdgeDelta

### EdgeDelta Pipeline

**Configuration**: `pipeline-http.yaml`

**Inputs**:
- Port 8080: Primary HTTP input for logs
- Port 8081: Secondary HTTP input for logs (load balancing)
- Port 4317: OTLP gRPC input for metrics

**Processing**:
- Source multiprocessors parse JSONL and fix malformed JSON
- Destination multiprocessor for additional transformations
- Output to EdgeDelta backend

## Configuration

### S3 Settings

```yaml
s3:
  bucket: "mgxm-collections-useast1-853533826717"
  prefix: "/_weblog/feedname=Threat Team - Web/"
  region: "us-east-1"
```

**Important**: Bucket name should NOT include `s3://` prefix (this causes InvalidBucketName errors).

### HTTP Streaming Settings

```yaml
http:
  endpoints:
    - "http://localhost:8080"
    - "http://localhost:8081"
  batch_lines: 1000          # Max lines per batch
  batch_bytes: 1048576       # Max 1MB per batch
  flush_interval: 1s         # Force flush every second
  workers: 10                # HTTP workers (round-robin across endpoints)
  timeout: 30s
  max_idle_conns: 100
  idle_conn_timeout: 90s
```

### Processing Settings

```yaml
processing:
  worker_count: 15           # S3 download workers
  queue_size: 1000           # Job queue size
  scan_interval: 15s         # S3 scan frequency
  delay_window: 60s          # Process files at least 60s old
```

**Worker Count Tuning**: Started at 30 workers but reduced to 15 to prevent buffer overflow (40% packet loss). The ratio of 15 S3 workers → 10 HTTP workers provides stable throughput.

### OTLP Metrics

```yaml
otlp:
  enabled: true
  endpoint: "localhost:4317"
  export_interval: 10s
  service_name: "s3-edgedelta-streamer"
  service_version: "1.0.0"
  insecure: true
```

## S3 Data Characteristics

### Directory Structure
- Hive-style partitioning: `year=YYYY/month=M/day=D/`
- Example: `/_weblog/feedname=Threat Team - Web/year=2025/month=10/day=12/`

### File Format
- **Naming**: `<unix_timestamp>_<id>_<id>_<seq>[.gz]`
- **Example**: `1760305292_56442_130_1.gz`
- **Timestamp**: Unix epoch seconds (1760305292 = 2025-10-12 21:41:32 UTC)

### Compression
- **CRITICAL**: ALL files are gzip compressed, regardless of extension
- Files without `.gz` extension are STILL gzipped (magic bytes: 1f8b)
- Must use `gzip.NewReader()` for all files
- Compression ratio: ~15:1 (10 MB → 640-660 KB)

### Data Content
- **Format**: JSONL (newline-delimited JSON)
- **Source**: Zscaler NSS web logs
- **Uncompressed size**: ~10 MB per file (~6,554 lines)
- **Content**: Each line is a JSON object with `sourcetype` and `event` fields

## Performance Characteristics

### Throughput (Steady State)
- **Files per scan**: 14-32 files every 15 seconds (~28 average)
- **Data per scan**: ~57 MB uncompressed
- **Sustained rate**: 228 MB/minute (13.7 GB/hour)
- **Lines processed**: ~180K-280K lines per scan

### Processing Lag
- **Target**: < 60 seconds (delay_window)
- **Achieved**: 31 seconds average (better than target)
- **Initial lag**: 220 seconds during startup
- **Steady state**: 31-35 seconds

### Resource Usage

**Streamer**:
- CPU: 13-14% (plenty of headroom)
- Memory: 147 MB
- Binary size: 29 MB

**EdgeDelta**:
- CPU: 110-120% (~1 core)
- Memory: 2.3 GB (reduced from 4.8 GB with dual endpoints)

**System**:
- 4 CPU cores, 15.9 GB RAM
- ~60% CPU idle during normal operation
- 6-7 GB RAM available

## Monitoring & Metrics

### OTLP Metrics Exported

The streamer exports metrics to EdgeDelta every 10 seconds via OTLP/gRPC (port 4317):

**S3 Worker Metrics**:
- `s3_files_processed_total` (counter): Files successfully processed
- `s3_bytes_processed_total` (counter): Bytes processed from S3
- `s3_files_errored_total` (counter): File processing errors
- `s3_processing_latency_seconds` (histogram): Time to process each file

**HTTP Sender Metrics**:
- `http_batches_sent_total` (counter): Batches sent to EdgeDelta
- `http_lines_sent_total` (counter): Log lines sent
- `http_bytes_sent_total` (counter): Bytes sent via HTTP
- `http_errors_total` (counter): HTTP send errors
- `http_buffer_drops_total` (counter): Lines dropped due to buffer overflow

**Processing Lag**:
- `processing_lag_seconds` (gauge): Time between file timestamp and processing time

### Dashboard

A monitoring dashboard is available at `/root/s3_work/s3-edgedelta-streamer/dashboard-header.md` with:
- Architecture overview
- Throughput metrics
- Reliability indicators (errors, buffer drops)
- Performance metrics (lag, latency)

### Understanding Error Metrics

**Error counts are cumulative** (lifetime totals, not per-scan):
- Errors increment during operation but never reset
- A stable error count means zero new errors (good!)
- Most errors occur during startup when processing large backlogs
- Steady-state operation typically has zero buffer drops

Example progression:
```
Scan #1: 63,554 errors (startup backlog)
Scan #2: 79,317 errors (15,763 new)
Scan #6: 92,221 errors (12,904 new)
Scan #7-9: 92,221 errors (0 new - stable!)
```

## Installation

### Quick Install

The streamer includes an interactive installer that handles all setup:

```bash
cd /root/s3_work/s3-edgedelta-streamer
sudo ./install.sh
```

**What the installer does:**
1. Validates EdgeDelta is installed and running
2. Prompts for AWS credentials and S3 configuration
3. Tests AWS and S3 access
4. Encrypts credentials (AES-256, machine-specific)
5. Installs to `/opt/edgedelta/s3-streamer/`
6. Creates systemd service with EdgeDelta dependencies
7. Starts service automatically

**Important**: Deploy EdgeDelta pipeline first to ensure ports are ready.

### Installation Details

**Location**: `/opt/edgedelta/s3-streamer/`
```
├── bin/s3-edgedelta-streamer      # Binary
├── config/config.yaml             # Configuration
└── logs/streamer.log              # Application logs
```

**Credentials**: `/etc/systemd/creds/s3-streamer/` (encrypted, root-only)

**State**: `/var/lib/s3-streamer/state.json` (resumable processing)

### Systemd Service

The service is tightly integrated with EdgeDelta:

**Service Dependencies:**
- `BindsTo=edgedelta.service` - Stops when EdgeDelta stops
- `PartOf=edgedelta.service` - Restarts when EdgeDelta restarts
- `After=edgedelta.service` - Starts after EdgeDelta

**Automatic Behavior:**
- Starts at boot (if EdgeDelta starts)
- Stops when EdgeDelta stops
- Restarts when EdgeDelta restarts
- Auto-restarts on crashes (10s delay)

## Operational Guide

### Service Management

```bash
# Check status
sudo systemctl status s3-streamer

# View logs (journald)
sudo journalctl -u s3-streamer -f

# View logs (file)
tail -f /opt/edgedelta/s3-streamer/logs/streamer.log

# Restart service
sudo systemctl restart s3-streamer

# Stop service (will auto-start with EdgeDelta)
sudo systemctl stop s3-streamer

# Disable auto-start
sudo systemctl disable s3-streamer

# Re-enable auto-start
sudo systemctl enable s3-streamer
```

### Testing EdgeDelta Dependencies

```bash
# Stop EdgeDelta - streamer should stop too
sudo systemctl stop edgedelta
sudo systemctl status s3-streamer  # Should be inactive

# Start EdgeDelta - streamer should start too
sudo systemctl start edgedelta
sleep 10
sudo systemctl status s3-streamer  # Should be active

# Restart EdgeDelta - streamer should restart too
sudo systemctl restart edgedelta
sudo journalctl -u s3-streamer --since "1 minute ago"
```

### Checking Status

```bash
# Check both services
sudo systemctl status edgedelta s3-streamer

# Check HTTP endpoints are listening
ss -tuln | grep -E "8080|8081|4317"

# View recent activity
sudo journalctl -u s3-streamer -n 50

# Check EdgeDelta logs
tail -f /var/log/edgedelta/edgedelta.log
```

### Reconfiguration

To change configuration after installation:

```bash
# Stop service
sudo systemctl stop s3-streamer

# Edit configuration
sudo nano /opt/edgedelta/s3-streamer/config/config.yaml

# Restart
sudo systemctl start s3-streamer
```

To change AWS credentials, reinstall:
```bash
sudo ./install.sh
# Installer detects existing installation and offers reconfiguration
```

### Uninstallation

```bash
sudo ./uninstall.sh
```

Removes service, binary, configuration, and encrypted credentials. Optionally preserves state file.

### Credential Security

**Encryption Method:**
- AES-256-CBC encryption
- Machine-specific key derived from `/etc/machine-id` + salt
- OpenSSL pbkdf2 key derivation

**Storage:**
- Location: `/etc/systemd/creds/s3-streamer/`
- Permissions: 0700 (directory), 0600 (files)
- Owner: root

**Decryption:**
- Go binary decrypts at startup
- Uses same machine-id + salt to derive key
- Credentials not portable to other machines
- No plaintext credentials on disk

**Security Properties:**
- File permissions prevent unauthorized access
- Machine-specific encryption prevents credential theft via file copy
- systemd security directives limit process capabilities
- Service runs as non-root user (edgedelta)

### Building from Source

```bash
cd /root/s3_work/s3-edgedelta-streamer

# Build
/usr/local/go/bin/go build -o s3-edgedelta-streamer-http ./cmd/streamer/main_http.go

# Verify
ls -lh s3-edgedelta-streamer-http
```

### Deploying Pipeline Changes

1. Update `pipeline-http.yaml`
2. Deploy via EdgeDelta UI (not via CLI)
3. EdgeDelta automatically reloads the configuration
4. Verify ports are listening: `ss -tuln | grep -E "8080|8081|4317"`

## Troubleshooting

### High Buffer Drops

**Symptoms**: `http_buffer_drops_total` increasing rapidly

**Causes**:
- Too many S3 workers overwhelming the buffer (10,000 line capacity)
- EdgeDelta processing too slowly

**Solutions**:
1. **Reduce S3 workers**: Decrease `processing.worker_count` from 15 to 10
2. **Increase buffer**: Change `lineChan: make(chan []byte, 10000)` → 50000 in http_sender.go
3. **Add backpressure**: Make SendLine blocking instead of dropping (guarantees zero data loss)

### High Processing Lag

**Symptoms**: `processing_lag_seconds` > 60 seconds consistently

**Causes**:
- S3 producing data faster than pipeline can consume
- EdgeDelta CPU bottleneck
- Network issues

**Solutions**:
1. **Add more HTTP endpoints**: Scale to 3-4 endpoints
2. **Increase HTTP workers**: Scale from 10 to 20 workers
3. **Increase EdgeDelta resources**: More CPU/memory
4. **Check EdgeDelta logs**: Look for processing bottlenecks

### HTTP Send Errors

**Symptoms**: `http_errors_total` increasing

**Causes**:
- EdgeDelta not responding (down, restarting, overloaded)
- Network connectivity issues
- Port not listening

**Solutions**:
1. Check EdgeDelta status: `systemctl status edgedelta`
2. Verify ports listening: `ss -tuln | grep -E "8080|8081"`
3. Check EdgeDelta logs: `tail -f /var/log/edgedelta/edgedelta.log`
4. Restart EdgeDelta if needed: `systemctl restart edgedelta`

### S3 Access Errors

**Symptoms**: "InvalidBucketName" or access denied errors

**Common Issues**:
1. **Bucket name includes s3:// prefix**: Remove it from config.yaml
2. **Wrong credentials**: Verify AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY
3. **Wrong region**: Verify `s3.region` matches bucket region
4. **Permissions**: Ensure IAM user has `s3:GetObject` and `s3:ListBucket` permissions

### State Recovery

The streamer tracks the last processed timestamp in `/var/lib/s3-streamer/state.json`:

```json
{
  "last_processed_timestamp": 1760305468,
  "last_processed_time": "2025-10-12T21:44:28Z"
}
```

**To reprocess data**:
1. Stop streamer
2. Edit state.json to set earlier timestamp
3. Restart streamer

**To start fresh**:
1. Stop streamer
2. Delete state.json
3. Restart streamer (will process files from current time - delay_window)

## Architecture Evolution

### V1: File-based with Markers (Deprecated)
- Downloaded files to disk
- Used Redis markers for file rotation
- Complex state management
- High disk I/O

### V2: HTTP Streaming Single Endpoint
- Direct HTTP streaming
- Eliminated file I/O and Redis
- Simpler architecture
- EdgeDelta memory pressure at high load

### V3: HTTP Streaming Dual Endpoints (Current)
- Load balanced across 2 HTTP inputs
- 50% memory reduction on EdgeDelta
- Stable performance during bursts
- Zero data loss in steady state

## Best Practices

### Configuration Tuning

1. **Worker Ratio**: Maintain ~1.5:1 ratio of S3 workers to HTTP workers (e.g., 15:10)
2. **Batch Size**: Keep at 1000 lines or 1MB for optimal HTTP performance
3. **Scan Interval**: 15 seconds provides good balance of latency vs. overhead
4. **Delay Window**: 60 seconds ensures files are fully written before processing

### Monitoring

1. **Track cumulative metrics**: Understand error counts are lifetime totals
2. **Watch rate of change**: Use metric dashboards to show errors/second, not totals
3. **Alert on steady-state drops**: Buffer drops during startup are normal, steady-state drops indicate issues
4. **Monitor processing lag**: Keep under 60 seconds for near real-time processing

### Deployment

1. **Test pipeline changes first**: Deploy pipeline updates via UI before restarting streamer
2. **Graceful restarts**: Use pkill (not kill -9) to allow graceful shutdown
3. **Monitor startup**: Watch logs for first 2-3 minutes to catch startup issues
4. **Verify endpoints**: Always check ports are listening after deployment

### Scaling

**To increase throughput**:
1. Add more HTTP endpoints (ports 8082, 8083, etc.)
2. Increase HTTP workers proportionally
3. Monitor EdgeDelta CPU/memory
4. Consider horizontal scaling (multiple streamer instances)

**To reduce resource usage**:
1. Decrease S3 workers
2. Increase scan_interval (less frequent scans)
3. Increase delay_window (process older data)

## File Locations

```
/root/s3_work/
├── s3-edgedelta-streamer/          # Go project
│   ├── cmd/streamer/
│   │   └── main_http.go            # Main entrypoint
│   ├── internal/
│   │   ├── config/                 # Configuration loading
│   │   ├── scanner/                # S3 scanning logic
│   │   ├── state/                  # State persistence
│   │   ├── worker/http_pool.go     # S3 worker pool
│   │   ├── output/http_sender.go   # HTTP batching & sending
│   │   └── metrics/metrics.go      # OTLP metrics
│   ├── config.yaml                 # Runtime configuration
│   ├── pipeline-http.yaml          # EdgeDelta pipeline config
│   ├── dashboard-header.md         # Dashboard markdown snippet
│   └── s3-edgedelta-streamer-http  # Compiled binary (29MB)
├── streamer.log                    # Runtime logs (nohup output)
└── CLAUDE.md                       # This file

/var/lib/s3-streamer/
└── state.json                      # Persisted state (last processed timestamp)

/var/log/edgedelta/
└── edgedelta.log                   # EdgeDelta agent logs
```

## Development Notes

### Adding New Metrics

1. Define metric in `internal/metrics/metrics.go`:
   ```go
   MyMetric, err := meter.Int64Counter("my_metric_name", ...)
   ```

2. Add recording method:
   ```go
   func (m *Metrics) RecordMyMetric(ctx context.Context, value int64) {
       m.MyMetric.Add(ctx, value)
   }
   ```

3. Instrument the code where measurement happens

4. Rebuild and restart streamer

5. Verify in EdgeDelta dashboard

### Testing Changes

```bash
# Run all tests
go test ./...

# Run tests with verbose output
go test ./... -v

# Run tests with coverage
go test ./... -cover

# Run specific package tests
go test ./internal/config -v

# Generate HTML coverage report
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out

# Build
/usr/local/go/bin/go build -o s3-edgedelta-streamer-http ./cmd/streamer/main_http.go

# Test run (foreground for debugging)
./s3-edgedelta-streamer-http --config config.yaml

# Watch logs in real-time
tail -f streamer.log

# Monitor resource usage
top -p $(pgrep -f s3-edgedelta-streamer-http)
```

**Test Coverage Goals**:
- config: 77.6%
- health: 63.9%
- logging: 95.8%
- metrics: 55.1%
- scanner: 40.4%
- state: 89.3%
- tcppool: 37.8%
- credentials: 36.4%
- worker: 22.8%
- output: 7.2%

### Adding New HTTP Endpoints

1. Update `config.yaml`:
   ```yaml
   http:
     endpoints:
       - "http://localhost:8080"
       - "http://localhost:8081"
       - "http://localhost:8082"  # Add new endpoint
   ```

2. Update `pipeline-http.yaml`:
   ```yaml
   - name: http_logs_3
     type: http_input
     port: 8082
     path: /
   ```

3. Deploy pipeline via EdgeDelta UI

4. Restart streamer

5. Verify: `ss -tuln | grep 8082`

## Support

For issues or questions:
1. Check logs: `tail -f streamer.log` and `tail -f /var/log/edgedelta/edgedelta.log`
2. Verify configuration matches this guide
3. Review troubleshooting section above
4. Check EdgeDelta dashboard for metric anomalies
