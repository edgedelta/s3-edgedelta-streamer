#!/bin/bash
# S3-EdgeDelta Streamer Uninstaller
# Removes service, binaries, and configuration
# Usage: sudo ./uninstall.sh

set -e

# Constants
INSTALL_DIR="/opt/edgedelta/s3-streamer"
STATE_DIR="/var/lib/s3-streamer"
CREDS_DIR="/etc/systemd/creds/s3-streamer"
SERVICE_FILE="/etc/systemd/system/s3-streamer.service"
ENV_FILE="/etc/sysconfig/s3-streamer"

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
    echo -e "${YELLOW}⚠ $1${NC}"
}

success() {
    echo -e "${GREEN}✓ $1${NC}"
}

info() {
    echo -e "$1"
}

# Check root
if [[ $EUID -ne 0 ]]; then
    error "Must run as root (use sudo)"
fi

# Check if installed
if [[ ! -d "$INSTALL_DIR" ]] && [[ ! -f "$SERVICE_FILE" ]]; then
    error "S3-EdgeDelta Streamer is not installed"
fi

# Confirmation prompt
info ""
info "=== Uninstall S3-EdgeDelta Streamer ==="
info ""
info "This will remove:"
info "  - Systemd service ($SERVICE_FILE)"
info "  - Binary and configuration ($INSTALL_DIR)"
info "  - Encrypted credentials ($CREDS_DIR)"
info "  - Environment file ($ENV_FILE)"
info ""
info "⚠️  EdgeDelta agent will NOT be affected."
info ""

read -p "Proceed with uninstallation? [y/N]: " CONFIRM
if [[ "${CONFIRM,,}" != "y" ]]; then
    info "Uninstallation cancelled"
    exit 0
fi

info ""
info "Uninstalling..."

# Stop and disable service
if systemctl is-active --quiet s3-streamer 2>/dev/null; then
    info "Stopping service..."
    systemctl stop s3-streamer || true
    success "Service stopped"
fi

if systemctl is-enabled --quiet s3-streamer 2>/dev/null; then
    info "Disabling service..."
    systemctl disable s3-streamer || true
    success "Service disabled"
fi

# Remove systemd service file
if [[ -f "$SERVICE_FILE" ]]; then
    rm -f "$SERVICE_FILE"
    success "Systemd service file removed"
fi

# Remove environment file
if [[ -f "$ENV_FILE" ]]; then
    rm -f "$ENV_FILE"
    success "Environment file removed"
fi

# Remove credentials
if [[ -d "$CREDS_DIR" ]]; then
    rm -rf "$CREDS_DIR"
    success "Encrypted credentials removed"
fi

# Remove installation directory
if [[ -d "$INSTALL_DIR" ]]; then
    rm -rf "$INSTALL_DIR"
    success "Installation directory removed"
fi

# Reload systemd
systemctl daemon-reload

# Ask about state file
info ""
if [[ -d "$STATE_DIR" ]]; then
    read -p "Remove state file (processing history)? [y/N]: " REMOVE_STATE
    if [[ "${REMOVE_STATE,,}" == "y" ]]; then
        rm -rf "$STATE_DIR"
        success "State directory removed"
    else
        info "State file preserved: $STATE_DIR/state.json"
        info "(Can be used if reinstalling later)"
    fi
fi

info ""
success "Uninstallation complete!"
info ""
info "To reinstall:"
info "  sudo ./install.sh"
info ""
