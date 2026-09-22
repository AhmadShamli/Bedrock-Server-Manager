export interface User {
  id: number;
  username: string;
  role: 'admin' | 'operator';
  created_at: string;
}

export interface Server {
  id: string;
  name: string;
  version: string;
  port: number;
  portv6: number;
  status: 'stopped' | 'starting' | 'running' | 'stopping' | 'crashed';
  mode: string;
  difficulty: string;
  autostart_on_boot: boolean;
  port_gate_enabled: boolean;
  port_gate_mode: 'gamertag' | 'passphrase' | 'combined';
  port_gate_timeout: number;
  memory_limit: string;
  cpu_limit: number;
  container_id?: string;
  created_at: string;
  updated_at: string;
}

export interface PortGateKey {
  id: number;
  server_id?: string;
  label: string;
  key_prefix: string;
  max_uses: number;
  used_count: number;
  lease_duration_seconds: number;
  expires_at?: string;
  is_active: boolean;
  created_at: string;
  plaintext_passphrase?: string;
}

export interface PortGateLease {
  id: number;
  server_id: string;
  key_id?: number;
  ip_address: string;
  gamertag?: string;
  knock_method: string;
  granted_at: string;
  expires_at: string;
  comment: string;
  status: 'active' | 'expired' | 'revoked';
}

export interface KnockConfig {
  server_id: string;
  server_name: string;
  port: number;
  port_gate_enabled: boolean;
  port_gate_mode: 'gamertag' | 'passphrase' | 'combined';
  heartbeat_interval_seconds: number;
}

export interface Backup {
  id: number;
  server_id: string;
  filename: string;
  size_bytes: number;
  type: string;
  is_locked: boolean;
  status: string;
  created_at: string;
}

export interface Task {
  id: number;
  server_id?: string;
  name: string;
  cron_expr: string;
  action: string;
  payload: string;
  last_run?: string;
  next_run?: string;
  enabled: boolean;
  created_at: string;
}

export interface AddonPack {
  type: 'behavior' | 'resource';
  folder: string;
  name: string;
  description: string;
  uuid: string;
  version: string;
}

export interface AuditLog {
  id: number;
  user_id?: number;
  actor_type: string;
  actor_name: string;
  action: string;
  target: string;
  details: string;
  client_ip: string;
  timestamp: string;
}

export interface GlobalPlayer {
  id: number;
  name: string;
  xuid: string;
  is_allowlisted: boolean;
  permission: 'operator' | 'member' | 'visitor' | 'none';
  ignores_player_limit: boolean;
  created_at: string;
  updated_at: string;
}

