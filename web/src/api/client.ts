import { Server, User, KnockConfig } from '../types';

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

  private async request<T>(path: string, options: RequestInit = {}): Promise<T> {
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

  async getMe(): Promise<{ user: User; allowed_servers: string[] }> {
    return this.request('/api/auth/me');
  }

  // Servers
  async listServers(): Promise<Server[]> {
    return this.request('/api/servers');
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

  async updateServer(id: string, server: Partial<Server>): Promise<{ status: string }> {
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

  // Properties, Allowlist & Permissions
  async getProperties(id: string): Promise<{ properties: Record<string, string>; keys: string[] }> {
    return this.request(`/api/servers/${id}/properties`);
  }

  async updateProperties(id: string, properties: Record<string, string>, keys: string[]): Promise<{ status: string }> {
    return this.request(`/api/servers/${id}/properties`, {
      method: 'PUT',
      body: JSON.stringify({ properties, keys }),
    });
  }

  async getAllowlist(id: string): Promise<Array<{ name: string; xuid?: string; ignoresPlayerLimit: boolean }>> {
    return this.request(`/api/servers/${id}/allowlist`);
  }

  async updateAllowlist(id: string, list: Array<{ name: string; xuid?: string; ignoresPlayerLimit: boolean }>): Promise<{ status: string }> {
    return this.request(`/api/servers/${id}/allowlist`, {
      method: 'PUT',
      body: JSON.stringify(list),
    });
  }

  async getPermissions(id: string): Promise<Array<{ permission: string; xuid: string }>> {
    return this.request(`/api/servers/${id}/permissions`);
  }

  async updatePermissions(id: string, list: Array<{ permission: string; xuid: string }>): Promise<{ status: string }> {
    return this.request(`/api/servers/${id}/permissions`, {
      method: 'PUT',
      body: JSON.stringify(list),
    });
  }

  // Player Hub & Live Chat
  async getPlayers(id: string): Promise<{
    online_players: Array<{ server_id: string; gamertag: string; xuid: string; joined_at: string }>;
    online_count: number;
  }> {
    return this.request(`/api/servers/${id}/players`);
  }

  async getChat(id: string, limit = 50): Promise<Array<{ server_id: string; gamertag: string; message: string; timestamp: string }>> {
    return this.request(`/api/servers/${id}/chat?limit=${limit}`);
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

  // Clone & Export
  async cloneServer(id: string, newId: string, newName: string): Promise<Server> {
    return this.request(`/api/servers/${id}/clone`, {
      method: 'POST',
      body: JSON.stringify({ new_id: newId, new_name: newName }),
    });
  }

  getExportUrl(id: string): string {
    return `/api/servers/${id}/export`;
  }

  // Presets & Updates
  async getPresets(): Promise<Array<{ id: string; name: string; description: string; mode: string; difficulty: string; properties: Record<string, string> }>> {
    return this.request('/api/presets');
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
    server_name: string;
    server_port: number;
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
  }> {
    return this.request(`/api/knock/${serverId}/status`);
  }
}

export const api = new APIClient();
