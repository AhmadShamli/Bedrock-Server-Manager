import React, { useEffect, useState, useMemo } from 'react';
import { Link } from 'react-router-dom';
import {
  Plus,
  Server as ServerIcon,
  Shield,
  ExternalLink,
  HardDrive,
  AlertTriangle,
  Loader2,
  Play,
  Square,
  LayoutGrid,
  List,
  Search,
  Cpu,
  RefreshCw,
} from 'lucide-react';
import { api } from '../api/client';
import { Server, User, UserPlanStatus } from '../types';
import { usePagination } from '../hooks/usePagination';
import { Pagination } from '../components/Pagination';
import { DeployModal } from '../components/DeployModal';

interface ServersProps {
  user: User;
}

export const Servers: React.FC<ServersProps> = ({ user }) => {
  const [servers, setServers] = useState<Server[]>([]);
  const [userPlan, setUserPlan] = useState<UserPlanStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [showModal, setShowModal] = useState(false);
  const [actionLoading, setActionLoading] = useState<string | null>(null);
  const [refreshing, setRefreshing] = useState(false);

  // Search, filter, and view mode state
  const [searchQuery, setSearchQuery] = useState('');
  const [statusFilter, setStatusFilter] = useState<'all' | 'running' | 'stopped'>('all');
  const [viewMode, setViewMode] = useState<'grid' | 'table'>(() => {
    return (localStorage.getItem('bsm_server_view_mode') as 'grid' | 'table') || 'grid';
  });

  const handleViewModeChange = (mode: 'grid' | 'table') => {
    setViewMode(mode);
    localStorage.setItem('bsm_server_view_mode', mode);
  };

  const fetchServers = async (isManualRefresh = false) => {
    if (isManualRefresh) setRefreshing(true);
    try {
      const data = await api.listServers();
      setServers(data);
      if (user.role === 'user') {
        const plan = await api.getMyPlan();
        setUserPlan(plan);
      }
      setError(null);
    } catch (err: any) {
      setError(err.message || 'Failed to load servers');
    } finally {
      setLoading(false);
      if (isManualRefresh) setRefreshing(false);
    }
  };

  const isUserQuotaFull = Boolean(
    user.role === 'user' && userPlan && userPlan.usage.servers_count >= userPlan.usage.servers_max
  );

  useEffect(() => {
    fetchServers();
  }, []);

  // Filtered servers based on search and status
  const filteredServers = useMemo(() => {
    return (servers || []).filter((s) => {
      if (!s) return false;
      const q = searchQuery.toLowerCase().trim();
      const matchesSearch =
        !q ||
        s.name.toLowerCase().includes(q) ||
        s.id.toLowerCase().includes(q) ||
        String(s.port).includes(q) ||
        (s.version && s.version.toLowerCase().includes(q));

      const matchesStatus =
        statusFilter === 'all' ||
        (statusFilter === 'running' && s.status === 'running') ||
        (statusFilter === 'stopped' && s.status !== 'running');

      return matchesSearch && matchesStatus;
    });
  }, [servers, searchQuery, statusFilter]);

  // Pagination hook
  const {
    currentPage,
    pageSize,
    totalItems,
    paginatedItems: paginatedServers,
    setCurrentPage,
    setPageSize,
  } = usePagination(filteredServers, viewMode === 'grid' ? 12 : 10);

  const openDeployModal = () => {
    setShowModal(true);
  };

  const handleStart = async (id: string) => {
    setActionLoading(id);
    try {
      await api.startServer(id);
      await fetchServers();
    } catch (err: any) {
      alert(err.message || 'Failed to start server');
    } finally {
      setActionLoading(null);
    }
  };

  const handleStop = async (id: string) => {
    setActionLoading(id);
    try {
      await api.stopServer(id);
      await fetchServers();
    } catch (err: any) {
      alert(err.message || 'Failed to stop server');
    } finally {
      setActionLoading(null);
    }
  };

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
      {/* Top Banner */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4 mb-8">
        <div>
          <h1 className="text-2xl font-bold font-mono text-slate-100 tracking-wide flex items-center gap-2.5">
            <ServerIcon className="w-6 h-6 text-emerald-400" />
            <span>SERVER INSTANCES</span>
          </h1>
          <p className="text-sm text-slate-400 mt-1">
            Manage your containerized Bedrock Dedicated Server instances with isolated resources.
          </p>
        </div>

        <div className="flex items-center gap-3">
          <button
            onClick={() => fetchServers(true)}
            disabled={refreshing || loading}
            title="Refresh instances"
            className="p-2.5 rounded-lg bg-obsidian-900 border border-obsidian-700/80 hover:border-emerald-500/50 text-slate-300 hover:text-emerald-400 transition-all shadow-sm disabled:opacity-50"
          >
            <RefreshCw className={`w-4 h-4 ${refreshing ? 'animate-spin text-emerald-400' : ''}`} />
          </button>

          {user.role === 'admin' ? (
            <button
              onClick={openDeployModal}
              className="px-4 py-2.5 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-slate-950 font-bold font-mono text-sm tracking-wider flex items-center space-x-2 transition-all shadow-[0_0_15px_rgba(16,185,129,0.25)] hover:shadow-[0_0_20px_rgba(16,185,129,0.4)]"
            >
              <Plus className="w-4 h-4" />
              <span>Deploy Instance</span>
            </button>
          ) : user.role === 'user' ? (
            <button
              onClick={openDeployModal}
              disabled={isUserQuotaFull}
              className={`px-4 py-2.5 rounded-lg font-bold font-mono text-sm tracking-wider flex items-center space-x-2 transition-all shadow-sm ${
                isUserQuotaFull
                  ? 'bg-obsidian-800 text-slate-500 border border-obsidian-700 cursor-not-allowed opacity-60'
                  : 'bg-emerald-600 hover:bg-emerald-500 text-slate-950 shadow-[0_0_15px_rgba(16,185,129,0.25)] hover:shadow-[0_0_20px_rgba(16,185,129,0.4)]'
              }`}
            >
              {isUserQuotaFull ? (
                <>
                  <AlertTriangle className="w-4 h-4 text-amber-500" />
                  <span>Quota Full ({userPlan?.usage.servers_count}/{userPlan?.usage.servers_max})</span>
                </>
              ) : (
                <>
                  <Plus className="w-4 h-4" />
                  <span>Deploy Instance</span>
                </>
              )}
            </button>
          ) : null}
        </div>
      </div>

      {/* Normal User Plan Quota Overview */}
      {user.role === 'user' && userPlan && (
        <div className="mb-8 p-5 rounded-xl bg-obsidian-900 border border-obsidian-700/80 shadow-lg">
          <div className="flex flex-col md:flex-row md:items-center justify-between gap-4 pb-4 border-b border-obsidian-800">
            <div>
              <div className="flex items-center gap-2">
                <span className="text-xs uppercase font-mono tracking-wider text-slate-400">Current Plan:</span>
                <span className="font-mono font-bold text-slate-100 text-base">{userPlan.plan.name}</span>
                <span className={`text-[10px] px-2 py-0.5 rounded font-mono font-semibold uppercase ${
                  userPlan.user.plan_status === 'active' ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/30' : 'bg-slate-800 text-slate-400'
                }`}>
                  {userPlan.user.plan_status || 'active'}
                </span>
              </div>
              <p className="text-xs text-slate-400 mt-1 font-mono">{userPlan.plan.description || 'Self-service Bedrock Dedicated Server allocation.'}</p>
            </div>

            <div className="flex items-center gap-3">
              <div className="text-right">
                <div className="text-xs text-slate-400 font-mono">Server Quota</div>
                <div className="font-mono font-bold text-sm text-slate-100">
                  <span className={userPlan.usage.servers_count >= userPlan.usage.servers_max ? 'text-amber-400' : 'text-emerald-400'}>
                    {userPlan.usage.servers_count}
                  </span>
                  <span className="text-slate-500"> / </span>
                  <span>{userPlan.usage.servers_max} Active</span>
                </div>
              </div>

              {/* Progress mini bar */}
              <div className="w-24 h-3 bg-obsidian-950 rounded-full border border-obsidian-800 overflow-hidden">
                <div
                  className={`h-full transition-all ${
                    userPlan.usage.servers_count >= userPlan.usage.servers_max ? 'bg-amber-500' : 'bg-emerald-500'
                  }`}
                  style={{
                    width: `${Math.min(100, (userPlan.usage.servers_count / Math.max(1, userPlan.usage.servers_max)) * 100)}%`,
                  }}
                />
              </div>
            </div>
          </div>

          {/* Plan Limits Grid */}
          <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-6 gap-3 pt-4 text-xs font-mono">
            <div className="bg-obsidian-950 p-2.5 rounded-lg border border-obsidian-800/80">
              <span className="text-[10px] text-slate-500 block uppercase">RAM Limit</span>
              <span className="font-bold text-slate-200">{userPlan.plan.max_memory}</span>
            </div>
            <div className="bg-obsidian-950 p-2.5 rounded-lg border border-obsidian-800/80">
              <span className="text-[10px] text-slate-500 block uppercase">CPU Cores</span>
              <span className="font-bold text-slate-200">{userPlan.plan.max_cpu} Cores</span>
            </div>
            <div className="bg-obsidian-950 p-2.5 rounded-lg border border-obsidian-800/80">
              <span className="text-[10px] text-slate-500 block uppercase">Max Backups</span>
              <span className="font-bold text-slate-200">{userPlan.plan.max_backups_per_server} / srv</span>
            </div>
            <div className="bg-obsidian-950 p-2.5 rounded-lg border border-obsidian-800/80">
              <span className="text-[10px] text-slate-500 block uppercase">Max Players</span>
              <span className="font-bold text-slate-200">{userPlan.plan.max_player_slots}</span>
            </div>
            <div className="bg-obsidian-950 p-2.5 rounded-lg border border-obsidian-800/80">
              <span className="text-[10px] text-slate-500 block uppercase">Custom Port</span>
              <span className={userPlan.plan.allow_custom_port ? 'text-emerald-400 font-bold' : 'text-slate-400'}>
                {userPlan.plan.allow_custom_port ? 'Allowed' : 'Auto'}
              </span>
            </div>
            <div className="bg-obsidian-950 p-2.5 rounded-lg border border-obsidian-800/80">
              <span className="text-[10px] text-slate-500 block uppercase">Port Gate</span>
              <span className={userPlan.plan.allow_port_gate_keys ? 'text-emerald-400 font-bold' : 'text-slate-500'}>
                {userPlan.plan.allow_port_gate_keys ? 'Included' : 'Off'}
              </span>
            </div>
          </div>
        </div>
      )}

      {loading ? (
        <div className="flex flex-col items-center justify-center py-24 text-slate-400">
          <Loader2 className="w-8 h-8 animate-spin text-emerald-400 mb-3" />
          <span className="font-mono text-sm">Querying active instances...</span>
        </div>
      ) : error ? (
        <div className="p-4 rounded-xl bg-rose-950/40 border border-rose-500/40 text-rose-300 flex items-center space-x-3">
          <AlertTriangle className="w-5 h-5 text-rose-400 flex-shrink-0" />
          <span>{error}</span>
        </div>
      ) : (servers || []).length === 0 ? (
        <div className="text-center py-20 bg-obsidian-900/50 border border-obsidian-800 rounded-2xl p-8">
          <ServerIcon className="w-12 h-12 text-slate-600 mx-auto mb-3" />
          <h3 className="text-lg font-mono font-medium text-slate-200">No Bedrock instances running</h3>
          <p className="text-sm text-slate-400 max-w-sm mx-auto mt-1 mb-6">
            Create your first Minecraft Bedrock server instance with custom port and resource limits.
          </p>
          {(user.role === 'admin' || (user.role === 'user' && !isUserQuotaFull)) && (
            <button
              onClick={openDeployModal}
              className="px-4 py-2 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-slate-950 font-mono font-bold text-sm"
            >
              + Create Server
            </button>
          )}
        </div>
      ) : (
        <div>
          {/* Controls Bar: Search, Status Filter, View Toggle */}
          <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3 mb-6 bg-obsidian-900/70 border border-obsidian-800/80 p-3 rounded-xl">
            {/* Search Input */}
            <div className="relative flex-1 max-w-md">
              <Search className="w-4 h-4 absolute left-3 top-1/2 -translate-y-1/2 text-slate-500" />
              <input
                type="text"
                placeholder="Search instances by name, ID, port..."
                value={searchQuery}
                onChange={(e) => {
                  setSearchQuery(e.target.value);
                  setCurrentPage(1);
                }}
                className="w-full pl-9 pr-3 py-1.5 bg-obsidian-950 border border-obsidian-800 rounded-lg text-xs font-mono text-slate-200 placeholder-slate-500 focus:outline-none focus:border-emerald-500/60"
              />
            </div>

            {/* Filter & View Mode */}
            <div className="flex items-center gap-3">
              <div className="flex items-center space-x-1.5 text-xs font-mono">
                <span className="text-slate-500">Status:</span>
                <select
                  value={statusFilter}
                  onChange={(e) => {
                    setStatusFilter(e.target.value as any);
                    setCurrentPage(1);
                  }}
                  className="bg-obsidian-950 border border-obsidian-800 rounded-lg px-2.5 py-1.5 text-xs font-mono text-slate-200 focus:outline-none focus:border-emerald-500/60"
                >
                  <option value="all">All ({(servers || []).length})</option>
                  <option value="running">Running ({(servers || []).filter((s) => s.status === 'running').length})</option>
                  <option value="stopped">Stopped ({(servers || []).filter((s) => s.status !== 'running').length})</option>
                </select>
              </div>

              {/* View Mode Toggle: Grid vs Table */}
              <div className="flex items-center bg-obsidian-950 border border-obsidian-800 rounded-lg p-0.5">
                <button
                  onClick={() => handleViewModeChange('grid')}
                  title="Card Grid View"
                  className={`px-2.5 py-1.5 rounded flex items-center gap-1.5 text-xs font-mono transition-colors ${
                    viewMode === 'grid'
                      ? 'bg-obsidian-800 text-emerald-400 font-bold shadow-sm'
                      : 'text-slate-400 hover:text-slate-200'
                  }`}
                >
                  <LayoutGrid className="w-3.5 h-3.5" />
                  <span className="hidden sm:inline">Grid</span>
                </button>
                <button
                  onClick={() => handleViewModeChange('table')}
                  title="Table / List View"
                  className={`px-2.5 py-1.5 rounded flex items-center gap-1.5 text-xs font-mono transition-colors ${
                    viewMode === 'table'
                      ? 'bg-obsidian-800 text-emerald-400 font-bold shadow-sm'
                      : 'text-slate-400 hover:text-slate-200'
                  }`}
                >
                  <List className="w-3.5 h-3.5" />
                  <span className="hidden sm:inline">Table</span>
                </button>
              </div>
            </div>
          </div>

          {/* Filter Empty State */}
          {filteredServers.length === 0 ? (
            <div className="text-center py-16 bg-obsidian-900/50 border border-obsidian-800 rounded-xl p-8">
              <ServerIcon className="w-10 h-10 text-slate-600 mx-auto mb-3" />
              <h3 className="text-base font-mono font-medium text-slate-200">No instances match your filter</h3>
              <p className="text-xs text-slate-400 max-w-sm mx-auto mt-1 mb-4 font-mono">
                Try adjusting your search query or status filter.
              </p>
              <button
                onClick={() => {
                  setSearchQuery('');
                  setStatusFilter('all');
                }}
                className="px-3 py-1.5 rounded-lg bg-obsidian-800 hover:bg-obsidian-700 text-slate-300 font-mono text-xs transition-colors"
              >
                Reset Filters
              </button>
            </div>
          ) : viewMode === 'table' ? (
            /* Table / List View */
            <div className="bg-obsidian-900 border border-obsidian-800 rounded-xl overflow-hidden shadow-xl">
              <div className="overflow-x-auto">
                <table className="w-full text-left border-collapse">
                  <thead>
                    <tr className="border-b border-obsidian-800 bg-obsidian-950/70 text-[11px] font-mono text-slate-400 uppercase tracking-wider">
                      <th className="py-3 px-4">Status</th>
                      <th className="py-3 px-4">Server</th>
                      <th className="py-3 px-4">Port (UDP)</th>
                      <th className="py-3 px-4">Version</th>
                      <th className="py-3 px-4">Resources</th>
                      <th className="py-3 px-4">Port Gate</th>
                      <th className="py-3 px-4 text-right">Actions</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-obsidian-800/60 font-mono text-xs text-slate-300">
                    {paginatedServers.map((s) => (
                      <tr key={s.id} className="hover:bg-obsidian-850/50 transition-colors group">
                        {/* Status */}
                        <td className="py-3.5 px-4 whitespace-nowrap">
                          <span
                            className={`inline-flex items-center gap-1.5 text-[11px] font-mono px-2 py-0.5 rounded-full border uppercase tracking-wider ${
                              s.status === 'running'
                                ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/30'
                                : s.status === 'crashed'
                                ? 'bg-rose-500/10 text-rose-400 border-rose-500/30'
                                : 'bg-slate-800 text-slate-400 border-slate-700'
                            }`}
                          >
                            <span
                              className={`w-1.5 h-1.5 rounded-full ${
                                s.status === 'running'
                                  ? 'bg-emerald-400 animate-pulse'
                                  : s.status === 'crashed'
                                  ? 'bg-rose-400'
                                  : 'bg-slate-500'
                              }`}
                            />
                            {s.status}
                          </span>
                        </td>

                        {/* Server Name & ID */}
                        <td className="py-3.5 px-4">
                          <div className="flex flex-col">
                            <Link
                              to={`/servers/${s.id}`}
                              className="font-bold text-slate-100 group-hover:text-emerald-400 transition-colors text-sm"
                            >
                              {s.name}
                            </Link>
                            <div className="flex items-center gap-2 mt-0.5">
                              <span className="text-[11px] text-slate-400">ID: {s.id}</span>
                              {s.seed && (
                                <span className="text-[10px] px-1.5 py-0.2 rounded bg-obsidian-950 border border-obsidian-800 text-slate-300 font-mono">
                                  Seed: {s.seed}
                                </span>
                              )}
                              <span className="text-[10px] px-1.5 py-0.2 rounded bg-obsidian-950 border border-obsidian-800 text-slate-400 uppercase">
                                {s.mode}
                              </span>
                              <span className="text-[10px] px-1.5 py-0.2 rounded bg-obsidian-950 border border-obsidian-800 text-slate-400 uppercase">
                                {s.difficulty}
                              </span>
                            </div>
                          </div>
                        </td>

                        {/* Port (UDP) */}
                        <td className="py-3.5 px-4 whitespace-nowrap text-slate-200">
                          <div className="flex items-center gap-1.5">
                            <span className="font-bold text-emerald-400/90">{s.port}</span>
                            <span className="text-[10px] text-slate-500">/ v6: {s.portv6 || s.port + 1}</span>
                          </div>
                        </td>

                        {/* Version */}
                        <td className="py-3.5 px-4 whitespace-nowrap text-slate-400">
                          <span className="px-2 py-0.5 rounded bg-obsidian-950 border border-obsidian-800 text-[11px] text-slate-300">
                            v{s.version || 'latest'}
                          </span>
                        </td>

                        {/* Resources */}
                        <td className="py-3.5 px-4 whitespace-nowrap">
                          <div className="flex items-center gap-2 text-[11px] text-slate-300">
                            <span className="flex items-center gap-1 bg-obsidian-950 px-2 py-0.5 rounded border border-obsidian-800">
                              <HardDrive className="w-3 h-3 text-emerald-400/70" />
                              {s.memory_limit}
                            </span>
                            <span className="flex items-center gap-1 bg-obsidian-950 px-2 py-0.5 rounded border border-obsidian-800">
                              <Cpu className="w-3 h-3 text-cyan-400/70" />
                              {s.cpu_limit} Cores
                            </span>
                          </div>
                        </td>

                        {/* Port Gate */}
                        <td className="py-3.5 px-4 whitespace-nowrap">
                          {s.port_gate_enabled ? (
                            <div className="inline-flex items-center gap-1.5 px-2 py-0.5 rounded bg-emerald-950/40 border border-emerald-500/20 text-[11px] text-emerald-400">
                              <Shield className="w-3 h-3" />
                              <span className="capitalize">{s.port_gate_mode}</span>
                              <Link
                                to={`/knock/${s.id}`}
                                target="_blank"
                                title="Open Knock Portal"
                                className="text-emerald-300 hover:text-emerald-100 ml-1"
                              >
                                <ExternalLink className="w-2.5 h-2.5" />
                              </Link>
                            </div>
                          ) : (
                            <span className="text-slate-500 text-[11px]">Disabled</span>
                          )}
                        </td>

                        {/* Actions */}
                        <td className="py-3.5 px-4 whitespace-nowrap text-right">
                          <div className="flex items-center justify-end space-x-2">
                            {s.status === 'running' ? (
                              <button
                                onClick={() => handleStop(s.id)}
                                disabled={actionLoading === s.id}
                                title="Stop Server"
                                className="p-1.5 rounded-lg bg-obsidian-950 hover:bg-rose-950/60 text-slate-400 hover:text-rose-400 border border-obsidian-700 transition-colors"
                              >
                                {actionLoading === s.id ? (
                                  <Loader2 className="w-3.5 h-3.5 animate-spin" />
                                ) : (
                                  <Square className="w-3.5 h-3.5 fill-current" />
                                )}
                              </button>
                            ) : (
                              <button
                                onClick={() => handleStart(s.id)}
                                disabled={actionLoading === s.id}
                                title="Start Server"
                                className="p-1.5 rounded-lg bg-obsidian-950 hover:bg-emerald-950/60 text-slate-400 hover:text-emerald-400 border border-obsidian-700 transition-colors"
                              >
                                {actionLoading === s.id ? (
                                  <Loader2 className="w-3.5 h-3.5 animate-spin" />
                                ) : (
                                  <Play className="w-3.5 h-3.5 fill-current" />
                                )}
                              </button>
                            )}

                            <Link
                              to={`/servers/${s.id}`}
                              className="px-2.5 py-1.5 rounded-lg bg-obsidian-800 hover:bg-obsidian-700 text-slate-200 hover:text-emerald-400 text-xs font-mono font-medium transition-colors"
                            >
                              Manage Hub →
                            </Link>
                          </div>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>

              {/* Table Pagination */}
              <Pagination
                currentPage={currentPage}
                totalItems={totalItems}
                pageSize={pageSize}
                onPageChange={setCurrentPage}
                onPageSizeChange={setPageSize}
                pageSizeOptions={[10, 25, 50, 100]}
              />
            </div>
          ) : (
            /* Card Grid View */
            <div>
              <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
                {paginatedServers.map((s) => (
                  <div
                    key={s.id}
                    className="bg-obsidian-900 border border-obsidian-700/80 hover:border-emerald-500/50 rounded-xl p-5 shadow-lg transition-all flex flex-col justify-between group"
                  >
                    <div>
                      <div className="flex items-start justify-between mb-3">
                        <div>
                          <h3 className="font-bold text-base text-slate-100 font-mono group-hover:text-emerald-400 transition-colors">
                            {s.name}
                          </h3>
                          <span className="text-xs text-slate-400 font-mono">ID: {s.id}</span>
                        </div>
                        <span
                          className={`text-[11px] font-mono px-2 py-0.5 rounded-full border uppercase tracking-wider ${
                            s.status === 'running'
                              ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/30'
                              : s.status === 'crashed'
                              ? 'bg-rose-500/10 text-rose-400 border-rose-500/30'
                              : 'bg-slate-800 text-slate-400 border-slate-700'
                          }`}
                        >
                          {s.status}
                        </span>
                      </div>

                      <div className="grid grid-cols-2 gap-2 my-4 text-xs font-mono text-slate-300">
                        <div className="bg-obsidian-950 p-2 rounded-lg border border-obsidian-800 flex items-center space-x-2">
                          <span className="text-slate-500">UDP:</span>
                          <span className="text-slate-200">{s.port}</span>
                        </div>
                        <div className="bg-obsidian-950 p-2 rounded-lg border border-obsidian-800 flex items-center space-x-2">
                          <HardDrive className="w-3.5 h-3.5 text-emerald-500/70" />
                          <span>{s.memory_limit}</span>
                        </div>
                      </div>

                      {s.seed && (
                        <div className="mb-3 px-2.5 py-1 rounded bg-obsidian-950 border border-obsidian-800 text-[11px] font-mono text-slate-400 flex items-center justify-between">
                          <span className="text-slate-500">Seed:</span>
                          <span className="text-slate-200 font-semibold truncate max-w-[160px]">{s.seed}</span>
                        </div>
                      )}

                      {s.port_gate_enabled && (
                        <div className="mb-4 flex items-center justify-between px-2.5 py-1 rounded bg-emerald-950/40 border border-emerald-500/20 text-[11px] font-mono text-emerald-400">
                          <span className="flex items-center space-x-1.5">
                            <Shield className="w-3.5 h-3.5" />
                            <span>Port Gate ({s.port_gate_mode})</span>
                          </span>
                          <Link
                            to={`/knock/${s.id}`}
                            target="_blank"
                            className="hover:underline flex items-center space-x-1 text-emerald-300"
                          >
                            <span>Knock Portal</span>
                            <ExternalLink className="w-3 h-3" />
                          </Link>
                        </div>
                      )}
                    </div>

                    <div className="pt-3 border-t border-obsidian-800 flex items-center justify-between">
                      <div className="flex items-center space-x-2">
                        <Link
                          to={`/servers/${s.id}`}
                          className="px-3 py-1.5 rounded-lg bg-obsidian-800 hover:bg-obsidian-700 text-slate-200 hover:text-emerald-400 text-xs font-mono font-medium transition-colors"
                        >
                          Manage Hub →
                        </Link>

                        {s.status === 'running' ? (
                          <button
                            onClick={() => handleStop(s.id)}
                            disabled={actionLoading === s.id}
                            title="Stop Server"
                            className="p-1.5 rounded-lg bg-obsidian-950 hover:bg-rose-950/60 text-slate-400 hover:text-rose-400 border border-obsidian-700 transition-colors"
                          >
                            {actionLoading === s.id ? (
                              <Loader2 className="w-3.5 h-3.5 animate-spin" />
                            ) : (
                              <Square className="w-3.5 h-3.5 fill-current" />
                            )}
                          </button>
                        ) : (
                          <button
                            onClick={() => handleStart(s.id)}
                            disabled={actionLoading === s.id}
                            title="Start Server"
                            className="p-1.5 rounded-lg bg-obsidian-950 hover:bg-emerald-950/60 text-slate-400 hover:text-emerald-400 border border-obsidian-700 transition-colors"
                          >
                            {actionLoading === s.id ? (
                              <Loader2 className="w-3.5 h-3.5 animate-spin" />
                            ) : (
                              <Play className="w-3.5 h-3.5 fill-current" />
                            )}
                          </button>
                        )}
                      </div>

                      <div className="text-[11px] text-slate-500 font-mono">
                        v{s.version}
                      </div>
                    </div>
                  </div>
                ))}
              </div>

              {/* Grid Pagination */}
              <div className="mt-6 bg-obsidian-900 border border-obsidian-800 rounded-xl overflow-hidden shadow-xl">
                <Pagination
                  currentPage={currentPage}
                  totalItems={totalItems}
                  pageSize={pageSize}
                  onPageChange={setCurrentPage}
                  onPageSizeChange={setPageSize}
                  pageSizeOptions={[6, 12, 24, 48]}
                />
              </div>
            </div>
          )}
        </div>
      )}

      {/* Deploy Instance Modal (Quick Deploy or Guided Wizard with Popular Seeds) */}
      <DeployModal
        isOpen={showModal}
        onClose={() => setShowModal(false)}
        onSuccess={() => fetchServers(true)}
        userPlan={userPlan}
        user={user}
      />
    </div>
  );
};
