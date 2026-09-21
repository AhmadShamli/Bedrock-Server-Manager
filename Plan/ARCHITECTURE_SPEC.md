# Bedrock Server Manager - Architecture & Technical Specification

## 1. Executive Summary

Bedrock Server Manager is a lightweight, self-hosted management platform for Minecraft Bedrock Dedicated Servers (BDS). It is designed to run seamlessly either on **bare-metal Linux hosts** (as a single compiled Go binary) or inside **Docker environments** (managing sibling server containers via Docker socket).

---

## 2. Core Decisions & Tech Stack

| Domain | Technology / Design Choice | Rationale |
| :--- | :--- | :--- |
| **Backend Language** | **Go (Golang 1.22+)** | Zero-dependency static compilation, low RAM/CPU footprint, strong concurrency model for process and WebSocket handling. |
| **Frontend Framework** | **React 18 + Vite + Tailwind CSS + Lucide Icons** | Fast, responsive SPA with clean modern UI; built and embedded directly into the Go binary using `embed.FS`. |
| **Data Persistence** | **Embedded SQLite (`modernc.org/sqlite`)** | Zero configuration, single-file storage (`data/manager.db`), ACID compliance, and automatic schema migrations without requiring CGO. |
| **Authentication** | **JWT (JSON Web Tokens) with Argon2id Password Hashing** | Role-based access control (RBAC) with `Admin` (full system access) and `Server Operator` (per-instance control) roles. |
| **Real-time Comms** | **WebSockets (`gorilla/websocket` or `coder/websocket`)** | Low-latency bi-directional communication for interactive BDS console streams, real-time player events, and system metrics. |
| **Terminal Emulator** | **xterm.js + fit-addon** | Browser-based interactive console with 1,000-line ring buffer, ANSI coloring, and command history. |

---

## 3. Dual Execution Engine Architecture

The platform abstracts server lifecycle through a unified `ServerDriver` interface:

```mermaid
flowchart TD
    API["Manager API & Supervisor Core"] --> Driver{"Engine Selector"}
    Driver -->|"Bare-Metal Mode"| ProcessDriver["ProcessDriver (Native Child Process)"]
    Driver -->|"Docker Mode"| DockerDriver["DockerDriver (Docker Socket Engine API)"]

    ProcessDriver --> BDS_Proc["bedrock_server binary (LD_LIBRARY_PATH=.)\nIsolated folder: ./servers/{id}"]
    DockerDriver --> BDS_Cont["itzg/minecraft-bedrock-server Container\nVolume: data/servers/{id}:/data"]
```

### 3.1. Bare-Metal Driver (`ProcessDriver`)
- Runs `bedrock_server` directly on the host operating system.
- Manages Unix signals (`SIGINT`, `SIGTERM`), standard I/O pipes (`stdin`, `stdout`, `stderr`).
- Enforces proper environment variables (e.g. `LD_LIBRARY_PATH=.`).
- Isolates server files in dedicated subdirectories: `./servers/{server_id}/`.
- Downloads official Bedrock zip packages directly from Mojang download servers.

### 3.2. Docker Driver (`DockerDriver`)
- Connects to `/var/run/docker.sock` using the official Docker Go SDK.
- Launches and manages isolated containers based on `itzg/minecraft-bedrock-server`.
- Automatically maps UDP ports and binds persistent host volumes for configuration and world persistence.
- Streams container stdout/stderr and attaches to container stdin for real-time console interaction.

---

## 4. Bedrock Server Management & Lifecycle

### 4.1. Server Installation & Updates
- **Bare-metal**: Automatic scraping/fetching of official BDS Linux zips from Mojang with version selector (`latest`, `preview`, or pin to specific version). Updates extract server binaries while preserving `server.properties`, `allowlist.json`, `permissions.json`, and the `worlds/` directory.
- **Docker**: Version tags mapped to `itzg/minecraft-bedrock-server` (e.g. `VERSION=LATEST`, `VERSION=PREVIEW`, or specific BDS version string).

### 4.2. Configuration Management (Configuration-Only Editor)
- Visual form editors and structured JSON/properties editors restricted to designated server files:
  - `server.properties` (Gamemode, difficulty, max players, allow-cheats, level-seed, tick-distance, etc.).
  - `allowlist.json` (`[{"name": "...", "xuid": "...", "ignoresPlayerLimit": false}]`).
  - `permissions.json` (`[{"permission": "operator"|"member"|"visitor", "xuid": "..."}]`).
- Arbitrary file browsing is disabled by design to eliminate path traversal vulnerabilities and prevent accidental file deletion.

