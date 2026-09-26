#!/usr/bin/env bash
set -euo pipefail

# Bedrock Server Manager (BSM) Native Bare-Metal Linux Installer & Upgrade Assistant
# Supported: Debian/Ubuntu, RHEL/Rocky/Fedora, Arch Linux, Alpine

REPO="AhmadShamli/Bedrock-Server-Manager"
DEFAULT_FALLBACK_TAG="v1.2.1"

# Ensure standard binary directories are in PATH (important under sudo/secure_path)
for extra_path in /usr/local/go/bin /usr/local/bin /usr/bin; do
    if [ -d "${extra_path}" ] && [[ ":$PATH:" != *":${extra_path}:"* ]]; then
        export PATH="${extra_path}:$PATH"
    fi
done

# Shell color helpers (only if attached to terminal)
if [ -t 1 ]; then
    BOLD="\033[1m"
    GREEN="\033[0;32m"
    YELLOW="\033[0;33m"
    RED="\033[0;31m"
    CYAN="\033[0;36m"
    EMERALD="\033[38;5;48m"
    RESET="\033[0m"
else
    BOLD=""
    GREEN=""
    YELLOW=""
    RED=""
    CYAN=""
    EMERALD=""
    RESET=""
fi

log_info()    { echo -e "${CYAN}[INFO]${RESET} $*"; }
log_success() { echo -e "${GREEN}[OK]${RESET}   $*"; }
log_warn()    { echo -e "${YELLOW}[WARN]${RESET} $*"; }
log_error()   { echo -e "${RED}[ERROR]${RESET} $*" >&2; }

# Helper: Print reusable ASCII header and banner
print_ascii_banner() {
    local subtitle="${1:-}"
    echo -e "${EMERALD}${BOLD}"
    cat <<'EOF'
 ███████████   █████████  ██████   ██████
░░███░░░░░███ ███░░░░░███░░██████ ██████ 
 ░███    ░███░███    ░░░  ░███░█████░███ 
 ░██████████ ░░█████████  ░███░░███ ░███ 
 ░███░░░░░███ ░░░░░░░░███ ░███ ░░░  ░███ 
 ░███    ░███ ███    ░███ ░███      ░███ 
 ███████████ ░░█████████  █████     █████
░░░░░░░░░░░   ░░░░░░░░░  ░░░░░     ░░░░░ 
EOF
    echo -e -n "${RESET}"
    if [ -n "${subtitle}" ]; then
        echo "================================================================================"
        local title="Bedrock Server Manager - ${subtitle}"
        local pad=$(( (80 - ${#title}) / 2 ))
        if [ "${pad}" -lt 0 ]; then pad=0; fi
        printf "%*s%s\n" "${pad}" "" "${title}"
        echo "================================================================================"
    fi
}

print_help() {
    print_ascii_banner "Bare-Metal Linux Installer & Upgrade Assistant"
    echo ""
    cat <<EOF
Usage:
  sudo bash install.sh [OPTIONS]
  # or: sudo ./install.sh [OPTIONS]

Options:
  -u, --upgrade            Run in upgrade mode (automatically detected if BSM is already installed)
  -v, --version <tag>      Specify target release version (e.g. v1.2.1 or latest)
  -m, --method <method>    Installation method: 'download' (GitHub release), 'build' (compile from source), or 'local'
  -y, --yes                Automatic yes to confirmation prompt (run non-interactively)
  --check                  Check currently installed version against latest release without upgrading
  --skip-backup            Skip pre-upgrade database and configuration backup
  -f, --force              Force reinstall/upgrade without confirmation prompt
  -h, --help               Show this help message

Environment Variables:
  BSM_VERSION              Target release version
  BSM_INSTALL_METHOD       'download', 'build', or 'local'
  BSM_ASSUME_YES           Set to 'true' to run non-interactively
  BSM_SKIP_BACKUP          Set to 'true' to skip pre-upgrade backups

Examples:
  sudo ./install.sh                                 # Interactive install or upgrade with diagnostics
  sudo ./install.sh -y                              # Non-interactive automated install/upgrade
  sudo ./install.sh --upgrade                       # Explicit upgrade with automated backup
  sudo ./install.sh -v v1.2.1                       # Upgrade or install specific version
  sudo ./install.sh --check                         # Check for available updates
  sudo ./install.sh -m build                        # Compile latest binary from local source
EOF
}

# Resolve location of this script
if [ -n "${BASH_SOURCE[0]:-}" ] && [ -f "${BASH_SOURCE[0]}" ]; then
    SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
else
    SCRIPT_DIR="$(pwd)"
fi

# Locate repository root relative to script location
if [ -f "${SCRIPT_DIR}/cmd/manager/main.go" ]; then
    REPO_ROOT="${SCRIPT_DIR}"
elif [ -f "${SCRIPT_DIR}/../cmd/manager/main.go" ]; then
    REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
elif [ -f "./cmd/manager/main.go" ]; then
    REPO_ROOT="$(pwd)"
else
    REPO_ROOT="${SCRIPT_DIR}"
fi

# Parse Command-line Options
TARGET_VERSION="${BSM_VERSION:-${VERSION:-}}"
INSTALL_METHOD="${BSM_INSTALL_METHOD:-${INSTALL_METHOD:-}}"
ASSUME_YES="${BSM_ASSUME_YES:-false}"
FORCE_UPGRADE=false
FORCE_ACTION=false
SKIP_BACKUP="${BSM_SKIP_BACKUP:-false}"
CHECK_ONLY=false

while [ $# -gt 0 ]; do
    case "$1" in
        -u|--upgrade)
            FORCE_UPGRADE=true
            shift
            ;;
        -y|--yes|--assume-yes|--non-interactive)
            ASSUME_YES=true
            shift
            ;;
        -v|--version)
            if [ -z "${2:-}" ]; then
                log_error "Option '$1' requires a version argument (e.g. v1.0.0)."
                exit 1
            fi
            TARGET_VERSION="$2"
            shift 2
            ;;
        -m|--method)
            if [ -z "${2:-}" ]; then
                log_error "Option '$1' requires a method argument ('download', 'build', or 'local')."
                exit 1
            fi
            INSTALL_METHOD="$2"
            shift 2
            ;;
        --skip-backup)
            SKIP_BACKUP=true
            shift
            ;;
        --check)
            CHECK_ONLY=true
            shift
            ;;
        -f|--force)
            FORCE_ACTION=true
            shift
            ;;
        -h|--help)
            print_help
            exit 0
            ;;
        *)
            log_error "Unknown option: $1"
            print_help
            exit 1
            ;;
    esac
