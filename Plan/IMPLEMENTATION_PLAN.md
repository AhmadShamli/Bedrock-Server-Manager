# Bedrock Server Manager - Implementation Plan

This implementation plan outlines the phased development roadmap for Bedrock Server Manager based on the agreed [ARCHITECTURE_SPEC.md](file:///workspace/Bedrock-Server-Manager/Plan/ARCHITECTURE_SPEC.md).

---

## Phase 1: Project Scaffolding & Foundation
- [ ] Initialize Go module (`go.mod`).
- [ ] Set up project directory structure (`cmd/`, `internal/`, `web/`, `Plan/`).
- [ ] Implement embedded SQLite database (`internal/database`) with schema migrations:
  - Users table (id, username, password_hash, role, created_at)
  - Servers table (id, name, version, port, portv6, status, mode, created_at, memory_limit, cpu_limit)
  - Backups table (id, server_id, filename, size, type, status, created_at)
  - AuditLogs table (id, user_id, action, details, timestamp)
- [ ] Implement JWT Authentication & User management (`internal/auth`, `internal/api/auth.go`):
  - Password hashing via Argon2id
  - Login / refresh / me endpoints
  - RBAC middleware (`Admin` vs `Server Operator`)
  - Initial setup wizard endpoint & check (`/api/setup`) with automatic JWT secret generation and optional ENV override

---

## Phase 2: Core Server Engine & Drivers
- [ ] Define the `ServerDriver` interface in `internal/driver/driver.go`:
  - `Start(server *models.Server) error`
  - `Stop(server *models.Server, gracefulTimeout time.Duration) error`
  - `Restart(server *models.Server) error`
  - `GetStatus(server *models.Server) (ServerStatus, error)`
  - `WriteStdin(server *models.Server, command string) error`
  - `SubscribeLogs(server *models.Server) (<-chan LogMessage, func())`
  - `GetMetrics(server *models.Server) (SystemMetrics, error)`
- [ ] Implement `ProcessDriver` (Bare-metal Linux native supervisor):
  - Manages `bedrock_server` child process with `LD_LIBRARY_PATH=.`
  - Ring buffer (1,000 lines) for stdout/stderr streaming
  - Graceful stop via `stop` command before fallback to `SIGTERM`/`SIGKILL`
  - Crash-loop circuit breaker (stops auto-restarting after 5 crashes in 5 mins) and exponential backoff
- [ ] Implement `DockerDriver` (Docker API container driver):
  - Uses `github.com/docker/docker/client` to interact with `/var/run/docker.sock`
  - Manages container lifecycle using `itzg/minecraft-bedrock-server`
  - Container volume binding to `data/servers/{id}:/data`
  - Container stdout/stdin streaming and memory/CPU limits
- [ ] Implement Port Allocator:
  - Scans for free UDP ports starting at 19132 (IPv4) / 19133 (IPv6)
  - Validates against active network listeners and registered servers

---

## Phase 3: Bedrock Installation, Configuration & Player Hub
- [ ] BDS Downloader & Version Selector (`internal/downloader`):
  - Queries and downloads official Mojang BDS Linux archives
  - Unzips and installs into server instance directory
  - Preserves user configs (`server.properties`, `allowlist.json`, `permissions.json`, `worlds/`) on update
- [ ] Configuration-Only Editor:
  - Type-safe reader/writer for `server.properties`
  - JSON sync for `allowlist.json` and `permissions.json`
  - Path-traversal proof security (no arbitrary filesystem browsing)
- [ ] Player Hub Engine (`internal/player`):
  - Real-time player detection via stdout log parsing
  - RakNet UDP Ping poller (`internal/raknet`) for latency, MOTD, and online player counts
  - Player quick actions (`kick`, `ban`, `op`, `deop`, `teleport`, `say`)
  - XUID / Gamertag resolution and persistence
- [ ] Discord Webhook Dispatcher (`internal/webhook`):
  - Crash alerts with log context
  - Start/stop notifications
  - Player join/leave broadcasts
  - Backup result notifications

---

## Phase 4: Hot Backups, World Management & Addons
- [ ] Hot Backup Engine (`internal/backup`):
  - Bedrock `save hold` -> `save query` -> snapshot file copy -> `save resume` workflow
  - Zip compression of world state without taking server offline
  - Backup restore and download endpoints
  - Scheduled backup cron (`robfig/cron/v3`)
- [ ] World Import / Export:
  - Upload `.mcworld` or `.zip` and extract into `worlds/`
  - Export current world as downloadable `.mcworld`
- [ ] Addon / Pack Manager:
  - Upload and extract `.mcpack` / `.mcaddon` into `behavior_packs` / `resource_packs`
  - Update `world_behavior_packs.json` and `world_resource_packs.json`

---

## Phase 5: Modern Web Dashboard (React + Vite + Tailwind)
- [ ] Frontend setup in `web/`:
  - Vite + React + TypeScript + Tailwind CSS + Lucide Icons + React Query
- [ ] Core Views:
  - **Setup Wizard**: Onboarding screen for new installations (`/setup`)
  - **Dashboard / Server List**: Cards displaying live server status, players online, CPU/RAM, quick start/stop
  - **Interactive Terminal**: `xterm.js` console with real-time WebSocket log streaming, ANSI coloring, auto-scroll, command history, and quick command buttons
  - **Player Hub**: Active player list, Allowlist management, Operator management, kick/ban/op modals
  - **Server Settings**: GUI form editor for `server.properties`, `allowlist.json`, and `permissions.json`
  - **Backups & Worlds**: Hot backup triggers, backup history, restore, world upload/export
  - **Addon Manager**: Drag-and-drop `.mcpack`/`.mcaddon` installer
  - **Notifications & Webhooks**: Discord webhook configuration and in-app toast alerts
  - **User & Role Management**: Admin user creation and role assignments
- [ ] Embedding:
  - Embed Vite production `dist/` into Go binary via `//go:embed all:dist`

---

## Phase 6: Packaging, Docker & Verification
- [ ] Multi-stage `Dockerfile`:
  - Stage 1: Build React frontend (`npm run build`)
  - Stage 2: Compile Go static binary (`CGO_ENABLED=0 go build`)
  - Stage 3: Minimal runtime image (Ubuntu-based or Debian slim with `libcurl4` and required libraries for BDS on Linux, plus Docker CLI/socket support)
- [ ] `docker-compose.yml` with `/var/run/docker.sock` and volume mount setup.
- [ ] Automated tests (unit tests for drivers, config parsers, RakNet ping, backup workflow).
