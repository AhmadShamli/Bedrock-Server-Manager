# Bedrock Server Manager - Implementation Plan

This implementation plan outlines the phased development roadmap for Bedrock Server Manager based on the agreed [ARCHITECTURE_SPEC.md](file:///workspace/Bedrock-Server-Manager/Plan/ARCHITECTURE_SPEC.md).

---

## Phase 1: Project Scaffolding & Foundation
- [x] Initialize Go module (`go.mod`).
- [x] Set up project directory structure (`cmd/`, `internal/`, `web/`, `Plan/`).
- [x] Implement dual embedded SQLite databases (`internal/database`) with schema migrations:
  - **Primary DB (`data/manager.db`)**:
    - Users table (id, username, password_hash, role, created_at)
    - UserServerAccess table (user_id, server_id) for granular per-server Operator access control
    - Servers table (id, name, version, port, portv6, status, mode, autostart_on_boot, port_gate_enabled, port_gate_mode, port_gate_timeout, created_at, memory_limit, cpu_limit)
    - PortGateKeys table (id, server_id, label, key_hash, key_prefix, max_uses, used_count, lease_duration_seconds, expires_at, is_active, created_at)
    - PortGateLeases table (id, server_id, key_id, ip_address, gamertag, knock_method, granted_at, expires_at, comment)
    - Backups table (id, server_id, filename, size, type, status, created_at)
    - Tasks table (id, server_id, cron_expr, action, payload, last_run, next_run, enabled)
    - AuditLogs table (id, user_id, action, details, timestamp)
  - **Telemetry DB (`data/metrics.db`)**:
    - SQLite configured in WAL mode (`PRAGMA journal_mode=WAL`)
    - Configurable collection interval (Admin setting, default: 2 seconds)
    - In-memory collection buffer flushed in transactions every 30 seconds
    - `metrics_raw` table (server_id, timestamp, cpu_percent, ram_bytes, player_count)
    - `metrics_5m` rollup table (server_id, timestamp, avg/max/min cpu, ram, player_count)
    - `metrics_1h` rollup table (server_id, timestamp, avg/max/min cpu, ram, player_count)
    - Background rollup worker (aggregates raw into 5m every 5 minutes, 5m into 1h every hour)
    - Tiered data retention pruner (raw: 6h, 5m: 7d, 1h: 30d)
- [x] Implement JWT Authentication & User management (`internal/auth`, `internal/api/auth.go`):
  - Password hashing via bcrypt (matching `../Funnel`)
  - Keyed HMAC-SHA256 with constant-time verification for knock passphrases (<0.01ms CPU, 0 MB RAM)
  - Login / refresh / me endpoints with 24-hour sliding session and 30-day "Remember Me"
  - RBAC middleware (`Admin` vs `Server Operator` with per-server instance authorization)
  - Strict internal-only API scope (no remote API keys / bot tokens)
  - Two-tier rate limiter (`internal/auth/ratelimit.go`, matching `../Funnel`): Tier 1 per-IP progressive backoff (30s–5m after 5 failures) + Tier 2 distributed circuit breaker (10+ distinct failed IPs trips 15m global pause)
  - Initial setup wizard endpoint & check (`/api/setup`) with automatic JWT secret generation and optional ENV override
  - Configurable server binding via `PORT` (default: 8080) and `DATA_DIR` (default: `data`) with reverse-proxy header support (`X-Forwarded-For`, `X-Forwarded-Proto`)

---

## Phase 2: Docker Server Orchestrator & Port Allocator
- [x] Implement `DockerEngine` (`internal/engine/docker.go`) using `github.com/docker/docker/client`:
  - Connect to `/var/run/docker.sock` or `DOCKER_HOST`
  - Automated image management (`itzg/minecraft-bedrock-server` version pulling)
  - Container creation with strict hardware resource capping:
    - Memory & swap limits (`Resources.Memory`, `Resources.MemorySwap`)
    - CPU quota & limit (`Resources.NanoCPUs`)
  - Volume binding: maps host `./data/servers/{id}` into container `/data`
  - Container lifecycle: `Start`, `Stop` (graceful BDS stop with timeout fallback), `Restart`, `Remove`
  - Boot Manager: queries servers with `autostart_on_boot = true` on daemon launch and starts them cleanly
  - Live console streaming via `ContainerAttach` (stdin/stdout WebSockets with 1,000-line ring buffer)
  - Real-time hardware telemetry sampling via `ContainerStats` (CPU %, RAM RSS/usage, network I/O)
  - Crash-loop circuit breaker (stops auto-restarting if 5 crashes occur within 5 mins) and exponential backoff
- [x] Implement Port Allocator (`internal/allocator/port.go`):
  - Scans for free UDP ports starting at 19132 (IPv4) / 19133 (IPv6)
  - Validates against active Docker port bindings, host listeners, and registered servers

---

## Phase 3: Bedrock Installation, Configuration, Cloning & Player Hub
- [x] BDS Version Checker & 1-Click Update (`internal/updater`):
  - Queries upstream Bedrock release metadata
  - Displays update banner when a newer version is available
  - 1-click update with automated pre-upgrade safety backup
- [x] Server Creation Presets (`internal/preset`):
  - Preset templates (Vanilla Survival, Creative Building, Hardcore Challenge, Custom)
- [x] Server Cloning, Full Export & Safe Deletion:
  - 1-click Server Cloning (copies data/configs and auto-allocates free UDP port)
  - Full Server Export (.zip bundle containing world, configs, and packs for migration)
  - Safe Deletion (requires matching server name verification, optional pre-deletion archive, and audit logging)
- [x] Configuration-Only Editor:
  - Type-safe reader/writer for `server.properties`
  - JSON sync for `allowlist.json` and `permissions.json`
  - Path-traversal proof security (no arbitrary filesystem browsing)
- [x] Player Hub Engine (`internal/player`):
  - Real-time player detection via stdout log parsing
  - Dedicated in-game chat feed parser (streams player chat into isolated chat UI feed)
  - Broadcaster API: send global styled alerts (`say`/`tellraw`) or private direct messages (`tell`)
  - RakNet UDP Ping poller (`internal/raknet`) for latency, MOTD, and online player counts
  - Player quick actions (`kick`, `ban`, `op`, `deop`, `teleport`, `say`)
  - XUID / Gamertag resolution and persistence
- [x] Discord Webhook Dispatcher (`internal/webhook`):
  - Crash alerts with log context
  - Start/stop notifications
  - Player join/leave broadcasts
  - Backup result notifications
- [x] Pluggable Dynamic Firewall Engine (`internal/firewall`, referenced from `../Funnel`):
  - `NetNSDetector` & `nsenter` wrapper (`ShouldUseNsenter()`, `WrapCommand()`) for host firewall execution from Docker
  - `FirewallDriver` interface (`AllowIP(ip, port, comment)`, `RevokeIP(ip, port, comment)`, `ListRules()`)
  - `UFWDriver` (default, executes `ufw allow proto udp from ...`)
  - `IPTablesDriver` (manages dedicated `BSM_PORT_GATE` chain)
  - `CustomScriptDriver` (configurable shell hooks for custom environments)
  - Public Port-Knock handler (`/api/knock/:id` with Gamertag / Passphrase validation)
  - Mobile Roaming & Heartbeat API (`/api/knock/:id/heartbeat`): configurable interval (default: 10 seconds), detects IP changes via signed session cookie and hot-swaps firewall rules seamlessly
  - Background Lease Auditor (purges expired IP rules every 15s)
- [x] Trusted Proxy Client IP Resolver (`internal/ipresolver`, referenced from `../Funnel`):
  - Supports `direct`, `reverse_proxy`, and `cloudflare` modes
  - Parses `CF-Connecting-IP`, RFC 7239 `Forwarded`, `X-Forwarded-For` (right-to-left untrusted parsing), `X-Real-IP`
  - Dynamic Cloudflare IP prefix fetcher and trusted CIDR verification

---

## Phase 4: Hot Backups, World Management, Retention & Lifecycle
- [x] Hot Backup Engine (`internal/backup`):
  - Bedrock `save hold` -> `save query` -> snapshot file copy -> `save resume` workflow
  - Zip compression of world state without taking server offline
  - Backup restore and download endpoints
  - Scheduled backup cron (`robfig/cron/v3`)
- [x] Comprehensive Backup Retention & Disk Quotas:
  - Count retention (retain last N backups, default: 10)
  - Age retention (purge unpinned backups older than X days, default: 14 days)
  - Disk quota enforcement (purge oldest unpinned backups when quota exceeded)
  - Pin / Lock protection flag for milestone backups
- [x] World Import / Export:
  - Upload `.mcworld` or `.zip` and extract into `worlds/`
  - Export current world as downloadable `.mcworld`
- [x] Addon / Pack Manager:
  - Upload and extract `.mcpack` / `.mcaddon` into `behavior_packs` / `resource_packs`
  - Automatic manifest parsing, UUID extraction, and directory placement
- [x] Shutdown & Restart Workflows:
  - Graceful stop: countdown broadcast (`say ...`), disconnect active players with maintenance reason, issue `stop`, await LevelDB flush
  - Direct BDS stop: instant `stop` to stdin without broadcasts
  - Emergency force kill: immediate container termination fallback

---

## Phase 5: Modern Web Dashboard (React + Vite + Tailwind)
- [x] Frontend setup in `web/`:
  - Vite + React + TypeScript + Tailwind CSS + Lucide Icons + React Query
  - **Dark Mode Only UI**: Sleek obsidian/cyberpunk aesthetic with Minecraft emerald accents, optimized for desktop and mobile devices
- [x] Core Views:
  - **Setup Wizard**: Onboarding screen for new installations (`/setup`)
  - **Dashboard / Server List**: Cards displaying live server status, players online, CPU/RAM, quick start/stop
  - **Shutdown Action Modal**: Admin choice between Graceful Stop (with countdown) and Direct Stop
  - **Interactive Terminal**: Real-time WebSocket log streaming, ANSI coloring, auto-scroll, command history, and quick command buttons
  - **Player Hub & Live Chat**: Active player list, Allowlist editor, Operator editor, kick/ban/op modals, real-time in-game chat feed panel, and broadcast/DM modal
  - **Port Gate & IP Leases**: Dynamic firewall toggle, active IP leases table with countdowns, manual IP allowlist modal, QR code generator, and shareable link
  - **Player Knock Portal (`/knock/:id`)**: Lightweight public web portal for players to enter Gamertag/passphrase, unlock their IP address, background heartbeat (configurable, default: 10s) for mobile 4G/5G roaming, 1-tap IP reconnect button, and direct game launch via `minecraft://?addExternalServer=<Name>|<Host>:<Port>`
  - **Server Settings**: GUI form editor for `server.properties`, `allowlist.json`, and `permissions.json`
  - **Backups & Worlds**: Hot backup triggers, retention settings, pin/lock toggle, restore, world upload/export
  - **Server Cloning & Export**: 1-click Clone modal and Full Server Export download button
  - **Addon Manager**: Drag-and-drop `.mcpack`/`.mcaddon` installer
  - **Audit Log Viewer**: Dedicated audit dashboard with user/action/server filtering, date picker, and CSV/JSON export
  - **Notifications & Webhooks**: Discord webhook configuration and in-app toast alerts
  - **Task Scheduler**: Cron routines for automated hot backups, graceful server restarts with player countdowns, and scheduled commands
  - **User & Role Management**: Admin user creation, per-server Operator access assignments
- [x] Embedding:
  - Embed Vite production `dist/` into Go binary via `//go:embed all:dist`

---

## Phase 6: Packaging, Docker & Verification
- [x] Multi-stage `Dockerfile`:
  - Stage 1: Build React frontend (`npm run build`)
  - Stage 2: Compile Go static binary (`CGO_ENABLED=0 go build`)
  - Stage 3: Minimal runtime image (Debian bookworm-slim with `iptables`, `iproute2`, `ufw`, `util-linux` [nsenter], `tini`, `ca-certificates`)
- [x] `docker-compose.yml` (referencing `../Funnel` patterns):
  - Mounts `/var/run/docker.sock` for Minecraft container orchestration
  - Configures `pid: host` and `cap_add: [SYS_ADMIN, NET_ADMIN]` for transparent host firewall manipulation via `nsenter`
  - Persistent volume binding for `data/`
- [x] Automated tests (unit tests for drivers, config parsers, RakNet ping, backup workflow, scheduler, addon manager).

