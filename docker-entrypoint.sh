#!/usr/bin/env bash
set -euo pipefail

# Bedrock Server Manager Docker Entrypoint
DATA_DIR="${DATA_DIR:-/data}"

mkdir -p "${DATA_DIR}" "${DATA_DIR}/servers" "${DATA_DIR}/backups"

exec "$@"
