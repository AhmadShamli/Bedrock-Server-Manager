import { Server, User, Plan, UserPlanStatus, KnockConfig, Backup, Task, AddonPack, AuditLog, PortGateLease, PortGateKey, PortGateAllowRule, PortGateBanRule, BannedPlayer, GlobalPlayer, MetricsData, Preset, SeedPreset, ActivePlayerInfo, DashboardSummary } from '../types';

class APIClient {
  private token: string | null = localStorage.getItem('bsm_token');

  setToken(token: string | null) {
    this.token = token;
    if (token) {
      localStorage.setItem('bsm_token', token);
    } else {
      localStorage.removeItem('bsm_token');
    }
  }

  getToken(): string | null {
    return this.token;
  }

  private async request<T = any>(path: string, options: RequestInit = {}): Promise<T> {
    const headers = new Headers(options.headers || {});
    if (this.token && !headers.has('Authorization')) {
      headers.set('Authorization', `Bearer ${this.token}`);
    }
    if (!headers.has('Content-Type') && !(options.body instanceof FormData)) {
      headers.set('Content-Type', 'application/json');
    }

    const response = await fetch(path, {
      ...options,
      headers,
      credentials: 'include',
    });

    if (!response.ok) {
      let errorMsg = 'Request failed';
      try {
        const data = await response.json();
        errorMsg = data.error || data.message || errorMsg;
      } catch {
        errorMsg = response.statusText || errorMsg;
      }
      throw new Error(errorMsg);
    }

    return response.json();
  }

  // Auth & Setup
  async getSetupStatus(): Promise<{ needs_setup: boolean }> {
    return this.request('/api/setup/status');
  }

  async setup(payload: { username: string; password: string }): Promise<{ token: string; user: User }> {
    const res = await this.request<{ token: string; user: User }>('/api/setup', {
      method: 'POST',
      body: JSON.stringify(payload),
    });
    this.setToken(res.token);
    return res;
  }

  async login(payload: { username: string; password: string; remember_me?: boolean }): Promise<{ token: string; user: User }> {
    const res = await this.request<{ token: string; user: User }>('/api/auth/login', {
      method: 'POST',
      body: JSON.stringify(payload),
    });
    this.setToken(res.token);
    return res;
  }

  async logout(): Promise<void> {
    try {
      await this.request('/api/auth/logout', { method: 'POST' });
    } finally {
      this.setToken(null);
    }
  }

  async register(payload: { username: string; password: string; email?: string }): Promise<{ token: string; user: User }> {
    const res = await this.request<{ token: string; user: User }>('/api/auth/register', {
      method: 'POST',
      body: JSON.stringify(payload),
    });
    this.setToken(res.token);
    return res;
  }

  async getMe(): Promise<{ user: User; allowed_servers: string[] }> {
    return this.request('/api/auth/me');
  }

  async getMyPlan(): Promise<UserPlanStatus> {
    return this.request('/api/user/plan');
  }

  // Servers
  async listServers(): Promise<Server[]> {
    const res = await this.request('/api/servers');
    return Array.isArray(res) ? res : [];
  }

  async getServer(id: string): Promise<Server> {
    return this.request(`/api/servers/${id}`);
  }

  async createServer(server: Partial<Server>): Promise<Server> {
    return this.request('/api/servers', {
      method: 'POST',
      body: JSON.stringify(server),
    });
  }

  async suggestPorts(): Promise<{ port: number; portv6: number }> {
    return this.request('/api/servers/suggest-ports');
  }

  async listNetworks(): Promise<{ networks: string[]; default: string }> {
    return this.request('/api/servers/networks');
  }

  async startServer(id: string): Promise<{ status: string }> {
    return this.request(`/api/servers/${id}/start`, { method: 'POST' });
  }

  async stopServer(id: string): Promise<{ status: string }> {
    return this.request(`/api/servers/${id}/stop`, { method: 'POST' });
  }

  async restartServer(id: string): Promise<{ status: string }> {
    return this.request(`/api/servers/${id}/restart`, { method: 'POST' });
  }

