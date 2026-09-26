# Bedrock Server Manager (BSM)

[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?style=flat&logo=go)](https://golang.org)
[![React](https://img.shields.io/badge/React-18-61DAFB?style=flat&logo=react)](https://reactjs.org)
[![Docker](https://img.shields.io/badge/Docker-Orchestrated-2496ED?style=flat&logo=docker)](https://www.docker.com)
[![License](https://img.shields.io/badge/License-MIT-emerald)](LICENSE)

**Bedrock Server Manager (BSM)** is a modern, lightweight, self-hosted web-only management platform for Minecraft Bedrock Dedicated Servers (BDS). Designed with a dark obsidian/cyberpunk aesthetic (`#06090c`) and Minecraft emerald accents, BSM runs entirely as a single static binary or container, orchestrating BDS instances with strict Docker hardware limits and dynamic firewall gating.

---

## ✨ Highlights & Features

- 🎮 **Strict Multi-Server Container Orchestration**:
  - All BDS instances run as isolated Docker containers (`itzg/minecraft-bedrock-server`) with hard CPU (`--cpus`) and RAM (`--memory`) caps.
  - Interactive web console with bidirectional WebSocket stdin/stdout streaming and ANSI coloring.
  - Crash-loop circuit breaker (halts restarts if a container crashes 5 times within 5 minutes).
  - Auto-start on boot support.

- 🛡️ **Dynamic UDP Port Gate & Mobile Roaming Knock**:
  - Keep Bedrock server UDP ports completely hidden and closed from the public internet.
  - **Lightweight Web Knock Portal (`/knock/:id`)**: Players authenticate via Gamertag, fast keyed HMAC-SHA256 Passphrase, or both.
  - **Mobile 4G/5G Roaming Handoff**: Browser maintains a lightweight background heartbeat (configurable, default: 10s). When a player moves between cell towers and their IP changes, BSM instantly detects the new IP via signed session cookies and hot-swaps host firewall rules seamlessly.
  - **Direct Game Launch**: Generates 1-tap `minecraft://?addExternalServer=<Name>|<Host>:<Port>` deep links for instant join on Android, iOS, Windows, and consoles.
  - **Two-Tier Rate Limiter**: Per-IP progressive delays + 15-minute distributed circuit breaker against distributed brute-force attacks.

- 💾 **Zero-Downtime Hot Backups & World Management**:
  - Non-disruptive hot backups using Bedrock's native `save hold` -> `save query` -> snapshot -> `save resume` protocol without kicking players.
  - **Retention Engine**: Enforces max backup count, age expiration, and disk quotas while respecting manual Pin/Lock protection.
  - 1-click World Export (`.mcworld`) and World Import for custom maps.

- 📦 **Bedrock Addon & Pack Manager**:
  - Upload and manage `.mcpack`, `.mcaddon`, and `.zip` behavior and resource packs.
  - Automatic manifest parsing, UUID extraction, and directory placement.

- ⏰ **Automated Cron Task Scheduler**:
  - Cron automation for scheduled hot backups, timed commands, and graceful server restarts (with 30s, 15s, and 5s in-game countdown announcements).

- 👥 **Player Hub & Live In-Game Chat Feed**:
  - Real-time BDS log parser tracking joins, leaves, and in-game chat messages (`<Gamertag> ...`).
  - Web composer for server-wide broadcasts and direct player whispers.
  - Quick Kick, Op, and De-op management actions.

- 🌐 **Native Host Firewall Manipulation via `nsenter`**:
  - Supports UFW, IPTables (`BSM_PORT_GATE` chain), and Custom Scripts.
  - Transparently manipulates host packet filters even when BSM itself runs inside Docker via `nsenter --net=/proc/1/ns/net` and `cap_add: [SYS_ADMIN, NET_ADMIN]`.

---

## 🚀 Quickstart

### Option A: Using Docker Compose

1. Start the daemon with Docker Compose:
   ```bash
   docker compose up -d
   ```
2. Open your browser to `http://localhost:8080`.
3. Follow the initial setup wizard to create your primary administrator account.

---

### Option B: Standalone Host Deployment (Systemd / Linux)

BSM can run directly on the host without wrapping the manager itself in Docker. It interacts with the local Docker daemon (`/var/run/docker.sock`) to orchestrate BDS containers and manages the host firewall (`ufw`/`iptables`) natively with an isolated unprivileged system user (`bedrock`).

#### 1. One-Line Remote Installer
Run directly on any supported Linux distribution (Ubuntu, Debian, RHEL, Rocky, Fedora, Arch, Alpine):
```bash
curl -fsSL https://raw.githubusercontent.com/AhmadShamli/Bedrock-Server-Manager/main/install.sh | sudo bash
```

#### 2. Local Repository Installation
Alternatively, install or build from a local git clone:
```bash
git clone https://github.com/AhmadShamli/Bedrock-Server-Manager.git
cd Bedrock-Server-Manager
sudo ./install.sh
```

> [!NOTE]
> The installer displays a comprehensive host environment diagnostic report (OS, kernel, CPU, RAM, disk space, Docker socket state, and firewall statuses for UFW, IPTables, and Firewalld) and asks for interactive confirmation before making changes. Use `-y` or `--yes` for automated, non-interactive environments.

#### 3. Installer Options & Usage

```bash
sudo ./install.sh [OPTIONS]
```

| Flag | Description |
| :--- | :--- |
| `-y`, `--yes` | Non-interactive mode (automatically answer yes to confirmation) |
| `-u`, `--upgrade` | Upgrade existing installation to the latest release with automatic backup |
| `-v`, `--version <tag>` | Install or upgrade to a specific release tag (e.g. `v1.2.1`) |
| `--check` | Check current version against latest GitHub release without changing files |
| `-m`, `--method <method>` | Source method: `download` (GitHub release), `build` (compile from source), or `local` |
| `--skip-backup` | Skip pre-upgrade database and configuration backup |
| `-f`, `--force` | Force reinstall or upgrade without confirmation |
| `-h`, `--help` | Show full installer help and options |

#### 4. Managing the Standalone Service
```bash
sudo systemctl status bedrock-server-manager
sudo journalctl -u bedrock-server-manager -f
sudo systemctl restart bedrock-server-manager
```

#### 5. Updating the Standalone Service
```bash
# Automated upgrade to the latest GitHub release:
sudo ./install.sh --upgrade

# Or compile and update from local git repository:
git pull
sudo ./install.sh -m build

# Or with Makefile:
sudo make update
```

#### 6. Running Locally / Portable (without systemd)
```bash
./scripts/build.sh
./bin/bedrock-server-manager
```

---

## ⚙️ Configuration Reference

### Host Mode Configuration (`/etc/bedrock-server-manager/bsm.env`)

When running as a systemd service, BSM automatically loads its environment from `/etc/bedrock-server-manager/bsm.env` (managed with strict `0600` permissions and owned by `bedrock:bedrock`). You can also specify any custom config file when launching the binary with `--config` or `-c`:

```bash
bedrock-server-manager --config /etc/bedrock-server-manager/bsm.env
```

### Environment Variables

| Variable | Default (Host / Docker) | Description |
| :--- | :--- | :--- |
| `PORT` | `8080` | Web dashboard & API HTTP listening port |
| `DATA_DIR` | `/var/lib/bedrock-server-manager/data` | Storage directory for SQLite databases (`manager.db`, `metrics.db`), server files, and backups |
| `PROXY_MODE` | `direct` | Client IP resolution mode (`direct`, `reverse_proxy`, `cloudflare`) |
| `TRUSTED_PROXIES` | `""` | Comma-separated list of trusted upstream CIDRs (e.g. `10.0.0.0/8,172.16.0.0/12`) |
| `FIREWALL_DRIVER` | `ufw` | Host firewall backend (`ufw`, `iptables`, `custom`, `mock`) |
| `BSM_FIREWALL_SCRIPT` | `""` | Path to executable script when `FIREWALL_DRIVER=custom` |
| `HEARTBEAT_INTERVAL_SECONDS` | `10` | Default interval for mobile roaming background heartbeats |
| `DOCKER_HOST` | `unix:///var/run/docker.sock` | Docker daemon socket URI |
| `JWT_SECRET` | *(auto-generated)* | 256-bit secret key for JWT session tokens |
| `PEPPER` | *(auto-generated)* | Persistent pepper for keyed HMAC-SHA256 knock keys |

### Customizing `DATA_DIR` Storage Location

To move server configurations, worlds, and backups to a dedicated disk mount or custom directory:

1. Edit `/etc/bedrock-server-manager/bsm.env`:
   ```bash
   DATA_DIR=/mnt/storage/bedrock-data
   ```
2. Create the target directory and grant ownership to the `bedrock` service account:
   ```bash
   sudo mkdir -p /mnt/storage/bedrock-data
   sudo chown -R bedrock:bedrock /mnt/storage/bedrock-data
   ```
3. Restart the service:
   ```bash
   sudo systemctl restart bedrock-server-manager
   ```

> [!TIP]
> The systemd unit file is pre-configured with `ProtectHome=false` to ensure custom paths placed under `/home` or secondary storage mounts are fully accessible to BSM.

### Firewall Drivers & Diagnostic Fallback

BSM provides flexible host-level UDP firewall control:

- **`ufw`**: Interacts with Ubuntu/Debian Uncomplicated Firewall. If UFW is inactive or disabled on the host, BSM automatically falls back to the `iptables` driver with an informative log notice.
- **`iptables`**: Directly manages kernel packet filter rules in a dedicated `BSM_PORT_GATE` chain across both IPv4 (`iptables`) and IPv6 (`ip6tables`).
- **`custom`**: Executes an external script specified by `BSM_FIREWALL_SCRIPT` with arguments `allow <ip> <port>` and `revoke <ip> <port>`.
- **`mock`**: Operates in-memory without running system firewall commands (ideal for local development or restricted environments).

The installer automatically deploys `/etc/sudoers.d/bedrock-server-manager` allowing the `bedrock` user to run required firewall commands (`ufw`, `iptables`, `ip6tables`) without password prompts.

---

## 🏗️ Architecture & Internals

```
                      +-----------------------------+
                      |       Client Browser        |
                      |   React 18 + Tailwind SPA   |
                      +--------------+--------------+
                                     |
               HTTPS (Dashboard)     |     HTTP Heartbeat (10s)
                                     v
+-------------------------------------------------------------------------+
|                  Bedrock Server Manager (Go Daemon)                    |
|                                                                         |
|   +-----------------------+     +-----------------------------------+   |
|   |   Dual SQLite Store   |     |      Dynamic Firewall Driver      |   |
|   |  - manager.db (WAL)   |     |    (UFW / IPTables / nsenter)     |   |
|   |  - metrics.db (WAL)   |     +-----------------+-----------------+   |
|   +-----------------------+                       |                     |
|                                                   | Allow/Revoke UDP    |
|   +-----------------------+                       v                     |
|   |  Docker Orchestrator  |-----> +---------------------------------+   |
|   |  - itzg/bds Container |       |     Host Linux UDP Firewall     |   |
|   |  - CPU / RAM Caps     |       | (Ports 19132-19142 Only Allowed |   |
|   +-----------------------+       |   For Active Client Leases)     |   |
|                                   +---------------------------------+   |
+-------------------------------------------------------------------------+
```

### Dynamic Port-Knocking Flow
1. **Initial Closed State**: UDP port 19132 is blocked by host firewall rules.
2. **Knock Request**: Player visits `https://bsm.example.com/knock/<server_id>` and enters credentials.
3. **Grant & Cookie**: Server validates key via constant-time HMAC-SHA256, issues an `AllowPort(clientIP, 19132)` rule, and sets an `HttpOnly` signed session cookie.
4. **Direct Launch**: The browser displays a 1-tap `minecraft://?addExternalServer=...` link that directly opens the Minecraft client.
5. **Mobile Roaming**: If the player moves to another cellular cell tower, their browser heartbeat automatically sends `POST /api/knock/:id/heartbeat`. BSM detects the IP change, immediately deletes the old rule, and opens the new IP.
6. **Lease Auditor**: A background goroutine audits leases every 15 seconds, purging expired grants from both the database and the firewall.

---

## 🛠️ Development & Testing

### Prerequisites
- Go 1.26+
- Node.js 20+ & npm
- Docker daemon (optional; will gracefully fall back to mock engine for local non-Docker development)

### Running Tests
Execute the full test suite across all packages:
```bash
go test -v ./...
```

### Building Binary & Makefile Commands

BSM provides convenient build scripts and Makefile shortcuts:

```bash
# Compile web assets and binary
make build
# or: ./scripts/build.sh

# Run standalone binary directly
make run
# or: ./bin/bedrock-server-manager

# Install as systemd service on Linux host
sudo make install
# or: sudo ./install.sh

# Update from git, recompile, and restart service
sudo make update
# or: sudo ./scripts/update.sh
```

---

## 📄 License
Licensed under the [MIT License](LICENSE).
