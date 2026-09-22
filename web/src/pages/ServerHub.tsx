import React, { useEffect, useState, useRef } from 'react';
import { useParams, Link } from 'react-router-dom';
import {
  ArrowLeft, Play, Square, RefreshCw, Shield, Terminal, Settings, ExternalLink,
  HardDrive, Cpu, AlertTriangle, Loader2, Send, Users, MessageSquare, Copy,
  Download, UserPlus, ShieldAlert, Check
} from 'lucide-react';
import { api } from '../api/client';
import { Server, User } from '../types';

interface ServerHubProps {
  user: User;
}

export const ServerHub: React.FC<ServerHubProps> = () => {
  const { id } = useParams<{ id: string }>();
  const [server, setServer] = useState<Server | null>(null);
  const [loading, setLoading] = useState(true);
  const [activeTab, setActiveTab] = useState<'overview' | 'players' | 'portgate' | 'settings' | 'actions'>('overview');
  const [error, setError] = useState<string | null>(null);
  const [actionLoading, setActionLoading] = useState(false);

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
        <div className="bg-obsidian-900 border border-obsidian-700/80 rounded-xl p-6">
          <div className="flex items-center justify-between mb-4">
            <div>
              <h3 className="font-mono text-base font-bold text-slate-100 flex items-center gap-2">
                <Shield className="w-4 h-4 text-emerald-400" />
                <span>Dynamic Firewall Port Gate</span>
              </h3>
              <p className="text-xs text-slate-400 mt-1 font-mono">
                Port {server.port}/udp is dynamically granted to verified players.
              </p>
            </div>
            <Link
              to={`/knock/${server.id}`}
              target="_blank"
              className="px-3 py-1.5 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-slate-950 font-mono text-xs font-bold flex items-center space-x-1.5"
            >
              <span>Test Unlock Portal</span>
              <ExternalLink className="w-3 h-3" />
            </Link>
          </div>

          <div className="p-4 bg-obsidian-950 border border-obsidian-800 rounded-lg text-xs font-mono text-slate-300">
            <p className="text-emerald-400 mb-1 font-bold">Mobile Roaming Handoff Ready</p>
            <p className="text-slate-400">
              When players authenticate via the Knock Portal, their browser keeps a lightweight background heartbeat (default: 10s).
              If their cellular IP changes while moving between cell towers or networks, BSM automatically detects the change and hot-swaps the firewall rule.
            </p>
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