  async sendCommand(id: string, command: string): Promise<{ status: string }> {
    return this.request(`/api/servers/${id}/command`, {
      method: 'POST',
      body: JSON.stringify({ command }),
    });
  }

  async getStats(id: string): Promise<{
    cpu_percent: number;
    ram_bytes: number;
    player_count: number;
  }> {
    return this.request(`/api/servers/${id}/stats`);
  }

  async getMetrics(id: string, range: string = '1h'): Promise<MetricsData> {
    return this.request(`/api/servers/${id}/metrics?range=${encodeURIComponent(range)}`);
  }

  async updateServer(id: string, server: Partial<Server>): Promise<Server> {
    return this.request(`/api/servers/${id}`, {
      method: 'PUT',
      body: JSON.stringify(server),
    });
  }

  async deleteServer(id: string): Promise<{ status: string }> {
    return this.request(`/api/servers/${id}`, {
      method: 'DELETE',
    });
  }

  async getProperties(id: string): Promise<{ properties: Record<string, string>; keys: string[]; uninitialized?: boolean; pending?: boolean }> {
    const res = await this.request(`/api/servers/${id}/properties`);
    return {
      properties: res?.properties || {},
      keys: Array.isArray(res?.keys) ? res.keys : Object.keys(res?.properties || {}),
      uninitialized: !!res?.uninitialized,
      pending: !!res?.pending,
    };
  }

  async updateProperties(id: string, properties: Record<string, string>, keys: string[]): Promise<{ status: string }> {
    return this.request(`/api/servers/${id}/properties`, {
      method: 'PUT',
      body: JSON.stringify({ properties, keys }),
    });
  }

  async getAllowlist(id: string): Promise<Array<{ name: string; xuid?: string; ignoresPlayerLimit: boolean }>> {
    const res = await this.request(`/api/servers/${id}/allowlist`);
    return Array.isArray(res) ? res : [];
  }

  async updateAllowlist(id: string, list: Array<{ name: string; xuid?: string; ignoresPlayerLimit: boolean }>): Promise<{ status: string }> {
    return this.request(`/api/servers/${id}/allowlist`, {
      method: 'PUT',
      body: JSON.stringify(list),
    });
  }

  async getPermissions(id: string): Promise<Array<{ permission: string; xuid: string }>> {
    const res = await this.request(`/api/servers/${id}/permissions`);
    return Array.isArray(res) ? res : [];
  }

  async updatePermissions(id: string, list: Array<{ permission: string; xuid: string }>): Promise<{ status: string }> {
    return this.request(`/api/servers/${id}/permissions`, {
      method: 'PUT',
      body: JSON.stringify(list),
    });
  }

  async copyConfigs(sourceId: string, payload: {
    target_server_ids: string[];
    copy_allowlist: boolean;
    copy_permissions: boolean;
    copy_properties: boolean;
    mode: 'replace' | 'merge';
  }): Promise<{
    source_server_id: string;
    results: Array<{
      server_id: string;
      success: boolean;
      error?: string;
      copied: string[];
    }>;
  }> {
    const res = await this.request(`/api/servers/${sourceId}/copy-configs`, {
      method: 'POST',
      body: JSON.stringify(payload),
    });
    return {
      source_server_id: res?.source_server_id || sourceId,
      results: Array.isArray(res?.results) ? res.results : [],
    };
  }

  // Player Hub & Live Chat
  async getPlayers(id: string): Promise<{
    online_players: Array<{
      server_id: string;
      gamertag: string;
      xuid: string;
      joined_at: string;
      is_op?: boolean;
      permission?: string;
    }>;
    online_count: number;
  }> {
    const res = await this.request(`/api/servers/${id}/players`);
    return {
      online_players: Array.isArray(res?.online_players) ? res.online_players : [],
      online_count: typeof res?.online_count === 'number' ? res.online_count : 0,
    };
  }

  async getChat(id: string, limit = 50): Promise<Array<{ server_id: string; gamertag: string; message: string; timestamp: string }>> {
    const res = await this.request(`/api/servers/${id}/chat?limit=${limit}`);
    return Array.isArray(res) ? res : [];
  }

