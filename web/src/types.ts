export interface User {
  id: number;
  username: string;
  email?: string;
  role: 'admin' | 'operator' | 'user';
  plan_id?: number;
  plan_name?: string;
  plan_status?: string;
  plan_expires_at?: string;
  created_at: string;
}

export interface Plan {
  id: number;
  name: string;
  description: string;
  is_default: boolean;
  billing_interval: string;
  trial_duration_days: number;
  max_servers: number;
  max_memory: string;
  max_cpu: number;
  max_backups_per_server: number;
  max_disk_mb: number;
  max_player_slots: number;
  max_collaborators: number;
  idle_timeout_minutes: number;
  allow_custom_seed: boolean;
  allow_custom_port: boolean;
  allow_preview_versions: boolean;
  allow_addons: boolean;
  allow_port_gate_keys: boolean;
  allow_tasks: boolean;
  user_count?: number;
  created_at?: string;
  updated_at?: string;
}

export interface UserPlanStatus {
  plan: Plan;
  usage: {
    servers_count: number;
    servers_max: number;
    can_deploy_server: boolean;
  };
  user: {
    id: number;
    username: string;
    role: string;
    plan_status: string;
    plan_expires_at?: string;
  };
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
  seed?: string;
  game_server_address?: string;
  network_mode?: string;
  owner_user_id?: number;
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

export interface PortGateAllowRule {
  id: number;
  server_id?: string | null;
  ip_or_subnet: string;
  comment: string;
  created_at: string;
}

export interface PortGateBanRule {
  id: number;
  server_id?: string | null;
  ip_or_subnet: string;
  reason: string;
  banned_by: string;
  created_at: string;
}

export interface BannedPlayer {
  id: number;
  server_id?: string | null;
  gamertag: string;
  xuid?: string;
  reason: string;
  banned_by: string;
  created_at: string;
}

export interface KnockConfig {
  server_id: string;
  server_name: string;
  port: number;
  game_server_address?: string;
  port_gate_enabled: boolean;
  port_gate_mode: 'gamertag' | 'passphrase' | 'combined';
  port_gate_timeout?: number;
  heartbeat_interval_seconds: number;
  client_ip?: string;
  always_allowed?: boolean;
  rule_comment?: string;
  is_banned?: boolean;
  ban_reason?: string;
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

export interface MarketplaceItem {
  id: string;
  provider: 'curseforge' | 'modrinth';
  name: string;
  summary: string;
  author: string;
  icon_url: string;
  downloads: number;
  version?: string;
  file_id?: number;
  file_name?: string;
  download_url?: string;
  categories?: string[];
  page_url?: string;
}

export interface MarketplaceSearchResult {
  provider: string;
  items: MarketplaceItem[];
  total: number;
  curseforge_configured: boolean;
}

export interface MarketplaceConfig {
  curseforge_configured: boolean;
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

export interface MetricPoint {
  timestamp: string;
  cpu_percent: number;
  ram_bytes: number;
  active_players: number;
}

export interface MetricsData {
  server_id: string;
  range: string;
  cpu_limit: number;
  memory_limit_bytes: number;
  max_players: number;
  total_allowlist: number;
  current?: {
    cpu_percent: number;
    ram_bytes: number;
    player_count: number;
  };
  series: MetricPoint[];
}
export interface Preset {
  id: string;
  name: string;
  description: string;
  mode: string;
  difficulty: string;
  properties: Record<string, string>;
}

export interface SeedPreset {
  id: string;
  name: string;
  seed: string;
  category: string;
  description: string;
  biomes: string[];
  features: string[];
  difficulty?: string;
  icon?: string;
}

export interface ActivePlayerInfo {
  server_id: string;
  server_name: string;
  gamertag: string;
  xuid: string;
  joined_at: string;
  permission?: string;
  is_op?: boolean;
}

export interface ServerPlayer {
  id?: number;
  server_id: string;
  gamertag: string;
  xuid: string;
  first_seen: string;
  last_seen: string;
  total_connections: number;
  is_online: boolean;
  permission?: string;
  is_op?: boolean;
}

export interface ServerSummary {
  id: string;
  name: string;
  status: 'stopped' | 'starting' | 'running' | 'stopping' | 'crashed';
  port: number;
  mode: string;
  difficulty: string;
  cpu_limit: number;
  memory_limit: string;
  port_gate_enabled: boolean;
  online_players: number;
  cpu_percent?: number;
  ram_bytes?: number;
}

export interface HostSystemSummary {
  version: string;
  app_name: string;
  go_version: string;
  goroutines: number;
  os: string;
  arch: string;
  uptime_sec: number;
  alloc_mb: number;
  sys_mb: number;
  host_cpu_cores: number;
  host_cpu_percent: number;
  host_total_ram_bytes: number;
  host_used_ram_bytes: number;
  host_ram_percent: number;
  host_load_avg_1: number;
  host_load_avg_5: number;
  host_load_avg_15: number;
}

export interface DashboardSummary {
  total_servers: number;
  running_servers: number;
  stopped_servers: number;
  total_online_players: number;
  total_global_players: number;
  total_banned_players: number;
  active_players: ActivePlayerInfo[];
  total_allocated_cores: number;
  total_allocated_ram: number;
  total_used_ram: number;
  total_used_cpu_percent: number;
  total_used_cpu_cores: number;
  average_cpu_percent: number;
  host_system: HostSystemSummary;
  total_backups_count: number;
  total_backups_bytes: number;
  active_leases_count: number;
  allow_rules_count: number;
  portgate_bans_count: number;
  servers: ServerSummary[];
}

