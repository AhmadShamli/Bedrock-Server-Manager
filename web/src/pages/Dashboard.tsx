import React, { useEffect, useState, useCallback } from 'react';
import { Link } from 'react-router-dom';
import {
  Server as ServerIcon,
  Users,
  Shield,
  HardDrive,
  Cpu,
  Activity,
  RefreshCw,
  Clock,
  Layers,
  ArrowRight,
  ExternalLink,
  AlertCircle,
  Lock,
  Terminal,
  ChevronRight,
  UserCheck,
} from 'lucide-react';
import { api } from '../api/client';
import { DashboardSummary, User } from '../types';

interface DashboardProps {
  user: User;
}

function formatBytes(bytes: number, decimals = 1): string {
  if (!bytes || bytes <= 0) return '0 B';
  const k = 1024;
  const dm = decimals < 0 ? 0 : decimals;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  const index = Math.min(i, sizes.length - 1);
  return `${parseFloat((bytes / Math.pow(k, index)).toFixed(dm))} ${sizes[index]}`;
}

function formatUptime(seconds: number): string {
  if (!seconds || seconds <= 0) return '0s';
  const d = Math.floor(seconds / (3600 * 24));
  const h = Math.floor((seconds % (3600 * 24)) / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  const s = seconds % 60;
  if (d > 0) return `${d}d ${h}h ${m}m`;
  if (h > 0) return `${h}h ${m}m ${s}s`;
  if (m > 0) return `${m}m ${s}s`;
  return `${s}s`;
}

export const Dashboard: React.FC<DashboardProps> = ({ user }) => {
  const [summary, setSummary] = useState<DashboardSummary | null>(null);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [autoRefresh, setAutoRefresh] = useState(true);
  const [lastUpdated, setLastUpdated] = useState<Date>(new Date());

  const fetchSummary = useCallback(async (isManual = false) => {
    if (isManual) setRefreshing(true);
    try {
      const data = await api.getDashboardSummary();
      setSummary(data);
      setLastUpdated(new Date());
      setError(null);
    } catch (err: any) {
      setError(err.message || 'Failed to fetch dashboard summary');
    } finally {
      setLoading(false);
      if (isManual) setRefreshing(false);
    }
  }, []);

  useEffect(() => {
    fetchSummary();
  }, [fetchSummary]);

  // Polling interval for live metrics
  useEffect(() => {
    if (!autoRefresh) return;
    const interval = setInterval(() => {
      fetchSummary(false);
    }, 6000);
    return () => clearInterval(interval);
  }, [autoRefresh, fetchSummary]);

  if (loading) {
    return (
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-12 flex flex-col items-center justify-center min-h-[60vh]">
        <div className="w-12 h-12 rounded-xl bg-obsidian-850 border border-obsidian-700 flex items-center justify-center text-emerald-400 mb-4 animate-pulse">
          <Activity className="w-6 h-6 animate-spin" />
        </div>
        <div className="text-slate-300 font-mono font-medium text-base">Loading telemetry and system metrics...</div>
        <div className="text-slate-500 text-xs mt-1">Aggregating live instance statuses, host resources, and player records</div>
      </div>
    );
  }

  const runningPercent = summary && summary.total_servers > 0 
    ? Math.round((summary.running_servers / summary.total_servers) * 100) 
    : 0;

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8 space-y-8">
      {/* Top Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 border-b border-obsidian-800 pb-6">
        <div>
          <div className="flex items-center gap-3">
            <h1 className="text-2xl font-bold tracking-tight text-slate-100 flex items-center gap-2">
              <span>Overview Dashboard</span>
              <span className="text-xs px-2 py-0.5 rounded-full bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 font-mono font-normal">
                v{summary?.host_system?.version || '1.5.8'}
              </span>
            </h1>
          </div>
          <p className="text-slate-400 text-sm mt-1">
            Aggregate fleet metrics, real-time resource allocations, and online player monitoring.
          </p>
        </div>

        <div className="flex items-center gap-3">
          <div className="flex items-center gap-2 bg-obsidian-900 border border-obsidian-750 px-3 py-1.5 rounded-lg text-xs text-slate-400">
            <span className="relative flex h-2 w-2">
              <span className={`animate-ping absolute inline-flex h-full w-full rounded-full ${autoRefresh ? 'bg-emerald-400 opacity-75' : 'bg-slate-600 opacity-20'}`}></span>
              <span className={`relative inline-flex rounded-full h-2 w-2 ${autoRefresh ? 'bg-emerald-500' : 'bg-slate-500'}`}></span>
            </span>
            <button
              onClick={() => setAutoRefresh(!autoRefresh)}
              className="hover:text-slate-200 transition-colors cursor-pointer"
              title={autoRefresh ? 'Click to pause auto-refresh' : 'Click to enable auto-refresh (6s)'}
            >
              {autoRefresh ? 'Live Sync' : 'Sync Paused'}
            </button>
            <span className="text-slate-600">|</span>
            <span className="font-mono text-[11px] text-slate-400">
              {lastUpdated.toLocaleTimeString()}
            </span>
          </div>

          <button
            onClick={() => fetchSummary(true)}
            disabled={refreshing}
            className="flex items-center gap-1.5 px-3 py-1.5 bg-obsidian-850 hover:bg-obsidian-800 border border-obsidian-700/80 rounded-lg text-slate-200 text-xs font-medium transition-all shadow-sm disabled:opacity-50"
            title="Refresh statistics now"
          >
            <RefreshCw className={`w-3.5 h-3.5 ${refreshing ? 'animate-spin text-emerald-400' : 'text-slate-400'}`} />
            <span className="hidden sm:inline">Refresh</span>
          </button>

          <Link
            to="/servers"
            className="flex items-center gap-1.5 px-3.5 py-1.5 bg-emerald-600 hover:bg-emerald-500 text-white rounded-lg text-xs font-medium transition-all shadow-[0_0_12px_rgba(16,185,129,0.25)]"
          >
            <ServerIcon className="w-3.5 h-3.5" />
            <span>Manage Instances</span>
          </Link>
        </div>
      </div>

      {error && (
        <div className="bg-rose-500/10 border border-rose-500/30 rounded-xl p-4 flex items-center gap-3 text-rose-300 text-sm">
          <AlertCircle className="w-5 h-5 flex-shrink-0 text-rose-400" />
          <span>{error}</span>
        </div>
      )}

      {/* Primary KPI Metrics Grid */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        {/* Card 1: Server Fleet */}
        <div className="bg-obsidian-900 border border-obsidian-750/80 hover:border-obsidian-600/80 rounded-xl p-5 shadow-sm transition-all relative overflow-hidden group">
          <div className="flex items-center justify-between">
            <span className="text-xs font-medium text-slate-400 uppercase tracking-wider">Fleet Status</span>
            <div className="w-8 h-8 rounded-lg bg-emerald-500/10 border border-emerald-500/20 flex items-center justify-center text-emerald-400">
              <ServerIcon className="w-4 h-4" />
            </div>
          </div>
          <div className="mt-3 flex items-baseline gap-2">
            <span className="text-3xl font-mono font-bold text-slate-100">
              {summary?.running_servers ?? 0}
            </span>
            <span className="text-sm font-mono text-slate-400">
              / {summary?.total_servers ?? 0} Running
            </span>
          </div>
          {/* Progress bar */}
          <div className="mt-3 w-full bg-obsidian-800 rounded-full h-1.5 overflow-hidden">
            <div
              className="bg-emerald-500 h-1.5 rounded-full transition-all duration-500"
              style={{ width: `${runningPercent}%` }}
            ></div>
          </div>
          <div className="mt-2.5 flex items-center justify-between text-xs text-slate-400">
            <span className="flex items-center gap-1.5">
              <span className="w-2 h-2 rounded-full bg-emerald-400"></span>
              {summary?.running_servers ?? 0} active
            </span>
            <span className="flex items-center gap-1.5">
              <span className="w-2 h-2 rounded-full bg-slate-500"></span>
              {summary?.stopped_servers ?? 0} stopped
            </span>
          </div>
        </div>

        {/* Card 2: Live Players */}
        <div className="bg-obsidian-900 border border-obsidian-750/80 hover:border-obsidian-600/80 rounded-xl p-5 shadow-sm transition-all relative overflow-hidden group">
          <div className="flex items-center justify-between">
            <span className="text-xs font-medium text-slate-400 uppercase tracking-wider">Online Players</span>
            <div className="w-8 h-8 rounded-lg bg-emerald-500/10 border border-emerald-500/20 flex items-center justify-center text-emerald-400">
              <Users className="w-4 h-4" />
            </div>
          </div>
          <div className="mt-3 flex items-baseline gap-2">
            <span className="text-3xl font-mono font-bold text-slate-100">
              {summary?.total_online_players ?? 0}
            </span>
            <span className="text-xs text-emerald-400 font-medium flex items-center gap-1 bg-emerald-500/10 px-2 py-0.5 rounded border border-emerald-500/20">
              <span className="w-1.5 h-1.5 rounded-full bg-emerald-400 animate-pulse"></span>
              Live Now
            </span>
          </div>
          <div className="mt-4 flex items-center justify-between text-xs text-slate-400 border-t border-obsidian-800/80 pt-2.5">
            <span>Global Registry:</span>
            <span className="font-mono text-slate-300 font-medium">
              {summary?.total_global_players ?? 0} registered
            </span>
          </div>
        </div>

        {/* Card 3: Memory Allocation & Usage */}
        <div className="bg-obsidian-900 border border-obsidian-750/80 hover:border-obsidian-600/80 rounded-xl p-5 shadow-sm transition-all relative overflow-hidden group">
          <div className="flex items-center justify-between">
            <span className="text-xs font-medium text-slate-400 uppercase tracking-wider">RAM Allocation</span>
            <div className="w-8 h-8 rounded-lg bg-emerald-500/10 border border-emerald-500/20 flex items-center justify-center text-emerald-400">
              <Cpu className="w-4 h-4" />
            </div>
          </div>
          <div className="mt-3 flex items-baseline gap-2">
            <span className="text-2xl font-mono font-bold text-slate-100">
              {formatBytes(summary?.total_used_ram ?? 0)}
            </span>
            <span className="text-xs font-mono text-slate-400">
              / {formatBytes(summary?.total_allocated_ram ?? 0)} cap
            </span>
          </div>
          <div className="mt-4 flex items-center justify-between text-xs text-slate-400 border-t border-obsidian-800/80 pt-2.5">
            <span>Allocated Cores:</span>
            <span className="font-mono text-slate-300 font-medium">
              {(summary?.total_allocated_cores ?? 0).toFixed(1)} vCPU
            </span>
          </div>
        </div>

        {/* Card 4: Backups & Storage */}
        <div className="bg-obsidian-900 border border-obsidian-750/80 hover:border-obsidian-600/80 rounded-xl p-5 shadow-sm transition-all relative overflow-hidden group">
          <div className="flex items-center justify-between">
            <span className="text-xs font-medium text-slate-400 uppercase tracking-wider">Backups Archive</span>
            <div className="w-8 h-8 rounded-lg bg-emerald-500/10 border border-emerald-500/20 flex items-center justify-center text-emerald-400">
              <HardDrive className="w-4 h-4" />
            </div>
          </div>
          <div className="mt-3 flex items-baseline gap-2">
            <span className="text-3xl font-mono font-bold text-slate-100">
              {summary?.total_backups_count ?? 0}
            </span>
            <span className="text-xs text-slate-400 font-mono">
              snapshots
            </span>
          </div>
          <div className="mt-4 flex items-center justify-between text-xs text-slate-400 border-t border-obsidian-800/80 pt-2.5">
            <span>Total Storage:</span>
            <span className="font-mono text-slate-300 font-medium">
              {formatBytes(summary?.total_backups_bytes ?? 0)}
            </span>
          </div>
        </div>
      </div>

      {/* Middle Row: Live Active Players & Security / Port Gate Stats */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Live Players Table (2 cols wide) */}
        <div className="lg:col-span-2 bg-obsidian-900 border border-obsidian-750/80 rounded-xl p-5 shadow-sm">
          <div className="flex items-center justify-between mb-4">
            <div className="flex items-center gap-2.5">
              <div className="w-2.5 h-2.5 rounded-full bg-emerald-400 animate-pulse"></div>
              <h2 className="text-base font-semibold text-slate-100">Live Active Players</h2>
              <span className="text-xs font-mono px-2 py-0.5 rounded-full bg-obsidian-800 border border-obsidian-700 text-slate-300">
                {summary?.active_players.length ?? 0}
              </span>
            </div>
            {user.role === 'admin' && (
              <Link
                to="/global-players"
                className="text-xs text-emerald-400 hover:text-emerald-300 flex items-center gap-1 font-medium transition-colors"
              >
                <span>Global Registry</span>
                <ChevronRight className="w-3.5 h-3.5" />
              </Link>
            )}
          </div>

          {summary && summary.active_players.length > 0 ? (
            <div className="overflow-x-auto">
              <table className="w-full text-left text-xs">
                <thead>
                  <tr className="border-b border-obsidian-800 text-slate-400 font-mono uppercase tracking-wider">
                    <th className="pb-2.5 font-medium">Gamertag</th>
                    <th className="pb-2.5 font-medium">Server Instance</th>
                    <th className="pb-2.5 font-medium">Role / Status</th>
                    <th className="pb-2.5 font-medium">Connected Since</th>
                    <th className="pb-2.5 text-right font-medium">Action</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-obsidian-800/60">
                  {summary.active_players.map((p) => (
                    <tr key={`${p.server_id}-${p.gamertag}`} className="hover:bg-obsidian-850/50 transition-colors">
                      <td className="py-3 font-mono font-medium text-slate-100 flex items-center gap-2">
                        <div className="w-7 h-7 rounded bg-obsidian-800 border border-obsidian-700 flex items-center justify-center text-slate-300 font-bold">
                          {p.gamertag.charAt(0).toUpperCase()}
                        </div>
                        <div>
                          <div>{p.gamertag}</div>
                          {p.xuid && <div className="text-[10px] text-slate-500 font-mono">XUID: {p.xuid}</div>}
                        </div>
                      </td>
                      <td className="py-3 text-slate-300 font-medium">
                        <Link
                          to={`/servers/${p.server_id}`}
                          className="hover:text-emerald-400 transition-colors inline-flex items-center gap-1"
                        >
                          <span>{p.server_name}</span>
                          <ExternalLink className="w-3 h-3 text-slate-500" />
                        </Link>
                      </td>
                      <td className="py-3">
                        {p.is_op ? (
                          <span className="px-2 py-0.5 rounded bg-emerald-500/10 text-emerald-400 border border-emerald-500/30 text-[10px] font-mono font-semibold uppercase">
                            Operator
                          </span>
                        ) : (
                          <span className="px-2 py-0.5 rounded bg-obsidian-800 text-slate-400 border border-obsidian-700 text-[10px] font-mono">
                            {p.permission || 'Member'}
                          </span>
                        )}
                      </td>
                      <td className="py-3 text-slate-400 font-mono text-[11px]">
                        {p.joined_at ? new Date(p.joined_at).toLocaleTimeString() : 'Active'}
                      </td>
                      <td className="py-3 text-right">
                        <Link
                          to={`/servers/${p.server_id}`}
                          className="px-2 py-1 bg-obsidian-800 hover:bg-obsidian-750 text-slate-200 rounded border border-obsidian-700 text-[11px] transition-colors inline-block"
                        >
                          View Instance
                        </Link>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ) : (
            <div className="py-8 text-center border border-dashed border-obsidian-800 rounded-xl bg-obsidian-950/40">
              <Users className="w-8 h-8 text-slate-600 mx-auto mb-2" />
              <div className="text-slate-300 text-sm font-medium">No players currently connected</div>
              <p className="text-slate-500 text-xs mt-1">
                When players join any running Bedrock server instance, they will appear here in real time.
              </p>
            </div>
          )}
        </div>

        {/* Security & Host Environment Specs (1 col) */}
        <div className="space-y-6">
          {/* Host Telemetry Box */}
          <div className="bg-obsidian-900 border border-obsidian-750/80 rounded-xl p-5 shadow-sm">
            <div className="flex items-center justify-between mb-4">
              <h2 className="text-base font-semibold text-slate-100 flex items-center gap-2">
                <Terminal className="w-4 h-4 text-emerald-400" />
                <span>Host Environment</span>
              </h2>
              <span className="text-[10px] font-mono uppercase bg-obsidian-800 px-2 py-0.5 rounded border border-obsidian-700 text-slate-400">
                {summary?.host_system.os} / {summary?.host_system.arch}
              </span>
            </div>

            <div className="space-y-3 text-xs">
              <div className="flex items-center justify-between py-1.5 border-b border-obsidian-800/80">
                <span className="text-slate-400">Uptime</span>
                <span className="font-mono text-slate-200 font-medium">
                  {formatUptime(summary?.host_system.uptime_sec ?? 0)}
                </span>
              </div>
              <div className="flex items-center justify-between py-1.5 border-b border-obsidian-800/80">
                <span className="text-slate-400">Go Runtime</span>
                <span className="font-mono text-slate-200">
                  {summary?.host_system.go_version}
                </span>
              </div>
              <div className="flex items-center justify-between py-1.5 border-b border-obsidian-800/80">
                <span className="text-slate-400">Active Goroutines</span>
                <span className="font-mono text-slate-200">
                  {summary?.host_system.goroutines ?? 0}
                </span>
              </div>
              <div className="flex items-center justify-between py-1.5 border-b border-obsidian-800/80">
                <span className="text-slate-400">Go Heap Alloc / Sys</span>
                <span className="font-mono text-slate-200">
                  {(summary?.host_system.alloc_mb ?? 0).toFixed(1)} MB / {(summary?.host_system.sys_mb ?? 0).toFixed(1)} MB
                </span>
              </div>
              <div className="flex items-center justify-between py-1.5">
                <span className="text-slate-400">Manager Core</span>
                <span className="font-mono text-emerald-400 font-medium">
                  {summary?.host_system.app_name}
                </span>
              </div>
            </div>
          </div>

          {/* Port Gate & Access Control Box */}
          <div className="bg-obsidian-900 border border-obsidian-750/80 rounded-xl p-5 shadow-sm">
            <div className="flex items-center justify-between mb-4">
              <h2 className="text-base font-semibold text-slate-100 flex items-center gap-2">
                <Shield className="w-4 h-4 text-emerald-400" />
                <span>Port Gate & Security</span>
              </h2>
              {user.role === 'admin' && (
                <Link
                  to="/portgate"
                  className="text-xs text-emerald-400 hover:text-emerald-300 font-medium flex items-center gap-1 transition-colors"
                >
                  <span>Rules</span>
                  <ChevronRight className="w-3.5 h-3.5" />
                </Link>
              )}
            </div>

            <div className="grid grid-cols-3 gap-2 text-center">
              <div className="bg-obsidian-850 border border-obsidian-700/80 rounded-lg p-2.5">
                <div className="text-lg font-mono font-bold text-slate-100">
                  {summary?.active_leases_count ?? 0}
                </div>
                <div className="text-[10px] text-slate-400 uppercase tracking-wider mt-0.5">Active Leases</div>
              </div>

              <div className="bg-obsidian-850 border border-obsidian-700/80 rounded-lg p-2.5">
                <div className="text-lg font-mono font-bold text-slate-100">
                  {summary?.allow_rules_count ?? 0}
                </div>
                <div className="text-[10px] text-slate-400 uppercase tracking-wider mt-0.5">Allow Rules</div>
              </div>

              <div className="bg-obsidian-850 border border-obsidian-700/80 rounded-lg p-2.5">
                <div className="text-lg font-mono font-bold text-slate-100">
                  {summary?.portgate_bans_count ?? 0}
                </div>
                <div className="text-[10px] text-slate-400 uppercase tracking-wider mt-0.5">Banned IPs</div>
              </div>
            </div>

            <div className="mt-4 pt-3 border-t border-obsidian-800 text-xs text-slate-400 flex items-center justify-between">
              <span>Banned Player Profiles:</span>
              <span className="font-mono text-slate-200 font-medium">
                {summary?.total_banned_players ?? 0}
              </span>
            </div>
          </div>
        </div>
      </div>

      {/* Fleet Quick Table */}
      <div className="bg-obsidian-900 border border-obsidian-750/80 rounded-xl p-5 shadow-sm">
        <div className="flex items-center justify-between mb-4">
          <div>
            <h2 className="text-base font-semibold text-slate-100 flex items-center gap-2">
              <Layers className="w-4 h-4 text-emerald-400" />
              <span>Server Instances Fleet</span>
            </h2>
            <p className="text-slate-400 text-xs mt-0.5">
              Live status, port mappings, and performance metrics across all configured game containers.
            </p>
          </div>
          <Link
            to="/servers"
            className="text-xs text-emerald-400 hover:text-emerald-300 flex items-center gap-1 font-medium transition-colors"
          >
            <span>Open Instance Manager</span>
            <ChevronRight className="w-3.5 h-3.5" />
          </Link>
        </div>

        {summary && summary.servers.length > 0 ? (
          <div className="overflow-x-auto">
            <table className="w-full text-left text-xs">
              <thead>
                <tr className="border-b border-obsidian-800 text-slate-400 font-mono uppercase tracking-wider">
                  <th className="pb-2.5 font-medium">Status</th>
                  <th className="pb-2.5 font-medium">Instance Name</th>
                  <th className="pb-2.5 font-medium">Port (UDP)</th>
                  <th className="pb-2.5 font-medium">Mode / Diff</th>
                  <th className="pb-2.5 font-medium">Online</th>
                  <th className="pb-2.5 font-medium">CPU & Memory</th>
                  <th className="pb-2.5 font-medium">Port Gate</th>
                  <th className="pb-2.5 text-right font-medium">Action</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-obsidian-800/60">
                {summary.servers.map((s) => {
                  const isRunning = s.status === 'running';
                  return (
                    <tr key={s.id} className="hover:bg-obsidian-850/50 transition-colors">
                      <td className="py-3">
                        <span
                          className={`inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-[10px] font-mono uppercase font-semibold border ${
                            isRunning
                              ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/30'
                              : s.status === 'crashed'
                              ? 'bg-rose-500/10 text-rose-400 border-rose-500/30'
                              : 'bg-slate-500/10 text-slate-400 border-slate-500/30'
                          }`}
                        >
                          <span
                            className={`w-1.5 h-1.5 rounded-full ${
                              isRunning ? 'bg-emerald-400 animate-pulse' : s.status === 'crashed' ? 'bg-rose-400' : 'bg-slate-500'
                            }`}
                          ></span>
                          {s.status}
                        </span>
                      </td>

                      <td className="py-3 font-medium text-slate-100">
                        <Link
                          to={`/servers/${s.id}`}
                          className="hover:text-emerald-400 transition-colors font-mono"
                        >
                          {s.name}
                        </Link>
                      </td>

                      <td className="py-3 font-mono text-slate-300">
                        {s.port}
                      </td>

                      <td className="py-3 text-slate-300 capitalize">
                        {s.mode} <span className="text-slate-500 text-[11px]">({s.difficulty})</span>
                      </td>

                      <td className="py-3">
                        <span className="font-mono text-slate-200 font-medium">
                          {s.online_players}
                        </span>
                      </td>

                      <td className="py-3 text-slate-400 font-mono text-[11px]">
                        {isRunning && (s.cpu_percent !== undefined || s.ram_bytes !== undefined) ? (
                          <span className="text-slate-200">
                            {(s.cpu_percent ?? 0).toFixed(1)}% CPU / {formatBytes(s.ram_bytes ?? 0)}
                          </span>
                        ) : (
                          <span className="text-slate-500">
                            {s.cpu_limit} cores / {s.memory_limit}
                          </span>
                        )}
                      </td>

                      <td className="py-3">
                        {s.port_gate_enabled ? (
                          <span className="inline-flex items-center gap-1 text-[10px] font-mono text-emerald-400 bg-emerald-500/10 px-2 py-0.5 rounded border border-emerald-500/20">
                            <Lock className="w-2.5 h-2.5" />
                            Active
                          </span>
                        ) : (
                          <span className="text-[10px] font-mono text-slate-500">
                            Disabled
                          </span>
                        )}
                      </td>

                      <td className="py-3 text-right">
                        <Link
                          to={`/servers/${s.id}`}
                          className="inline-flex items-center gap-1 px-2.5 py-1 bg-obsidian-800 hover:bg-obsidian-750 text-slate-200 rounded border border-obsidian-700 text-xs font-medium transition-colors"
                        >
                          <span>Manage</span>
                          <ArrowRight className="w-3 h-3 text-slate-400" />
                        </Link>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        ) : (
          <div className="py-8 text-center border border-dashed border-obsidian-800 rounded-xl bg-obsidian-950/40">
            <ServerIcon className="w-8 h-8 text-slate-600 mx-auto mb-2" />
            <div className="text-slate-300 text-sm font-medium">No server instances configured yet</div>
            <p className="text-slate-500 text-xs mt-1">
              Deploy your first Bedrock dedicated server to begin monitoring.
            </p>
            <Link
              to="/servers"
              className="mt-4 inline-flex items-center gap-1.5 px-3 py-1.5 bg-emerald-600 hover:bg-emerald-500 text-white rounded-lg text-xs font-medium transition-colors"
            >
              <span>Deploy First Instance</span>
            </Link>
          </div>
        )}
      </div>

      {/* Quick Navigation Cards */}
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
        <Link
          to="/servers"
          className="bg-obsidian-900 border border-obsidian-750/80 hover:border-emerald-500/50 p-4 rounded-xl flex items-center justify-between group transition-all"
        >
          <div className="flex items-center gap-3">
            <div className="w-8 h-8 rounded-lg bg-emerald-500/10 border border-emerald-500/20 flex items-center justify-center text-emerald-400 group-hover:scale-110 transition-transform">
              <ServerIcon className="w-4 h-4" />
            </div>
            <div>
              <div className="text-xs font-semibold text-slate-200">Server Fleet</div>
              <div className="text-[10px] text-slate-400">Deploy & control</div>
            </div>
          </div>
          <ChevronRight className="w-4 h-4 text-slate-500 group-hover:text-emerald-400 transition-colors" />
        </Link>

        {user.role === 'admin' && (
          <>
            <Link
              to="/portgate"
              className="bg-obsidian-900 border border-obsidian-750/80 hover:border-emerald-500/50 p-4 rounded-xl flex items-center justify-between group transition-all"
            >
              <div className="flex items-center gap-3">
                <div className="w-8 h-8 rounded-lg bg-emerald-500/10 border border-emerald-500/20 flex items-center justify-center text-emerald-400 group-hover:scale-110 transition-transform">
                  <Shield className="w-4 h-4" />
                </div>
                <div>
                  <div className="text-xs font-semibold text-slate-200">Port Gate</div>
                  <div className="text-[10px] text-slate-400">Firewall & passes</div>
                </div>
              </div>
              <ChevronRight className="w-4 h-4 text-slate-500 group-hover:text-emerald-400 transition-colors" />
            </Link>

            <Link
              to="/global-players"
              className="bg-obsidian-900 border border-obsidian-750/80 hover:border-emerald-500/50 p-4 rounded-xl flex items-center justify-between group transition-all"
            >
              <div className="flex items-center gap-3">
                <div className="w-8 h-8 rounded-lg bg-emerald-500/10 border border-emerald-500/20 flex items-center justify-center text-emerald-400 group-hover:scale-110 transition-transform">
                  <UserCheck className="w-4 h-4" />
                </div>
                <div>
                  <div className="text-xs font-semibold text-slate-200">Global Players</div>
                  <div className="text-[10px] text-slate-400">Roles & syncing</div>
                </div>
              </div>
              <ChevronRight className="w-4 h-4 text-slate-500 group-hover:text-emerald-400 transition-colors" />
            </Link>

            <Link
              to="/tasks"
              className="bg-obsidian-900 border border-obsidian-750/80 hover:border-emerald-500/50 p-4 rounded-xl flex items-center justify-between group transition-all"
            >
              <div className="flex items-center gap-3">
                <div className="w-8 h-8 rounded-lg bg-emerald-500/10 border border-emerald-500/20 flex items-center justify-center text-emerald-400 group-hover:scale-110 transition-transform">
                  <Clock className="w-4 h-4" />
                </div>
                <div>
                  <div className="text-xs font-semibold text-slate-200">Automations</div>
                  <div className="text-[10px] text-slate-400">Schedules & tasks</div>
                </div>
              </div>
              <ChevronRight className="w-4 h-4 text-slate-500 group-hover:text-emerald-400 transition-colors" />
            </Link>
          </>
        )}
      </div>
    </div>
  );
};
