import React, { useEffect, useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import { ArrowLeft, Play, Shield, Terminal, Settings, ExternalLink, HardDrive, Cpu, AlertTriangle, Loader2 } from 'lucide-react';
import { api } from '../api/client';
import { Server, User } from '../types';

interface ServerHubProps {
  user: User;
}

export const ServerHub: React.FC<ServerHubProps> = () => {
  const { id } = useParams<{ id: string }>();
  const [server, setServer] = useState<Server | null>(null);
  const [loading, setLoading] = useState(true);
  const [activeTab, setActiveTab] = useState<'overview' | 'portgate' | 'settings'>('overview');
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!id) return;
    const fetchServer = async () => {
      try {
        const data = await api.getServer(id);
        setServer(data);
      } catch (err: any) {
        setError(err.message || 'Server not found');
      } finally {
        setLoading(false);
      }
    };
    fetchServer();
  }, [id]);

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

          <button
            className="px-3 py-2 rounded-lg bg-obsidian-850 hover:bg-obsidian-800 border border-obsidian-700 text-slate-200 font-mono text-xs flex items-center space-x-1.5"
            onClick={() => alert('Server lifecycle management active in Phase 2!')}
          >
            <Play className="w-3.5 h-3.5 text-emerald-400" />
            <span>Start</span>
          </button>
        </div>
      </div>

      {/* Tabs */}
      <div className="flex border-b border-obsidian-700/80 mb-6 font-mono text-xs">
        <button
          onClick={() => setActiveTab('overview')}
          className={`pb-3 px-4 border-b-2 font-medium flex items-center space-x-2 transition-colors ${
            activeTab === 'overview'
              ? 'border-emerald-500 text-emerald-400'
              : 'border-transparent text-slate-400 hover:text-slate-200'
          }`}
        >
          <Terminal className="w-3.5 h-3.5" />
          <span>Overview</span>
        </button>

        <button
          onClick={() => setActiveTab('portgate')}
          className={`pb-3 px-4 border-b-2 font-medium flex items-center space-x-2 transition-colors ${
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
          className={`pb-3 px-4 border-b-2 font-medium flex items-center space-x-2 transition-colors ${
            activeTab === 'settings'
              ? 'border-emerald-500 text-emerald-400'
              : 'border-transparent text-slate-400 hover:text-slate-200'
          }`}
        >
          <Settings className="w-3.5 h-3.5" />
          <span>Configuration</span>
        </button>
      </div>

      {/* Tab Contents */}
      {activeTab === 'overview' && (
        <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
          <div className="bg-obsidian-900 border border-obsidian-700/80 rounded-xl p-5 md:col-span-2">
            <h3 className="font-mono text-sm font-bold text-slate-200 mb-4 flex items-center gap-2">
              <Terminal className="w-4 h-4 text-emerald-400" />
              <span>Live Console Stream</span>
            </h3>
            <div className="h-64 bg-obsidian-950 rounded-lg p-4 font-mono text-xs text-slate-300 overflow-y-auto border border-obsidian-800">
              <p className="text-slate-500">[BSM] Container orchestrator ready.</p>
              <p className="text-slate-500">[BSM] Waiting for server boot...</p>
              <p className="text-emerald-400/80">[BSM] Port gating active on UDP {server.port}.</p>
            </div>
          </div>

          <div className="space-y-6">
            <div className="bg-obsidian-900 border border-obsidian-700/80 rounded-xl p-5">
              <h3 className="font-mono text-sm font-bold text-slate-200 mb-3">Resource Limits</h3>
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

            <div className="bg-obsidian-900 border border-obsidian-700/80 rounded-xl p-5">
              <h3 className="font-mono text-sm font-bold text-slate-200 mb-3">Port Gating</h3>
              <div className="font-mono text-xs space-y-2">
                <div className="flex justify-between text-slate-400">
                  <span>Status:</span>
                  <span className={server.port_gate_enabled ? 'text-emerald-400 font-bold' : 'text-slate-500'}>
                    {server.port_gate_enabled ? 'ENABLED' : 'DISABLED'}
                  </span>
                </div>
                <div className="flex justify-between text-slate-400">
                  <span>Verification:</span>
                  <span className="text-slate-200">{server.port_gate_mode}</span>
                </div>
                <div className="flex justify-between text-slate-400">
                  <span>Lease Timeout:</span>
                  <span className="text-slate-200">{server.port_gate_timeout / 3600} hours</span>
                </div>
              </div>
            </div>
          </div>
        </div>
      )}

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

      {activeTab === 'settings' && (
        <div className="bg-obsidian-900 border border-obsidian-700/80 rounded-xl p-6 max-w-2xl">
          <h3 className="font-mono text-base font-bold text-slate-100 mb-4 flex items-center gap-2">
            <Settings className="w-4 h-4 text-emerald-400" />
            <span>Server Properties & Settings</span>
          </h3>

          <div className="space-y-4 font-mono text-xs">
            <div>
              <label className="block text-slate-400 mb-1">Server Name</label>
              <input
                type="text"
                defaultValue={server.name}
                className="w-full px-3 py-2 rounded-lg bg-obsidian-950 border border-obsidian-700 text-slate-100 text-sm"
              />
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="block text-slate-400 mb-1">Default Game Mode</label>
                <select
                  defaultValue={server.mode}
                  className="w-full px-3 py-2 rounded-lg bg-obsidian-950 border border-obsidian-700 text-slate-100 text-sm"
                >
                  <option value="survival">Survival</option>
                  <option value="creative">Creative</option>
                  <option value="adventure">Adventure</option>
                </select>
              </div>

              <div>
                <label className="block text-slate-400 mb-1">Difficulty</label>
                <select
                  defaultValue={server.difficulty}
                  className="w-full px-3 py-2 rounded-lg bg-obsidian-950 border border-obsidian-700 text-slate-100 text-sm"
                >
                  <option value="peaceful">Peaceful</option>
                  <option value="easy">Easy</option>
                  <option value="normal">Normal</option>
                  <option value="hard">Hard</option>
                </select>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};
