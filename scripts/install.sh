#!/usr/bin/env bash
set -euo pipefail

# Redirect to comprehensive deploy/install.sh
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"

exec bash "${ROOT_DIR}/deploy/install.sh" "$@"
