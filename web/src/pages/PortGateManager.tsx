import React, { useState, useEffect, useMemo } from 'react';
import { Link } from 'react-router-dom';
import {
  Shield, Globe, Server as ServerIcon, Ban, Clock, Search, RefreshCw,
  Plus, AlertCircle, ExternalLink, Copy, Check,
  Loader2, Filter, AlertTriangle, ArrowUpRight
} from 'lucide-react';
import { api } from '../api/client';
import { Server, PortGateAllowRule, PortGateBanRule, PortGateLease } from '../types';
import { Pagination } from '../components/Pagination';
import { usePagination } from '../hooks/usePagination';

export const PortGateManager: React.FC = () => {
  const [activeTab, setActiveTab] = useState<'whitelist' | 'bans' | 'leases' | 'instances'>('whitelist');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [actionLoading, setActionLoading] = useState(false);

  // Data states
  const [servers, setServers] = useState<Server[]>([]);
  const [whitelistRules, setWhitelistRules] = useState<PortGateAllowRule[]>([]);
  const [banRules, setBanRules] = useState<PortGateBanRule[]>([]);
  const [activeLeases, setActiveLeases] = useState<PortGateLease[]>([]);
  const [detectedClientIP, setDetectedClientIP] = useState<string | null>(null);

  // Filters
  const [scopeFilter, setScopeFilter] = useState<string>('all'); // 'all', 'global', or serverId
  const [search, setSearch] = useState('');

  // Modals
  const [showAddAllowModal, setShowAddAllowModal] = useState(false);
  const [newAllowIP, setNewAllowIP] = useState('');
  const [newAllowScope, setNewAllowScope] = useState<'global' | string>('global');
  const [newAllowComment, setNewAllowComment] = useState('');

  const [showAddBanModal, setShowAddBanModal] = useState(false);
  const [newBanIP, setNewBanIP] = useState('');
  const [newBanScope, setNewBanScope] = useState<'global' | string>('global');
  const [newBanReason, setNewBanReason] = useState('');

  // Copied state
  const [copiedText, setCopiedText] = useState<string | null>(null);

  const copyToClipboard = (text: string, id: string) => {
    navigator.clipboard.writeText(text);
    setCopiedText(id);
    setTimeout(() => setCopiedText(null), 2000);
  };

  const loadData = async () => {
    setLoading(true);
    setError(null);
    try {
      const [serversRes, allowRes, banRes, leasesRes] = await Promise.all([
        api.listServers().catch(() => []),
        api.listPortGateAllowRules().catch(() => []),
        api.listPortGateBans().catch(() => []),
        api.listAllPortGateLeases().catch(() => []),
      ]);

      const srvList = Array.isArray(serversRes) ? serversRes : [];
      setServers(srvList);
      setWhitelistRules(Array.isArray(allowRes) ? allowRes : []);
      setBanRules(Array.isArray(banRes) ? banRes : []);
      setActiveLeases(Array.isArray(leasesRes) ? leasesRes : []);

      // Detect client IP if servers exist
      if (srvList.length > 0) {
        try {
          const conf = await api.getKnockConfig(srvList[0].id);
          if (conf && conf.client_ip) {
            setDetectedClientIP(conf.client_ip);
          }
        } catch {
          // ignore
        }
      }
    } catch (err: any) {
      setError(err.message || 'Failed to load Port Gate data');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadData();
  }, []);

  const serverMap = useMemo(() => {
    const map = new Map<string, Server>();
    servers.forEach((s) => map.set(s.id, s));
    return map;
  }, [servers]);

  // Filter Whitelist Rules
  const filteredWhitelistRules = useMemo(() => {
    return whitelistRules.filter((r) => {
      const isGlobal = !r.server_id;
      if (scopeFilter === 'global' && !isGlobal) return false;
      if (scopeFilter !== 'all' && scopeFilter !== 'global' && r.server_id !== scopeFilter) return false;

      if (search.trim()) {
        const q = search.toLowerCase();
        const serverName = r.server_id ? (serverMap.get(r.server_id)?.name || r.server_id).toLowerCase() : '';
        const matchIP = r.ip_or_subnet.toLowerCase().includes(q);
        const matchComment = r.comment.toLowerCase().includes(q);
        const matchServer = serverName.includes(q);
        if (!matchIP && !matchComment && !matchServer) return false;
      }
      return true;
    });
  }, [whitelistRules, scopeFilter, search, serverMap]);

  // Filter Ban Rules
  const filteredBanRules = useMemo(() => {
    return banRules.filter((b) => {
      const isGlobal = !b.server_id;
      if (scopeFilter === 'global' && !isGlobal) return false;
      if (scopeFilter !== 'all' && scopeFilter !== 'global' && b.server_id !== scopeFilter) return false;

      if (search.trim()) {
        const q = search.toLowerCase();
        const serverName = b.server_id ? (serverMap.get(b.server_id)?.name || b.server_id).toLowerCase() : '';
        const matchIP = b.ip_or_subnet.toLowerCase().includes(q);
        const matchReason = b.reason.toLowerCase().includes(q);
        const matchServer = serverName.includes(q);
        const matchAuthor = b.banned_by.toLowerCase().includes(q);
        if (!matchIP && !matchReason && !matchServer && !matchAuthor) return false;
      }
      return true;
    });
  }, [banRules, scopeFilter, search, serverMap]);

  // Filter Active Leases
  const filteredActiveLeases = useMemo(() => {
    return activeLeases.filter((l) => {
      if (scopeFilter !== 'all' && scopeFilter !== 'global' && l.server_id !== scopeFilter) return false;

      if (search.trim()) {
        const q = search.toLowerCase();
        const serverName = (serverMap.get(l.server_id)?.name || l.server_id).toLowerCase();
        const matchIP = l.ip_address.toLowerCase().includes(q);
        const matchGamer = (l.gamertag || '').toLowerCase().includes(q);
        const matchServer = serverName.includes(q);
        if (!matchIP && !matchGamer && !matchServer) return false;
      }
      return true;
    });
  }, [activeLeases, scopeFilter, search, serverMap]);

  // Filter Servers
  const filteredServers = useMemo(() => {
    return servers.filter((s) => {
      if (scopeFilter !== 'all' && scopeFilter !== 'global' && s.id !== scopeFilter) return false;
      if (search.trim()) {
        const q = search.toLowerCase();
        return s.name.toLowerCase().includes(q) || s.id.toLowerCase().includes(q) || s.port.toString().includes(q);
      }
      return true;
    });
  }, [servers, scopeFilter, search]);

  // Paginations
  const whitelistPagination = usePagination(filteredWhitelistRules, 10);
  const banPagination = usePagination(filteredBanRules, 10);
  const leasesPagination = usePagination(filteredActiveLeases, 10);
  const serversPagination = usePagination(filteredServers, 10);

  // Stats Counters
  const stats = useMemo(() => {
    const totalServers = servers.length;
    const protectedServers = servers.filter((s) => s.port_gate_enabled).length;
    const globalAllow = whitelistRules.filter((r) => !r.server_id).length;
    const instanceAllow = whitelistRules.length - globalAllow;
    const globalBans = banRules.filter((b) => !b.server_id).length;
    const instanceBans = banRules.length - globalBans;
    const totalLeases = activeLeases.length;

    return {
      totalServers,
      protectedServers,
      globalAllow,
      instanceAllow,
      totalAllow: whitelistRules.length,
      globalBans,
      instanceBans,
      totalBans: banRules.length,
      totalLeases,
    };
  }, [servers, whitelistRules, banRules, activeLeases]);

  // Handlers for Allow Rules
  const handleCreateAllowRule = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newAllowIP.trim()) return;

    setActionLoading(true);
    try {
      const isGlobal = newAllowScope === 'global';
      const serverId = isGlobal ? null : newAllowScope;
      await api.createPortGateAllowRule({
        ip_or_subnet: newAllowIP.trim(),
        is_global: isGlobal,
        server_id: serverId,
        comment: newAllowComment.trim(),
      });

      setShowAddAllowModal(false);
      setNewAllowIP('');
      setNewAllowComment('');
      setNewAllowScope('global');
      await loadData();
    } catch (err: any) {
      alert(`Failed to add allow rule: ${err.message || 'Unknown error'}`);
    } finally {
      setActionLoading(false);
    }
  };

  const handleDeleteAllowRule = async (ruleId: number, serverId?: string | null) => {
    if (!window.confirm('Are you sure you want to remove this allowed IP/subnet rule? Traffic from this IP will require normal port-knocking authorization.')) {
      return;
    }

    setActionLoading(true);
    try {
      await api.deletePortGateAllowRule(ruleId, serverId || undefined);
      await loadData();
    } catch (err: any) {
      alert(`Failed to delete rule: ${err.message || 'Unknown error'}`);
    } finally {
      setActionLoading(false);
    }
  };

  // Handlers for Ban Rules
  const handleCreateBanRule = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newBanIP.trim()) return;

    setActionLoading(true);
    try {
      const isGlobal = newBanScope === 'global';
      const serverId = isGlobal ? null : newBanScope;
      await api.createPortGateBan({
        ip_or_subnet: newBanIP.trim(),
        is_global: isGlobal,
        server_id: serverId,
        reason: newBanReason.trim() || 'Blocked by administrator',
      });

      setShowAddBanModal(false);
      setNewBanIP('');
      setNewBanReason('');
      setNewBanScope('global');
      await loadData();
    } catch (err: any) {
      alert(`Failed to ban IP: ${err.message || 'Unknown error'}`);
    } finally {
      setActionLoading(false);
    }
  };

  const handleDeleteBanRule = async (banId: number, serverId?: string | null) => {
    if (!window.confirm('Are you sure you want to unban this IP/subnet? The client will be allowed to access the Knock Portal.')) {
      return;
    }

    setActionLoading(true);
    try {
      await api.deletePortGateBan(banId, serverId || undefined);
      await loadData();
    } catch (err: any) {
      alert(`Failed to unban rule: ${err.message || 'Unknown error'}`);
    } finally {
      setActionLoading(false);
    }
  };

  // Handlers for Leases
  const handleRevokeLease = async (leaseId: number) => {
    if (!window.confirm('Immediately revoke this active port gate lease? The player UDP port grant will be closed instantly.')) {
      return;
    }

    setActionLoading(true);
    try {
      await api.revokeCentralizedLease(leaseId);
      await loadData();
    } catch (err: any) {
      alert(`Failed to revoke lease: ${err.message || 'Unknown error'}`);
    } finally {
      setActionLoading(false);
    }
  };

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8 space-y-6">
      {/* Header */}
      <div className="flex flex-col md:flex-row md:items-center justify-between gap-4 pb-6 border-b border-obsidian-800">
        <div>
          <div className="flex items-center space-x-3">
            <div className="w-10 h-10 rounded-xl bg-emerald-950 border border-emerald-500/40 flex items-center justify-center text-emerald-400 shadow-[0_0_20px_rgba(16,185,129,0.2)]">
              <Shield className="w-5 h-5" />
            </div>
            <div>
              <h1 className="text-xl sm:text-2xl font-bold font-mono text-slate-100 flex items-center gap-2">
                <span>Port Gate Manager</span>
                <span className="text-xs px-2 py-0.5 rounded bg-emerald-500/10 text-emerald-400 border border-emerald-500/30">
                  Global & Multi-Instance
                </span>
              </h1>
              <p className="text-xs text-slate-400 font-mono mt-0.5">
                Centralized dynamic firewall security, permanent allowlists, IP ban blocklists, and live player leases.
              </p>
            </div>
          </div>
        </div>

        <div className="flex items-center space-x-3">
          <button
            onClick={loadData}
            disabled={loading}
            className="px-3 py-2 rounded-lg bg-obsidian-850 hover:bg-obsidian-800 border border-obsidian-700 text-slate-300 font-mono text-xs flex items-center space-x-2 transition-colors"
          >
            <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin text-emerald-400' : ''}`} />
            <span>Refresh</span>
          </button>

          <button
            onClick={() => setShowAddAllowModal(true)}
            className="px-3.5 py-2 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-slate-950 font-mono text-xs font-bold flex items-center space-x-1.5 transition-colors shadow-[0_0_15px_rgba(16,185,129,0.2)]"
          >
            <Plus className="w-3.5 h-3.5" />
            <span>Allow IP / Subnet</span>
          </button>

          <button
            onClick={() => setShowAddBanModal(true)}
            className="px-3.5 py-2 rounded-lg bg-rose-600 hover:bg-rose-500 text-white font-mono text-xs font-bold flex items-center space-x-1.5 transition-colors shadow-[0_0_15px_rgba(244,63,94,0.2)]"
          >
            <Ban className="w-3.5 h-3.5" />
            <span>Ban IP / Subnet</span>
          </button>
        </div>
      </div>

      {error && (
        <div className="p-4 rounded-xl bg-rose-950/40 border border-rose-500/50 flex items-start space-x-3 text-rose-300 text-xs font-mono">
          <AlertCircle className="w-4 h-4 flex-shrink-0 mt-0.5 text-rose-400" />
          <span>{error}</span>
        </div>
      )}

      {/* Metric Summary Cards */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        {/* Card 1: Protected Instances */}
        <div className="bg-obsidian-900 border border-obsidian-700/80 rounded-xl p-4 relative overflow-hidden">
          <div className="flex items-center justify-between mb-2">
            <span className="text-xs font-mono text-slate-400">Protected Instances</span>
            <div className="p-2 rounded-lg bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
              <ServerIcon className="w-4 h-4" />
            </div>
          </div>
          <div className="flex items-baseline space-x-2">
            <span className="text-2xl font-bold font-mono text-slate-100">
              {stats.protectedServers}
            </span>
            <span className="text-xs font-mono text-slate-400">
              / {stats.totalServers} servers active
            </span>
          </div>
          <div className="mt-2 text-[11px] font-mono text-slate-400 flex items-center gap-1.5">
            <span className="w-1.5 h-1.5 rounded-full bg-emerald-400 animate-pulse"></span>
            <span>Default-deny firewall active</span>
          </div>
        </div>

        {/* Card 2: Always-Allowed Whitelist */}
        <div className="bg-obsidian-900 border border-obsidian-700/80 rounded-xl p-4 relative overflow-hidden">
          <div className="flex items-center justify-between mb-2">
            <span className="text-xs font-mono text-slate-400">Always-Allowed Rules</span>
            <div className="p-2 rounded-lg bg-sky-500/10 text-sky-400 border border-sky-500/20">
              <Globe className="w-4 h-4" />
            </div>
          </div>
          <div className="flex items-baseline space-x-2">
            <span className="text-2xl font-bold font-mono text-sky-400">
              {stats.totalAllow}
            </span>
            <span className="text-xs font-mono text-slate-400">
              ({stats.globalAllow} Global, {stats.instanceAllow} Local)
            </span>
          </div>
          <div className="mt-2 text-[11px] font-mono text-slate-400">
            Bypass knocking automatically
          </div>
        </div>

        {/* Card 3: Banned Blocklist */}
        <div className="bg-obsidian-900 border border-obsidian-700/80 rounded-xl p-4 relative overflow-hidden">
          <div className="flex items-center justify-between mb-2">
            <span className="text-xs font-mono text-slate-400">Banned IPs & Subnets</span>
            <div className="p-2 rounded-lg bg-rose-500/10 text-rose-400 border border-rose-500/20">
              <Ban className="w-4 h-4" />
            </div>
          </div>
          <div className="flex items-baseline space-x-2">
            <span className="text-2xl font-bold font-mono text-rose-400">
              {stats.totalBans}
            </span>
            <span className="text-xs font-mono text-slate-400">
              ({stats.globalBans} Global, {stats.instanceBans} Local)
            </span>
          </div>
          <div className="mt-2 text-[11px] font-mono text-slate-400">
            Knocking & port access denied
          </div>
        </div>

        {/* Card 4: Active Leases */}
        <div className="bg-obsidian-900 border border-obsidian-700/80 rounded-xl p-4 relative overflow-hidden">
          <div className="flex items-center justify-between mb-2">
            <span className="text-xs font-mono text-slate-400">Live Dynamic Leases</span>
            <div className="p-2 rounded-lg bg-amber-500/10 text-amber-400 border border-amber-500/20">
              <Clock className="w-4 h-4" />
            </div>
          </div>
          <div className="flex items-baseline space-x-2">
            <span className="text-2xl font-bold font-mono text-amber-400">
              {stats.totalLeases}
            </span>
            <span className="text-xs font-mono text-slate-400">active sessions</span>
          </div>
          <div className="mt-2 text-[11px] font-mono text-slate-400">
            Dynamic temporary firewall grants
          </div>
        </div>
      </div>

      {/* Main Tabs & Filters Toolbar */}
      <div className="bg-obsidian-900 border border-obsidian-700/80 rounded-xl p-4 space-y-4">
        {/* Tabs Row */}
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 border-b border-obsidian-800 pb-3">
          <div className="flex items-center space-x-2 overflow-x-auto pb-1 sm:pb-0">
            <button
              onClick={() => setActiveTab('whitelist')}
              className={`px-3 py-2 rounded-lg font-mono text-xs font-bold flex items-center space-x-2 transition-colors whitespace-nowrap ${
                activeTab === 'whitelist'
                  ? 'bg-sky-500/10 text-sky-400 border border-sky-500/30'
                  : 'text-slate-400 hover:text-slate-200 hover:bg-obsidian-800'
              }`}
            >
              <Globe className="w-3.5 h-3.5" />
              <span>Always Allowed (Whitelist)</span>
              <span className="px-1.5 py-0.2 rounded bg-obsidian-800 text-[10px]">
                {whitelistRules.length}
              </span>
            </button>

            <button
              onClick={() => setActiveTab('bans')}
              className={`px-3 py-2 rounded-lg font-mono text-xs font-bold flex items-center space-x-2 transition-colors whitespace-nowrap ${
                activeTab === 'bans'
                  ? 'bg-rose-500/10 text-rose-400 border border-rose-500/30'
                  : 'text-slate-400 hover:text-slate-200 hover:bg-obsidian-800'
              }`}
            >
              <Ban className="w-3.5 h-3.5" />
              <span>Banned IPs (Blocklist)</span>
              <span className="px-1.5 py-0.2 rounded bg-obsidian-800 text-[10px]">
                {banRules.length}
              </span>
            </button>

            <button
              onClick={() => setActiveTab('leases')}
              className={`px-3 py-2 rounded-lg font-mono text-xs font-bold flex items-center space-x-2 transition-colors whitespace-nowrap ${
                activeTab === 'leases'
                  ? 'bg-amber-500/10 text-amber-400 border border-amber-500/30'
                  : 'text-slate-400 hover:text-slate-200 hover:bg-obsidian-800'
              }`}
            >
              <Clock className="w-3.5 h-3.5" />
              <span>Active Leases</span>
              <span className="px-1.5 py-0.2 rounded bg-obsidian-800 text-[10px]">
                {activeLeases.length}
              </span>
            </button>

            <button
              onClick={() => setActiveTab('instances')}
              className={`px-3 py-2 rounded-lg font-mono text-xs font-bold flex items-center space-x-2 transition-colors whitespace-nowrap ${
                activeTab === 'instances'
                  ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/30'
                  : 'text-slate-400 hover:text-slate-200 hover:bg-obsidian-800'
              }`}
            >
              <ServerIcon className="w-3.5 h-3.5" />
              <span>Instances Gate Status</span>
              <span className="px-1.5 py-0.2 rounded bg-obsidian-800 text-[10px]">
                {servers.length}
              </span>
            </button>
          </div>
        </div>

        {/* Filter Controls Row */}
        <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3">
          <div className="flex items-center space-x-2">
            <Filter className="w-4 h-4 text-slate-400" />
            <span className="text-xs font-mono text-slate-400">Filter Scope:</span>
            <select
              value={scopeFilter}
              onChange={(e) => setScopeFilter(e.target.value)}
              className="bg-obsidian-950 border border-obsidian-700 text-slate-200 rounded-lg px-2.5 py-1.5 font-mono text-xs focus:outline-none focus:border-emerald-500"
            >
              <option value="all">All Scopes (Global & Instances)</option>
              <option value="global">Global Only (All Instances)</option>
              {servers.map((s) => (
                <option key={s.id} value={s.id}>
                  Instance: {s.name} ({s.id})
                </option>
              ))}
            </select>
          </div>

          <div className="relative flex-1 max-w-sm">
            <Search className="w-4 h-4 absolute left-3 top-2.5 text-slate-500" />
            <input
              type="text"
              placeholder="Search IP, CIDR, comment, or server..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="w-full bg-obsidian-950 border border-obsidian-700 rounded-lg pl-9 pr-3 py-1.5 text-xs font-mono text-slate-200 focus:outline-none focus:border-emerald-500 placeholder-slate-500"
            />
          </div>
        </div>
      </div>

      {/* TAB CONTENT */}

      {/* 1. WHITELIST TAB */}
      {activeTab === 'whitelist' && (
        <div className="bg-obsidian-900 border border-obsidian-700/80 rounded-xl overflow-hidden shadow-xl">
          <div className="p-4 bg-obsidian-850/60 border-b border-obsidian-800 flex items-center justify-between">
            <div className="flex items-center space-x-2">
              <Globe className="w-4 h-4 text-sky-400" />
              <h2 className="font-mono text-sm font-bold text-slate-200">
                Always-Allowed IP / Subnet Whitelist
              </h2>
            </div>
            <button
              onClick={() => setShowAddAllowModal(true)}
              className="px-3 py-1.5 rounded-lg bg-sky-600 hover:bg-sky-500 text-white font-mono text-xs font-bold flex items-center space-x-1.5 transition-colors"
            >
              <Plus className="w-3.5 h-3.5" />
              <span>Add Allowed IP</span>
            </button>
          </div>

          {whitelistPagination.paginatedItems.length === 0 ? (
            <div className="p-8 text-center font-mono text-xs text-slate-400">
              No allowed rules match the current filters.
            </div>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-left text-xs font-mono">
                <thead className="bg-obsidian-800/60 text-slate-400 uppercase tracking-wider text-[11px] border-b border-obsidian-700">
                  <tr>
                    <th className="px-4 py-3">IP / CIDR Subnet</th>
                    <th className="px-4 py-3">Scope</th>
                    <th className="px-4 py-3">Comment / Description</th>
                    <th className="px-4 py-3">Added Date</th>
                    <th className="px-4 py-3 text-right">Actions</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-obsidian-800/80">
                  {whitelistPagination.paginatedItems.map((rule) => {
                    const isGlobal = !rule.server_id;
                    const serverObj = rule.server_id ? serverMap.get(rule.server_id) : null;
                    return (
                      <tr key={rule.id} className="hover:bg-obsidian-800/30 transition-colors">
                        <td className="px-4 py-3">
                          <div className="flex items-center space-x-2">
                            <span className="font-bold text-sky-400 text-sm">
                              {rule.ip_or_subnet}
                            </span>
                            <button
                              onClick={() => copyToClipboard(rule.ip_or_subnet, `allow-${rule.id}`)}
                              className="text-slate-500 hover:text-slate-300"
                              title="Copy IP"
                            >
                              {copiedText === `allow-${rule.id}` ? (
                                <Check className="w-3.5 h-3.5 text-emerald-400" />
                              ) : (
                                <Copy className="w-3.5 h-3.5" />
                              )}
                            </button>
                          </div>
                        </td>
                        <td className="px-4 py-3">
                          {isGlobal ? (
                            <span className="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-[10px] font-bold bg-sky-500/10 text-sky-400 border border-sky-500/30">
                              <Globe className="w-3 h-3" />
                              <span>Global (All Instances)</span>
                            </span>
                          ) : (
                            <Link
                              to={`/servers/${rule.server_id}`}
                              className="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-[10px] font-bold bg-emerald-500/10 text-emerald-400 border border-emerald-500/30 hover:bg-emerald-500/20 transition-colors"
                            >
                              <ServerIcon className="w-3 h-3" />
                              <span>{serverObj ? serverObj.name : rule.server_id}</span>
                            </Link>
                          )}
                        </td>
                        <td className="px-4 py-3 text-slate-300">
                          {rule.comment || <span className="text-slate-500 italic">None</span>}
                        </td>
                        <td className="px-4 py-3 text-slate-400">
                          {new Date(rule.created_at).toLocaleString()}
                        </td>
                        <td className="px-4 py-3 text-right">
                          <button
                            onClick={() => handleDeleteAllowRule(rule.id, rule.server_id)}
                            disabled={actionLoading}
                            className="px-2.5 py-1 rounded bg-rose-950/60 hover:bg-rose-900 border border-rose-800 text-rose-300 text-xs font-bold transition-colors"
                          >
                            Delete
                          </button>
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
              <Pagination
                currentPage={whitelistPagination.currentPage}
                totalItems={whitelistPagination.totalItems}
                pageSize={whitelistPagination.pageSize}
                onPageChange={whitelistPagination.setCurrentPage}
                onPageSizeChange={whitelistPagination.setPageSize}
                pageSizeOptions={[10, 25, 50]}
              />
            </div>
          )}
        </div>
      )}

      {/* 2. BANS TAB */}
      {activeTab === 'bans' && (
        <div className="bg-obsidian-900 border border-obsidian-700/80 rounded-xl overflow-hidden shadow-xl">
          <div className="p-4 bg-obsidian-850/60 border-b border-obsidian-800 flex items-center justify-between">
            <div className="flex items-center space-x-2">
              <Ban className="w-4 h-4 text-rose-400" />
              <h2 className="font-mono text-sm font-bold text-slate-200">
                Banned IP / Subnet Blocklist
              </h2>
            </div>
            <button
              onClick={() => setShowAddBanModal(true)}
              className="px-3 py-1.5 rounded-lg bg-rose-600 hover:bg-rose-500 text-white font-mono text-xs font-bold flex items-center space-x-1.5 transition-colors shadow-[0_0_15px_rgba(244,63,94,0.2)]"
            >
              <Plus className="w-3.5 h-3.5" />
              <span>Ban New IP / Subnet</span>
            </button>
          </div>

          {banPagination.paginatedItems.length === 0 ? (
            <div className="p-8 text-center font-mono text-xs text-slate-400">
              No IP addresses are currently banned under this filter.
            </div>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-left text-xs font-mono">
                <thead className="bg-obsidian-800/60 text-slate-400 uppercase tracking-wider text-[11px] border-b border-obsidian-700">
                  <tr>
                    <th className="px-4 py-3">Banned IP / CIDR</th>
                    <th className="px-4 py-3">Scope</th>
                    <th className="px-4 py-3">Reason</th>
                    <th className="px-4 py-3">Banned By</th>
                    <th className="px-4 py-3">Banned At</th>
                    <th className="px-4 py-3 text-right">Actions</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-obsidian-800/80">
                  {banPagination.paginatedItems.map((ban) => {
                    const isGlobal = !ban.server_id;
                    const serverObj = ban.server_id ? serverMap.get(ban.server_id) : null;
                    return (
                      <tr key={ban.id} className="hover:bg-obsidian-800/30 transition-colors">
                        <td className="px-4 py-3">
                          <div className="flex items-center space-x-2">
                            <span className="font-bold text-rose-400 text-sm">
                              {ban.ip_or_subnet}
                            </span>
                            <button
                              onClick={() => copyToClipboard(ban.ip_or_subnet, `ban-${ban.id}`)}
                              className="text-slate-500 hover:text-slate-300"
                              title="Copy IP"
                            >
                              {copiedText === `ban-${ban.id}` ? (
                                <Check className="w-3.5 h-3.5 text-emerald-400" />
                              ) : (
                                <Copy className="w-3.5 h-3.5" />
                              )}
                            </button>
                          </div>
                        </td>
                        <td className="px-4 py-3">
                          {isGlobal ? (
                            <span className="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-[10px] font-bold bg-rose-500/10 text-rose-400 border border-rose-500/30">
                              <Globe className="w-3 h-3" />
                              <span>Global (All Instances)</span>
                            </span>
                          ) : (
                            <Link
                              to={`/servers/${ban.server_id}`}
                              className="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-[10px] font-bold bg-amber-500/10 text-amber-400 border border-amber-500/30 hover:bg-amber-500/20 transition-colors"
                            >
                              <ServerIcon className="w-3 h-3" />
                              <span>{serverObj ? serverObj.name : ban.server_id}</span>
                            </Link>
                          )}
                        </td>
                        <td className="px-4 py-3 text-slate-300">
                          {ban.reason || <span className="text-slate-500 italic">No reason provided</span>}
                        </td>
                        <td className="px-4 py-3 text-slate-400">
                          {ban.banned_by || 'admin'}
                        </td>
                        <td className="px-4 py-3 text-slate-400">
                          {new Date(ban.created_at).toLocaleString()}
                        </td>
                        <td className="px-4 py-3 text-right">
                          <button
                            onClick={() => handleDeleteBanRule(ban.id, ban.server_id)}
                            disabled={actionLoading}
                            className="px-2.5 py-1 rounded bg-emerald-950/60 hover:bg-emerald-900 border border-emerald-800 text-emerald-300 text-xs font-bold transition-colors"
                          >
                            Unban
                          </button>
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
              <Pagination
                currentPage={banPagination.currentPage}
                totalItems={banPagination.totalItems}
                pageSize={banPagination.pageSize}
                onPageChange={banPagination.setCurrentPage}
                onPageSizeChange={banPagination.setPageSize}
                pageSizeOptions={[10, 25, 50]}
              />
            </div>
          )}
        </div>
      )}

      {/* 3. ACTIVE LEASES TAB */}
      {activeTab === 'leases' && (
        <div className="bg-obsidian-900 border border-obsidian-700/80 rounded-xl overflow-hidden shadow-xl">
          <div className="p-4 bg-obsidian-850/60 border-b border-obsidian-800 flex items-center justify-between">
            <div className="flex items-center space-x-2">
              <Clock className="w-4 h-4 text-amber-400" />
              <h2 className="font-mono text-sm font-bold text-slate-200">
                Live Dynamic Firewall Leases
              </h2>
            </div>
            <span className="text-xs font-mono text-slate-400">
              {activeLeases.length} total active player ports
            </span>
          </div>

          {leasesPagination.paginatedItems.length === 0 ? (
            <div className="p-8 text-center font-mono text-xs text-slate-400">
              No active client leases found.
            </div>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-left text-xs font-mono">
                <thead className="bg-obsidian-800/60 text-slate-400 uppercase tracking-wider text-[11px] border-b border-obsidian-700">
                  <tr>
                    <th className="px-4 py-3">Server Instance</th>
                    <th className="px-4 py-3">Client IP Address</th>
                    <th className="px-4 py-3">Gamertag</th>
                    <th className="px-4 py-3">Knock Method</th>
                    <th className="px-4 py-3">Expires At</th>
                    <th className="px-4 py-3 text-right">Actions</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-obsidian-800/80">
                  {leasesPagination.paginatedItems.map((lease) => {
                    const serverObj = serverMap.get(lease.server_id);
                    return (
                      <tr key={lease.id} className="hover:bg-obsidian-800/30 transition-colors">
                        <td className="px-4 py-3 font-bold text-slate-200">
                          <Link
                            to={`/servers/${lease.server_id}`}
                            className="hover:text-emerald-400 flex items-center gap-1.5"
                          >
                            <ServerIcon className="w-3.5 h-3.5 text-emerald-400" />
                            <span>{serverObj ? serverObj.name : lease.server_id}</span>
                          </Link>
                        </td>
                        <td className="px-4 py-3">
                          <div className="flex items-center space-x-2">
                            <span className="font-bold text-emerald-400">
                              {lease.ip_address}
                            </span>
                            <button
                              onClick={() => copyToClipboard(lease.ip_address, `lease-${lease.id}`)}
                              className="text-slate-500 hover:text-slate-300"
                              title="Copy IP"
                            >
                              {copiedText === `lease-${lease.id}` ? (
                                <Check className="w-3.5 h-3.5 text-emerald-400" />
                              ) : (
                                <Copy className="w-3.5 h-3.5" />
                              )}
                            </button>
                          </div>
                        </td>
                        <td className="px-4 py-3 text-slate-300">
                          {lease.gamertag ? (
                            <span className="px-2 py-0.5 rounded bg-obsidian-800 border border-obsidian-700 text-slate-200">
                              {lease.gamertag}
                            </span>
                          ) : (
                            <span className="text-slate-500 italic">None</span>
                          )}
                        </td>
                        <td className="px-4 py-3 text-slate-400 capitalize">
                          {lease.knock_method}
                        </td>
                        <td className="px-4 py-3 text-amber-400 font-bold">
                          {new Date(lease.expires_at).toLocaleString()}
                        </td>
                        <td className="px-4 py-3 text-right">
                          <button
                            onClick={() => handleRevokeLease(lease.id)}
                            disabled={actionLoading}
                            className="px-2.5 py-1 rounded bg-rose-950/60 hover:bg-rose-900 border border-rose-800 text-rose-300 text-xs font-bold transition-colors"
                          >
                            Revoke Lease
                          </button>
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
              <Pagination
                currentPage={leasesPagination.currentPage}
                totalItems={leasesPagination.totalItems}
                pageSize={leasesPagination.pageSize}
                onPageChange={leasesPagination.setCurrentPage}
                onPageSizeChange={leasesPagination.setPageSize}
                pageSizeOptions={[10, 25, 50]}
              />
            </div>
          )}
        </div>
      )}

      {/* 4. INSTANCES GATE STATUS TAB */}
      {activeTab === 'instances' && (
        <div className="bg-obsidian-900 border border-obsidian-700/80 rounded-xl overflow-hidden shadow-xl">
          <div className="p-4 bg-obsidian-850/60 border-b border-obsidian-800 flex items-center justify-between">
            <div className="flex items-center space-x-2">
              <ServerIcon className="w-4 h-4 text-emerald-400" />
              <h2 className="font-mono text-sm font-bold text-slate-200">
                Instances Port Gate Overview
              </h2>
            </div>
          </div>

          <div className="overflow-x-auto">
            <table className="w-full text-left text-xs font-mono">
              <thead className="bg-obsidian-800/60 text-slate-400 uppercase tracking-wider text-[11px] border-b border-obsidian-700">
                <tr>
                  <th className="px-4 py-3">Server Instance</th>
                  <th className="px-4 py-3">UDP Port</th>
                  <th className="px-4 py-3">Game Address</th>
                  <th className="px-4 py-3">Port Gate Status</th>
                  <th className="px-4 py-3">Auth Mode</th>
                  <th className="px-4 py-3">Portal Link</th>
                  <th className="px-4 py-3 text-right">Hub Link</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-obsidian-800/80">
                {serversPagination.paginatedItems.map((srv) => {
                  const portalUrl = `${window.location.origin}/knock/${srv.id}`;
                  return (
                    <tr key={srv.id} className="hover:bg-obsidian-800/30 transition-colors">
                      <td className="px-4 py-3 font-bold text-slate-200">
                        <Link
                          to={`/servers/${srv.id}`}
                          className="hover:text-emerald-400 flex items-center gap-2"
                        >
                          <ServerIcon className="w-3.5 h-3.5 text-emerald-400" />
                          <span>{srv.name}</span>
                        </Link>
                      </td>
                      <td className="px-4 py-3 font-bold text-slate-300">
                        UDP {srv.port} {srv.portv6 > 0 ? `/ ${srv.portv6}` : ''}
                      </td>
                      <td className="px-4 py-3">
                        {srv.game_server_address ? (
                          <span className="text-emerald-400 font-mono font-semibold">{srv.game_server_address}</span>
                        ) : (
                          <span className="text-slate-500 font-mono text-[11px] italic">Host ({window.location.hostname})</span>
                        )}
                      </td>
                      <td className="px-4 py-3">
                        {srv.port_gate_enabled ? (
                          <span className="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-[10px] font-bold bg-emerald-500/10 text-emerald-400 border border-emerald-500/30">
                            <span className="w-1.5 h-1.5 rounded-full bg-emerald-400 animate-pulse"></span>
                            <span>Enabled (Default Deny)</span>
                          </span>
                        ) : (
                          <span className="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-[10px] font-bold bg-slate-500/10 text-slate-400 border border-slate-500/30">
                            <span>Disabled (Open Port)</span>
                          </span>
                        )}
                      </td>
                      <td className="px-4 py-3 capitalize text-slate-300">
                        {srv.port_gate_mode || 'passphrase'}
                      </td>
                      <td className="px-4 py-3">
                        <div className="flex items-center space-x-2">
                          <button
                            onClick={() => copyToClipboard(portalUrl, `portal-${srv.id}`)}
                            className="px-2 py-1 rounded bg-obsidian-800 hover:bg-obsidian-750 border border-obsidian-700 text-slate-300 text-[10px] font-bold flex items-center gap-1"
                            title="Copy Portal Link"
                          >
                            {copiedText === `portal-${srv.id}` ? (
                              <Check className="w-3 h-3 text-emerald-400" />
                            ) : (
                              <Copy className="w-3 h-3" />
                            )}
                            <span>Copy Link</span>
                          </button>
                          <a
                            href={`/knock/${srv.id}`}
                            target="_blank"
                            rel="noopener noreferrer"
                            className="p-1 rounded bg-obsidian-800 hover:bg-obsidian-750 text-slate-400 hover:text-emerald-400"
                            title="Open Knock Portal"
                          >
                            <ExternalLink className="w-3.5 h-3.5" />
                          </a>
                        </div>
                      </td>
                      <td className="px-4 py-3 text-right">
                        <Link
                          to={`/servers/${srv.id}`}
                          className="inline-flex items-center gap-1 px-2.5 py-1 rounded bg-obsidian-800 hover:bg-obsidian-750 border border-obsidian-700 text-slate-200 text-xs font-bold transition-colors"
                        >
                          <span>Manage</span>
                          <ArrowUpRight className="w-3.5 h-3.5" />
                        </Link>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
            <Pagination
              currentPage={serversPagination.currentPage}
              totalItems={serversPagination.totalItems}
              pageSize={serversPagination.pageSize}
              onPageChange={serversPagination.setCurrentPage}
              onPageSizeChange={serversPagination.setPageSize}
              pageSizeOptions={[10, 25, 50]}
            />
          </div>
        </div>
      )}

      {/* MODAL: ADD ALLOWLIST RULE */}
      {showAddAllowModal && (
        <div className="fixed inset-0 bg-black/80 backdrop-blur-sm z-50 flex items-center justify-center p-4">
          <div className="bg-obsidian-900 border border-obsidian-700 rounded-xl max-w-md w-full p-6 shadow-2xl">
            <h3 className="text-base font-mono font-bold text-slate-100 mb-4 flex items-center gap-2">
              <Globe className="w-4 h-4 text-sky-400" />
              <span>Add Permanent Allowed IP / Subnet</span>
            </h3>
            <form onSubmit={handleCreateAllowRule} className="space-y-4 font-mono text-xs">
              <div>
                <div className="flex items-center justify-between mb-1">
                  <label className="text-slate-300 font-bold">IP Address or CIDR Subnet</label>
                  {detectedClientIP && (
                    <button
                      type="button"
                      onClick={() => setNewAllowIP(detectedClientIP)}
                      className="text-[11px] text-sky-400 hover:text-sky-300 underline"
                    >
                      Use My IP ({detectedClientIP})
                    </button>
                  )}
                </div>
                <input
                  type="text"
                  required
                  placeholder="e.g. 192.168.1.100 or 10.0.0.0/16"
                  value={newAllowIP}
                  onChange={(e) => setNewAllowIP(e.target.value)}
                  className="w-full px-3 py-2 bg-obsidian-950 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-sky-500"
                />
                <p className="text-[11px] text-slate-500 mt-1">
                  Supports single IPv4/IPv6 or CIDR ranges (e.g. <code>192.168.1.0/24</code>).
                </p>
              </div>

              <div>
                <label className="block text-slate-300 mb-1 font-bold">Target Scope</label>
                <select
                  value={newAllowScope}
                  onChange={(e) => setNewAllowScope(e.target.value)}
                  className="w-full px-3 py-2 bg-obsidian-950 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-sky-500"
                >
                  <option value="global">Global (All Instances)</option>
                  {servers.map((s) => (
                    <option key={s.id} value={s.id}>
                      Single Instance: {s.name} (UDP {s.port})
                    </option>
                  ))}
                </select>
                <p className="text-[11px] text-slate-500 mt-1">
                  Global rules grant permanent port access to all server instances automatically.
                </p>
              </div>

              <div>
                <label className="block text-slate-300 mb-1 font-bold">Comment / Note (optional)</label>
                <input
                  type="text"
                  placeholder="e.g. Home Office, LAN Network, Admin VPN"
                  value={newAllowComment}
                  onChange={(e) => setNewAllowComment(e.target.value)}
                  className="w-full px-3 py-2 bg-obsidian-950 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-sky-500"
                />
              </div>

              <div className="flex justify-end space-x-3 pt-3 border-t border-obsidian-800">
                <button
                  type="button"
                  onClick={() => setShowAddAllowModal(false)}
                  className="px-4 py-2 rounded-lg bg-obsidian-800 text-slate-300 hover:bg-obsidian-750 font-bold"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={actionLoading}
                  className="px-4 py-2 rounded-lg bg-sky-600 hover:bg-sky-500 text-white font-bold flex items-center gap-1.5"
                >
                  {actionLoading ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : null}
                  <span>Save Allowed Rule</span>
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* MODAL: BAN IP RULE */}
      {showAddBanModal && (
        <div className="fixed inset-0 bg-black/80 backdrop-blur-sm z-50 flex items-center justify-center p-4">
          <div className="bg-obsidian-900 border border-obsidian-700 rounded-xl max-w-md w-full p-6 shadow-2xl">
            <h3 className="text-base font-mono font-bold text-slate-100 mb-4 flex items-center gap-2">
              <Ban className="w-4 h-4 text-rose-500" />
              <span>Ban IP Address / CIDR Subnet</span>
            </h3>
            <form onSubmit={handleCreateBanRule} className="space-y-4 font-mono text-xs">
              <div>
                <label className="block text-slate-300 mb-1 font-bold">IP Address or CIDR Range to Ban</label>
                <input
                  type="text"
                  required
                  placeholder="e.g. 198.51.100.42 or 203.0.113.0/24"
                  value={newBanIP}
                  onChange={(e) => setNewBanIP(e.target.value)}
                  className="w-full px-3 py-2 bg-obsidian-950 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-rose-500"
                />
              </div>

              <div>
                <label className="block text-slate-300 mb-1 font-bold">Target Scope</label>
                <select
                  value={newBanScope}
                  onChange={(e) => setNewBanScope(e.target.value)}
                  className="w-full px-3 py-2 bg-obsidian-950 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-rose-500"
                >
                  <option value="global">Global (Block on All Instances)</option>
                  {servers.map((s) => (
                    <option key={s.id} value={s.id}>
                      Single Instance: {s.name} (UDP {s.port})
                    </option>
                  ))}
                </select>
              </div>

              <div>
                <label className="block text-slate-300 mb-1 font-bold">Reason for Ban</label>
                <input
                  type="text"
                  placeholder="e.g. Repeated malicious scanning, unauthorized attempts, griefing"
                  value={newBanReason}
                  onChange={(e) => setNewBanReason(e.target.value)}
                  className="w-full px-3 py-2 bg-obsidian-950 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-rose-500"
                />
              </div>

              <div className="p-3 bg-rose-950/30 border border-rose-500/30 rounded-lg flex items-start space-x-2 text-rose-300 text-[11px]">
                <AlertTriangle className="w-4 h-4 flex-shrink-0 text-rose-400 mt-0.5" />
                <span>
                  Banning will immediately revoke all active leases matching this IP/subnet and reject any future knock attempts.
                </span>
              </div>

              <div className="flex justify-end space-x-3 pt-3 border-t border-obsidian-800">
                <button
                  type="button"
                  onClick={() => setShowAddBanModal(false)}
                  className="px-4 py-2 rounded-lg bg-obsidian-800 text-slate-300 hover:bg-obsidian-750 font-bold"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={actionLoading}
                  className="px-4 py-2 rounded-lg bg-rose-600 hover:bg-rose-500 text-white font-bold flex items-center gap-1.5"
                >
                  {actionLoading ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : null}
                  <span>Confirm Ban</span>
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
};
