#!/bin/bash
# S3-EdgeDelta Streamer Installer
# Interactive installation with encrypted credentials
# Usage: sudo ./install.sh

set -e

# Constants
VERSION="1.0.0"
INSTALL_DIR="/opt/edgedelta/s3-streamer"
STATE_DIR="/var/lib/s3-streamer"
CREDS_DIR="/etc/systemd/creds/s3-streamer"
SERVICE_FILE="/etc/systemd/system/s3-streamer.service"
ENV_FILE="/etc/sysconfig/s3-streamer"
SERVICE_USER="edgedelta"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Helper functions
error() {
    echo -e "${RED}✗ Error: $1${NC}" >&2
    exit 1
}

warn() {
    echo -e "${YELLOW}⚠ Warning: $1${NC}"
}

success() {
    echo -e "${GREEN}✓ $1${NC}"
}

info() {
    echo -e "$1"
}

# Check prerequisites
check_prerequisites() {
    info ""
    info "=== S3-EdgeDelta Streamer Installer v${VERSION} ==="
    info ""
    info "Checking prerequisites..."

    # Check root
    if [[ $EUID -ne 0 ]]; then
        error "Must run as root (use sudo)"
    fi
    success "Running as root"

    # Check EdgeDelta installation
    if [[ ! -f /opt/edgedelta/agent/edgedelta ]]; then
        error "EdgeDelta agent not found at /opt/edgedelta/agent/edgedelta"
    fi
    success "EdgeDelta agent found"

    # Check EdgeDelta service
    if ! systemctl is-active --quiet edgedelta; then
        error "EdgeDelta service is not running (systemctl start edgedelta)"
    fi
    success "EdgeDelta service running"

    # Check edgedelta user
    if ! id -u "$SERVICE_USER" &>/dev/null; then
        error "User '$SERVICE_USER' not found"
    fi
    success "EdgeDelta user exists"

    # Check EdgeDelta ports
    if ! ss -tuln | grep -q ":8080"; then
        warn "EdgeDelta HTTP port 8080 not listening"
    else
        success "EdgeDelta port 8080 listening"
    fi

    if ! ss -tuln | grep -q ":8081"; then
        warn "EdgeDelta HTTP port 8081 not listening"
    else
        success "EdgeDelta port 8081 listening"
    fi

    if ! ss -tuln | grep -q ":4317"; then
        warn "EdgeDelta OTLP port 4317 not listening"
    else
        success "EdgeDelta OTLP port 4317 listening"
    fi

    # Check required tools
    for cmd in openssl systemctl aws; do
        if ! command -v "$cmd" &>/dev/null; then
            error "'$cmd' command not found"
        fi
    done
    success "Required tools available"

    # Check for existing installation
    if systemctl is-active --quiet s3-streamer 2>/dev/null; then
        warn "S3-streamer service is already running"
        read -p "Reinstall/reconfigure? [y/N]: " REINSTALL
        if [[ "${REINSTALL,,}" != "y" ]]; then
            info "Installation cancelled"
            exit 0
        fi
        info "Stopping existing service..."
        systemctl stop s3-streamer || true
    fi

    success "All prerequisites met"
}

