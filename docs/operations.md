# Operations Playbook

This guide collects day-to-day operational commands, health checks, and deployment options for the S3 → EdgeDelta streamer.

## Service Management (systemd)

```bash
# Check status
sudo systemctl status s3-streamer

# Follow logs
sudo journalctl -fu s3-streamer

# Restart / stop
sudo systemctl restart s3-streamer
sudo systemctl stop s3-streamer
```

> **Note:** The installer wires the streamer to the EdgeDelta agent. The service starts and stops with the agent and auto-restarts on failure.

## Reconfiguration Workflow

```bash
sudo systemctl stop s3-streamer
sudo nano /opt/edgedelta/s3-streamer/config/config.yaml
sudo systemctl start s3-streamer
```

For new AWS credentials, re-run `install.sh`; it detects the existing deployment and updates secrets in-place.

## Health Endpoints

| Endpoint | Description |
| --- | --- |
| `GET /health` | Full dependency check (S3, Redis, HTTP endpoints) |
| `GET /ready` | Alias of `/health` used by Kubernetes / load balancers |

Example response:

```json
{
  "status": "healthy",
  "checks": {
    "s3": "OK",
    "http": "OK"
  },
  "timestamp": "2025-01-13T16:21:50Z"
}
```

Configure in `config.yaml`:

```yaml
health:
  enabled: true
  address: ":8080"
  path: "/health"
```

## Common CLI Operations (manual run)

```bash
# Ensure the process is running
pgrep -f s3-edgedelta-streamer-http

# Graceful stop
pkill -f s3-edgedelta-streamer-http

# Restart in background
nohup ./s3-edgedelta-streamer-http --config config.yaml > streamer.log 2>&1 &

# Tail application logs
tail -f streamer.log

# Inspect EdgeDelta agent logs
sudo tail -f /var/log/edgedelta/edgedelta.log

# Verify ingress ports
ss -tuln | grep -E "8080|8081|4317"
```

## Container Deployments

### Docker

```bash
docker build -t s3-edgedelta-streamer .

docker run -p 8080:8080 \
  -e AWS_ACCESS_KEY_ID=your_key \
  -e AWS_SECRET_ACCESS_KEY=your_secret \
  -e AWS_REGION=us-east-1 \
  s3-edgedelta-streamer
```

### Docker Compose

```bash
docker-compose up -d
docker-compose logs -f s3-edgedelta-streamer
docker-compose down
```

## Redis Migration & Recovery

```bash
./s3-edgedelta-streamer --migrate-state
```

> **Tip:** Redis is optional but required for safe horizontal scaling. When Redis is unreachable, the streamer automatically logs a warning and falls back to the local state file.

Manual state reset:

1. Stop the streamer (`pkill -f s3-edgedelta-streamer-http`).
2. Edit `/var/lib/s3-streamer/state.json` to the desired timestamp.
3. Restart the service.

To process from scratch, delete the state file instead of editing it.
