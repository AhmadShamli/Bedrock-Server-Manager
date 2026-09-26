#!/usr/bin/env bash
set -euo pipefail

# Bedrock Server Manager (BSM) Release Packaging Script
# Builds multi-arch standalone binaries and archives

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"

VERSION="${1:-${VERSION:-1.1.0}}"
DIST_DIR="${ROOT_DIR}/dist"
LDFLAGS="-s -w -X github.com/AhmadShamli/Bedrock-Server-Manager/internal/version.Version=${VERSION#v}"

echo "=========================================================="
echo "    Bedrock Server Manager - Release Packaging (v${VERSION#v})"
echo "=========================================================="

# Build web frontend first
echo "==> Building embedded React web assets..."
(
    cd "${ROOT_DIR}/web"
    if [ ! -d "node_modules" ]; then
        npm ci || npm install
    fi
    npm run build
)

rm -rf "${DIST_DIR}"
mkdir -p "${DIST_DIR}"

PLATFORMS=(
  "linux/amd64"
  "linux/arm64"
  "linux/arm/7"
)

for PLATFORM in "${PLATFORMS[@]}"; do
  IFS="/" read -r OS ARCH EXTRA <<< "${PLATFORM}"
  
  TARGET_NAME="bedrock-server-manager-v${VERSION#v}-${OS}-${ARCH}"
  if [ -n "${EXTRA:-}" ]; then
    TARGET_NAME="bedrock-server-manager-v${VERSION#v}-${OS}-${ARCH}v${EXTRA}"
  fi
  
  echo "==> Building ${TARGET_NAME}..."
  BUILD_DIR="${DIST_DIR}/${TARGET_NAME}"
  mkdir -p "${BUILD_DIR}"
  
  BINARY_NAME="bedrock-server-manager"
  if [ "${OS}" = "windows" ]; then
    BINARY_NAME="bedrock-server-manager.exe"
  fi
  
  export CGO_ENABLED=0
  export GOOS="${OS}"
  export GOARCH="${ARCH}"
  if [ -n "${EXTRA:-}" ]; then
    export GOARM="${EXTRA}"
  else
    unset GOARM || true
  fi
  
  (
    cd "${ROOT_DIR}"
    go build -trimpath -ldflags="${LDFLAGS}" -o "${BUILD_DIR}/${BINARY_NAME}" ./cmd/manager
  )
  
  cp "${ROOT_DIR}/README.md" "${BUILD_DIR}/"
  cp "${ROOT_DIR}/.env.example" "${BUILD_DIR}/"
  if [ -f "${ROOT_DIR}/LICENSE" ]; then
    cp "${ROOT_DIR}/LICENSE" "${BUILD_DIR}/"
  fi
  
  (
    cd "${DIST_DIR}"
    if [ "${OS}" = "windows" ]; then
      zip -q -r "${TARGET_NAME}.zip" "${TARGET_NAME}"
    else
      tar -czf "${TARGET_NAME}.tar.gz" "${TARGET_NAME}"
    fi
    rm -rf "${TARGET_NAME}"
  )
done

(
  cd "${DIST_DIR}"
  sha256sum bedrock-server-manager-*.tar.gz > checksums.txt 2>/dev/null || true
)

echo "==> Build completed successfully! Generated release artifacts in ${DIST_DIR}:"
ls -lh "${DIST_DIR}"