# Interactive configuration
interactive_configuration() {
    info ""
    info "=== AWS Configuration ==="
    info ""

    # Detect existing credentials from environment or existing installation
    EXISTING_KEY="${AWS_ACCESS_KEY_ID:-}"
    EXISTING_SECRET="${AWS_SECRET_ACCESS_KEY:-}"
    EXISTING_REGION="${AWS_REGION:-us-east-1}"

    # Try to decrypt existing credentials if reinstalling
    if [[ -f "$CREDS_DIR/aws_access_key_id" ]]; then
        info "Existing credentials found. Press Enter to keep current values."
        EXISTING_KEY="<current>"
        EXISTING_SECRET="<current>"
        EXISTING_REGION="<current>"
    fi

    # Prompt for AWS Access Key
    if [[ "$EXISTING_KEY" == "<current>" ]]; then
        read -p "AWS Access Key ID [keep current]: " AWS_KEY
    else
        read -p "AWS Access Key ID${EXISTING_KEY:+ [${EXISTING_KEY:0:8}...]}: " AWS_KEY
    fi

    # Use existing if empty
    if [[ -z "$AWS_KEY" ]]; then
        if [[ "$EXISTING_KEY" == "<current>" ]]; then
            # Decrypt existing
            AWS_KEY=$(decrypt_existing_credential "aws_access_key_id") || error "Failed to decrypt existing credentials"
        else
            AWS_KEY="$EXISTING_KEY"
        fi
    fi

    [[ -z "$AWS_KEY" ]] && error "AWS Access Key ID is required"

    # Prompt for AWS Secret Key
    if [[ "$EXISTING_SECRET" == "<current>" ]]; then
        read -sp "AWS Secret Access Key [keep current]: " AWS_SECRET
    else
        read -sp "AWS Secret Access Key${EXISTING_SECRET:+ [hidden]}: " AWS_SECRET
    fi
    echo ""

    if [[ -z "$AWS_SECRET" ]]; then
        if [[ "$EXISTING_SECRET" == "<current>" ]]; then
            AWS_SECRET=$(decrypt_existing_credential "aws_secret_access_key") || error "Failed to decrypt existing credentials"
        else
            AWS_SECRET="$EXISTING_SECRET"
        fi
    fi

    [[ -z "$AWS_SECRET" ]] && error "AWS Secret Access Key is required"

    # Prompt for AWS Region
    if [[ "$EXISTING_REGION" == "<current>" ]]; then
        read -p "AWS Region [keep current]: " AWS_REGION
    else
        read -p "AWS Region [$EXISTING_REGION]: " AWS_REGION
    fi

    if [[ -z "$AWS_REGION" ]]; then
        if [[ "$EXISTING_REGION" == "<current>" ]]; then
            AWS_REGION=$(decrypt_existing_credential "aws_region") || error "Failed to decrypt existing credentials"
        else
            AWS_REGION="$EXISTING_REGION"
        fi
    fi

    # Validate AWS credentials
    info ""
    info "Testing AWS credentials..."
    export AWS_ACCESS_KEY_ID="$AWS_KEY"
    export AWS_SECRET_ACCESS_KEY="$AWS_SECRET"
    export AWS_DEFAULT_REGION="$AWS_REGION"

    if aws sts get-caller-identity >/dev/null 2>&1; then
        success "AWS credentials valid"
    else
        error "Invalid AWS credentials"
    fi

    # S3 Configuration
    info ""
    info "=== S3 Configuration ==="
    info ""

    read -p "S3 Bucket Name (no s3:// prefix): " S3_BUCKET
    [[ -z "$S3_BUCKET" ]] && error "S3 Bucket name is required"

    # Check for s3:// prefix
    if [[ "$S3_BUCKET" =~ ^s3:// ]]; then
        error "Remove 's3://' prefix from bucket name"
    fi

    read -p "S3 Prefix [/_weblog/]: " S3_PREFIX
    S3_PREFIX="${S3_PREFIX:-/_weblog/}"

    # Test S3 access
    info "Testing S3 access..."
    if aws s3 ls "s3://${S3_BUCKET}${S3_PREFIX}" --recursive --max-items 1 >/dev/null 2>&1; then
        FILE_COUNT=$(aws s3 ls "s3://${S3_BUCKET}${S3_PREFIX}" --recursive 2>/dev/null | wc -l)
        success "S3 access successful (found $FILE_COUNT objects)"
    else
        error "Failed to access S3 bucket. Check permissions and bucket name."
    fi

    # Performance Settings
    info ""
    info "=== Performance Settings ==="
    info ""

    read -p "S3 Worker Count [15]: " S3_WORKERS
    S3_WORKERS="${S3_WORKERS:-15}"

    read -p "HTTP Worker Count [10]: " HTTP_WORKERS
    HTTP_WORKERS="${HTTP_WORKERS:-10}"

    read -p "Scan Interval [15s]: " SCAN_INTERVAL
    SCAN_INTERVAL="${SCAN_INTERVAL:-15s}"

    # Configuration Summary
    info ""
    info "=== Configuration Summary ==="
    info "AWS Region: $AWS_REGION"
    info "S3 Bucket: $S3_BUCKET"
    info "S3 Prefix: $S3_PREFIX"
    info "S3 Workers: $S3_WORKERS"
    info "HTTP Workers: $HTTP_WORKERS"
    info "Scan Interval: $SCAN_INTERVAL"
    info ""

    read -p "Proceed with installation? [Y/n]: " CONFIRM
    if [[ "${CONFIRM,,}" == "n" ]]; then
        info "Installation cancelled"
        exit 0
    fi
}

# Decrypt existing credential (helper for reinstallation)
decrypt_existing_credential() {
    local name=$1
    local machine_id=$(cat /etc/machine-id)
    local salt="s3-edgedelta-streamer-v1"
    local enc_key=$(echo -n "${machine_id}${salt}" | sha256sum | cut -d' ' -f1)

    openssl enc -aes-256-cbc -d -pbkdf2 \
        -pass pass:"$enc_key" \
        -in "$CREDS_DIR/$name" 2>/dev/null || echo ""
}

# Encrypt credentials
encrypt_credentials() {
    info ""
    info "Encrypting credentials..."

    # Create credentials directory
    mkdir -p "$CREDS_DIR"
    chmod 700 "$CREDS_DIR"

    # Generate machine-specific encryption key
    local machine_id=$(cat /etc/machine-id)
    local salt="s3-edgedelta-streamer-v1"
    local enc_key=$(echo -n "${machine_id}${salt}" | sha256sum | cut -d' ' -f1)

    # Encrypt each credential
    encrypt_value() {
        local name=$1
        local value=$2
        local outfile="${CREDS_DIR}/${name}"

        echo -n "$value" | openssl enc -aes-256-cbc -salt -pbkdf2 \
            -pass pass:"$enc_key" -out "$outfile" 2>/dev/null

        if [[ $? -eq 0 ]]; then
            chmod 600 "$outfile"
            return 0
        else
            return 1
        fi
    }

    encrypt_value "aws_access_key_id" "$AWS_KEY" || error "Failed to encrypt access key"
    encrypt_value "aws_secret_access_key" "$AWS_SECRET" || error "Failed to encrypt secret key"
    encrypt_value "aws_region" "$AWS_REGION" || error "Failed to encrypt region"

    success "Credentials encrypted (machine-specific AES-256)"
}

# Install files
install_files() {
    info ""
    info "Installing files..."

    # Stop any running instances
    pkill -f s3-edgedelta-streamer-http || true
    pkill -f s3-edgedelta-streamer || true

    # Create directories
    mkdir -p "$INSTALL_DIR"/{bin,config,logs}
    mkdir -p "$STATE_DIR"

    # Find and copy binary
    BINARY_NAME=""
    if [[ -f "$SCRIPT_DIR/s3-edgedelta-streamer-http" ]]; then
        BINARY_NAME="s3-edgedelta-streamer-http"
    elif [[ -f "$SCRIPT_DIR/s3-edgedelta-streamer" ]]; then
        BINARY_NAME="s3-edgedelta-streamer"
    else
        error "Binary not found in current directory (s3-edgedelta-streamer-http or s3-edgedelta-streamer)"
    fi

    cp "$SCRIPT_DIR/$BINARY_NAME" "$INSTALL_DIR/bin/s3-edgedelta-streamer"
    chmod 755 "$INSTALL_DIR/bin/s3-edgedelta-streamer"
    success "Binary installed"

    # Create config.yaml
    cat > "$INSTALL_DIR/config/config.yaml" <<EOF
s3:
  bucket: "$S3_BUCKET"
  prefix: "$S3_PREFIX"
  region: "$AWS_REGION"

http:
  endpoints:
    - "http://localhost:8080"
    - "http://localhost:8081"
  batch_lines: 1000
  batch_bytes: 1048576
  flush_interval: 1s
  workers: $HTTP_WORKERS
  timeout: 30s
  max_idle_conns: 100
  idle_conn_timeout: 90s

processing:
  worker_count: $S3_WORKERS
  queue_size: 1000
  scan_interval: $SCAN_INTERVAL
  delay_window: 60s

state:
  file_path: "$STATE_DIR/state.json"
  save_interval: 30s

logging:
  level: "info"
  format: "json"

otlp:
  enabled: true
  endpoint: "localhost:4317"
  export_interval: 10s
  service_name: "s3-edgedelta-streamer"
  service_version: "$VERSION"
  insecure: true
EOF
    success "Configuration file created"

    # Create environment file
    cat > "$ENV_FILE" <<EOF
# S3-EdgeDelta Streamer Environment
STREAMER_VERSION=$VERSION
CREDENTIALS_DIR=$CREDS_DIR
EOF
    chmod 644 "$ENV_FILE"

    # Set ownership
    chown -R $SERVICE_USER:$SERVICE_USER "$INSTALL_DIR"
    chown -R $SERVICE_USER:$SERVICE_USER "$STATE_DIR"

    success "Files installed to $INSTALL_DIR"
}

# Create systemd service
create_systemd_service() {
    info ""
    info "Creating systemd service..."

    cat > "$SERVICE_FILE" <<'EOF'
[Unit]
Description=S3 to EdgeDelta Streamer
Documentation=https://github.com/yourorg/s3-edgedelta-streamer
After=edgedelta.service network-online.target
Requires=network-online.target
BindsTo=edgedelta.service
PartOf=edgedelta.service

[Service]
Type=simple
User=edgedelta
Group=edgedelta
WorkingDirectory=/opt/edgedelta/s3-streamer
ExecStart=/opt/edgedelta/s3-streamer/bin/s3-edgedelta-streamer --config config/config.yaml
EnvironmentFile=/etc/sysconfig/s3-streamer
Restart=on-failure
RestartSec=10
StandardOutput=append:/opt/edgedelta/s3-streamer/logs/streamer.log
StandardError=append:/opt/edgedelta/s3-streamer/logs/streamer.log

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/s3-streamer /opt/edgedelta/s3-streamer/logs /etc/systemd/creds/s3-streamer

[Install]
WantedBy=multi-user.target
EOF

    # Reload systemd
    systemctl daemon-reload
    systemctl enable s3-streamer

    success "Systemd service created and enabled"
}

# Start service
start_service() {
    info ""
    info "Starting service..."

    systemctl start s3-streamer

    # Wait for service to start
    sleep 3

    if systemctl is-active --quiet s3-streamer; then
        success "Service started successfully"
    else
        error "Service failed to start. Check logs: journalctl -u s3-streamer -n 50"
    fi
}

# Post-installation verification
post_install_verification() {
    info ""
    info "=== Verification ==="
    info ""

    # Show service status
    systemctl status s3-streamer --no-pager --lines=0

    info ""
    info "Waiting for initial data processing (20 seconds)..."
    sleep 20

    # Check for processing activity
    if journalctl -u s3-streamer --since "30 seconds ago" -n 100 | grep -qi "processing\|scan\|batch"; then
        success "Processing activity detected"
    else
        warn "No processing activity yet (this may be normal during startup)"
    fi

    info ""
    info "=== Installation Complete ==="
    info ""
    info "Service Management:"
    info "  Status:  systemctl status s3-streamer"
    info "  Stop:    systemctl stop s3-streamer"
    info "  Restart: systemctl restart s3-streamer"
    info ""
    info "View Logs:"
    info "  journalctl -u s3-streamer -f"
    info "  tail -f $INSTALL_DIR/logs/streamer.log"
    info ""
    info "Configuration:"
    info "  Config:  $INSTALL_DIR/config/config.yaml"
    info "  Creds:   $CREDS_DIR/ (encrypted)"
    info "  State:   $STATE_DIR/state.json"
    info ""
    info "The service will automatically:"
    info "  - Start when EdgeDelta starts"
    info "  - Stop when EdgeDelta stops"
    info "  - Restart when EdgeDelta restarts"
    info "  - Start automatically at boot"
    info ""
    info "EdgeDelta Dashboard:"
    info "  Metrics available via OTLP at localhost:4317"
    info "  See dashboard-header.md for visualization template"
    info ""
}

# Main installation flow
main() {
    check_prerequisites
    interactive_configuration
    encrypt_credentials
    install_files
    create_systemd_service
    start_service
    post_install_verification
}

# Run main function
main "$@"
