package database

// ManagerSchemaSQL contains table definitions and indices for the primary metadata database.
const ManagerSchemaSQL = `
CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT 'admin',
    created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);

CREATE TABLE IF NOT EXISTS user_server_access (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    server_id TEXT NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    created_at TEXT NOT NULL,
    UNIQUE(user_id, server_id)
);
CREATE INDEX IF NOT EXISTS idx_user_server_access_user ON user_server_access(user_id);
CREATE INDEX IF NOT EXISTS idx_user_server_access_server ON user_server_access(server_id);

CREATE TABLE IF NOT EXISTS servers (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    version TEXT NOT NULL DEFAULT 'latest',
    port INTEGER NOT NULL DEFAULT 19132,
    portv6 INTEGER NOT NULL DEFAULT 19133,
    status TEXT NOT NULL DEFAULT 'stopped',
    mode TEXT NOT NULL DEFAULT 'survival',
    difficulty TEXT NOT NULL DEFAULT 'normal',
    autostart_on_boot INTEGER NOT NULL DEFAULT 0,
    port_gate_enabled INTEGER NOT NULL DEFAULT 0,
    port_gate_mode TEXT NOT NULL DEFAULT 'gamertag',
    port_gate_timeout INTEGER NOT NULL DEFAULT 7200,
    memory_limit TEXT NOT NULL DEFAULT '2G',
    cpu_limit REAL NOT NULL DEFAULT 2.0,
    container_id TEXT NOT NULL DEFAULT '',
    seed TEXT NOT NULL DEFAULT '',
    game_server_address TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_servers_status ON servers(status);

CREATE TABLE IF NOT EXISTS port_gate_keys (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id TEXT NULL REFERENCES servers(id) ON DELETE CASCADE,
    label TEXT NOT NULL,
    key_hash TEXT NOT NULL,
    key_prefix TEXT NOT NULL DEFAULT '',
    max_uses INTEGER NOT NULL DEFAULT 0,
    used_count INTEGER NOT NULL DEFAULT 0,
    lease_duration_seconds INTEGER NOT NULL DEFAULT 0,
    expires_at TEXT NULL,
    is_active INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_port_gate_keys_server ON port_gate_keys(server_id, is_active);

CREATE TABLE IF NOT EXISTS port_gate_leases (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id TEXT NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    key_id INTEGER NULL REFERENCES port_gate_keys(id) ON DELETE SET NULL,
    ip_address TEXT NOT NULL,
    gamertag TEXT NOT NULL DEFAULT '',
    knock_method TEXT NOT NULL DEFAULT 'passphrase',
    session_token_hash TEXT NOT NULL DEFAULT '',
    granted_at TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    comment TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'active'
);
CREATE INDEX IF NOT EXISTS idx_port_gate_leases_active ON port_gate_leases(server_id, status, expires_at);
CREATE INDEX IF NOT EXISTS idx_port_gate_leases_ip ON port_gate_leases(ip_address, status);
CREATE INDEX IF NOT EXISTS idx_port_gate_leases_token ON port_gate_leases(session_token_hash);

CREATE TABLE IF NOT EXISTS port_gate_allowlist (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id TEXT NULL REFERENCES servers(id) ON DELETE CASCADE,
    ip_or_subnet TEXT NOT NULL,
    comment TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_port_gate_allowlist_server ON port_gate_allowlist(server_id);
CREATE INDEX IF NOT EXISTS idx_port_gate_allowlist_ip ON port_gate_allowlist(ip_or_subnet);

CREATE TABLE IF NOT EXISTS port_gate_bans (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id TEXT NULL REFERENCES servers(id) ON DELETE CASCADE,
    ip_or_subnet TEXT NOT NULL,
    reason TEXT NOT NULL DEFAULT '',
    banned_by TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_port_gate_bans_server ON port_gate_bans(server_id);
CREATE INDEX IF NOT EXISTS idx_port_gate_bans_ip ON port_gate_bans(ip_or_subnet);

CREATE TABLE IF NOT EXISTS backups (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id TEXT NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    filename TEXT NOT NULL,
    size_bytes INTEGER NOT NULL DEFAULT 0,
    type TEXT NOT NULL DEFAULT 'manual',
    is_locked INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'completed',
    created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_backups_server ON backups(server_id, created_at);

CREATE TABLE IF NOT EXISTS tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id TEXT NULL REFERENCES servers(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    cron_expr TEXT NOT NULL,
    action TEXT NOT NULL,
    payload TEXT NOT NULL DEFAULT '',
    last_run TEXT NULL,
    next_run TEXT NULL,
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_tasks_server ON tasks(server_id, enabled);

CREATE TABLE IF NOT EXISTS audit_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NULL REFERENCES users(id) ON DELETE SET NULL,
    actor_type TEXT NOT NULL DEFAULT 'user',
    actor_name TEXT NOT NULL DEFAULT '',
    action TEXT NOT NULL,
    target TEXT NOT NULL DEFAULT '',
    details TEXT NOT NULL DEFAULT '{}',
    client_ip TEXT NOT NULL DEFAULT '',
    timestamp TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_audit_logs_timestamp ON audit_logs(timestamp);
CREATE INDEX IF NOT EXISTS idx_audit_logs_action ON audit_logs(action);

CREATE TABLE IF NOT EXISTS system_settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS global_players (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    xuid TEXT NOT NULL DEFAULT '',
    is_allowlisted INTEGER NOT NULL DEFAULT 1,
    permission TEXT NOT NULL DEFAULT 'member',
    ignores_player_limit INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE(name)
);
CREATE INDEX IF NOT EXISTS idx_global_players_xuid ON global_players(xuid);
CREATE INDEX IF NOT EXISTS idx_global_players_allowlisted ON global_players(is_allowlisted);
CREATE INDEX IF NOT EXISTS idx_global_players_permission ON global_players(permission);

CREATE TABLE IF NOT EXISTS banned_players (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id TEXT NULL,
    gamertag TEXT NOT NULL,
    xuid TEXT NOT NULL DEFAULT '',
    reason TEXT NOT NULL DEFAULT '',
    banned_by TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_banned_players_server ON banned_players(server_id);
CREATE INDEX IF NOT EXISTS idx_banned_players_gamertag ON banned_players(gamertag);
CREATE INDEX IF NOT EXISTS idx_banned_players_xuid ON banned_players(xuid);
`

