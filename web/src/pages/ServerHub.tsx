import React, { useEffect, useState, useRef } from 'react';
import { useParams, Link } from 'react-router-dom';
import {
  ArrowLeft, Play, Square, RefreshCw, Shield, Terminal, Settings, ExternalLink,
  HardDrive, Cpu, AlertTriangle, Loader2, Send, Users, MessageSquare, Copy,
  Download, UserPlus, ShieldAlert, Check, Lock, Unlock, Package, Archive,
  Upload, Trash2
} from 'lucide-react';
import { api } from '../api/client';
import { Server, User, Backup, AddonPack, PortGateLease } from '../types';

interface ServerHubProps {
  user: User;
}

export const ServerHub: React.FC<ServerHubProps> = () => {
  const { id } = useParams<{ id: string }>();
  const [server, setServer] = useState<Server | null>(null);
  const [loading, setLoading] = useState(true);
  const [activeTab, setActiveTab] = useState<'overview' | 'players' | 'portgate' | 'backups' | 'addons' | 'settings' | 'actions'>('overview');
  const [error, setError] = useState<string | null>(null);
  const [actionLoading, setActionLoading] = useState(false);

  // Port Gate Leases state
  const [leases, setLeases] = useState<PortGateLease[]>([]);
  const [showManualLeaseModal, setShowManualLeaseModal] = useState(false);
  const [manualIP, setManualIP] = useState('');
  const [manualGamertag, setManualGamertag] = useState('');
  const [manualDuration, setManualDuration] = useState(60);
  const [manualComment, setManualComment] = useState('');

  // Backups state
  const [backupsList, setBackupsList] = useState<Backup[]>([]);
  const [backupLoading, setBackupLoading] = useState(false);

  // Addons state
  const [addonsList, setAddonsList] = useState<AddonPack[]>([]);
  const [addonLoading, setAddonLoading] = useState(false);

  // Console WebSocket state
  const [logs, setLogs] = useState<string[]>([]);
  const [command, setCommand] = useState('');
  const [wsConnected, setWsConnected] = useState(false);
  const wsRef = useRef<WebSocket | null>(null);
  const consoleBottomRef = useRef<HTMLDivElement | null>(null);

  // Live Stats state
  const [stats, setStats] = useState<{ cpu_percent: number; ram_bytes: number; player_count: number } | null>(null);

  // Player Hub & Chat state
  const [onlinePlayers, setOnlinePlayers] = useState<Array<{ gamertag: string; xuid: string; joined_at: string }>>([]);
  const [chatFeed, setChatFeed] = useState<Array<{ gamertag: string; message: string; timestamp: string }>>([]);
  const [broadcastMsg, setBroadcastMsg] = useState('');
  const [broadcastTarget, setBroadcastTarget] = useState('');

  // Configuration state
  const [properties, setProperties] = useState<Record<string, string>>({});
  const [propKeys, setPropKeys] = useState<string[]>([]);
  const [allowlist, setAllowlist] = useState<Array<{ name: string; xuid?: string; ignoresPlayerLimit: boolean }>>([]);
  const [newAllowlistPlayer, setNewAllowlistPlayer] = useState('');
  const [configSaving, setConfigSaving] = useState(false);
  const [configSaved, setConfigSaved] = useState(false);

  // Clone & Export state
  const [cloneId, setCloneId] = useState('');
  const [cloneName, setCloneName] = useState('');
  const [cloning, setCloning] = useState(false);

  // Version update info
  const [updateInfo, setUpdateInfo] = useState<{ current_version: string; latest_version: string; update_available: boolean; release_url: string } | null>(null);

  const fetchServer = async () => {
    if (!id) return;
    try {
      const data = await api.getServer(id);
      setServer(data);
    } catch (err: any) {
      setError(err.message || 'Server not found');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchServer();
  }, [id]);

  // Connect WebSocket for live logs
  useEffect(() => {
    if (!id || activeTab !== 'overview') return;

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const token = localStorage.getItem('bsm_token') || '';
    const wsUrl = `${protocol}//${window.location.host}/api/servers/${id}/console/ws?token=${encodeURIComponent(token)}`;

    const ws = new WebSocket(wsUrl);
    wsRef.current = ws;

    ws.onopen = () => setWsConnected(true);
    ws.onmessage = (evt) => {
      try {
        const msg = JSON.parse(evt.data);
        if (msg.type === 'history' || msg.type === 'log') {
          setLogs((prev) => [...prev, msg.payload]);
        }
      } catch {
        setLogs((prev) => [...prev, evt.data]);
      }
    };
    ws.onclose = () => setWsConnected(false);

    return () => {
      ws.close();
      wsRef.current = null;
    };
  }, [id, activeTab]);

  // Auto-scroll console
  useEffect(() => {
    consoleBottomRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [logs]);

  // Live Stats Poller
  useEffect(() => {
    if (!id || server?.status !== 'running') return;
    const interval = setInterval(async () => {
      try {
        const res = await api.getStats(id);
        setStats(res);
      } catch {}
    }, 2000);
    return () => clearInterval(interval);
  }, [id, server?.status]);

  // Fetch tab data when active tab switches
  useEffect(() => {
    if (!id) return;
    if (activeTab === 'players') {
      api.getPlayers(id).then((res) => setOnlinePlayers(res.online_players)).catch(() => {});
      api.getChat(id).then((res) => setChatFeed(res)).catch(() => {});
    } else if (activeTab === 'portgate') {
      api.listLeases(id).then(setLeases).catch(() => {});
    } else if (activeTab === 'backups') {
      api.listBackups(id).then(setBackupsList).catch(() => {});
    } else if (activeTab === 'addons') {
      api.listAddons(id).then(setAddonsList).catch(() => {});
    } else if (activeTab === 'settings') {
      api.getProperties(id).then((res) => {
        setProperties(res.properties);
        setPropKeys(res.keys);
      }).catch(() => {});
      api.getAllowlist(id).then((res) => setAllowlist(res)).catch(() => {});
    } else if (activeTab === 'actions') {
      api.checkUpdates(server?.version || 'latest').then((res) => setUpdateInfo(res)).catch(() => {});
    }
  }, [id, activeTab, server?.version]);

  const handleStart = async () => {
    if (!id) return;
    setActionLoading(true);
    try {
      await api.startServer(id);
      await fetchServer();
    } catch (err: any) {
      alert(err.message || 'Failed to start server');
    } finally {
      setActionLoading(false);
    }
  };

  const handleStop = async () => {
    if (!id) return;
    setActionLoading(true);
    try {
      await api.stopServer(id);
      await fetchServer();
    } catch (err: any) {
      alert(err.message || 'Failed to stop server');
    } finally {
      setActionLoading(false);
    }
  };

  const handleRestart = async () => {
    if (!id) return;
    setActionLoading(true);
    try {
      await api.restartServer(id);
      await fetchServer();
    } catch (err: any) {
      alert(err.message || 'Failed to restart server');
    } finally {
      setActionLoading(false);
    }
  };

  const handleSendCommand = (e: React.FormEvent) => {
    e.preventDefault();
    if (!command.trim()) return;

    if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({ type: 'command', payload: command.trim() }));
    } else if (id) {
      api.sendCommand(id, command.trim()).catch(() => {});
    }
    setCommand('');
  };

  const handleBroadcast = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!id || !broadcastMsg.trim()) return;
    try {
      await api.broadcast(id, broadcastMsg.trim(), broadcastTarget.trim() || undefined);
      setBroadcastMsg('');
      setBroadcastTarget('');
      alert('Message broadcasted to server!');
    } catch (err: any) {
      alert(err.message || 'Failed to broadcast');
    }
  };

  const handleKick = async (gt: string) => {
    if (!id) return;
    if (!confirm(`Are you sure you want to kick ${gt}?`)) return;
    try {
      await api.kickPlayer(id, gt);
      const res = await api.getPlayers(id);
      setOnlinePlayers(res.online_players);
    } catch (err: any) {
      alert(err.message || 'Failed to kick player');
    }
  };

  const handleOp = async (gt: string) => {
    if (!id) return;
    try {
      await api.opPlayer(id, gt);
      alert(`Granted operator status to ${gt}`);
    } catch (err: any) {
      alert(err.message || 'Failed to op player');
    }
  };

  const handleSaveProperties = async () => {
    if (!id) return;
    setConfigSaving(true);
    try {
      await api.updateProperties(id, properties, propKeys);
      setConfigSaved(true);
      setTimeout(() => setConfigSaved(false), 2500);
    } catch (err: any) {
      alert(err.message || 'Failed to save properties');
    } finally {
      setConfigSaving(false);
    }
  };

  const handleAddAllowlistPlayer = async () => {
    if (!id || !newAllowlistPlayer.trim()) return;
    const updated = [...allowlist, { name: newAllowlistPlayer.trim(), ignoresPlayerLimit: false }];
    try {
      await api.updateAllowlist(id, updated);
      setAllowlist(updated);
      setNewAllowlistPlayer('');
    } catch (err: any) {
      alert(err.message || 'Failed to update allowlist');
    }
  };

  const handleRemoveAllowlistPlayer = async (name: string) => {
    if (!id) return;
    const updated = allowlist.filter((p) => p.name !== name);
    try {
      await api.updateAllowlist(id, updated);
      setAllowlist(updated);
    } catch (err: any) {
      alert(err.message || 'Failed to update allowlist');
    }
  };

  const handleClone = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!id || !cloneId || !cloneName) return;
    setCloning(true);
    try {
      await api.cloneServer(id, cloneId, cloneName);
      alert(`Successfully cloned server to ${cloneName} (${cloneId})!`);
      setCloneId('');
      setCloneName('');
    } catch (err: any) {
      alert(err.message || 'Failed to clone server');
    } finally {
      setCloning(false);
    }
  };

  if (loading) {
    return (
      <div className="flex flex-col items-center justify-center py-24 text-slate-400">
        <Loader2 className="w-8 h-8 animate-spin text-emerald-400 mb-3" />
        <span className="font-mono text-sm">Loading server configuration...</span>
      </div>
    );
  }

  if (error || !server) {
    return (
      <div className="max-w-4xl mx-auto px-4 py-12">
        <div className="p-4 rounded-xl bg-rose-950/40 border border-rose-500/40 text-rose-300 flex items-center space-x-3">
          <AlertTriangle className="w-5 h-5 text-rose-400 flex-shrink-0" />
          <span>{error || 'Server not found'}</span>
        </div>
        <Link to="/" className="mt-4 inline-flex items-center space-x-2 text-emerald-400 hover:underline font-mono text-sm">
          <ArrowLeft className="w-4 h-4" />
          <span>Back to Dashboard</span>
        </Link>
      </div>
    );
  }

  // Leases handlers
  const handleRevokeLease = async (leaseId: number) => {
    if (!id) return;
    try {
      await api.revokeLease(id, leaseId);
      const updated = await api.listLeases(id);
      setLeases(updated);
    } catch (err: any) {
      alert(err.message || 'Failed to revoke lease');
    }
  };

  const handleCreateManualLease = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!id || !manualIP) return;
    try {
      await api.createManualLease(id, {
        ip_address: manualIP,
        gamertag: manualGamertag,
        duration_minutes: manualDuration,
        comment: manualComment,
      });
      setShowManualLeaseModal(false);
      setManualIP('');
      setManualGamertag('');
      setManualComment('');
      const updated = await api.listLeases(id);
      setLeases(updated);
    } catch (err: any) {
      alert(err.message || 'Failed to grant lease');
    }
  };

  // Backups handlers
  const handleCreateBackup = async () => {
    if (!id) return;
    setBackupLoading(true);
    try {
      await api.createBackup(id);
      const updated = await api.listBackups(id);
      setBackupsList(updated);
    } catch (err: any) {
      alert(err.message || 'Failed to create backup');
    } finally {
      setBackupLoading(false);
    }
  };

  const handleToggleBackupLock = async (backupId: number) => {
    if (!id) return;
    try {
      await api.toggleBackupLock(id, backupId);
      const updated = await api.listBackups(id);
      setBackupsList(updated);
    } catch (err: any) {
      alert(err.message || 'Failed to toggle lock');
    }
  };

  const handleDeleteBackup = async (backupId: number) => {
    if (!id) return;
    if (!confirm('Are you sure you want to delete this backup?')) return;
    try {
      await api.deleteBackup(id, backupId);
      const updated = await api.listBackups(id);
      setBackupsList(updated);
    } catch (err: any) {
      alert(err.message || 'Failed to delete backup');
    }
  };

  const handleRestoreBackup = async (backupId: number) => {
    if (!id) return;
    if (server?.status === 'running') {
      alert('Server must be stopped before restoring a backup.');
      return;
    }
    if (!confirm('Restoring will replace the current world state. Continue?')) return;
    try {
      await api.restoreBackup(id, backupId);
      alert('Backup restored successfully!');
    } catch (err: any) {
      alert(err.message || 'Failed to restore backup');
    }
  };

  const handleImportWorld = async (e: React.ChangeEvent<HTMLInputElement>) => {
    if (!id || !e.target.files || e.target.files.length === 0) return;
    if (server?.status === 'running') {
      alert('Server must be stopped before importing a new world.');
      return;
    }
    const file = e.target.files[0];
    try {
      await api.importWorld(id, file);
      alert('World imported successfully!');
    } catch (err: any) {
      alert(err.message || 'Failed to import world');
    }
  };

  // Addon handlers
  const handleInstallAddon = async (e: React.ChangeEvent<HTMLInputElement>) => {
    if (!id || !e.target.files || e.target.files.length === 0) return;
    const file = e.target.files[0];
    setAddonLoading(true);
    try {
      await api.installAddon(id, file);
      const updated = await api.listAddons(id);
      setAddonsList(updated);
      alert('Addon installed successfully!');
    } catch (err: any) {
      alert(err.message || 'Failed to install addon');
    } finally {
      setAddonLoading(false);
    }
  };

  const handleDeleteAddon = async (packType: string, folder: string) => {
    if (!id) return;
    if (!confirm(`Delete addon "${folder}"?`)) return;
    try {
      await api.deleteAddon(id, packType, folder);
      const updated = await api.listAddons(id);
      setAddonsList(updated);
    } catch (err: any) {
      alert(err.message || 'Failed to delete addon');
    }
  };

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
      {/* Header */}
      <div className="flex flex-col md:flex-row items-start md:items-center justify-between gap-4 mb-6">
        <div className="flex items-center space-x-4">
          <Link
            to="/"
            className="p-2 rounded-lg bg-obsidian-900 border border-obsidian-700 text-slate-400 hover:text-emerald-400 transition-colors"
          >
            <ArrowLeft className="w-4 h-4" />
          </Link>
          <div>
            <div className="flex items-center space-x-3">
              <h1 className="text-2xl font-bold font-mono text-slate-100">{server.name}</h1>
              <span
                className={`text-[11px] font-mono px-2 py-0.5 rounded-full border uppercase ${
                  server.status === 'running'
                    ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/30'
                    : server.status === 'crashed'
                    ? 'bg-rose-500/10 text-rose-400 border-rose-500/30'
                    : 'bg-slate-800 text-slate-400 border-slate-700'
                }`}
              >
                {server.status}
              </span>
            </div>
            <div className="text-xs text-slate-400 font-mono mt-0.5">
              ID: {server.id} | UDP: {server.port} | v{server.version}
            </div>
          </div>
        </div>

        <div className="flex items-center space-x-3">
          {server.port_gate_enabled && (
            <Link
              to={`/knock/${server.id}`}
              target="_blank"
              className="px-3 py-2 rounded-lg bg-emerald-950 border border-emerald-500/40 text-emerald-400 hover:bg-emerald-900/50 font-mono text-xs flex items-center space-x-1.5 transition-all shadow-[0_0_10px_rgba(16,185,129,0.15)]"
            >
              <Shield className="w-3.5 h-3.5" />
              <span>Knock Portal</span>
              <ExternalLink className="w-3 h-3 ml-0.5" />
            </Link>
          )}

          {server.status === 'running' ? (
            <>
              <button
                onClick={handleRestart}
                disabled={actionLoading}
                className="px-3 py-2 rounded-lg bg-obsidian-850 hover:bg-obsidian-800 border border-obsidian-700 text-slate-200 font-mono text-xs flex items-center space-x-1.5"
              >
                <RefreshCw className={`w-3.5 h-3.5 text-cyber-cyan ${actionLoading ? 'animate-spin' : ''}`} />
                <span>Restart</span>
              </button>
              <button
                onClick={handleStop}
                disabled={actionLoading}
                className="px-3 py-2 rounded-lg bg-rose-950/40 hover:bg-rose-900/60 border border-rose-500/40 text-rose-300 font-mono text-xs flex items-center space-x-1.5"
              >
                <Square className="w-3.5 h-3.5 fill-current text-rose-400" />
                <span>Stop</span>
              </button>
            </>
          ) : (
            <button
              onClick={handleStart}
              disabled={actionLoading}
              className="px-4 py-2 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-slate-950 font-mono text-xs font-bold flex items-center space-x-1.5 shadow-[0_0_15px_rgba(16,185,129,0.3)]"
            >
              {actionLoading ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <Play className="w-3.5 h-3.5 fill-current" />}
              <span>Start Server</span>
            </button>
          )}
        </div>
      </div>

      {/* Tabs */}
      <div className="flex border-b border-obsidian-700/80 mb-6 font-mono text-xs overflow-x-auto">
        <button
          onClick={() => setActiveTab('overview')}
          className={`pb-3 px-4 border-b-2 font-medium flex items-center space-x-2 whitespace-nowrap transition-colors ${
            activeTab === 'overview'
              ? 'border-emerald-500 text-emerald-400'
              : 'border-transparent text-slate-400 hover:text-slate-200'
          }`}
        >
          <Terminal className="w-3.5 h-3.5" />
          <span>Console & Overview</span>
        </button>

        <button
          onClick={() => setActiveTab('players')}
          className={`pb-3 px-4 border-b-2 font-medium flex items-center space-x-2 whitespace-nowrap transition-colors ${
            activeTab === 'players'
              ? 'border-emerald-500 text-emerald-400'
              : 'border-transparent text-slate-400 hover:text-slate-200'
          }`}
        >
          <Users className="w-3.5 h-3.5" />
          <span>Players & Chat</span>
        </button>

        <button
          onClick={() => setActiveTab('portgate')}
          className={`pb-3 px-4 border-b-2 font-medium flex items-center space-x-2 whitespace-nowrap transition-colors ${
            activeTab === 'portgate'
              ? 'border-emerald-500 text-emerald-400'
              : 'border-transparent text-slate-400 hover:text-slate-200'
          }`}
        >
          <Shield className="w-3.5 h-3.5" />
          <span>Port Gate & Leases</span>
        </button>

        <button
          onClick={() => setActiveTab('backups')}
          className={`pb-3 px-4 border-b-2 font-medium flex items-center space-x-2 whitespace-nowrap transition-colors ${
            activeTab === 'backups'
              ? 'border-emerald-500 text-emerald-400'
              : 'border-transparent text-slate-400 hover:text-slate-200'
          }`}
        >
          <Archive className="w-3.5 h-3.5" />
          <span>Backups & Worlds</span>
        </button>

        <button
          onClick={() => setActiveTab('addons')}
          className={`pb-3 px-4 border-b-2 font-medium flex items-center space-x-2 whitespace-nowrap transition-colors ${
            activeTab === 'addons'
              ? 'border-emerald-500 text-emerald-400'
              : 'border-transparent text-slate-400 hover:text-slate-200'
          }`}
        >
          <Package className="w-3.5 h-3.5" />
          <span>Addons & Packs</span>
        </button>

        <button
          onClick={() => setActiveTab('settings')}
          className={`pb-3 px-4 border-b-2 font-medium flex items-center space-x-2 whitespace-nowrap transition-colors ${
            activeTab === 'settings'
              ? 'border-emerald-500 text-emerald-400'
              : 'border-transparent text-slate-400 hover:text-slate-200'
          }`}
        >
          <Settings className="w-3.5 h-3.5" />
          <span>Configuration Editor</span>
        </button>

        <button
          onClick={() => setActiveTab('actions')}
          className={`pb-3 px-4 border-b-2 font-medium flex items-center space-x-2 whitespace-nowrap transition-colors ${
            activeTab === 'actions'
              ? 'border-emerald-500 text-emerald-400'
              : 'border-transparent text-slate-400 hover:text-slate-200'
          }`}
        >
          <Copy className="w-3.5 h-3.5" />
          <span>Clone & Export</span>
        </button>
      </div>

      {/* OVERVIEW TAB */}
      {activeTab === 'overview' && (
        <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
          <div className="bg-obsidian-900 border border-obsidian-700/80 rounded-xl p-5 md:col-span-2 flex flex-col h-[520px]">
            <div className="flex items-center justify-between mb-3">
              <h3 className="font-mono text-sm font-bold text-slate-200 flex items-center gap-2">
                <Terminal className="w-4 h-4 text-emerald-400" />
                <span>Interactive BDS Terminal</span>
              </h3>
              <div className="flex items-center space-x-2 text-[11px] font-mono">
                <span className={`w-2 h-2 rounded-full ${wsConnected ? 'bg-emerald-400 animate-pulse' : 'bg-slate-600'}`}></span>
                <span className="text-slate-400">{wsConnected ? 'Connected' : 'Offline'}</span>
              </div>
            </div>

            <div className="flex-1 bg-obsidian-950 rounded-lg p-4 font-mono text-xs text-slate-300 overflow-y-auto border border-obsidian-800 space-y-1">
              {logs.length === 0 ? (
                <p className="text-slate-600 italic">No console logs received yet...</p>
              ) : (
                logs.map((line, idx) => (
                  <div key={idx} className="leading-relaxed hover:bg-obsidian-900/60 px-1 rounded">
                    {line}
                  </div>
                ))
              )}
              <div ref={consoleBottomRef} />
            </div>

            <form onSubmit={handleSendCommand} className="mt-3 flex items-center gap-2">
              <input
                type="text"
                value={command}
                onChange={(e) => setCommand(e.target.value)}
                placeholder="Enter BDS console command (e.g. say Hello, op Steve, time set day)..."
                className="flex-1 px-3 py-2 rounded-lg bg-obsidian-950 border border-obsidian-700 text-slate-100 font-mono text-xs focus:border-emerald-500"
              />
              <button
                type="submit"
                className="px-3.5 py-2 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-slate-950 font-bold font-mono text-xs flex items-center space-x-1"
              >
                <Send className="w-3.5 h-3.5" />
                <span>Send</span>
              </button>
            </form>
          </div>

          <div className="space-y-6">
            <div className="bg-obsidian-900 border border-obsidian-700/80 rounded-xl p-5">
              <h3 className="font-mono text-sm font-bold text-slate-200 mb-3">Hardware Limits</h3>
              <div className="space-y-3 font-mono text-xs">
                <div className="flex justify-between items-center pb-2 border-b border-obsidian-800">
                  <span className="text-slate-400 flex items-center gap-1.5">
                    <HardDrive className="w-3.5 h-3.5 text-emerald-400" />
                    <span>Memory Cap</span>
                  </span>
                  <span className="text-slate-200">{server.memory_limit}</span>
                </div>
                <div className="flex justify-between items-center">
                  <span className="text-slate-400 flex items-center gap-1.5">
                    <Cpu className="w-3.5 h-3.5 text-cyber-cyan" />
                    <span>CPU Limit</span>
                  </span>
                  <span className="text-slate-200">{server.cpu_limit} Cores</span>
                </div>
              </div>
            </div>

            {stats && (
              <div className="bg-obsidian-900 border border-obsidian-700/80 rounded-xl p-5">
                <h3 className="font-mono text-sm font-bold text-slate-200 mb-3">Live Telemetry</h3>
                <div className="space-y-3 font-mono text-xs">
                  <div className="flex justify-between items-center pb-2 border-b border-obsidian-800">
                    <span className="text-slate-400">Current CPU</span>
                    <span className="text-emerald-400 font-bold">{stats.cpu_percent.toFixed(1)}%</span>
                  </div>
                  <div className="flex justify-between items-center pb-2 border-b border-obsidian-800">
                    <span className="text-slate-400">Current RAM</span>
                    <span className="text-slate-200">{(stats.ram_bytes / (1024 * 1024)).toFixed(0)} MB</span>
                  </div>
                  <div className="flex justify-between items-center">
                    <span className="text-slate-400">Online Players</span>
                    <span className="text-cyber-cyan font-bold">{stats.player_count}</span>
                  </div>
                </div>
              </div>
            )}
          </div>
        </div>
      )}

      {/* PLAYERS & CHAT TAB */}
      {activeTab === 'players' && (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
          {/* Online Players */}
          <div className="bg-obsidian-900 border border-obsidian-700/80 rounded-xl p-5 flex flex-col h-[520px]">
            <h3 className="font-mono text-sm font-bold text-slate-200 mb-3 flex items-center justify-between">
              <span className="flex items-center gap-2">
                <Users className="w-4 h-4 text-emerald-400" />
                <span>Online Players ({onlinePlayers.length})</span>
              </span>
            </h3>

            <div className="flex-1 bg-obsidian-950 rounded-lg p-3 overflow-y-auto border border-obsidian-800 space-y-2 font-mono text-xs">
              {onlinePlayers.length === 0 ? (
                <p className="text-slate-600 italic">No players online right now.</p>
              ) : (
                onlinePlayers.map((p) => (
                  <div key={p.gamertag} className="p-2.5 bg-obsidian-900 border border-obsidian-700/80 rounded-lg flex items-center justify-between">
                    <div>
                      <div className="font-bold text-slate-100">{p.gamertag}</div>
                      <div className="text-[10px] text-slate-500">XUID: {p.xuid}</div>
                    </div>
                    <div className="flex items-center space-x-2">
                      <button
                        onClick={() => handleOp(p.gamertag)}
                        title="Promote to Operator"
                        className="px-2 py-1 bg-obsidian-800 hover:bg-emerald-950 hover:text-emerald-400 rounded text-[11px]"
                      >
                        OP
                      </button>
                      <button
                        onClick={() => handleKick(p.gamertag)}
                        title="Kick Player"
                        className="px-2 py-1 bg-obsidian-800 hover:bg-rose-950 hover:text-rose-400 rounded text-[11px]"
                      >
                        Kick
                      </button>
                    </div>
                  </div>
                ))
              )}
            </div>

            {/* Broadcast Box */}
            <form onSubmit={handleBroadcast} className="mt-4 pt-3 border-t border-obsidian-800 space-y-2">
              <label className="block text-[11px] font-mono text-slate-400 uppercase">In-Game Broadcast</label>
              <div className="flex gap-2">
                <input
                  type="text"
                  value={broadcastMsg}
                  onChange={(e) => setBroadcastMsg(e.target.value)}
                  placeholder="Broadcast message to server chat..."
                  className="flex-1 px-3 py-2 bg-obsidian-950 border border-obsidian-700 rounded-lg text-slate-100 font-mono text-xs focus:border-emerald-500"
                />
                <button
                  type="submit"
                  className="px-3.5 py-2 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-slate-950 font-bold font-mono text-xs flex items-center space-x-1"
                >
                  <Send className="w-3.5 h-3.5" />
                  <span>Send</span>
                </button>
              </div>
            </form>
          </div>

          {/* Live In-Game Chat Feed */}
          <div className="bg-obsidian-900 border border-obsidian-700/80 rounded-xl p-5 flex flex-col h-[520px]">
            <h3 className="font-mono text-sm font-bold text-slate-200 mb-3 flex items-center gap-2">
              <MessageSquare className="w-4 h-4 text-cyber-cyan" />
              <span>In-Game Chat Feed</span>
            </h3>

            <div className="flex-1 bg-obsidian-950 rounded-lg p-3 overflow-y-auto border border-obsidian-800 space-y-2 font-mono text-xs">
              {chatFeed.length === 0 ? (
                <p className="text-slate-600 italic">No chat messages captured yet.</p>
              ) : (
                chatFeed.map((msg, i) => (
                  <div key={i} className="p-2 rounded bg-obsidian-900/60 border border-obsidian-800 flex items-start space-x-2">
                    <span className="text-emerald-400 font-bold">&lt;{msg.gamertag}&gt;</span>
                    <span className="text-slate-200">{msg.message}</span>
                  </div>
                ))
              )}
            </div>
          </div>
        </div>
      )}

      {/* PORT GATE TAB */}
      {activeTab === 'portgate' && (
        <div className="space-y-6">
          <div className="bg-obsidian-900 border border-obsidian-700/80 rounded-xl p-6">
            <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 mb-6">
              <div>
                <h3 className="font-mono text-base font-bold text-slate-100 flex items-center gap-2">
                  <Shield className="w-4 h-4 text-emerald-400" />
                  <span>Dynamic Firewall Port Gate</span>
                </h3>
                <p className="text-xs text-slate-400 mt-1 font-mono">
                  Port {server.port}/udp is dynamically granted to verified players.
                </p>
              </div>
              <div className="flex items-center space-x-3">
                <button
                  onClick={() => setShowManualLeaseModal(true)}
                  className="px-3 py-1.5 rounded-lg bg-obsidian-800 hover:bg-obsidian-700 text-slate-200 border border-obsidian-600 font-mono text-xs font-bold flex items-center space-x-1.5"
                >
                  <UserPlus className="w-3.5 h-3.5 text-emerald-400" />
                  <span>Manual IP Grant</span>
                </button>
                <Link
                  to={`/knock/${server.id}`}
                  target="_blank"
                  className="px-3 py-1.5 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-slate-950 font-mono text-xs font-bold flex items-center space-x-1.5"
                >
                  <span>Test Unlock Portal</span>
                  <ExternalLink className="w-3 h-3" />
                </Link>
              </div>
            </div>

            <div className="p-4 bg-obsidian-950 border border-obsidian-800 rounded-lg text-xs font-mono text-slate-300 mb-6">
              <p className="text-emerald-400 mb-1 font-bold">Mobile Roaming Handoff Ready</p>
              <p className="text-slate-400">
                When players authenticate via the Knock Portal, their browser keeps a lightweight background heartbeat (default: 10s).
                If their cellular IP changes while moving between cell towers or networks, BSM automatically detects the change and hot-swaps the firewall rule.
              </p>
            </div>

            <h4 className="font-mono text-xs font-bold uppercase tracking-wider text-slate-400 mb-3">
              Active Client IP Leases ({leases.length})
            </h4>

            {leases.length === 0 ? (
              <div className="text-center py-8 bg-obsidian-950/60 border border-obsidian-800 rounded-lg text-slate-500 font-mono text-xs">
                No active firewall grants at this moment. Players must authenticate via the Knock Portal to unlock UDP port access.
              </div>
            ) : (
              <div className="border border-obsidian-800 rounded-lg overflow-hidden">
                <table className="w-full text-left font-mono text-xs">
                  <thead className="bg-obsidian-950 text-slate-400 border-b border-obsidian-800 uppercase">
                    <tr>
                      <th className="px-4 py-2.5">IP Address</th>
                      <th className="px-4 py-2.5">Gamertag</th>
                      <th className="px-4 py-2.5">Knock Method</th>
                      <th className="px-4 py-2.5">Granted At</th>
                      <th className="px-4 py-2.5">Expires At</th>
                      <th className="px-4 py-2.5 text-right">Action</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-obsidian-800 bg-obsidian-950/40">
                    {leases.map((lease) => (
                      <tr key={lease.id} className="hover:bg-obsidian-800/40">
                        <td className="px-4 py-2.5 text-emerald-400 font-bold">{lease.ip_address}</td>
                        <td className="px-4 py-2.5 text-slate-300">{lease.gamertag || '—'}</td>
                        <td className="px-4 py-2.5 text-slate-400 capitalize">{lease.knock_method}</td>
                        <td className="px-4 py-2.5 text-slate-400">{new Date(lease.granted_at).toLocaleTimeString()}</td>
                        <td className="px-4 py-2.5 text-slate-400">{new Date(lease.expires_at).toLocaleTimeString()}</td>
                        <td className="px-4 py-2.5 text-right">
                          <button
                            onClick={() => handleRevokeLease(lease.id)}
                            className="px-2 py-1 rounded bg-rose-950/60 hover:bg-rose-900 border border-rose-800 text-rose-300 text-[10px] font-bold"
                          >
                            Revoke
                          </button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>

          {/* MANUAL LEASE MODAL */}
          {showManualLeaseModal && (
            <div className="fixed inset-0 bg-black/80 backdrop-blur-sm z-50 flex items-center justify-center p-4">
              <div className="bg-obsidian-900 border border-obsidian-700 rounded-xl max-w-md w-full p-6 shadow-2xl">
                <h3 className="text-base font-mono font-bold text-slate-100 mb-4 flex items-center gap-2">
                  <UserPlus className="w-4 h-4 text-emerald-400" />
                  <span>Grant Manual IP Bypass</span>
                </h3>
                <form onSubmit={handleCreateManualLease} className="space-y-4 font-mono text-xs">
                  <div>
                    <label className="block text-slate-300 mb-1 font-bold">Client IP Address</label>
                    <input
                      type="text"
                      required
                      placeholder="e.g. 198.51.100.24"
                      value={manualIP}
                      onChange={(e) => setManualIP(e.target.value)}
                      className="w-full px-3 py-2 bg-obsidian-950 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
                    />
                  </div>
                  <div>
                    <label className="block text-slate-300 mb-1 font-bold">Gamertag (optional)</label>
                    <input
                      type="text"
                      placeholder="e.g. DiamondMiner99"
                      value={manualGamertag}
                      onChange={(e) => setManualGamertag(e.target.value)}
                      className="w-full px-3 py-2 bg-obsidian-950 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
                    />
                  </div>
                  <div>
                    <label className="block text-slate-300 mb-1 font-bold">Duration (minutes)</label>
                    <input
                      type="number"
                      min="1"
                      value={manualDuration}
                      onChange={(e) => setManualDuration(parseInt(e.target.value) || 60)}
                      className="w-full px-3 py-2 bg-obsidian-950 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
                    />
                  </div>
                  <div>
                    <label className="block text-slate-300 mb-1 font-bold">Comment / Note</label>
                    <input
                      type="text"
                      placeholder="Reason for manual bypass"
                      value={manualComment}
                      onChange={(e) => setManualComment(e.target.value)}
                      className="w-full px-3 py-2 bg-obsidian-950 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
                    />
                  </div>
                  <div className="flex justify-end space-x-3 pt-3 border-t border-obsidian-800">
                    <button
                      type="button"
                      onClick={() => setShowManualLeaseModal(false)}
                      className="px-4 py-2 rounded-lg bg-obsidian-800 text-slate-300 hover:bg-obsidian-700"
                    >
                      Cancel
                    </button>
                    <button
                      type="submit"
                      className="px-4 py-2 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-slate-950 font-bold"
                    >
                      Apply Firewall Rule
                    </button>
                  </div>
                </form>
              </div>
            </div>
          )}
        </div>
      )}

      {/* BACKUPS & WORLDS TAB */}
      {activeTab === 'backups' && (
        <div className="space-y-6">
          <div className="bg-obsidian-900 border border-obsidian-700/80 rounded-xl p-6">
            <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 mb-6">
              <div>
                <h3 className="font-mono text-base font-bold text-slate-100 flex items-center gap-2">
                  <Archive className="w-4 h-4 text-emerald-400" />
                  <span>Zero-Downtime Hot Backups</span>
                </h3>
                <p className="text-xs text-slate-400 mt-1 font-mono">
                  LevelDB safe snapshots using Bedrock's save hold/resume protocol without taking the server offline.
                </p>
              </div>

              <div className="flex items-center flex-wrap gap-2">
                <label className="cursor-pointer px-3 py-1.5 rounded-lg bg-obsidian-800 hover:bg-obsidian-700 text-slate-200 border border-obsidian-600 font-mono text-xs font-bold flex items-center space-x-1.5">
                  <Upload className="w-3.5 h-3.5 text-emerald-400" />
                  <span>Import World</span>
                  <input
                    type="file"
                    accept=".mcworld,.zip"
                    onChange={handleImportWorld}
                    className="hidden"
                  />
                </label>

                <a
                  href={api.exportWorldUrl(server.id)}
                  download
                  className="px-3 py-1.5 rounded-lg bg-obsidian-800 hover:bg-obsidian-700 text-slate-200 border border-obsidian-600 font-mono text-xs font-bold flex items-center space-x-1.5"
                >
                  <Download className="w-3.5 h-3.5 text-emerald-400" />
                  <span>Export .mcworld</span>
                </a>

                <button
                  onClick={handleCreateBackup}
                  disabled={backupLoading}
                  className="px-4 py-1.5 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-slate-950 font-mono text-xs font-bold flex items-center space-x-1.5 disabled:opacity-50"
                >
                  {backupLoading ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <Play className="w-3.5 h-3.5" />}
                  <span>Hot Backup Now</span>
                </button>
              </div>
            </div>

            <h4 className="font-mono text-xs font-bold uppercase tracking-wider text-slate-400 mb-3">
              Stored World Archives ({backupsList.length})
            </h4>

            {backupsList.length === 0 ? (
              <div className="text-center py-10 bg-obsidian-950/60 border border-obsidian-800 rounded-lg text-slate-500 font-mono text-xs">
                No backups recorded yet. Click "Hot Backup Now" to create an instant LevelDB snapshot.
              </div>
            ) : (
              <div className="border border-obsidian-800 rounded-lg overflow-hidden">
                <table className="w-full text-left font-mono text-xs">
                  <thead className="bg-obsidian-950 text-slate-400 border-b border-obsidian-800 uppercase">
                    <tr>
                      <th className="px-4 py-2.5">Pin</th>
                      <th className="px-4 py-2.5">Archive File</th>
                      <th className="px-4 py-2.5">Size</th>
                      <th className="px-4 py-2.5">Type</th>
                      <th className="px-4 py-2.5">Timestamp</th>
                      <th className="px-4 py-2.5 text-right">Actions</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-obsidian-800 bg-obsidian-950/40">
                    {backupsList.map((b) => (
                      <tr key={b.id} className="hover:bg-obsidian-800/40">
                        <td className="px-4 py-2.5">
                          <button
                            onClick={() => handleToggleBackupLock(b.id)}
                            title={b.is_locked ? 'Pinned/Locked against retention purge' : 'Click to pin/lock'}
                            className={`p-1 rounded hover:bg-obsidian-800 transition-colors ${
                              b.is_locked ? 'text-amber-400' : 'text-slate-600 hover:text-slate-400'
                            }`}
                          >
                            {b.is_locked ? <Lock className="w-3.5 h-3.5" /> : <Unlock className="w-3.5 h-3.5" />}
                          </button>
                        </td>
                        <td className="px-4 py-2.5 font-bold text-slate-200">{b.filename}</td>
                        <td className="px-4 py-2.5 text-slate-300">
                          {(b.size_bytes / (1024 * 1024)).toFixed(2)} MB
                        </td>
                        <td className="px-4 py-2.5 capitalize text-slate-400">
                          <span className="px-2 py-0.5 rounded bg-obsidian-800 border border-obsidian-700">
                            {b.type}
                          </span>
                        </td>
                        <td className="px-4 py-2.5 text-slate-400">
                          {new Date(b.created_at).toLocaleString()}
                        </td>
                        <td className="px-4 py-2.5 text-right space-x-2">
                          <a
                            href={api.downloadBackupUrl(server.id, b.id)}
                            download
                            title="Download Archive"
                            className="inline-block p-1.5 rounded bg-obsidian-800 hover:bg-obsidian-700 text-slate-300 transition-colors"
                          >
                            <Download className="w-3.5 h-3.5" />
                          </a>
                          <button
                            onClick={() => handleRestoreBackup(b.id)}
                            title="Restore to Server"
                            className="p-1.5 rounded bg-amber-500/20 text-amber-400 hover:bg-amber-500 hover:text-slate-950 transition-colors"
                          >
                            <RefreshCw className="w-3.5 h-3.5" />
                          </button>
                          <button
                            onClick={() => handleDeleteBackup(b.id)}
                            disabled={b.is_locked}
                            title={b.is_locked ? 'Locked backup cannot be deleted' : 'Delete Backup'}
                            className="p-1.5 rounded bg-rose-600/20 text-rose-400 hover:bg-rose-600 hover:text-slate-950 transition-colors disabled:opacity-30 disabled:cursor-not-allowed"
                          >
                            <Trash2 className="w-3.5 h-3.5" />
                          </button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        </div>
      )}

      {/* ADDONS & PACKS TAB */}
      {activeTab === 'addons' && (
        <div className="space-y-6">
          <div className="bg-obsidian-900 border border-obsidian-700/80 rounded-xl p-6">
            <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 mb-6">
              <div>
                <h3 className="font-mono text-base font-bold text-slate-100 flex items-center gap-2">
                  <Package className="w-4 h-4 text-emerald-400" />
                  <span>Bedrock Addons & Resource Packs</span>
                </h3>
                <p className="text-xs text-slate-400 mt-1 font-mono">
                  Upload .mcpack or .zip archives to install behavior and texture packs into the server.
                </p>
              </div>

              <div>
                <label className="cursor-pointer px-4 py-2 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-slate-950 font-mono text-xs font-bold flex items-center space-x-2">
                  {addonLoading ? <Loader2 className="w-4 h-4 animate-spin" /> : <Upload className="w-4 h-4" />}
                  <span>Install .mcpack / .zip</span>
                  <input
                    type="file"
                    accept=".mcpack,.mcaddon,.zip"
                    onChange={handleInstallAddon}
                    disabled={addonLoading}
                    className="hidden"
                  />
                </label>
              </div>
            </div>

            <h4 className="font-mono text-xs font-bold uppercase tracking-wider text-slate-400 mb-3">
              Installed Packs ({addonsList.length})
            </h4>

            {addonsList.length === 0 ? (
              <div className="text-center py-10 bg-obsidian-950/60 border border-obsidian-800 rounded-lg text-slate-500 font-mono text-xs">
                No custom addons installed yet. Click "Install .mcpack / .zip" to add behavior or texture packs.
              </div>
            ) : (
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                {addonsList.map((pack) => (
                  <div
                    key={`${pack.type}-${pack.folder}`}
                    className="bg-obsidian-950 border border-obsidian-800 rounded-lg p-4 flex flex-col justify-between"
                  >
                    <div>
                      <div className="flex items-center justify-between mb-2">
                        <span
                          className={`text-[10px] font-mono px-2 py-0.5 rounded uppercase font-bold border ${
                            pack.type === 'behavior'
                              ? 'bg-purple-500/10 text-purple-400 border-purple-500/30'
                              : 'bg-cyan-500/10 text-cyan-400 border-cyan-500/30'
                          }`}
                        >
                          {pack.type === 'behavior' ? 'Behavior Pack' : 'Resource Pack'}
                        </span>
                        <span className="text-[11px] font-mono text-slate-400">v{pack.version}</span>
                      </div>
                      <h5 className="font-mono font-bold text-sm text-slate-200">{pack.name}</h5>
                      <p className="font-mono text-xs text-slate-400 mt-1 line-clamp-2">
                        {pack.description || 'No description provided.'}
                      </p>
                    </div>

                    <div className="mt-4 pt-3 border-t border-obsidian-800/80 flex items-center justify-between font-mono text-[11px] text-slate-500">
                      <span>Folder: {pack.folder}</span>
                      <button
                        onClick={() => handleDeleteAddon(pack.type, pack.folder)}
                        className="text-rose-400 hover:text-rose-300 flex items-center space-x-1"
                      >
                        <Trash2 className="w-3.5 h-3.5" />
                        <span>Remove</span>
                      </button>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>
      )}

      {/* CONFIGURATION EDITOR TAB */}
      {activeTab === 'settings' && (
        <div className="bg-obsidian-900 border border-obsidian-700/80 rounded-xl p-6 max-w-4xl space-y-8">
          {/* server.properties form */}
          <div>
            <div className="flex items-center justify-between mb-4">
              <h3 className="font-mono text-base font-bold text-slate-100 flex items-center gap-2">
                <Settings className="w-4 h-4 text-emerald-400" />
                <span>server.properties Editor</span>
              </h3>

              <button
                onClick={handleSaveProperties}
                disabled={configSaving}
                className="px-4 py-2 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-slate-950 font-bold font-mono text-xs flex items-center space-x-1.5"
              >
                {configSaving ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : configSaved ? <Check className="w-3.5 h-3.5" /> : null}
                <span>{configSaved ? 'Saved!' : 'Save Properties'}</span>
              </button>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-4 font-mono text-xs bg-obsidian-950 p-4 rounded-xl border border-obsidian-800">
              {propKeys.slice(0, 12).map((key) => (
                <div key={key}>
                  <label className="block text-slate-400 mb-1 text-[11px]">{key}</label>
                  <input
                    type="text"
                    value={properties[key] || ''}
                    onChange={(e) => setProperties({ ...properties, [key]: e.target.value })}
                    className="w-full px-3 py-1.5 rounded bg-obsidian-900 border border-obsidian-700 text-slate-100 text-xs focus:border-emerald-500"
                  />
                </div>
              ))}
            </div>
          </div>

          {/* allowlist.json manager */}
          <div>
            <h3 className="font-mono text-base font-bold text-slate-100 mb-3 flex items-center gap-2">
              <ShieldAlert className="w-4 h-4 text-cyber-cyan" />
              <span>Allowlist Manager (allowlist.json)</span>
            </h3>

            <div className="bg-obsidian-950 p-4 rounded-xl border border-obsidian-800 space-y-3 font-mono text-xs">
              <div className="flex gap-2">
                <input
                  type="text"
                  value={newAllowlistPlayer}
                  onChange={(e) => setNewAllowlistPlayer(e.target.value)}
                  placeholder="Enter player Gamertag..."
                  className="flex-1 px-3 py-1.5 rounded bg-obsidian-900 border border-obsidian-700 text-slate-100 text-xs"
                />
                <button
                  onClick={handleAddAllowlistPlayer}
                  className="px-3 py-1.5 rounded bg-emerald-600 hover:bg-emerald-500 text-slate-950 font-bold flex items-center space-x-1"
                >
                  <UserPlus className="w-3.5 h-3.5" />
                  <span>Add Player</span>
                </button>
              </div>

              <div className="space-y-1.5 pt-2">
                {allowlist.length === 0 ? (
                  <p className="text-slate-600 italic">No players in allowlist.</p>
                ) : (
                  allowlist.map((p) => (
                    <div key={p.name} className="flex items-center justify-between p-2 rounded bg-obsidian-900 border border-obsidian-800">
                      <span className="text-slate-200 font-bold">{p.name}</span>
                      <button
                        onClick={() => handleRemoveAllowlistPlayer(p.name)}
                        className="text-rose-400 hover:underline text-[11px]"
                      >
                        Remove
                      </button>
                    </div>
                  ))
                )}
              </div>
            </div>
          </div>
        </div>
      )}

      {/* CLONE & EXPORT TAB */}
      {activeTab === 'actions' && (
        <div className="bg-obsidian-900 border border-obsidian-700/80 rounded-xl p-6 max-w-2xl space-y-8">
          {/* Update Banner */}
          {updateInfo && (
            <div className="p-4 bg-obsidian-950 border border-obsidian-800 rounded-xl font-mono text-xs space-y-2">
              <div className="flex items-center justify-between">
                <span className="text-slate-400">Current BDS Version:</span>
                <span className="text-slate-200 font-bold">{updateInfo.current_version}</span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-slate-400">Latest Available:</span>
                <span className="text-emerald-400 font-bold">{updateInfo.latest_version}</span>
              </div>
              {updateInfo.update_available && (
                <div className="pt-2">
                  <a
                    href={updateInfo.release_url}
                    target="_blank"
                    className="inline-flex items-center space-x-1 text-emerald-400 hover:underline text-xs"
                  >
                    <span>View Release Notes</span>
                    <ExternalLink className="w-3 h-3" />
                  </a>
                </div>
              )}
            </div>
          )}

          {/* 1-Click Clone Form */}
          <div>
            <h3 className="font-mono text-base font-bold text-slate-100 mb-2 flex items-center gap-2">
              <Copy className="w-4 h-4 text-emerald-400" />
              <span>1-Click Server Cloning</span>
            </h3>
            <p className="text-xs text-slate-400 font-mono mb-4">
              Duplicate all worlds, configurations, and packs. An unused UDP port is automatically assigned.
            </p>

            <form onSubmit={handleClone} className="space-y-3 font-mono text-xs">
              <div>
                <label className="block text-slate-400 mb-1">Cloned Server Name</label>
                <input
                  type="text"
                  required
                  value={cloneName}
                  onChange={(e) => {
                    setCloneName(e.target.value);
                    if (!cloneId) setCloneId(e.target.value.toLowerCase().replace(/[^a-z0-9]/g, '-'));
                  }}
                  placeholder="Survival Clone"
                  className="w-full px-3 py-2 rounded bg-obsidian-950 border border-obsidian-700 text-slate-100 text-xs"
                />
              </div>

              <div>
                <label className="block text-slate-400 mb-1">Cloned Server ID</label>
                <input
                  type="text"
                  required
                  value={cloneId}
                  onChange={(e) => setCloneId(e.target.value)}
                  placeholder="survival-clone"
                  className="w-full px-3 py-2 rounded bg-obsidian-950 border border-obsidian-700 text-slate-100 text-xs"
                />
              </div>

              <button
                type="submit"
                disabled={cloning}
                className="px-4 py-2.5 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-slate-950 font-bold font-mono text-xs flex items-center space-x-2"
              >
                {cloning ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <Copy className="w-3.5 h-3.5" />}
                <span>Clone Instance</span>
              </button>
            </form>
          </div>

          {/* Export Bundle */}
          <div className="pt-6 border-t border-obsidian-800">
            <h3 className="font-mono text-base font-bold text-slate-100 mb-2 flex items-center gap-2">
              <Download className="w-4 h-4 text-cyber-cyan" />
              <span>Full Server Export</span>
            </h3>
            <p className="text-xs text-slate-400 font-mono mb-4">
              Download a complete .zip archive containing all worlds, packs, and configuration files for migration or safe-keeping.
            </p>

            <a
              href={api.getExportUrl(server.id)}
              download
              className="inline-flex items-center space-x-2 px-4 py-2.5 rounded-lg bg-obsidian-800 hover:bg-obsidian-750 border border-obsidian-700 text-slate-200 hover:text-emerald-400 font-mono text-xs font-bold transition-colors"
            >
              <Download className="w-4 h-4" />
              <span>Download .zip Bundle</span>
            </a>
          </div>
        </div>
      )}
    </div>
  );
};