### 4.3. Player Hub
- **Live Connection Tracking**: Parses stdout logs (`Player connected: <name>, xuid: <xuid>` / `Player disconnected`) and maintains active session list.
- **Gamertag & XUID Synchronization**: Automatically resolves and stores player XUIDs for allowlist and operator permissions.
- **Quick Player Actions**: Kick, ban, teleport, change permission level (`operator`, `member`, `visitor`), and broadcast in-game messages.

### 4.4. Zero-Downtime Hot Backups & World Management
- **Hot Backup Protocol**:
  1. Sends `save hold` to server stdin.
  2. Polls `save query` until BDS reports files are ready for copying.
  3. Copies snapshot files and world data to a compressed zip archive in `data/backups/{server_id}/`.
  4. Issues `save resume` to resume disk writes without stopping gameplay.
- **World Management**: Import and export `.mcworld` or `.zip` archives, reset world, seed customization.
- **Automated Scheduling**: Cron-based triggers for recurring backups and scheduled restarts.

### 4.5. Addons & Packs (Vanilla BDS Focus)
- Dedicated support for official Vanilla Bedrock Dedicated Server (BDS).
- Upload and install `.mcpack` and `.mcaddon` archives into `behavior_packs` and `resource_packs`.
- Automatic extraction and registration in `world_behavior_packs.json` and `world_resource_packs.json`.

---

## 5. Reliability, Networking & Health Monitoring

### 5.1. Crash Loop Detection & Resource Management
- **Circuit Breaker**: Auto-restart on unexpected exit with exponential backoff; halts auto-restart if 5 crashes occur within a 5-minute window.
- **Resource Constraints**: Configurable per-server RAM and CPU limits (enforced via Docker container flags in Docker mode, or process monitoring alerts on bare-metal).

### 5.2. Networking & RakNet Ping
- **Port Allocation**: Automatic suggestion and assignment of unused UDP ports starting at `19132` (IPv4) and `19133` (IPv6), with collision checks against host interfaces and existing servers.
- **RakNet UDP Ping Poller**: Periodically sends Bedrock Unconnected Ping packets to verify server responsiveness, retrieve real-time MOTD, latency, player count, and version.

---

## 6. Notifications, Webhooks & Onboarding

### 6.1. Discord Webhooks & Notification Center
- Per-server and global Discord webhook notifications for:
  - Server crash alerts (with crash log snippet).
  - Server start / stop status.
  - Player join / leave events.
  - Hot backup completion / failure.
- In-app notification center / toast alerts for real-time dashboard feedback.

### 6.2. First-Launch Setup Wizard
- Automatically detects fresh installation (empty database).
- Directs initial browser request to an onboarding wizard at `/setup`:
  - Create primary Admin username & password (hashed with Argon2id).
  - Set manager title / branding.
  - Configure optional global Discord webhook.
  - Generates secure cryptographic JWT signing secret.
- Supports optional environment variable overrides (`ADMIN_USER`, `ADMIN_PASSWORD`, `JWT_SECRET`) for headless/automated infrastructure deployments.

---

## 7. Directory Layout

```text
Bedrock-Server-Manager/
├── cmd/
│   └── manager/
│       └── main.go               # Application entrypoint
├── internal/
│   ├── api/                      # REST & WebSocket HTTP routes and middleware
│   │   ├── auth.go
│   │   ├── servers.go
│   │   ├── players.go
│   │   ├── backups.go
│   │   ├── console_ws.go
│   │   ├── webhooks.go
│   │   └── router.go
│   ├── config/                   # Global configuration loading (env, file)
│   ├── database/                 # SQLite setup and migrations
│   │   ├── db.go
│   │   └── models.go
│   ├── driver/                   # Server execution drivers
│   │   ├── driver.go             # ServerDriver interface
│   │   ├── process_driver.go     # Bare-metal child process supervisor
│   │   └── docker_driver.go      # Docker SDK container driver
│   ├── raknet/                   # Bedrock UDP ping client
│   ├── backup/                   # Hot backup and world export engine
│   ├── webhook/                  # Discord webhook dispatcher
│   └── downloader/               # Mojang BDS zip downloader and updater
├── web/                          # Embedded Frontend (React + Vite + Tailwind)
│   ├── src/
│   │   ├── components/
│   │   ├── pages/
│   │   └── services/
│   ├── index.html
│   ├── package.json
│   └── vite.config.ts
├── Plan/                         # Project plan and specifications
│   ├── ARCHITECTURE_SPEC.md
│   └── IMPLEMENTATION_PLAN.md
├── Dockerfile                    # Multi-stage Docker build
├── docker-compose.yml            # Out-of-the-box compose setup
├── go.mod
└── go.sum
```

