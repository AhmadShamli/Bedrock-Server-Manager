#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"

echo "=========================================================="
echo "    Bedrock Server Manager (BSM) - Update Script"
echo "=========================================================="

# Check if repo is git directory and pull latest changes
if [ -d "${ROOT_DIR}/.git" ]; then
    echo "[1/4] Pulling latest git updates..."
    cd "${ROOT_DIR}"
    if [ -n "${SUDO_USER:-}" ]; then
        sudo -u "${SUDO_USER}" git pull --rebase
    else
        git pull --rebase
    fi
else
    echo "[1/4] Skipping git pull (not a git repository)."
fi

# Rebuild frontend and binary
echo "[2/4] Rebuilding application..."
if [ -n "${SUDO_USER:-}" ]; then
    sudo -u "${SUDO_USER}" bash "${ROOT_DIR}/scripts/build.sh"
else
    bash "${ROOT_DIR}/scripts/build.sh"
fi

# If installed system-wide, update the system binary and restart service
INSTALL_BIN="/usr/local/bin/bedrock-server-manager"

if [ -f "${INSTALL_BIN}" ]; then
    echo "[3/4] Updating system binary at ${INSTALL_BIN}..."
    if [ "$EUID" -ne 0 ]; then
        echo "[INFO] Escalating to sudo to update system-wide installation..."
        exec sudo bash "$0" "$@"
    fi

    # Check if systemd service is active
    WAS_ACTIVE=false
    if command -v systemctl >/dev/null 2>&1 && systemctl is-active --quiet bedrock-server-manager.service 2>/dev/null; then
        WAS_ACTIVE=true
        echo "  -> Stopping bedrock-server-manager service..."
        systemctl stop bedrock-server-manager.service
    fi

    install -m 755 "${ROOT_DIR}/bin/bedrock-server-manager" "${INSTALL_BIN}"

    echo "[4/4] Restarting service..."
    if [ "$WAS_ACTIVE" = true ]; then
        systemctl start bedrock-server-manager.service
        echo "  -> Service restarted successfully."
    fi
    echo "=========================================================="
    echo "Bedrock Server Manager updated successfully!"
    echo "Service Status: sudo systemctl status bedrock-server-manager"
    echo "=========================================================="
else
    echo "[3/4] No system-wide installation found at ${INSTALL_BIN}."
    echo "[4/4] Local binary updated at ${ROOT_DIR}/bin/bedrock-server-manager."
    echo "=========================================================="
    echo "Update complete! You can run the binary directly:"
    echo "  ${ROOT_DIR}/bin/bedrock-server-manager"
    echo "Or install it as a systemd service by running:"
    echo "  sudo ./install.sh"
    echo "=========================================================="
fi
