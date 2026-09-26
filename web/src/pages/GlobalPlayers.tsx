import React, { useEffect, useState } from 'react';
import {
  Users, UserPlus, RefreshCw, Trash2, Crown,
  Search, Check, Loader2, Copy, Filter, Sparkles
} from 'lucide-react';
import { api } from '../api/client';
import { GlobalPlayer } from '../types';
import { Pagination } from '../components/Pagination';
import { usePagination } from '../hooks/usePagination';

export const GlobalPlayers: React.FC = () => {
  const [players, setPlayers] = useState<GlobalPlayer[]>([]);
  const [loading, setLoading] = useState(true);
  const [searchQuery, setSearchQuery] = useState('');
  const [roleFilter, setRoleFilter] = useState<string>('all');
  const [showAddModal, setShowAddModal] = useState(false);
  const [actionLoading, setActionLoading] = useState(false);
  const [syncLoading, setSyncLoading] = useState(false);
  const [syncStatus, setSyncStatus] = useState<string | null>(null);

  // New Player Form State
  const [name, setName] = useState('');
  const [xuid, setXuid] = useState('');
  const [permission, setPermission] = useState<'operator' | 'member' | 'visitor' | 'none'>('operator');
  const [isAllowlisted, setIsAllowlisted] = useState(true);
  const [ignoresLimit, setIgnoresLimit] = useState(false);

  const fetchPlayers = async () => {
    try {
      const data = await api.listGlobalPlayers();
      setPlayers(data);
    } catch (err: any) {
      alert(err.message || 'Failed to load global players');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchPlayers();
  }, []);

  const handleAddPlayer = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim()) return;

    setActionLoading(true);
    try {
      await api.createGlobalPlayer({
        name: name.trim(),
        xuid: xuid.trim(),
        permission,
        is_allowlisted: isAllowlisted,
        ignores_player_limit: ignoresLimit,
      });
      setShowAddModal(false);
      setName('');
      setXuid('');
      setPermission('operator');
      setIsAllowlisted(true);
      setIgnoresLimit(false);
      await fetchPlayers();
    } catch (err: any) {
      alert(err.message || 'Failed to save global player');
    } finally {
      setActionLoading(false);
    }
  };

  const handleDeletePlayer = async (id: number, playerName: string) => {
    if (!confirm(`Are you sure you want to remove '${playerName}' from the Global Player list?`)) {
      return;
    }

    try {
      await api.deleteGlobalPlayer(id);
      await fetchPlayers();
    } catch (err: any) {
      alert(err.message || 'Failed to delete global player');
    }
  };

  const handleSyncAllServers = async () => {
    setSyncLoading(true);
    setSyncStatus(null);
    try {
      const results = await api.syncAllServersGlobal();
      const serverCount = Object.keys(results).length;
      let totalAdded = 0;
      let totalPerms = 0;
      for (const key of Object.keys(results)) {
        totalAdded += results[key].allowlist_added.length;
        totalPerms += results[key].permissions_updated.length;
      }
      setSyncStatus(`Successfully synced to ${serverCount} server(s). (${totalAdded} allowlisted, ${totalPerms} permissions updated)`);
      setTimeout(() => setSyncStatus(null), 5000);
    } catch (err: any) {
      alert(err.message || 'Failed to sync servers');
    } finally {
      setSyncLoading(false);
    }
  };

  const filteredPlayers = players.filter((p) => {
    const matchesSearch =
      p.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
      p.xuid.toLowerCase().includes(searchQuery.toLowerCase());

    if (!matchesSearch) return false;

    if (roleFilter === 'operator') return p.permission === 'operator';
    if (roleFilter === 'member') return p.permission === 'member';
    if (roleFilter === 'visitor') return p.permission === 'visitor';
    if (roleFilter === 'allowlist') return p.is_allowlisted;

    return true;
  });

  const {
    currentPage,
    pageSize,
    totalItems,
    paginatedItems: paginatedPlayers,
    setCurrentPage,
    setPageSize,
  } = usePagination(filteredPlayers, 10);

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
      {/* Header */}
      <div className="flex flex-col md:flex-row md:items-center justify-between gap-4 mb-8">
        <div>
          <div className="flex items-center space-x-2 text-emerald-400 font-mono text-xs uppercase tracking-widest mb-1">
            <Sparkles className="w-3.5 h-3.5" />
            <span>Multi-Level Access Control</span>
          </div>
          <h1 className="text-2xl font-bold font-mono text-slate-100 flex items-center gap-2.5">
            <Users className="w-7 h-7 text-emerald-400" />
            <span>Global Players Hub</span>
          </h1>
          <p className="text-sm text-slate-400 font-mono mt-1 max-w-2xl">
            Centralized allowlists and operator roles automatically merged into server instances on start, deploy, and boot.
          </p>
        </div>

        <div className="flex items-center space-x-3">
          <button
            onClick={handleSyncAllServers}
            disabled={syncLoading}
            title="Push global player rules to all active servers"
            className="px-4 py-2.5 rounded-lg bg-obsidian-900 border border-obsidian-700 hover:border-emerald-500/50 text-slate-200 font-mono text-xs font-bold flex items-center space-x-2 transition-colors disabled:opacity-50"
          >
            {syncLoading ? <Loader2 className="w-4 h-4 animate-spin text-emerald-400" /> : <RefreshCw className="w-4 h-4 text-emerald-400" />}
            <span>Sync All Servers</span>
          </button>

          <button
            onClick={() => setShowAddModal(true)}
            className="px-4 py-2.5 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-slate-950 font-mono text-xs font-bold flex items-center space-x-2 shadow-[0_0_15px_rgba(16,185,129,0.2)] transition-colors"
          >
            <UserPlus className="w-4 h-4" />
            <span>Add Global Player</span>
          </button>
        </div>
      </div>

      {syncStatus && (
        <div className="mb-6 p-4 rounded-xl bg-emerald-950/40 border border-emerald-500/40 text-emerald-300 font-mono text-xs flex items-center space-x-2 animate-fadeIn">
          <Check className="w-4 h-4 text-emerald-400 shrink-0" />
          <span>{syncStatus}</span>
        </div>
      )}

      {/* Filter and Search Bar */}
      <div className="bg-obsidian-900 border border-obsidian-700/80 rounded-xl p-4 mb-6 flex flex-col sm:flex-row gap-4 justify-between items-center">
        <div className="relative w-full sm:w-80">
          <Search className="w-4 h-4 text-slate-500 absolute left-3 top-1/2 -translate-y-1/2" />
          <input
            type="text"
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            placeholder="Search gamertag or XUID..."
            className="w-full pl-9 pr-3 py-2 rounded-lg bg-obsidian-950 border border-obsidian-800 text-slate-200 text-xs font-mono focus:border-emerald-500 focus:outline-none"
          />
        </div>

        <div className="flex items-center space-x-2 overflow-x-auto w-full sm:w-auto font-mono text-xs">
          <span className="text-slate-500 flex items-center gap-1 text-[11px] uppercase mr-1">
            <Filter className="w-3 h-3" /> Filter:
          </span>
          {[
            { id: 'all', label: 'All Players' },
            { id: 'operator', label: 'Operators' },
            { id: 'member', label: 'Members' },
            { id: 'visitor', label: 'Visitors' },
            { id: 'allowlist', label: 'Allowlisted' },
          ].map((tab) => (
            <button
              key={tab.id}
              onClick={() => setRoleFilter(tab.id)}
              className={`px-3 py-1.5 rounded-lg transition-colors whitespace-nowrap ${
                roleFilter === tab.id
                  ? 'bg-emerald-500/20 text-emerald-400 border border-emerald-500/40 font-bold'
                  : 'bg-obsidian-950 text-slate-400 hover:text-slate-200 border border-obsidian-800'
              }`}
            >
              {tab.label}
            </button>
          ))}
        </div>
      </div>

      {/* Table */}
      {loading ? (
        <div className="text-center py-20 text-slate-500 font-mono text-xs flex flex-col items-center gap-2">
          <Loader2 className="w-6 h-6 animate-spin text-emerald-400" />
          <span>Loading global players...</span>
        </div>
      ) : filteredPlayers.length === 0 ? (
        <div className="text-center py-16 bg-obsidian-900 border border-obsidian-800 rounded-xl text-slate-400 font-mono text-xs space-y-3">
          <p className="text-slate-500">No global players found matching criteria.</p>
          <button
            onClick={() => setShowAddModal(true)}
            className="px-3 py-1.5 rounded bg-obsidian-800 hover:bg-obsidian-750 text-emerald-400 font-bold text-xs"
          >
            + Add First Global Player
          </button>
        </div>
      ) : (
        <div className="bg-obsidian-900 border border-obsidian-700/80 rounded-xl overflow-hidden">
          <div className="overflow-x-auto">
            <table className="w-full text-left font-mono text-xs">
              <thead className="bg-obsidian-950/80 border-b border-obsidian-800 text-slate-400 uppercase text-[10px] tracking-wider">
                <tr>
                  <th className="px-6 py-3.5">Gamertag</th>
                  <th className="px-6 py-3.5">XUID</th>
                  <th className="px-6 py-3.5">Role / Permission</th>
                  <th className="px-6 py-3.5">Allowlist Status</th>
                  <th className="px-6 py-3.5 text-right">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-obsidian-800/60 text-slate-300">
                {paginatedPlayers.map((p) => (
                  <tr key={p.id} className="hover:bg-obsidian-850/60 transition-colors">
                    <td className="px-6 py-4 font-bold text-slate-100 flex items-center gap-2">
                      <div className="w-2 h-2 rounded-full bg-emerald-400"></div>
                      <span>{p.name}</span>
                    </td>
                    <td className="px-6 py-4 text-slate-400">
                      {p.xuid ? (
                        <div className="flex items-center gap-1.5">
                          <code className="text-[11px] text-slate-300">{p.xuid}</code>
                          <button
                            onClick={() => {
                              navigator.clipboard.writeText(p.xuid);
                              alert(`Copied XUID: ${p.xuid}`);
                            }}
                            title="Copy XUID"
                            className="p-1 rounded hover:bg-obsidian-800 text-slate-500 hover:text-slate-300"
                          >
                            <Copy className="w-3 h-3" />
                          </button>
                        </div>
                      ) : (
                        <span className="text-slate-600 italic">Not set</span>
                      )}
                    </td>
                    <td className="px-6 py-4">
                      <span
                        className={`text-[10px] uppercase font-bold px-2 py-0.5 rounded border inline-flex items-center gap-1 ${
                          p.permission === 'operator'
                            ? 'bg-amber-500/10 text-amber-400 border-amber-500/30'
                            : p.permission === 'member'
                            ? 'bg-purple-500/10 text-purple-400 border-purple-500/30'
                            : p.permission === 'visitor'
                            ? 'bg-slate-500/10 text-slate-400 border-slate-500/30'
                            : 'bg-obsidian-950 text-slate-500 border-obsidian-800'
                        }`}
                      >
                        {p.permission === 'operator' && <Crown className="w-3 h-3" />}
                        <span>{p.permission || 'none'}</span>
                      </span>
                    </td>
                    <td className="px-6 py-4">
                      {p.is_allowlisted ? (
                        <span className="text-emerald-400 bg-emerald-500/10 border border-emerald-500/30 px-2 py-0.5 rounded text-[10px] font-bold uppercase inline-flex items-center gap-1">
                          <Check className="w-3 h-3" />
                          <span>Allowlisted {p.ignores_player_limit && '(Limit Bypass)'}</span>
                        </span>
                      ) : (
                        <span className="text-slate-500 bg-obsidian-950 border border-obsidian-800 px-2 py-0.5 rounded text-[10px]">
                          Not Allowlisted
                        </span>
                      )}
                    </td>
                    <td className="px-6 py-4 text-right">
                      <button
                        onClick={() => handleDeletePlayer(p.id, p.name)}
                        title="Remove from Global Access List"
                        className="p-1.5 rounded bg-rose-600/20 text-rose-400 hover:bg-rose-600 hover:text-slate-950 transition-colors"
                      >
                        <Trash2 className="w-3.5 h-3.5" />
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <Pagination
            currentPage={currentPage}
            totalItems={totalItems}
            pageSize={pageSize}
            onPageChange={setCurrentPage}
            onPageSizeChange={setPageSize}
            pageSizeOptions={[10, 25, 50, 100]}
          />
        </div>
      )}

      {/* Add Global Player Modal */}
      {showAddModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-obsidian-950/80 backdrop-blur-sm">
          <div className="bg-obsidian-900 border border-obsidian-700/80 rounded-xl max-w-md w-full p-6 shadow-2xl space-y-4 animate-scaleUp">
            <div className="flex items-center justify-between border-b border-obsidian-800 pb-3">
              <h3 className="font-mono text-base font-bold text-slate-100 flex items-center gap-2">
                <UserPlus className="w-4 h-4 text-emerald-400" />
                <span>Add Global Player</span>
              </h3>
              <button
                onClick={() => setShowAddModal(false)}
                className="text-slate-400 hover:text-slate-200 text-sm font-mono"
              >
                ✕
              </button>
            </div>

            <form onSubmit={handleAddPlayer} className="space-y-4 font-mono text-xs">
              <div>
                <label className="block text-slate-400 mb-1">Xbox Gamertag *</label>
                <input
                  type="text"
                  required
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="e.g. MasterMiner99"
                  className="w-full px-3 py-2 rounded bg-obsidian-950 border border-obsidian-700 text-slate-100 text-xs focus:border-emerald-500 focus:outline-none"
                />
              </div>

              <div>
                <label className="block text-slate-400 mb-1">Xbox User ID (XUID)</label>
                <input
                  type="text"
                  value={xuid}
                  onChange={(e) => setXuid(e.target.value)}
                  placeholder="e.g. 2535412345678901 (optional, required for offline op)"
                  className="w-full px-3 py-2 rounded bg-obsidian-950 border border-obsidian-700 text-slate-100 text-xs focus:border-emerald-500 focus:outline-none"
                />
                <p className="text-[10px] text-slate-500 mt-1">
                  If XUID is blank, Bedrock Dedicated Server can resolve it automatically upon the player's next join.
                </p>
              </div>

              <div>
                <label className="block text-slate-400 mb-1">Permission Role</label>
                <select
                  value={permission}
                  onChange={(e) => setPermission(e.target.value as any)}
                  className="w-full px-3 py-2 rounded bg-obsidian-950 border border-obsidian-700 text-slate-100 text-xs focus:border-emerald-500 focus:outline-none"
                >
                  <option value="operator">Operator (Op with admin commands)</option>
                  <option value="member">Member (Standard player)</option>
                  <option value="visitor">Visitor (Read-only view)</option>
                  <option value="none">None (Default role)</option>
                </select>
              </div>

              <div className="space-y-2 pt-1">
                <label className="flex items-center space-x-2 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={isAllowlisted}
                    onChange={(e) => setIsAllowlisted(e.target.checked)}
                    className="rounded bg-obsidian-950 border-obsidian-700 text-emerald-500 focus:ring-emerald-500"
                  />
                  <span className="text-slate-300">Add to Global Allowlist (allowlist.json)</span>
                </label>

                <label className="flex items-center space-x-2 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={ignoresLimit}
                    onChange={(e) => setIgnoresLimit(e.target.checked)}
                    className="rounded bg-obsidian-950 border-obsidian-700 text-emerald-500 focus:ring-emerald-500"
                  />
                  <span className="text-slate-300">Bypass Server Max Player Limit (ignoresPlayerLimit)</span>
                </label>
              </div>

              <div className="pt-3 border-t border-obsidian-800 flex justify-end space-x-2">
                <button
                  type="button"
                  onClick={() => setShowAddModal(false)}
                  className="px-4 py-2 rounded-lg bg-obsidian-800 hover:bg-obsidian-750 text-slate-300 text-xs font-bold"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={actionLoading || !name.trim()}
                  className="px-4 py-2 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-slate-950 text-xs font-bold flex items-center space-x-1.5 disabled:opacity-50"
                >
                  {actionLoading ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <Check className="w-3.5 h-3.5" />}
                  <span>Save Global Player</span>
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
};