done

# Helper: Detect active systemd init system
is_systemd_active() {
    if command -v systemctl >/dev/null 2>&1; then
        if [ -d /run/systemd/system ] && systemctl list-units >/dev/null 2>&1; then
            return 0
        fi
    fi
    return 1
}

# Helper: Query latest release tag from GitHub
get_latest_release_tag() {
    local tag=""
    if command -v curl >/dev/null 2>&1; then
        tag="$(curl -fsSL --connect-timeout 2 -m 3 "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/' || true)"
        if [ -z "${tag}" ]; then
            local effective_url
            effective_url="$(curl -sIL --connect-timeout 2 -m 3 -o /dev/null -w '%{url_effective}' "https://github.com/${REPO}/releases/latest" 2>/dev/null || true)"
            local base
            base="$(basename "${effective_url}")"
            if [ -n "${base}" ] && [ "${base}" != "latest" ] && [ "${base}" != "releases" ]; then
                tag="${base}"
            fi
        fi
    fi
    if [ -z "${tag}" ]; then
        tag="${DEFAULT_FALLBACK_TAG}"
    fi
    if [[ "${tag}" =~ ^[0-9]+\.[0-9]+ ]]; then
        tag="v${tag}"
    fi
    echo "${tag}"
}

# Detect existing installation status
IS_INSTALLED=false
CURRENT_VERSION="unknown"

if [ -f /usr/local/bin/bedrock-server-manager ] || [ -f /etc/bedrock-server-manager/bsm.env ] || [ -f /etc/systemd/system/bedrock-server-manager.service ]; then
    IS_INSTALLED=true
fi

if [ -x /usr/local/bin/bedrock-server-manager ]; then
    DETECTED_VER="$(timeout 3 /usr/local/bin/bedrock-server-manager version 2>/dev/null | grep -oE 'v?[0-9]+\.[0-9]+(\.[0-9]+)?(-[a-zA-Z0-9.]+)?' | head -n1 || true)"
    if [ -n "${DETECTED_VER}" ]; then
        CURRENT_VERSION="${DETECTED_VER}"
    fi
fi

if [ "${FORCE_UPGRADE}" = true ] || [ "${IS_INSTALLED}" = true ]; then
    MODE="upgrade"
else
    MODE="install"
fi

# Detect host architecture
ARCH_RAW="$(uname -m)"
case "${ARCH_RAW}" in
    x86_64|amd64)
        ARCH="amd64"
        ;;
    aarch64|arm64)
        ARCH="arm64"
        ;;
    armv7*|armhf)
        ARCH="armv7"
        ;;
    *)
        ARCH=""
        ;;
esac