// MetricsSchemaSQL contains table definitions and indices for the high-frequency telemetry database.
const MetricsSchemaSQL = `
CREATE TABLE IF NOT EXISTS metrics_raw (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id TEXT NOT NULL,
    timestamp TEXT NOT NULL,
    cpu_percent REAL NOT NULL,
    ram_bytes INTEGER NOT NULL,
    player_count INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_metrics_raw_server_ts ON metrics_raw(server_id, timestamp);

CREATE TABLE IF NOT EXISTS metrics_5m (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id TEXT NOT NULL,
    timestamp TEXT NOT NULL,
    avg_cpu REAL NOT NULL,
    max_cpu REAL NOT NULL,
    min_cpu REAL NOT NULL,
    avg_ram INTEGER NOT NULL,
    max_ram INTEGER NOT NULL,
    min_ram INTEGER NOT NULL,
    avg_players REAL NOT NULL,
    max_players INTEGER NOT NULL,
    sample_count INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_metrics_5m_server_ts ON metrics_5m(server_id, timestamp);

CREATE TABLE IF NOT EXISTS metrics_1h (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id TEXT NOT NULL,
    timestamp TEXT NOT NULL,
    avg_cpu REAL NOT NULL,
    max_cpu REAL NOT NULL,
    min_cpu REAL NOT NULL,
    avg_ram INTEGER NOT NULL,
    max_ram INTEGER NOT NULL,
    min_ram INTEGER NOT NULL,
    avg_players REAL NOT NULL,
    max_players INTEGER NOT NULL,
    sample_count INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_metrics_1h_server_ts ON metrics_1h(server_id, timestamp);
`
