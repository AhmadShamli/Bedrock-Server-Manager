#!/usr/bin/env bash
set -euo pipefail

# Determine repository root directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"

echo "=========================================================="
echo "    Bedrock Server Manager (BSM) - Build Script"
echo "=========================================================="

# 1. Dependency Checks
command -v node >/dev/null 2>&1 || { echo "[ERROR] 'node' is required but not installed. Please install Node.js (v20+)." >&2; exit 1; }
command -v npm >/dev/null 2>&1 || { echo "[ERROR] 'npm' is required but not installed." >&2; exit 1; }
command -v go >/dev/null 2>&1 || { echo "[ERROR] 'go' is required but not installed. Please install Go (v1.22+)." >&2; exit 1; }

echo "[1/3] Checking dependencies..."
echo "  - Node: $(node -v)"
echo "  - npm:  $(npm -v)"
echo "  - Go:   $(go version | awk '{print $3}')"

# 2. Build Web Frontend
echo "[2/3] Building React SPA frontend..."
cd "${ROOT_DIR}/web"

if [ ! -d "node_modules" ]; then
    echo "  -> Installing npm dependencies..."
    npm ci || npm install
fi

npm run build
cd "${ROOT_DIR}"

# 3. Build Go Static Binary
echo "[3/3] Compiling standalone Go daemon..."
mkdir -p "${ROOT_DIR}/bin"
CGO_ENABLED=0 go build \
    -ldflags="-s -w" \
    -o "${ROOT_DIR}/bin/bedrock-server-manager" \
    "${ROOT_DIR}/cmd/manager"

chmod +x "${ROOT_DIR}/bin/bedrock-server-manager"

BINARY_SIZE=$(du -h "${ROOT_DIR}/bin/bedrock-server-manager" | cut -f1)
echo "=========================================================="
echo "Build Successful!"
echo "Binary created at: ${ROOT_DIR}/bin/bedrock-server-manager (${BINARY_SIZE})"
echo "Run standalone:    ./bin/bedrock-server-manager"
echo "=========================================================="