# Helper: Resolve target/new version to be installed
resolve_target_version() {
    local target="${TARGET_VERSION:-}"
    if [ -n "${target}" ] && [ "${target}" != "latest" ]; then
        if [[ "${target}" =~ ^[0-9]+\.[0-9]+ ]]; then
            target="v${target}"
        fi
        echo "${target}"
        return 0
    fi

    # 1. Check local pre-compiled binary if bin/bedrock-server-manager exists
    local bin_source="${REPO_ROOT}/bin/bedrock-server-manager"
    if [ -f "${bin_source}" ] && [ -x "${bin_source}" ]; then
        local bin_ver
        bin_ver="$(timeout 3 "${bin_source}" version 2>/dev/null | grep -oE 'v?[0-9]+\.[0-9]+(\.[0-9]+)?(-[a-zA-Z0-9.]+)?' | head -n1 || true)"
        if [ -n "${bin_ver}" ]; then
            if [[ "${bin_ver}" =~ ^[0-9]+\.[0-9]+ ]]; then
                bin_ver="v${bin_ver}"
            fi
            echo "${bin_ver}"
            return 0
        fi
    fi

    # 2. Check bundled release archive in dist/
    if [ -n "${ARCH:-}" ]; then
        local dist_match
        dist_match="$(ls -1 "${REPO_ROOT}/dist/"*"-linux-${ARCH}.tar.gz" 2>/dev/null | sort -V | tail -n1 || true)"
        if [ -n "${dist_match}" ] && [ -f "${dist_match}" ]; then
            local dist_ver
            dist_ver="$(echo "${dist_match##*/}" | grep -oE 'v?[0-9]+\.[0-9]+(\.[0-9]+)?(-[a-zA-Z0-9.]+)?' | head -n1 || true)"
            if [ -n "${dist_ver}" ]; then
                if [[ "${dist_ver}" =~ ^[0-9]+\.[0-9]+ ]]; then
                    dist_ver="v${dist_ver}"
                fi
                echo "${dist_ver}"
                return 0
            fi
        fi
    fi

    # 3. Check internal/version/version.go if in source checkout
    if [ -f "${REPO_ROOT}/internal/version/version.go" ]; then
        local src_ver
        src_ver="$(grep -E '^\s*var\s+Version\s*=' "${REPO_ROOT}/internal/version/version.go" 2>/dev/null | sed -E 's/.*"([^"]+)".*/\1/' || true)"
        if [ -n "${src_ver}" ]; then
            if [[ "${src_ver}" =~ ^[0-9]+\.[0-9]+ ]]; then
                src_ver="v${src_ver}"
            fi
            echo "${src_ver}"
            return 0
        fi
    fi

    # 4. Check GitHub latest release tag
    local latest
    latest="$(get_latest_release_tag)"
    if [ -n "${latest}" ]; then
        echo "${latest}"
        return 0
    fi

    echo "${DEFAULT_FALLBACK_TAG}"
}

TARGET_VERSION="$(resolve_target_version)"

# Handle --check flag
if [ "${CHECK_ONLY}" = true ]; then
    LATEST_TAG="$(get_latest_release_tag)"
    print_ascii_banner "Version Check"
    echo "Installed version       : ${CURRENT_VERSION}"
    echo "Latest release          : ${LATEST_TAG}"
    echo "Target for install      : ${TARGET_VERSION}"
    echo "--------------------------------------------------------------------------------"
    if [ "${CURRENT_VERSION}" != "unknown" ] && [ "${CURRENT_VERSION}" = "${LATEST_TAG}" ]; then
        log_success "Bedrock Server Manager is already at the latest release (${LATEST_TAG})."
    else
        log_info "A different or newer release is available (${LATEST_TAG})."
        echo "To upgrade, run:"
        echo "  sudo ./install.sh --upgrade"
    fi
    echo "================================================================================"
    exit 0
fi

# Root privilege validation & automatic escalation
if [ "$(id -u)" -ne 0 ]; then
    if command -v sudo >/dev/null 2>&1; then
        log_info "Root privileges required for installation/upgrade. Escalating with sudo..."
        exec sudo bash "$0" "$@"
    else
        log_error "Installation and upgrade operations must be run as root (e.g. sudo ./install.sh)."
        exit 1
    fi
fi

# Detect Systemd Status and active state
SYSTEMD_ACTIVE=false
SERVICE_WAS_ACTIVE=false

if is_systemd_active; then
    SYSTEMD_ACTIVE=true
    if systemctl is-active --quiet bedrock-server-manager.service 2>/dev/null; then
        SERVICE_WAS_ACTIVE=true
    fi
fi

# -----------------------------------------------------------------------------
# Host Diagnostics & Environment Collection
# -----------------------------------------------------------------------------
# 1. Operating System & Kernel
HOST_OS="Linux"
if [ -f /etc/os-release ]; then
    HOST_OS="$(grep -E '^PRETTY_NAME=' /etc/os-release 2>/dev/null | cut -d= -f2- | tr -d '"')"
fi
HOST_KERNEL="$(uname -sr 2>/dev/null || uname -r)"

# 2. Hardware: CPU, Memory, Disk
HOST_CPU_COUNT="$(nproc 2>/dev/null || grep -c ^processor /proc/cpuinfo 2>/dev/null || echo 1)"
HOST_CPU_MODEL="$(grep -m1 "model name" /proc/cpuinfo 2>/dev/null | cut -d: -f2- | sed -E 's/^[ \t]+//' || echo "")"
if [ -n "${HOST_CPU_MODEL}" ]; then
    HOST_CPU_STR="${HOST_CPU_COUNT} Cores (${HOST_CPU_MODEL})"
else
    HOST_CPU_STR="${HOST_CPU_COUNT} Cores"
fi

if [ -f /proc/meminfo ]; then
    MEM_TOTAL_KB="$(grep -i MemTotal /proc/meminfo 2>/dev/null | awk '{print $2}')"
    MEM_AVAIL_KB="$(grep -i MemAvailable /proc/meminfo 2>/dev/null | awk '{print $2}')"
    if [ -n "${MEM_TOTAL_KB}" ] && [ -n "${MEM_AVAIL_KB}" ]; then
        MEM_TOTAL_MB=$((MEM_TOTAL_KB / 1024))
        MEM_AVAIL_MB=$((MEM_AVAIL_KB / 1024))
        HOST_MEM_STR="${MEM_AVAIL_MB} MB free / ${MEM_TOTAL_MB} MB total"
    else
        HOST_MEM_STR="$(free -h 2>/dev/null | awk '/^Mem:/ {print $7 " free / " $2 " total"}' || echo 'N/A')"
    fi
else
    HOST_MEM_STR="$(free -h 2>/dev/null | awk '/^Mem:/ {print $7 " free / " $2 " total"}' || echo 'N/A')"
fi

DISK_TARGET="/var/lib/bedrock-server-manager"
if [ ! -d "${DISK_TARGET}" ]; then
    DISK_TARGET="/"
fi
HOST_DISK_STR="$(df -h "${DISK_TARGET}" 2>/dev/null | awk 'NR==2 {print $4 " available (" $5 " used) on " $6}' || echo 'N/A')"

# 3. Container Engine (Docker)
DOCKER_STATUS="Not Detected"
if command -v docker >/dev/null 2>&1; then
    DOCKER_VER="$(docker --version 2>/dev/null | sed -E 's/, build [a-f0-9]+//' || echo 'Installed')"
    if [ -S /var/run/docker.sock ]; then
        if docker info >/dev/null 2>&1; then
            DOCKER_STATUS="Active & Accessible (${DOCKER_VER})"
        else
            DOCKER_STATUS="Socket Present (${DOCKER_VER})"
        fi
    elif systemctl is-active --quiet docker 2>/dev/null; then
        DOCKER_STATUS="Active (${DOCKER_VER})"
    else
        DOCKER_STATUS="Daemon Inactive (${DOCKER_VER})"
    fi
fi

# 4. Existing BSM Installation Status
if [ "${IS_INSTALLED}" = true ]; then
    if systemctl is-active --quiet bedrock-server-manager.service 2>/dev/null; then
        BSM_STATUS="${CURRENT_VERSION} (Service: Active / Running)"
    elif [ -f /etc/systemd/system/bedrock-server-manager.service ]; then
        BSM_STATUS="${CURRENT_VERSION} (Service: Inactive / Stopped)"
    else
        BSM_STATUS="${CURRENT_VERSION} (Binary Deployed)"
    fi
else
    BSM_STATUS="Not Installed"
fi

# 5. Host Firewall Subsystems
UFW_STATUS="Not Installed"
UFW_ACTIVE=false
if command -v ufw >/dev/null 2>&1 || [ -x /usr/sbin/ufw ]; then
    UFW_BIN="$(command -v ufw 2>/dev/null || echo '/usr/sbin/ufw')"
    UFW_OUT="$("${UFW_BIN}" status 2>&1 || true)"
    if echo "${UFW_OUT}" | grep -qi "status: active"; then
        UFW_STATUS="Active"
        UFW_ACTIVE=true
    elif echo "${UFW_OUT}" | grep -qi "status: inactive"; then
        UFW_STATUS="Installed (Inactive)"
    elif echo "${UFW_OUT}" | grep -qi "you need to be root"; then
        UFW_STATUS="Installed (Root check required)"
    else
        UFW_STATUS="Installed"
    fi
fi

IPTABLES_STATUS="Not Installed"
IPTABLES_AVAIL=false
if command -v iptables >/dev/null 2>&1 || [ -x /usr/sbin/iptables ]; then
    IPT_BIN="$(command -v iptables 2>/dev/null || echo '/usr/sbin/iptables')"
    IPT_VER="$("${IPT_BIN}" --version 2>&1 | head -n1 || echo 'Available')"
    IPTABLES_STATUS="${IPT_VER}"
    IPTABLES_AVAIL=true
fi

FIREWALLD_STATUS="Not Installed"
if command -v firewall-cmd >/dev/null 2>&1; then
    if systemctl is-active --quiet firewalld 2>/dev/null; then
        FIREWALLD_STATUS="Active"
    else
        FIREWALLD_STATUS="Installed (Inactive)"
    fi
fi

# Determine default firewall driver that BSM will select
if [ "${UFW_ACTIVE}" = true ]; then
    RECOMMENDED_FW="UFW Driver (Default & Active)"
elif [ "${UFW_STATUS}" = "Installed (Inactive)" ]; then
    RECOMMENDED_FW="IPTables Driver (Note: UFW is inactive; run 'ufw enable' to switch)"
elif [ "${IPTABLES_AVAIL}" = true ]; then
    RECOMMENDED_FW="IPTables Driver (dedicated BSM_PORT_GATE chain)"
else
    RECOMMENDED_FW="Mock Driver (Simulated firewall)"
fi

# -----------------------------------------------------------------------------
# Display Host Summary & Diagnostics
# -----------------------------------------------------------------------------
if [ "${MODE}" = "upgrade" ]; then
    print_ascii_banner "Bare-Metal Linux Upgrade"
else
    print_ascii_banner "Bare-Metal Linux Installer"
fi

echo "================================================================================"
echo -e "                   ${BOLD}Host Diagnostics & Environment Summary${RESET}"
echo "================================================================================"
echo -e "  Operating System      : ${HOST_OS} (${ARCH_RAW})"
echo -e "  Linux Kernel          : ${HOST_KERNEL}"
echo -e "  CPU Architecture      : ${HOST_CPU_STR}"
echo -e "  System Memory         : ${HOST_MEM_STR}"
echo -e "  Storage Space         : ${HOST_DISK_STR}"
echo "--------------------------------------------------------------------------------"
echo -e "  Container Engine      : ${DOCKER_STATUS}"
echo -e "  Existing BSM Status   : ${BSM_STATUS}"
echo -e "  Target BSM Version    : ${TARGET_VERSION}"
echo -e "  Operation Mode        : $([ "${MODE}" = "upgrade" ] && echo "Upgrade" || echo "Fresh Installation")"
echo "--------------------------------------------------------------------------------"
echo -e "  Host Firewalls        :"
echo -e "    - UFW               : ${UFW_STATUS}"
echo -e "    - IPTables          : ${IPTABLES_STATUS}"
echo -e "    - Firewalld         : ${FIREWALLD_STATUS}"
echo -e "    - Selected Driver   : ${RECOMMENDED_FW}"
echo "================================================================================"

# -----------------------------------------------------------------------------
# Interactive Confirmation Prompt (Defaults to Asking)
# -----------------------------------------------------------------------------
if [ "${ASSUME_YES}" = false ] && [ "${FORCE_ACTION}" = false ]; then
    echo ""
    if [ "${MODE}" = "upgrade" ]; then
        PROMPT_TEXT="Proceed with Upgrade to ${TARGET_VERSION}? [Y/n] "
    else
        PROMPT_TEXT="Proceed with Fresh Installation of ${TARGET_VERSION}? [Y/n] "
    fi

    CONFIRM_REPLY=""
    if [ -t 0 ]; then
        read -r -p "${PROMPT_TEXT}" CONFIRM_REPLY
    elif read -r -t 1 CONFIRM_REPLY 2>/dev/null; then
        :
    elif [ -r /dev/tty ]; then
        read -r -p "${PROMPT_TEXT}" CONFIRM_REPLY < /dev/tty
    else
        log_warn "Non-interactive shell detected and no confirmation response provided."
        log_warn "Run with -y or --yes to proceed automatically (e.g. sudo ./install.sh -y)."
        exit 1
    fi

    CONFIRM_REPLY="$(echo "${CONFIRM_REPLY}" | tr '[:upper:]' '[:lower:]')"
    if [ -n "${CONFIRM_REPLY}" ] && [ "${CONFIRM_REPLY}" != "y" ] && [ "${CONFIRM_REPLY}" != "yes" ]; then
        echo ""
        log_info "Operation cancelled by user. No changes were made to your system."
        exit 0
    fi
    echo ""
    log_info "Confirmation received. Proceeding with $([ "${MODE}" = "upgrade" ] && echo "upgrade" || echo "installation")..."
    echo ""
fi

# Setup staging workspace with cleanup trap
STAGING_DIR="$(mktemp -d /tmp/bsm-staging.XXXXXX)"
cleanup() {
    rm -rf "${STAGING_DIR}"
}
trap cleanup EXIT INT TERM

# -----------------------------------------------------------------------------
# 1. Detect Distribution & Verify Prerequisites
# -----------------------------------------------------------------------------
echo "[1/6] Verifying host prerequisites..."
MISSING_PKGS=()

if ! command -v curl >/dev/null 2>&1; then
    MISSING_PKGS+=("curl")
fi
if ! command -v tar >/dev/null 2>&1; then
    MISSING_PKGS+=("tar")
fi
if ! command -v sudo >/dev/null 2>&1; then
    MISSING_PKGS+=("sudo")
fi
if ! command -v nsenter >/dev/null 2>&1; then
    MISSING_PKGS+=("util-linux")
fi

# Check firewall tools (ufw or iptables)
if ! command -v ufw >/dev/null 2>&1 && ! command -v iptables >/dev/null 2>&1; then
    MISSING_PKGS+=("ufw")
fi

if [ ${#MISSING_PKGS[@]} -eq 0 ]; then
    log_success "Host prerequisites verified (curl, tar, sudo, util-linux, firewall)."
else
    log_info "Missing prerequisites: ${MISSING_PKGS[*]}. Installing via package manager..."
    if command -v apt-get >/dev/null 2>&1; then
        apt-get update -qq
        apt-get install -y -qq "${MISSING_PKGS[@]}"
    elif command -v dnf >/dev/null 2>&1; then
        dnf install -y -q "${MISSING_PKGS[@]}"
    elif command -v pacman >/dev/null 2>&1; then
        pacman -Sy --noconfirm "${MISSING_PKGS[@]}"
    elif command -v apk >/dev/null 2>&1; then
        apk add --no-cache "${MISSING_PKGS[@]}"
    else
        log_warn "Unrecognized package manager. Ensure ${MISSING_PKGS[*]} are installed manually."
    fi
fi

# Check Docker daemon status
if ! command -v docker >/dev/null 2>&1; then
    log_warn "Docker CLI is not detected on this host."
    log_warn "BSM orchestrates Bedrock instances via Docker. Please install Docker Engine."
    log_warn "BSM will start in simulated fallback mode until Docker is available."
elif [ ! -S /var/run/docker.sock ]; then
    log_warn "Docker daemon socket (/var/run/docker.sock) is not accessible."
    log_warn "Please ensure Docker service is started (e.g. sudo systemctl start docker)."
fi

# -----------------------------------------------------------------------------
# 2. Configure Dedicated System User & Directories
# -----------------------------------------------------------------------------
echo "[2/6] Configuring dedicated system user 'bedrock' and directories..."
if ! getent group bedrock >/dev/null 2>&1; then
    groupadd --system bedrock
fi

if ! id -u bedrock >/dev/null 2>&1; then
    useradd --system --gid bedrock --home-dir /var/lib/bedrock-server-manager --shell /usr/sbin/nologin bedrock
fi

# Add bedrock user to docker group if docker group exists
if getent group docker >/dev/null 2>&1; then
    usermod -aG docker bedrock
    log_success "Added 'bedrock' user to 'docker' group for BDS container orchestration."
fi

TARGET_DATA_DIR="/var/lib/bedrock-server-manager/data"
if [ -f /etc/bedrock-server-manager/bsm.env ]; then
    CUSTOM_DIR="$(grep -E '^\s*DATA_DIR=' /etc/bedrock-server-manager/bsm.env | cut -d= -f2- | tr -d '"' | tr -d "'" | sed -E 's/^[ \t]+//;s/[ \t]+$//' || true)"
    if [ -n "${CUSTOM_DIR}" ]; then
        TARGET_DATA_DIR="${CUSTOM_DIR}"
    fi
fi

mkdir -p /var/lib/bedrock-server-manager /run/bedrock-server-manager /etc/bedrock-server-manager \
         "${TARGET_DATA_DIR}" "${TARGET_DATA_DIR}/servers" "${TARGET_DATA_DIR}/backups"

chown -R bedrock:bedrock /var/lib/bedrock-server-manager /run/bedrock-server-manager "${TARGET_DATA_DIR}"
chmod 0750 /var/lib/bedrock-server-manager /run/bedrock-server-manager "${TARGET_DATA_DIR}"
chmod 0700 "${TARGET_DATA_DIR}/backups"

# -----------------------------------------------------------------------------
# 3. Pre-Upgrade Backup (if upgrading)
# -----------------------------------------------------------------------------
BACKUP_DIR=""
if [ "${MODE}" = "upgrade" ]; then
    if [ "${SKIP_BACKUP}" = true ]; then
        log_info "Skipping pre-upgrade backup as requested (--skip-backup)."
    else
        BACKUP_TIMESTAMP="$(date +%Y%m%d_%H%M%S)"
        BACKUP_DIR="/var/lib/bedrock-server-manager/backups/upgrade-${BACKUP_TIMESTAMP}"
        mkdir -p "${BACKUP_DIR}"
        chmod 0700 "${BACKUP_DIR}"
        chown bedrock:bedrock "${BACKUP_DIR}"

        BACKED_UP_ANY=false
        # Backup database files safely
        if [ -d /var/lib/bedrock-server-manager/data ]; then
            cp -p /var/lib/bedrock-server-manager/data/manager.db* "${BACKUP_DIR}/" 2>/dev/null || true
            cp -p /var/lib/bedrock-server-manager/data/metrics.db* "${BACKUP_DIR}/" 2>/dev/null || true
            BACKED_UP_ANY=true
        fi
        # Backup environment configuration
        if [ -f /etc/bedrock-server-manager/bsm.env ]; then
            cp -p /etc/bedrock-server-manager/bsm.env "${BACKUP_DIR}/bsm.env"
            chmod 0600 "${BACKUP_DIR}/bsm.env"
            BACKED_UP_ANY=true
        fi
        # Backup currently installed binary for rollback capability
        if [ -f /usr/local/bin/bedrock-server-manager ]; then
            cp -p /usr/local/bin/bedrock-server-manager "${BACKUP_DIR}/bedrock-server-manager.previous"
            chmod 0755 "${BACKUP_DIR}/bedrock-server-manager.previous"
            BACKED_UP_ANY=true
        fi

        if [ "${BACKED_UP_ANY}" = true ]; then
            log_success "Pre-upgrade backup successfully created at ${BACKUP_DIR}"
            # Retain the 5 most recent upgrade backups
            (ls -1dt /var/lib/bedrock-server-manager/backups/upgrade-* 2>/dev/null | tail -n +6 | xargs -r rm -rf 2>/dev/null || true)
        fi
    fi
fi

# -----------------------------------------------------------------------------
# 4. Acquire and Validate Candidate Binary
# -----------------------------------------------------------------------------
echo "[3/6] Acquiring and verifying Bedrock Server Manager static binary (${TARGET_VERSION})..."
BIN_SOURCE="${REPO_ROOT}/bin/bedrock-server-manager"

if [ -z "${INSTALL_METHOD}" ]; then
    if [ -f "${BIN_SOURCE}" ]; then
        INSTALL_METHOD="local"
    elif [ -f "${REPO_ROOT}/cmd/manager/main.go" ]; then
        INSTALL_METHOD="build"
    elif [ -t 0 ]; then
        echo ""
        if [ "${MODE}" = "upgrade" ]; then
            echo "Select upgrade source:"
        else
            echo "Select installation method:"
        fi
        echo "  1) Compile and build static binary from source code [Default]"
        echo "  2) Download pre-compiled release from GitHub (or use bundled dist archive)"
        read -r -p "Enter choice [1 or 2] (default: 1): " USER_CHOICE
        case "${USER_CHOICE}" in
            2|"download")
                INSTALL_METHOD="download"
                ;;
            *)
                INSTALL_METHOD="build"
                ;;
        esac
    else
        INSTALL_METHOD="build"
    fi
fi

BINARY_STAGED=false

# Method A: Local pre-compiled binary in bin/
if [ "${INSTALL_METHOD}" = "local" ] && [ -f "${BIN_SOURCE}" ]; then
    log_info "Using pre-compiled binary found at ${BIN_SOURCE}..."
    cp "${BIN_SOURCE}" "${STAGING_DIR}/bedrock-server-manager"
    BINARY_STAGED=true
fi

# Method B: Download release archive or bundled dist
if [ "${BINARY_STAGED}" = false ] && [ "${INSTALL_METHOD}" = "download" ]; then
    # Check bundled dist archive first
    if [ -n "${ARCH}" ]; then
        DIST_MATCH="$(ls -1 "${REPO_ROOT}/dist/"*"-linux-${ARCH}.tar.gz" 2>/dev/null | sort -V | tail -n1 || true)"
        if [ -n "${DIST_MATCH}" ] && [ -f "${DIST_MATCH}" ]; then
            log_info "Found bundled release archive in dist/ (${DIST_MATCH##*/}). Extracting..."
            if tar -xzf "${DIST_MATCH}" -C "${STAGING_DIR}" 2>/dev/null; then
                FOUND_BIN="$(find "${STAGING_DIR}" -type f -name bedrock-server-manager | head -n 1)"
                if [ -n "${FOUND_BIN}" ] && [ -f "${FOUND_BIN}" ]; then
                    if [ "${FOUND_BIN}" != "${STAGING_DIR}/bedrock-server-manager" ]; then
                        mv "${FOUND_BIN}" "${STAGING_DIR}/bedrock-server-manager"
                    fi
                    BINARY_STAGED=true
                fi
            fi
        fi
    fi

    # Download from GitHub Releases if not found in dist
    if [ "${BINARY_STAGED}" = false ] && [ -n "${ARCH}" ] && command -v curl >/dev/null 2>&1; then
        RELEASE_TAG="${TARGET_VERSION}"
        if [ -z "${RELEASE_TAG}" ] || [ "${RELEASE_TAG}" = "latest" ]; then
            RELEASE_TAG="$(get_latest_release_tag)"
        fi
        if [[ "${RELEASE_TAG}" =~ ^[0-9]+\.[0-9]+ ]]; then
            RELEASE_TAG="v${RELEASE_TAG}"
        fi

        DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${RELEASE_TAG}/bedrock-server-manager-${RELEASE_TAG}-linux-${ARCH}.tar.gz"
        log_info "Downloading Bedrock Server Manager ${RELEASE_TAG} from GitHub (${DOWNLOAD_URL})..."

        if curl -fsSL "${DOWNLOAD_URL}" -o "${STAGING_DIR}/bsm.tar.gz" 2>/dev/null; then
            if tar -xzf "${STAGING_DIR}/bsm.tar.gz" -C "${STAGING_DIR}" 2>/dev/null; then
                FOUND_BIN="$(find "${STAGING_DIR}" -type f -name bedrock-server-manager | head -n 1)"
                if [ -n "${FOUND_BIN}" ] && [ -f "${FOUND_BIN}" ]; then
                    if [ "${FOUND_BIN}" != "${STAGING_DIR}/bedrock-server-manager" ]; then
                        mv "${FOUND_BIN}" "${STAGING_DIR}/bedrock-server-manager"
                    fi
                    FOUND_ENV="$(find "${STAGING_DIR}" -type f -name .env.example | head -n 1)"
                    if [ -n "${FOUND_ENV}" ]; then
                        cp "${FOUND_ENV}" "${STAGING_DIR}/bsm.env.example"
                    fi
                    BINARY_STAGED=true
                    log_success "Downloaded and extracted Bedrock Server Manager ${RELEASE_TAG}."
                fi
            fi
        else
            log_warn "Could not download release archive from GitHub for tag '${RELEASE_TAG}'."
        fi
    fi

    if [ "${BINARY_STAGED}" = false ]; then
        log_warn "Pre-compiled release could not be obtained. Falling back to building from source..."
        INSTALL_METHOD="build"
    fi
fi

# Method C: Build directly from Go & React source
if [ "${BINARY_STAGED}" = false ] && [ "${INSTALL_METHOD}" = "build" ]; then
    # Verify Go and Node dependencies
    BUILD_DEPS_MISSING=()
    if ! command -v go >/dev/null 2>&1; then
        BUILD_DEPS_MISSING+=("golang-go")
    fi
    if ! command -v node >/dev/null 2>&1; then
        BUILD_DEPS_MISSING+=("nodejs")
    fi
    if ! command -v npm >/dev/null 2>&1; then
        BUILD_DEPS_MISSING+=("npm")
    fi
    if ! command -v git >/dev/null 2>&1; then
        BUILD_DEPS_MISSING+=("git")
    fi

    if [ ${#BUILD_DEPS_MISSING[@]} -gt 0 ]; then
        log_info "Missing build tools: ${BUILD_DEPS_MISSING[*]}. Installing..."
        if command -v apt-get >/dev/null 2>&1; then
            apt-get update -qq
            apt-get install -y -qq "${BUILD_DEPS_MISSING[@]}"
        elif command -v dnf >/dev/null 2>&1; then
            dnf install -y -q golang nodejs npm git
        elif command -v pacman >/dev/null 2>&1; then
            pacman -Sy --noconfirm go nodejs npm git
        elif command -v apk >/dev/null 2>&1; then
            apk add --no-cache go nodejs npm git
        else
            log_error "Unsupported package manager. Please install Go and Node.js manually."
            exit 1
        fi
    fi

    BUILD_DIR=""
    if [ -f "${REPO_ROOT}/cmd/manager/main.go" ]; then
        BUILD_DIR="${REPO_ROOT}"
    elif [ -f "./cmd/manager/main.go" ]; then
        BUILD_DIR="$(pwd)"
    fi

    if [ -z "${BUILD_DIR}" ]; then
        log_info "Cloning repository source from https://github.com/${REPO}.git to build..."
        git clone --depth 1 "https://github.com/${REPO}.git" "${STAGING_DIR}/repo"
        BUILD_DIR="${STAGING_DIR}/repo"
    fi

    # Build web frontend assets
    log_info "Building embedded React frontend in ${BUILD_DIR}/web..."
    (
        cd "${BUILD_DIR}/web"
        if [ ! -d "node_modules" ]; then
            npm ci 2>/dev/null || npm install --silent
        fi
        npm run build
    )

    # Compile static Go binary with embedded assets
    log_info "Compiling static Go binary from ${BUILD_DIR}..."
    (
        cd "${BUILD_DIR}"
        CGO_ENABLED=0 go build \
            -ldflags="-s -w -X github.com/AhmadShamli/Bedrock-Server-Manager/internal/version.Version=${TARGET_VERSION#v}" \
            -o "${STAGING_DIR}/bedrock-server-manager" \
            ./cmd/manager
    )
    BINARY_STAGED=true
fi

# Validate candidate binary
if [ "${BINARY_STAGED}" = false ] || [ ! -f "${STAGING_DIR}/bedrock-server-manager" ]; then
    log_error "Bedrock Server Manager binary could not be prepared."
    exit 1
fi

chmod +x "${STAGING_DIR}/bedrock-server-manager"

# Host execution test
if ! CANDIDATE_OUT="$(timeout 3 "${STAGING_DIR}/bedrock-server-manager" version 2>&1)"; then
    log_error "Candidate binary validation failed on this host:"
    echo "${CANDIDATE_OUT}" >&2
    log_error "Aborting. Existing installation has not been modified."
    exit 1
fi

CANDIDATE_VERSION="$(echo "${CANDIDATE_OUT}" | grep -oE 'v?[0-9]+\.[0-9]+(\.[0-9]+)?(-[a-zA-Z0-9.]+)?' | head -n1 || true)"
log_success "Candidate binary verified (${CANDIDATE_VERSION:-valid})."

# Check if already on the same version during upgrade
if [ "${MODE}" = "upgrade" ] && [ "${CURRENT_VERSION}" != "unknown" ] && [ "${CURRENT_VERSION}" = "${CANDIDATE_VERSION}" ] && [ "${FORCE_ACTION}" = false ]; then
    if [ -t 0 ]; then
        echo ""
        log_warn "Bedrock Server Manager is already at version ${CURRENT_VERSION}."
        read -r -p "Reinstall and refresh configuration anyway? [y/N]: " REINSTALL_PROMPT
        case "${REINSTALL_PROMPT}" in
            y*|Y*)
                log_info "Proceeding with reinstall..."
                ;;
            *)
                log_info "Bedrock Server Manager is up to date. Exiting."
                exit 0
                ;;
        esac
    else
        log_info "Bedrock Server Manager is already at version ${CURRENT_VERSION}. Re-applying binary and service configurations..."
    fi
fi

# -----------------------------------------------------------------------------
# 5. Quiesce Service & Install Binary Atomically
# -----------------------------------------------------------------------------
if [ "${MODE}" = "upgrade" ] && [ "${SERVICE_WAS_ACTIVE}" = true ]; then
    log_info "Temporarily stopping bedrock-server-manager.service to apply update..."
    systemctl stop bedrock-server-manager.service || true
fi

# Atomic binary replacement using install (unlinks inode, preventing ETXTBSY)
install -m 0755 -o root -g root "${STAGING_DIR}/bedrock-server-manager" /usr/local/bin/bedrock-server-manager
log_success "Installed static binary to /usr/local/bin/bedrock-server-manager."

# Also refresh local repo bin/ if applicable
if [ -d "${REPO_ROOT}/bin" ]; then
    cp "${STAGING_DIR}/bedrock-server-manager" "${REPO_ROOT}/bin/bedrock-server-manager"
fi

# -----------------------------------------------------------------------------
# 6. Sudoers Policy Configuration
# -----------------------------------------------------------------------------
echo "[4/6] Installing locked-down sudoers rule..."
SUDOERS_TMP="${STAGING_DIR}/bedrock-server-manager.sudoers"
if [ -f "${SCRIPT_DIR}/bedrock-server-manager.sudoers" ]; then
    cp "${SCRIPT_DIR}/bedrock-server-manager.sudoers" "${SUDOERS_TMP}"
elif [ -f "${REPO_ROOT}/deploy/bedrock-server-manager.sudoers" ]; then
    cp "${REPO_ROOT}/deploy/bedrock-server-manager.sudoers" "${SUDOERS_TMP}"
else
    cat <<'EOF' > "${SUDOERS_TMP}"
# Restricted execution rights for unprivileged 'bedrock' user
bedrock ALL=(root) NOPASSWD: /usr/sbin/ufw *, /usr/sbin/iptables *, /usr/sbin/ip6tables *, /usr/bin/nsenter --net=/proc/1/ns/net *
EOF
fi

if command -v visudo >/dev/null 2>&1; then
    if visudo -c -f "${SUDOERS_TMP}" >/dev/null 2>&1; then
        install -m 0440 -o root -g root "${SUDOERS_TMP}" /etc/sudoers.d/bedrock-server-manager
        log_success "Verified and installed sudoers policy to /etc/sudoers.d/bedrock-server-manager."
    else
        log_error "Generated sudoers rule failed visudo syntax check. Sudoers update skipped to protect system access."
    fi
else
    install -m 0440 -o root -g root "${SUDOERS_TMP}" /etc/sudoers.d/bedrock-server-manager
    log_success "Installed sudoers policy to /etc/sudoers.d/bedrock-server-manager."
fi

# -----------------------------------------------------------------------------
# 7. Environment & Configuration Management
# -----------------------------------------------------------------------------
echo "[5/6] Managing configuration in /etc/bedrock-server-manager..."
if [ ! -f /etc/bedrock-server-manager/bsm.env ]; then
    RAND_JWT=$(head -c 32 /dev/urandom | od -A n -t x | tr -d ' \n')
    RAND_PEPPER=$(head -c 32 /dev/urandom | od -A n -t x | tr -d ' \n')
    cat <<EOF > /etc/bedrock-server-manager/bsm.env
# Bedrock Server Manager (BSM) Daemon Configuration
PORT=8080
DATA_DIR=/var/lib/bedrock-server-manager/data
PROXY_MODE=direct
# Options: ufw, iptables, custom, mock
FIREWALL_DRIVER=ufw
HEARTBEAT_INTERVAL_SECONDS=10
DOCKER_HOST=unix:///var/run/docker.sock
JWT_SECRET=${RAND_JWT}
PEPPER=${RAND_PEPPER}
EOF
    chmod 0600 /etc/bedrock-server-manager/bsm.env
    chown bedrock:bedrock /etc/bedrock-server-manager/bsm.env
    log_success "Initialized configuration at /etc/bedrock-server-manager/bsm.env."
else
    log_success "Preserving existing configuration at /etc/bedrock-server-manager/bsm.env."
    chmod 0600 /etc/bedrock-server-manager/bsm.env
    chown bedrock:bedrock /etc/bedrock-server-manager/bsm.env
fi

# Deploy reference .env.example
ENV_EXAMPLE_SRC=""
if [ -f "${REPO_ROOT}/.env.example" ]; then
    ENV_EXAMPLE_SRC="${REPO_ROOT}/.env.example"
elif [ -f "${STAGING_DIR}/bsm.env.example" ]; then
    ENV_EXAMPLE_SRC="${STAGING_DIR}/bsm.env.example"
fi

if [ -n "${ENV_EXAMPLE_SRC}" ] && [ -f "${ENV_EXAMPLE_SRC}" ]; then
    install -m 0644 "${ENV_EXAMPLE_SRC}" /etc/bedrock-server-manager/bsm.env.example
    log_info "Updated reference configuration template at /etc/bedrock-server-manager/bsm.env.example."
fi

# -----------------------------------------------------------------------------
# 8. Deploy & Start / Restart Systemd Service
# -----------------------------------------------------------------------------
echo "[6/6] Configuring systemd service..."
SERVICE_TMP="${STAGING_DIR}/bedrock-server-manager.service"
if [ -f "${SCRIPT_DIR}/bedrock-server-manager.service" ]; then
    cp "${SCRIPT_DIR}/bedrock-server-manager.service" "${SERVICE_TMP}"
elif [ -f "${REPO_ROOT}/deploy/bedrock-server-manager.service" ]; then
    cp "${REPO_ROOT}/deploy/bedrock-server-manager.service" "${SERVICE_TMP}"
else
    cat <<'EOF' > "${SERVICE_TMP}"
[Unit]
Description=Bedrock Server Manager (BSM) Daemon
Documentation=https://github.com/AhmadShamli/Bedrock-Server-Manager
After=network.target network-online.target docker.service
Wants=network-online.target docker.service

[Service]
Type=simple
User=bedrock
Group=bedrock
SupplementaryGroups=docker
EnvironmentFile=-/etc/bedrock-server-manager/bsm.env
Environment="PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
ExecStart=/usr/local/bin/bedrock-server-manager
WorkingDirectory=/var/lib/bedrock-server-manager
Restart=always
RestartSec=3s
LimitNOFILE=65536

CapabilityBoundingSet=CAP_NET_ADMIN CAP_NET_RAW CAP_SYS_ADMIN
AmbientCapabilities=CAP_NET_ADMIN CAP_NET_RAW

ProtectSystem=full
ProtectHome=false
NoNewPrivileges=false
ReadWritePaths=-/etc/ufw

[Install]
WantedBy=multi-user.target
EOF
fi

install -m 0644 -o root -g root "${SERVICE_TMP}" /etc/systemd/system/bedrock-server-manager.service
log_success "Deployed systemd unit to /etc/systemd/system/bedrock-server-manager.service."

SERVICE_STARTED=false
if [ "${SYSTEMD_ACTIVE}" = true ]; then
    systemctl daemon-reload

    if [ "${MODE}" = "upgrade" ]; then
        if [ "${SERVICE_WAS_ACTIVE}" = true ]; then
            log_info "Restarting bedrock-server-manager.service with updated binary..."
            systemctl start bedrock-server-manager.service
            SERVICE_STARTED=true
        else
            log_info "bedrock-server-manager.service was inactive prior to upgrade. Keeping service stopped."
            echo "To start the upgraded service, run: sudo systemctl start bedrock-server-manager.service"
        fi
    else
        systemctl enable --now bedrock-server-manager.service
        SERVICE_STARTED=true
    fi

    if [ "${SERVICE_STARTED}" = true ]; then
        log_info "Verifying service startup..."
        SERVICE_OK=false
        for i in $(seq 1 10); do
            if systemctl is-active --quiet bedrock-server-manager.service 2>/dev/null; then
                SERVICE_OK=true
                break
            fi
            sleep 1
        done

        if [ "${SERVICE_OK}" = false ]; then
            log_error "bedrock-server-manager.service failed to become active after installation/upgrade!"
            echo "--- Recent journal logs for bedrock-server-manager.service ---" >&2
            journalctl -u bedrock-server-manager.service -n 25 --no-pager >&2 || true
            echo "--------------------------------------------------------------" >&2

            # Rollback to previous binary if available
            if [ "${MODE}" = "upgrade" ] && [ -n "${BACKUP_DIR}" ] && [ -f "${BACKUP_DIR}/bedrock-server-manager.previous" ]; then
                log_warn "Attempting automatic rollback to previous binary..."
                install -m 0755 -o root -g root "${BACKUP_DIR}/bedrock-server-manager.previous" /usr/local/bin/bedrock-server-manager
                systemctl restart bedrock-server-manager.service 2>/dev/null || true
                if systemctl is-active --quiet bedrock-server-manager.service 2>/dev/null; then
                    log_warn "Rolled back successfully to ${CURRENT_VERSION}. Service is running."
                else
                    log_error "Rollback attempted, but service remains inactive. Check journal logs."
                fi
            fi
            exit 1
        fi
        log_success "bedrock-server-manager.service is active and healthy."
    fi
else
    log_warn "Systemd is not running as PID 1 (container or custom environment detected)."
    log_info "Service file deployed. Start Bedrock Server Manager using your process supervisor or run:"
    echo "  /usr/local/bin/bedrock-server-manager"
fi

# -----------------------------------------------------------------------------
# Summary Output
# -----------------------------------------------------------------------------
echo ""
echo "================================================================================"
if [ "${MODE}" = "upgrade" ]; then
    echo "            Bedrock Server Manager Successfully Upgraded!"
    echo "================================================================================"
    echo "Previous Version        : ${CURRENT_VERSION}"
    echo "Current Version         : ${CANDIDATE_VERSION:-${TARGET_VERSION}}"
    if [ -n "${BACKUP_DIR}" ]; then
        echo "Database Backup         : ${BACKUP_DIR}"
    fi
    if [ "${SYSTEMD_ACTIVE}" = true ]; then
        echo "Service Status          : $(systemctl is-active bedrock-server-manager.service 2>/dev/null || echo 'inactive')"
    fi
    echo "Dashboard Web URL       : http://localhost:8080"
    echo ""
    echo "All existing BDS servers, databases, and firewall configs have been preserved."
else
    echo "         Bedrock Server Manager Successfully Installed & Started!"
    echo "================================================================================"
    echo "Installed Version       : ${CANDIDATE_VERSION:-${TARGET_VERSION}}"
    if [ "${SYSTEMD_ACTIVE}" = true ]; then
        echo "Service Status          : $(systemctl is-active bedrock-server-manager.service 2>/dev/null || echo 'inactive')"
    fi
    echo "Dashboard Web URL       : http://localhost:8080"
    echo "Configuration File      : /etc/bedrock-server-manager/bsm.env"
    echo "Data Directory          : /var/lib/bedrock-server-manager/data"
    echo ""
    echo "To view daemon logs, run:"
    echo "  journalctl -u bedrock-server-manager.service -n 25 --no-pager"
fi
echo "================================================================================"
