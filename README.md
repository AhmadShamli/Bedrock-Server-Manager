# Bedrock Server Manager (BSM)

[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?style=flat&logo=go)](https://golang.org)
[![React](https://img.shields.io/badge/React-18-61DAFB?style=flat&logo=react)](https://reactjs.org)
[![Docker](https://img.shields.io/badge/Docker-Orchestrated-2496ED?style=flat&logo=docker)](https://www.docker.com)
[![Platform](https://img.shields.io/badge/Platform-Linux%20%7C%20Windows-blue?style=flat)](https://github.com/AhmadShamli/Bedrock-Server-Manager)
[![License](https://img.shields.io/badge/License-MIT-emerald)](LICENSE)

**Bedrock Server Manager (BSM)** is an enterprise-grade, lightweight, self-hosted web-only management platform for Minecraft Bedrock Dedicated Servers (BDS). Designed with a dark obsidian/cyberpunk aesthetic (`#06090c`) and Minecraft emerald accents, BSM runs entirely as a single static binary or container, orchestrating BDS instances across both **Linux (Docker/host mode)** and **Windows (Native process execution & Windows Service)** with strict hardware limits, dynamic UDP firewall gating, automated hot backups, integrated addon marketplace, and real-time telemetry.

---

## ✨ Highlights & Features

### 🪟 First-Class Native Windows Support
- **Zero-Docker Host Engine**: Run official Mojang `bedrock_server.exe` directly as managed child processes without requiring Docker, Hyper-V, or WSL2.
- **Hardware Enforcements via Job Objects**: Enforces hard memory caps (`JOB_OBJECT_LIMIT_PROCESS_MEMORY`) and CPU rate limits via Windows Job Objects.
- **Native Windows Service**: Full service lifecycle integration (`bedrock-server-manager.exe service [install|uninstall|start|stop|status]`) configured with Delayed-Auto start.
- **Native Windows Defender Firewall Driver**: Automatic inbound UDP rule management (`netsh advfirewall`) with zero-lease drop policy.
- **Loopback & Antivirus Optimization**: Automated UWP loopback exemption (`CheckNetIsolation`) for local client gameplay and automated Windows Defender scanning exclusions.
- **Robust CPU Sample Telemetry**: Monotonic tick tracking, PID matching, and arithmetic underflow guards prevent false spikes or corrupted telemetry.
- **Automated PowerShell Installer (`install.ps1`)**: Single-script setup with automatic Microsoft Visual C++ 2015–2022 Redistributable verification and installation.

### 🎮 Multi-Server Container & Process Orchestration
- **Dual Engine Architecture**: Deploy isolated BDS instances via Docker containers (`itzg/minecraft-bedrock-server`) on Linux or native BDS processes on Windows.
- **Strict Resource Capping**: Enforce precise CPU core allocations and RAM limits per instance.
- **Interactive Live Web Console**: Bidirectional WebSocket console streaming with ANSI color decoding, search, and command history.
- **Crash-Loop Circuit Breaker**: Prevents server boot storms by halting automatic restarts if an instance crashes 5 times within 5 minutes.
- **Auto-Start & Crash Recovery**: Configurable auto-start on host boot and resilient daemon restart handling.

### 🛡️ Dynamic UDP Port Gate & Mobile Roaming Knock
- **Stealth BDS Ports**: Bedrock UDP ports (19132+) remain closed to the public internet by default, preventing scanner indexing and brute-force probing.
- **Lightweight Web Knock Portal (`/knock/:id`)**: Players unlock temporary access via Gamertag, HMAC-SHA256 passphrase, or dual-factor knock.
- **Mobile 4G/5G Roaming Handoff**: Browser maintains a configurable background heartbeat. When mobile players switch cell towers or Wi-Fi networks and their IP changes, BSM detects the new address and hot-swaps firewall rules seamlessly without kicking the active session.
- **1-Tap Direct Game Launch**: Deep link integration (`minecraft://?addExternalServer=<Name>|<Host>:<Port>`) launches Minecraft directly on Android, iOS, Windows, and consoles.
- **Multi-Unit Lease Durations**: Configure access grants in minutes, hours, or days with live expiration countdowns and centralized lease revocation.
- **Two-Tier Rate Limiter**: Per-IP progressive delays combined with a 15-minute distributed circuit breaker to neutralize brute-force knocking.
- **Multi-Backend Firewall Drivers**: Out-of-the-box drivers for UFW, IPTables (`BSM_PORT_GATE` chain for IPv4 & IPv6), Windows Defender Firewall (`netsh`), `nsenter` (for containerized host firewall control), and custom shell scripts.

### 💤 Auto-Stop on Idle & Knock Portal Wake-on-Demand
- **Zero-Resource Idle State**: Automatically stops BDS instances when player count remains at 0 for a configurable duration (e.g. 15m, 30m, 1h, 2h).
- **Wake-on-Demand**: When players visit the knock portal or authenticate, BSM automatically boots the sleeping server instance on demand.

### 🛒 Dedicated Bedrock Addon & World Maps Marketplace
- **CurseForge Bedrock Marketplace Integration**: Search, filter, and discover Bedrock Addons, Texture Packs, Behavior Packs, and World Maps directly in the management UI.
- **In-App Addon Details & Gallery**: Addon detail modals with screenshots carousels, formatted HTML descriptions, release channels, and author metadata.
- **1-Click World Map (.mcworld) Import**: Install adventure, survival, or minigame maps directly from CurseForge or custom archive uploads.
- **Direct Remote URL Installer**: Download and install addons, skin packs, and maps directly from any public HTTP/HTTPS URL.
- **Multi-Pack Archive Extraction**: Automatically unpacks nested multi-pack `.mcaddon` and `.zip` archives.
- **Pack Management & Prioritization**: 1-click pack activation/deactivation without file deletion, visual priority drag/reordering, and `texturepack-required` client enforcement.
- **Automatic World Pack Sync**: Dynamically synchronizes active packs with `world_resource_packs.json` and `world_behavior_packs.json`.

### 💾 Zero-Downtime Hot Backups & World Management
- **Non-Disruptive Snapshots**: Executes hot backups using Bedrock's native `save hold` → `save query` → snapshot → `save resume` protocol without interrupting connected players.
- **Retention Policy Engine**: Automated pruning based on maximum backup count, age in days, and disk quota limits, with manual Pin/Lock protection.
- **1-Click World Export & Import**: Seamless `.mcworld` export and custom world archive restoration.
- **Safe World Reset**: Re-initialize worlds with clean states, custom seeds, or preserved seed configurations.

### 📊 Real-Time Resource Utilization Alerts & Historical Telemetry
- **Dual SQLite Database Architecture**: Clean separation between operational state (`manager.db`) and high-frequency telemetry metrics (`metrics.db`) in WAL mode.
- **Proactive Resource Alerts**: Real-time background monitor watches host and instance CPU/RAM usage; broadcasts alerts to the UI and triggers Discord webhooks when thresholds (e.g. 90%) are exceeded.
- **Historical Metrics Rollup Engine**: Downsamples raw samples into 1-minute, 5-minute, and 1-hour rollup aggregates for fast long-term trend analysis.
- **Tiered Retention Policies**: Independent retention periods for raw, hourly, and daily metrics to preserve storage capacity.

### 🌐 Dynamic DNS (DDNS) & Public IP Auto-Discovery
- **Redundant WAN IP Discovery**: Automatic detection of host public WAN IP via redundant IP resolvers (Cloudflare trace, ipify, seeip).
- **Automated Background Sync**: Detects external IP modifications on dynamic residential or cloud connections and updates knock portal endpoints and connection deep links automatically.
- **Manual Trigger**: Instant one-click public IP refresh from the System Settings panel.

### 🤖 Cloudflare Turnstile Bot Protection
- **Anti-Bot Challenge**: Protects public login and knock portals against automated credential-stuffing and DDoS attacks.
- **Smart Local / Private LAN Bypass**: Automatically bypasses challenges for private LAN, loopback, and RFC 1918/4193 subnets.
- **Reverse Proxy Header Inspection**: Safely inspects upstream proxy client IP headers (`X-Forwarded-For`, `CF-Connecting-IP`, `X-Real-IP`, RFC 7239 `Forwarded`) to verify authentic origins.

### ✉️ Outbound SMTP Delivery, Verification & Password Reset
- **Integrated Mailer**: Complete SMTP transport support with STARTTLS/SSL encryption and authentication.
- **Self-Service Password Reset**: Secure, time-limited tokenized email workflows for password recovery.
- **Account Email Verification**: Enforce email address verification for newly registered accounts.
- **In-App Mail Tester**: Test email dispatch directly from the admin dashboard.

### 👥 User Management, Multi-Tier Plans & Quotas
- **Role-Based Access Control**: Granular Admin and User permission boundaries.
- **Multi-Tier Plan Engine**: Create tiered plans (e.g., Free, Pro, Enterprise) with resource constraints:
  - Maximum BDS instances
  - Maximum CPU core allocations
  - Maximum RAM allocations
  - Disk storage limits
  - Backup retention limits
  - Permission to deploy Preview/Beta BDS versions
  - Port Gate access permissions
- **Self-Service Deployment**: Standard users can spin up and manage their own BDS instances within their assigned plan quotas.

### 👥 Player Hub, Live Log Parsing & Chat Feed
- **Real-Time Log Stream Parser**: Tracks player joins, leaves, coordinates, and in-game chat messages (`<Gamertag> ...`).
- **Persistent Player History**: Records connection histories, first seen/last seen dates, total playtime, and live online/offline states.
- **Interactive Chat Hub**: Broadcast server-wide announcements or send direct private player whispers (`tellraw`).
- **Player Moderation**: 1-click Kick, Ban, Op, and De-op controls.

### ⚙️ Visual Server Properties Editor
- **Intuitive Web Editor**: Edit `server.properties` with validation and contextual help.
- **Smart BDS Presets**: Dropdown selections and default indicators for all Bedrock settings (`gamemode`, `difficulty`, `server-authoritative-movement`, tick thresholds, compression algorithms, etc.).

### ⏰ Automated Task Scheduler
- **Cron Automation**: Schedule automated hot backups, console commands, and server restarts.
- **Countdown Broadcasts**: Graceful restart notifications with in-game announcements at 30s, 15s, and 5s before reboot.

### 📦 Interactive Multi-Platform Installers
- **Linux (`install.sh`)**: Interactive version selection menu, GitHub release discovery, automated pre-upgrade backups, non-destructive rollbacks, and host diagnostics.
- **Windows (`install.ps1`)**: Automated dependency installation, firewall rule setup, UWP loopback exemption, and Windows Service registration.

---

## 🚀 Quickstart

### Option A: Standalone Host Deployment (Linux Systemd)

BSM runs directly on the host as a lightweight, secure systemd service (`bedrock:bedrock`). It orchestrates BDS containers through the local Docker daemon socket (`/var/run/docker.sock`) and controls host firewall rules natively.

#### 1. One-Line Remote Installer
Run directly on any supported Linux distribution (Ubuntu, Debian, RHEL, Rocky, Fedora, Arch, Alpine):
```bash
curl -fsSL https://raw.githubusercontent.com/AhmadShamli/Bedrock-Server-Manager/main/install.sh | sudo bash
```

#### 2. Local Repository Installation
Clone and run the installer interactively:
```bash
git clone https://github.com/AhmadShamli/Bedrock-Server-Manager.git
cd Bedrock-Server-Manager
sudo ./install.sh
```

> [!NOTE]
> The installer displays a comprehensive host diagnostic report (OS, kernel, CPU, RAM, disk space, Docker socket state, and firewall status for UFW, IPTables, and Firewalld) and provides an interactive release version selection menu. Use `-y` or `--yes` for automated, non-interactive environments.

#### 3. Installer CLI Options

```bash
sudo ./install.sh [OPTIONS]
```

| Flag | Description |
| :--- | :--- |
| `-y`, `--yes` | Non-interactive mode (automatically answer yes to prompts) |
| `-u`, `--upgrade` | Upgrade existing installation with automated pre-upgrade database backup |
| `-v`, `--version <tag>` | Install, upgrade, or rollback to a specific release tag (e.g. `v2.0.0`, `v1.9.9`, `latest`) |
| `--check` | Check current installed version against available releases without modifying files |
| `-m`, `--method <method>` | Source method: `download` (GitHub release), `build` (compile from source), or `local` |
| `--skip-backup` | Skip pre-upgrade database and configuration backup |
| `-f`, `--force` | Force reinstall or upgrade without confirmation |
| `-h`, `--help` | Display installer options and usage examples |

#### 4. Managing the Systemd Service
```bash
sudo systemctl status bedrock-server-manager
sudo journalctl -u bedrock-server-manager -f
sudo systemctl restart bedrock-server-manager
```

---

### Option B: Windows Host Deployment (Native BDS & Windows Service)

BSM supports Windows 10/11 and Windows Server 2019/2022 natively without Docker, running official Mojang `bedrock_server.exe` as managed child processes with Windows Job Objects for memory caps and CPU rate limits.

#### 1. Quick Automated Setup (PowerShell)
Download `bedrock-server-manager-v2.0.0-windows-amd64.zip` from GitHub Releases, extract the archive, and run PowerShell as Administrator:
```powershell
Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass
.\install.ps1
```

The installer automatically:
- Verifies and installs the Microsoft Visual C++ 2015–2022 x64 Redistributable if missing.
- Configures Windows Defender exclusions for `%ProgramData%\BedrockServerManager`.
- Grants UWP loopback exemption so the local Minecraft Bedrock client can connect to `127.0.0.1`.
- Sets up Windows Defender Firewall rules for the Web UI.
- Installs and starts `BedrockServerManager` as an automatic Windows background service.

#### 2. Managing the Windows Service
```powershell
.\bedrock-server-manager.exe service status
.\bedrock-server-manager.exe service stop
.\bedrock-server-manager.exe service start
.\bedrock-server-manager.exe service uninstall
```

#### 3. Portable Mode (No Service)
To run BSM in portable mode without registering a Windows service:
```powershell
.\bedrock-server-manager.exe --data-dir .\data --open
```

---

### Option C: Using Docker Compose

To deploy BSM as a containerized management daemon:

1. Start the daemon with Docker Compose:
   ```bash
   docker compose up -d
   ```
2. Open your browser to `http://localhost:8080`.
3. Complete the initial setup wizard to create your primary administrator account.

---

## ⚙️ Configuration Reference

### Host Mode Configuration File (`bsm.env`)
- **Linux**: `/etc/bedrock-server-manager/bsm.env` (managed with `0600` permissions, owned by `bedrock:bedrock`).
- **Windows**: `%ProgramData%\BedrockServerManager\bsm.env`.

You can also specify a custom configuration file at runtime:
```bash
bedrock-server-manager --config /path/to/custom.env
```

### Environment Variables

| Variable | Default (Host / Docker) | Description |
| :--- | :--- | :--- |
| `PORT` | `8080` | Web dashboard & REST API HTTP listening port |
| `DATA_DIR` | `/var/lib/bedrock-server-manager/data` (Linux)<br>`%ProgramData%\BedrockServerManager` (Windows)<br>`/data` (Docker) | Storage directory for SQLite databases (`manager.db`, `metrics.db`), server files, and backups |
| `PROXY_MODE` | `direct` | Client IP resolution mode: `direct`, `reverse_proxy`, or `cloudflare` |
| `TRUSTED_PROXIES` | `""` | Comma-separated list of trusted upstream proxy CIDRs (e.g. `10.0.0.0/8,172.16.0.0/12,192.168.0.0/16`) |
| `FIREWALL_DRIVER` | `ufw` (Linux)<br>`windows` (Windows) | Host firewall backend (`ufw`, `iptables`, `windows`, `custom`, `mock`) |
| `BSM_FIREWALL_SCRIPT` | `""` | Path to executable script when `FIREWALL_DRIVER=custom` |
| `HEARTBEAT_INTERVAL_SECONDS` | `10` | Default interval in seconds for mobile roaming background heartbeats |
| `DOCKER_HOST` | `unix:///var/run/docker.sock` | Docker daemon socket URI (Linux container orchestration) |
| `JWT_SECRET` | *(auto-generated)* | 256-bit secret key for JWT session tokens (stored in DB if omitted) |
| `PEPPER` | *(auto-generated)* | Persistent pepper for keyed HMAC-SHA256 knock authentication |

### Dynamic In-App Settings

Administrators can configure the following features live from the Web UI under **System Settings** (`/settings`):

| Setting Category | Configurable Parameters | Description |
| :--- | :--- | :--- |
| **Dynamic DNS (DDNS)** | Domain, Auto-Sync Toggle, Sync Interval | Automatically detects host public WAN IP changes and updates knock endpoints. |
| **Cloudflare Turnstile** | Site Key, Secret Key, Local LAN Bypass | Protects public knock and authentication portals against bots and brute force. |
| **Outbound SMTP Email** | Host, Port, Encryption, User, Password, From Address | Enables password reset workflows and account email verification. |
| **CurseForge Marketplace** | API Key | Unlocks integrated Bedrock Addon and World Map browsing and 1-click installation. |
| **Discord Notifications** | Webhook URL, Security Alerts, Resource Alerts | Delivers real-time notifications for critical resource consumption and security events. |
| **Resource Alerts** | CPU Threshold %, RAM Threshold %, Alert Cooldown | Real-time notifications when server or host hardware limits are saturated. |
| **Backup Retention** | Max Backups, Max Retention Days, Max Total GB | Automated cleanup policies for hot backup storage. |
| **Telemetry Retention** | Raw Retention Days, Rollup Retention Days | Manages retention for high-resolution samples and rollup aggregates in `metrics.db`. |

---

## 🏗️ Architecture & Internals

```mermaid
flowchart TD
    Client["Client Browser / Mobile Client<br/>(React 18 + Tailwind SPA)"] -->|HTTPS Dashboard & Console WS| Server["Bedrock Server Manager (BSM Daemon)"]
    Client -->|HTTP Knock Heartbeat (10s)| Server

    subgraph BSM["Bedrock Server Manager Runtime"]
        Server --> Auth["Auth & RBAC<br/>(JWT + HMAC-SHA256)"]
        Server --> Turnstile["Cloudflare Turnstile<br/>(Bot Protection & LAN Bypass)"]
        Server --> Telemetry["Telemetry Collector & Rollup Engine"]
        Server --> Addons["Addon & Marketplace Service"]
        Server --> Mail["Outbound SMTP Service"]
        Server --> DDNS["Dynamic DNS & Public IP Sync"]

        subgraph Storage["Dual SQLite Stores (WAL Mode)"]
            DB1["manager.db<br/>(Config, Users, Plans, Leases, Tasks)"]
            DB2["metrics.db<br/>(Time-Series Metrics & Rollups)"]
        end

        Server --> DB1
        Telemetry --> DB2

        subgraph Engines["Dual BDS Orchestration Engines"]
            DockerEng["Linux Docker Engine<br/>(itzg/minecraft-bedrock-server)"]
            WinEng["Windows Native Engine<br/>(bedrock_server.exe via Job Objects)"]
        end

        subgraph Firewalls["Firewall Drivers"]
            FW_Linux["Linux: UFW / IPTables / nsenter"]
            FW_Win["Windows: netsh advfirewall"]
        end
    end

    Server -->|Linux| DockerEng
    Server -->|Windows| WinEng
    Server -->|Manage Leases| FW_Linux
    Server -->|Manage Leases| FW_Win

    DockerEng --> BDS1["BDS Container 1<br/>Port 19132"]
    DockerEng --> BDS2["BDS Container 2<br/>Port 19134"]
    WinEng --> BDS3["BDS Process 1<br/>Port 19132"]

    FW_Linux -.->|Allow/Revoke UDP| BDS1
    FW_Win -.->|Allow/Revoke UDP| BDS3
```

### Dynamic Port-Knocking Flow
1. **Default Shielded State**: Server UDP ports (19132+) are blocked by host firewall rules.
2. **Knock Request**: Player navigates to `https://bsm.example.com/knock/<server_id>` and enters credentials (Gamertag, Passphrase, or both).
3. **Turnstile Verification**: Bot protection verifies the client (with automatic bypass for private LAN connections).
4. **Firewall Rule Insertion**: Server validates credentials via constant-time HMAC-SHA256, issues an `AllowPort(clientIP, port)` rule, and sets an `HttpOnly` signed session cookie.
5. **Direct Connect**: The browser displays a 1-tap `minecraft://?addExternalServer=...` button that launches the Minecraft client directly.
6. **Mobile Roaming Handoff**: If the client moves to a different cellular cell tower or Wi-Fi network, the background heartbeat sends `POST /api/knock/:id/heartbeat`. BSM detects the IP change, deletes the old rule, and opens the new IP seamlessly.
7. **Automated Lease Auditor**: A background worker audits leases every 15 seconds, purging expired grants from both the database and the active firewall.

---

## 🛠️ Development & Testing

### Prerequisites
- Go 1.26+
- Node.js 20+ & npm
- Docker daemon (optional; automatically falls back to mock engine for local non-Docker development)

### Running the Test Suite
Execute unit and integration tests across all packages:
```bash
go test -v ./...
```

### Building Release Artifacts
BSM includes automated build scripts to compile web assets and produce multi-arch binaries:

```bash
# Compile web frontend assets and local binary
make build
# or: ./scripts/build.sh

# Run standalone binary directly
make run
# or: ./bin/bedrock-server-manager

# Build multi-platform release packages (linux/amd64, linux/arm64, linux/armv7, windows/amd64)
./scripts/build_release.sh 2.0.0
```

---

## 📄 License
Licensed under the [MIT License](LICENSE).