  async broadcast(id: string, message: string, target?: string): Promise<{ status: string }> {
    return this.request(`/api/servers/${id}/broadcast`, {
      method: 'POST',
      body: JSON.stringify({ message, target }),
    });
  }

  async kickPlayer(id: string, gamertag: string, reason = 'Kicked by administrator'): Promise<{ status: string }> {
    return this.request(`/api/servers/${id}/players/kick`, {
      method: 'POST',
      body: JSON.stringify({ gamertag, reason }),
    });
  }

  async opPlayer(id: string, gamertag: string): Promise<{ status: string }> {
    return this.request(`/api/servers/${id}/players/op`, {
      method: 'POST',
      body: JSON.stringify({ gamertag }),
    });
  }

  async deopPlayer(id: string, gamertag: string): Promise<{ status: string }> {
    return this.request(`/api/servers/${id}/players/deop`, {
      method: 'POST',
      body: JSON.stringify({ gamertag }),
    });
  }

  async banPlayer(id: string, data: { gamertag: string; xuid?: string; reason?: string; scope?: 'instance' | 'global'; ban_ip?: boolean; ip_address?: string }): Promise<{ status: string; gamertag: string; scope: string; ip_banned: boolean; banned_ip?: string }> {
    return this.request(`/api/servers/${id}/players/ban`, {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async banGlobalPlayer(data: { gamertag: string; xuid?: string; reason?: string; ban_ip?: boolean; ip_address?: string }): Promise<{ status: string; gamertag: string; scope: string; ip_banned: boolean; banned_ip?: string }> {
    return this.request('/api/banned-players', {
      method: 'POST',
      body: JSON.stringify({ ...data, scope: 'global' }),
    });
  }

  async getActivePlayers(): Promise<ActivePlayerInfo[]> {
    const res = await this.request('/api/active-players');
    return Array.isArray(res) ? res : [];
  }

  async listPlayerBans(id: string): Promise<BannedPlayer[]> {
    return this.request(`/api/servers/${id}/players/bans`);
  }

  async unbanPlayer(id: string, data: { gamertag?: string; id?: number }): Promise<{ status: string }> {
    return this.request(`/api/servers/${id}/players/unban`, {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async listAllBannedPlayers(): Promise<BannedPlayer[]> {
    const res = await this.request('/api/banned-players');
    return Array.isArray(res) ? res : [];
  }

  async deleteBan(id: number): Promise<{ status: string }> {
    return this.request(`/api/banned-players/${id}`, {
      method: 'DELETE',
    });
  }

  // Clone & Export
  async cloneServer(id: string, newId: string, newName: string): Promise<Server> {
    return this.request(`/api/servers/${id}/clone`, {
      method: 'POST',
      body: JSON.stringify({ new_id: newId, new_name: newName }),
    });
  }

  getExportUrl(id: string): string {
    const token = this.getToken();
    return `/api/servers/${id}/export${token ? `?token=${encodeURIComponent(token)}` : ''}`;
  }

  // Presets & Updates
  async getPresets(): Promise<Array<{ id: string; name: string; description: string; mode: string; difficulty: string; properties: Record<string, string> }>> {
    const res = await this.request('/api/presets');
    return Array.isArray(res) ? res : [];
  }

  async checkUpdates(version = 'latest'): Promise<{ current_version: string; latest_version: string; update_available: boolean; release_url: string }> {
    return this.request(`/api/updater/check?version=${encodeURIComponent(version)}`);
  }

  // Knock Portal
  async getKnockConfig(serverId: string): Promise<KnockConfig> {
    return this.request(`/api/knock/${serverId}/config`);
  }

  async knock(serverId: string, payload: { gamertag?: string; passphrase?: string }): Promise<{
    success: boolean;
    ip_address: string;
    expires_at: string;
    expires_in_seconds: number;
    session_token: string;
    direct_launch_url: string;
    add_server_url?: string;
    server_name: string;
    server_port: number;
    game_server_address?: string;
    always_allowed?: boolean;
    rule_comment?: string;
  }> {
    return this.request(`/api/knock/${serverId}`, {
      method: 'POST',
      body: JSON.stringify(payload),
    });
  }

  async sendHeartbeat(serverId: string, token?: string): Promise<{
    status: string;
    ip_address: string;
    ip_updated: boolean;
    expires_in_seconds: number;
  }> {
    const headers: Record<string, string> = {};
    if (token) {
      headers['X-Knock-Token'] = token;
    }
    return this.request(`/api/knock/${serverId}/heartbeat`, {
      method: 'POST',
      headers,
    });
  }

  async getKnockStatus(serverId: string): Promise<{
    active: boolean;
    ip_address: string;
    gamertag?: string;
    expires_in_seconds?: number;
    direct_launch_url?: string;
    add_server_url?: string;
    game_server_address?: string;
    always_allowed?: boolean;
    rule_comment?: string;
  }> {
    return this.request(`/api/knock/${serverId}/status`);
  }

  // --- Port Gate Leases ---
  async listLeases(serverId: string): Promise<PortGateLease[]> {
    const res = await this.request(`/api/servers/${serverId}/leases`);
    return Array.isArray(res) ? res : [];
  }

  async revokeLease(serverId: string, leaseId: number): Promise<{ success: boolean }> {
    return this.request(`/api/servers/${serverId}/leases/${leaseId}/revoke`, {
      method: 'POST',
    });
  }

  async createManualLease(serverId: string, payload: {
    ip_address: string;
    gamertag?: string;
    duration_minutes: number;
    comment?: string;
  }): Promise<PortGateLease> {
    return this.request(`/api/servers/${serverId}/leases/manual`, {
      method: 'POST',
      body: JSON.stringify(payload),
    });
  }

  // --- Port Gate Access Keys ---
  async listAccessKeys(serverId: string): Promise<PortGateKey[]> {
    const res = await this.request(`/api/servers/${serverId}/access-keys`);
    return Array.isArray(res) ? res : [];
  }

  async createAccessKey(serverId: string, payload: {
    label: string;
    passphrase?: string;
    max_uses?: number;
    lease_duration_seconds?: number;
    expires_at?: string;
  }): Promise<PortGateKey & { plaintext_passphrase?: string }> {
    return this.request(`/api/servers/${serverId}/access-keys`, {
      method: 'POST',
      body: JSON.stringify(payload),
    });
  }

  async deleteAccessKey(serverId: string, keyId: number): Promise<{ success: boolean }> {
    return this.request(`/api/servers/${serverId}/access-keys/${keyId}`, {
      method: 'DELETE',
    });
  }

  // --- Port Gate Permanent Allowlist ---
  async listPortGateAllowRules(serverId?: string): Promise<PortGateAllowRule[]> {
    const url = serverId ? `/api/servers/${serverId}/portgate/allowlist` : '/api/portgate/allowlist';
    const res = await this.request(url);
    return Array.isArray(res) ? res : [];
  }

  async createPortGateAllowRule(payload: {
    server_id?: string | null;
    is_global?: boolean;
    ip_or_subnet: string;
    comment?: string;
  }, serverId?: string): Promise<PortGateAllowRule> {
    const url = serverId ? `/api/servers/${serverId}/portgate/allowlist` : '/api/portgate/allowlist';
    return this.request(url, {
      method: 'POST',
      body: JSON.stringify(payload),
    });
  }

  async deletePortGateAllowRule(ruleId: number, serverId?: string): Promise<{ success: boolean }> {
    const url = serverId ? `/api/servers/${serverId}/portgate/allowlist/${ruleId}` : `/api/portgate/allowlist/${ruleId}`;
    return this.request(url, {
      method: 'DELETE',
    });
  }

  // --- Port Gate Banlist ---
  async listPortGateBans(serverId?: string): Promise<PortGateBanRule[]> {
    const url = serverId ? `/api/servers/${serverId}/portgate/bans` : '/api/portgate/bans';
    const res = await this.request(url);
    return Array.isArray(res) ? res : [];
  }

  async createPortGateBan(payload: {
    server_id?: string | null;
    is_global?: boolean;
    ip_or_subnet: string;
    reason?: string;
  }, serverId?: string): Promise<PortGateBanRule> {
    const url = serverId ? `/api/servers/${serverId}/portgate/bans` : '/api/portgate/bans';
    return this.request(url, {
      method: 'POST',
      body: JSON.stringify(payload),
    });
  }

  async deletePortGateBan(banId: number, serverId?: string): Promise<{ success: boolean }> {
    const url = serverId ? `/api/servers/${serverId}/portgate/bans/${banId}` : `/api/portgate/bans/${banId}`;
    return this.request(url, {
      method: 'DELETE',
    });
  }

  // --- Centralized Leases ---
  async listAllPortGateLeases(serverId?: string): Promise<PortGateLease[]> {
    const url = serverId ? `/api/portgate/leases?server_id=${encodeURIComponent(serverId)}` : '/api/portgate/leases';
    const res = await this.request(url);
    return Array.isArray(res) ? res : [];
  }

  async revokeCentralizedLease(leaseId: number): Promise<{ success: boolean }> {
    return this.request(`/api/portgate/leases/${leaseId}/revoke`, {
      method: 'POST',
    });
  }

  // --- Backups & Worlds ---
  async listBackups(serverId: string): Promise<Backup[]> {
    const res = await this.request(`/api/servers/${serverId}/backups`);
    return Array.isArray(res) ? res : [];
  }

  async createBackup(serverId: string, payload: { type?: string; is_locked?: boolean } = {}): Promise<Backup> {
    return this.request(`/api/servers/${serverId}/backups`, {
      method: 'POST',
      body: JSON.stringify(payload),
    });
  }

  async toggleBackupLock(serverId: string, backupId: number): Promise<{ id: number; is_locked: boolean }> {
    return this.request(`/api/servers/${serverId}/backups/${backupId}/lock`, {
      method: 'POST',
    });
  }

  async deleteBackup(serverId: string, backupId: number): Promise<{ success: boolean }> {
    return this.request(`/api/servers/${serverId}/backups/${backupId}`, {
      method: 'DELETE',
    });
  }

  async restoreBackup(serverId: string, backupId: number): Promise<{ success: boolean }> {
    return this.request(`/api/servers/${serverId}/backups/${backupId}/restore`, {
      method: 'POST',
    });
  }

  downloadBackupUrl(serverId: string, backupId: number): string {
    const token = this.getToken();
    return `/api/servers/${serverId}/backups/${backupId}/download${token ? `?token=${encodeURIComponent(token)}` : ''}`;
  }

  exportWorldUrl(serverId: string): string {
    const token = this.getToken();
    return `/api/servers/${serverId}/world/export${token ? `?token=${encodeURIComponent(token)}` : ''}`;
  }

  async importWorld(serverId: string, file: File, worldName = 'Bedrock level'): Promise<{ success: boolean }> {
    const formData = new FormData();
    formData.append('world_file', file);
    formData.append('world_name', worldName);

    const headers: Record<string, string> = {};
    if (this.token) {
      headers['Authorization'] = `Bearer ${this.token}`;
    }

    const res = await fetch(`/api/servers/${serverId}/world/import`, {
      method: 'POST',
      headers,
      body: formData,
    });
    if (!res.ok) {
      const err = await res.json().catch(() => ({ error: 'Upload failed' }));
      throw new Error(err.error || 'Upload failed');
    }
    return res.json();
  }

  // --- Addon / Pack Manager ---
  async listAddons(serverId: string): Promise<AddonPack[]> {
    const res = await this.request(`/api/servers/${serverId}/addons`);
    return Array.isArray(res) ? res : [];
  }

  async installAddon(serverId: string, file: File): Promise<AddonPack> {
    const formData = new FormData();
    formData.append('addon_file', file);

    const headers: Record<string, string> = {};
    if (this.token) {
      headers['Authorization'] = `Bearer ${this.token}`;
    }

    const res = await fetch(`/api/servers/${serverId}/addons`, {
      method: 'POST',
      headers,
      body: formData,
    });
    if (!res.ok) {
      const err = await res.json().catch(() => ({ error: 'Installation failed' }));
      throw new Error(err.error || 'Installation failed');
    }
    return res.json();
  }

  async deleteAddon(serverId: string, type: string, folder: string): Promise<{ success: boolean }> {
    return this.request(`/api/servers/${serverId}/addons/${type}/${folder}`, {
      method: 'DELETE',
    });
  }

  // --- Task Scheduler ---
  async listTasks(serverId?: string): Promise<Task[]> {
    const url = serverId ? `/api/tasks?server_id=${encodeURIComponent(serverId)}` : '/api/tasks';
    const res = await this.request(url);
    return Array.isArray(res) ? res : [];
  }

  async getTask(id: number): Promise<Task> {
    return this.request(`/api/tasks/${id}`);
  }

  async createTask(task: Partial<Task>): Promise<Task> {
    return this.request('/api/tasks', {
      method: 'POST',
      body: JSON.stringify(task),
    });
  }

  async updateTask(id: number, task: Partial<Task>): Promise<Task> {
    return this.request(`/api/tasks/${id}`, {
      method: 'PUT',
      body: JSON.stringify(task),
    });
  }

  async deleteTask(id: number): Promise<{ success: boolean }> {
    return this.request(`/api/tasks/${id}`, {
      method: 'DELETE',
    });
  }

  async toggleTask(id: number): Promise<{ id: number; enabled: boolean }> {
    return this.request(`/api/tasks/${id}/toggle`, {
      method: 'POST',
    });
  }

  async runTaskNow(id: number): Promise<{ success: boolean }> {
    return this.request(`/api/tasks/${id}/run`, {
      method: 'POST',
    });
  }

  // --- System Audit & Settings ---
  async listAuditLogs(limit = 50, offset = 0): Promise<AuditLog[]> {
    const res = await this.request(`/api/system/audit?limit=${limit}&offset=${offset}`);
    return Array.isArray(res) ? res : [];
  }

  async getVersion(): Promise<{ version: string; app_name: string; author: string; repository_url: string }> {
    return this.request('/api/version');
  }

  async listPresets(): Promise<Preset[]> {
    const res = await this.request('/api/presets');
    return Array.isArray(res) ? res : [];
  }

  async listSeeds(): Promise<SeedPreset[]> {
    const res = await this.request('/api/presets/seeds');
    return Array.isArray(res) ? res : [];
  }

  async getSettings(): Promise<Record<string, string>> {
    const res = await this.request('/api/system/settings');
    return res && typeof res === 'object' ? res : {};
  }

  async updateSetting(key: string, value: string): Promise<{ success: boolean }> {
    return this.request('/api/system/settings', {
      method: 'POST',
      body: JSON.stringify({ key, value }),
    });
  }

  // --- User Management ---
  async listUsers(): Promise<User[]> {
    const res = await this.request('/api/users');
    return Array.isArray(res) ? res : [];
  }

  async createUser(payload: { username: string; password: string; role: 'admin' | 'operator' | 'user'; email?: string; plan_id?: number; plan_status?: string; plan_expires_at?: string }): Promise<User> {
    return this.request('/api/users', {
      method: 'POST',
      body: JSON.stringify(payload),
    });
  }

  async updateUser(id: number, payload: { role?: string; email?: string; plan_id?: number | null; plan_status?: string; plan_expires_at?: string | null }): Promise<User> {
    return this.request(`/api/users/${id}`, {
      method: 'PUT',
      body: JSON.stringify(payload),
    });
  }

  async updateUserPassword(id: number, password: string): Promise<{ success: boolean }> {
    return this.request(`/api/users/${id}/password`, {
      method: 'PUT',
      body: JSON.stringify({ password }),
    });
  }

  async deleteUser(id: number): Promise<{ success: boolean }> {
    return this.request(`/api/users/${id}`, {
      method: 'DELETE',
    });
  }

  async getUserServerAccess(id: number): Promise<string[]> {
    const res = await this.request(`/api/users/${id}/servers`);
    return Array.isArray(res) ? res : [];
  }

  async updateUserServerAccess(id: number, serverIds: string[]): Promise<{ success: boolean }> {
    return this.request(`/api/users/${id}/servers`, {
      method: 'PUT',
      body: JSON.stringify({ server_ids: serverIds }),
    });
  }

  // --- Plans Management ---
  async listPlans(): Promise<Plan[]> {
    const res = await this.request('/api/plans');
    return Array.isArray(res) ? res : [];
  }

  async getPlan(id: number): Promise<Plan> {
    return this.request(`/api/plans/${id}`);
  }

  async createPlan(payload: Partial<Plan>): Promise<Plan> {
    return this.request('/api/plans', {
      method: 'POST',
      body: JSON.stringify(payload),
    });
  }

  async updatePlan(id: number, payload: Partial<Plan>): Promise<Plan> {
    return this.request(`/api/plans/${id}`, {
      method: 'PUT',
      body: JSON.stringify(payload),
    });
  }

  async deletePlan(id: number): Promise<{ success: boolean }> {
    return this.request(`/api/plans/${id}`, {
      method: 'DELETE',
    });
  }

  async setDefaultPlan(id: number): Promise<{ success: boolean }> {
    return this.request(`/api/plans/${id}/set-default`, {
      method: 'POST',
    });
  }

  // --- Collaborators ---
  async listCollaborators(serverId: string): Promise<User[]> {
    const res = await this.request(`/api/servers/${serverId}/collaborators`);
    return Array.isArray(res) ? res : [];
  }

  async addCollaborator(serverId: string, username: string): Promise<{ success: boolean }> {
    return this.request(`/api/servers/${serverId}/collaborators`, {
      method: 'POST',
      body: JSON.stringify({ username }),
    });
  }

  async removeCollaborator(serverId: string, userId: number): Promise<{ success: boolean }> {
    return this.request(`/api/servers/${serverId}/collaborators/${userId}`, {
      method: 'DELETE',
    });
  }

  // --- Multi-Level Global Player Access Control ---
  async listGlobalPlayers(): Promise<GlobalPlayer[]> {
    const res = await this.request('/api/global-players');
    return Array.isArray(res) ? res : [];
  }

  async getGlobalPlayer(id: number): Promise<GlobalPlayer> {
    return this.request(`/api/global-players/${id}`);
  }

  async createGlobalPlayer(payload: Partial<GlobalPlayer>): Promise<GlobalPlayer> {
    return this.request('/api/global-players', {
      method: 'POST',
      body: JSON.stringify(payload),
    });
  }

  async updateGlobalPlayer(id: number, payload: Partial<GlobalPlayer>): Promise<GlobalPlayer> {
    return this.request(`/api/global-players/${id}`, {
      method: 'PUT',
      body: JSON.stringify(payload),
    });
  }

  async setGlobalPlayerRole(id: number, role: 'operator' | 'member' | 'visitor'): Promise<GlobalPlayer> {
    return this.request(`/api/global-players/${id}/role`, {
      method: 'POST',
      body: JSON.stringify({ role }),
    });
  }

  async deleteGlobalPlayer(id: number): Promise<{ status: string }> {
    return this.request(`/api/global-players/${id}`, {
      method: 'DELETE',
    });
  }

  async removeGlobalPlayerByName(name: string): Promise<{ status: string }> {
    return this.request('/api/global-players/remove-by-name', {
      method: 'POST',
      body: JSON.stringify({ name }),
    });
  }

  async syncAllServersGlobal(): Promise<Record<string, { server_id: string; allowlist_added: string[]; permissions_updated: string[] }>> {
    const res = await this.request('/api/global-players/sync-all', {
      method: 'POST',
    });
    return res && typeof res === 'object' ? res : {};
  }

  async syncServerGlobal(serverId: string): Promise<{ server_id: string; allowlist_added: string[]; permissions_updated: string[] }> {
    return this.request(`/api/servers/${serverId}/sync-global`, {
      method: 'POST',
    });
  }

  async promotePlayerToGlobal(serverId: string, payload: {
    name: string;
    xuid?: string;
    permission?: string;
    is_allowlisted?: boolean;
    ignores_player_limit?: boolean;
  }): Promise<GlobalPlayer> {
    return this.request(`/api/servers/${serverId}/players/promote-global`, {
      method: 'POST',
      body: JSON.stringify(payload),
    });
  }

  async getDashboardSummary(): Promise<DashboardSummary> {
    return this.request('/api/dashboard/summary');
  }
}

export const api = new APIClient();
