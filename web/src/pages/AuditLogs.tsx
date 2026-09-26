import React, { useState, useEffect } from 'react';
import { History, Search, Download, RefreshCw, AlertCircle, Shield } from 'lucide-react';
import { api } from '../api/client';
import { AuditLog } from '../types';
import { Pagination } from '../components/Pagination';
import { usePagination } from '../hooks/usePagination';

export const AuditLogs: React.FC = () => {
  const [logs, setLogs] = useState<AuditLog[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [filterAction, setFilterAction] = useState('');
  const [search, setSearch] = useState('');

  const loadLogs = async () => {
    try {
      setLoading(true);
      const data = await api.listAuditLogs(500, 0);
      setLogs(data);
    } catch (err: any) {
      setError(err.message || 'Failed to load audit logs');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadLogs();
  }, []);

  const filteredLogs = logs.filter((log) => {
    if (filterAction && log.action !== filterAction) return false;
    if (search) {
      const q = search.toLowerCase();
      return (
        log.actor_name.toLowerCase().includes(q) ||
        log.action.toLowerCase().includes(q) ||
        log.target.toLowerCase().includes(q) ||
        log.client_ip.toLowerCase().includes(q) ||
        log.details.toLowerCase().includes(q)
      );
    }
    return true;
  });

  const {
    currentPage,
    pageSize,
    totalItems,
    paginatedItems: paginatedLogs,
    setCurrentPage,
    setPageSize,
  } = usePagination(filteredLogs, 25);

  const exportJSON = () => {
    const blob = new Blob([JSON.stringify(filteredLogs, null, 2)], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `bsm_audit_logs_${new Date().toISOString().slice(0, 10)}.json`;
    a.click();
    URL.revokeObjectURL(url);
  };

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
      <div className="flex items-center justify-between mb-8 pb-4 border-b border-obsidian-800">
        <div>
          <h1 className="text-2xl font-mono font-bold text-slate-100 flex items-center gap-3">
            <History className="w-6 h-6 text-emerald-400" />
            <span>System Audit Logs</span>
          </h1>
          <p className="text-sm font-mono text-slate-400 mt-1">
            Immutable tracking of administrator actions, port gate grants, roaming handoffs, and system automations.
          </p>
        </div>

        <div className="flex items-center space-x-3">
          <button
            onClick={loadLogs}
            className="p-2 rounded-lg bg-obsidian-900 border border-obsidian-700 text-slate-400 hover:text-slate-200"
            title="Refresh"
          >
            <RefreshCw className="w-4 h-4" />
          </button>
          <button
            onClick={exportJSON}
            className="px-4 py-2 rounded-lg bg-obsidian-900 hover:bg-obsidian-800 border border-obsidian-700 text-slate-200 font-mono text-xs font-bold flex items-center space-x-2"
          >
            <Download className="w-4 h-4 text-emerald-400" />
            <span>Export JSON</span>
          </button>
        </div>
      </div>

      {error && (
        <div className="mb-6 p-4 rounded-lg bg-rose-950/40 border border-rose-800 text-rose-300 flex items-center space-x-3 text-sm font-mono">
          <AlertCircle className="w-5 h-5 flex-shrink-0" />
          <span>{error}</span>
        </div>
      )}

      {/* Filter Bar */}
      <div className="mb-6 flex flex-wrap gap-4 items-center justify-between">
        <div className="flex-1 min-w-[240px] relative">
          <Search className="w-4 h-4 text-slate-500 absolute left-3 top-2.5" />
          <input
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search by actor, target, IP, or details..."
            className="w-full pl-9 pr-4 py-2 bg-obsidian-900 border border-obsidian-700 rounded-lg text-xs font-mono text-slate-200 focus:outline-none focus:border-emerald-500"
          />
        </div>

        <div className="flex items-center space-x-3 font-mono text-xs">
          <select
            value={filterAction}
            onChange={(e) => setFilterAction(e.target.value)}
            className="px-3 py-2 bg-obsidian-900 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
          >
            <option value="">All Actions</option>
            <option value="login_success">login_success</option>
            <option value="login_failure">login_failure</option>
            <option value="knock_roaming_handoff">knock_roaming_handoff</option>
            <option value="knock_lease_expired">knock_lease_expired</option>
            <option value="knock_lease_revoked">knock_lease_revoked</option>
            <option value="backup_created">backup_created</option>
            <option value="backup_pruned">backup_pruned</option>
            <option value="task_executed">task_executed</option>
          </select>
        </div>
      </div>

      {loading ? (
        <div className="text-center py-12 text-slate-400 font-mono text-sm">
          Loading audit events...
        </div>
      ) : filteredLogs.length === 0 ? (
        <div className="text-center py-16 bg-obsidian-900 border border-obsidian-800 rounded-xl">
          <Shield className="w-12 h-12 text-slate-600 mx-auto mb-3" />
          <h3 className="font-mono text-base font-bold text-slate-200">No Audit Events Found</h3>
          <p className="text-xs font-mono text-slate-400 mt-1">
            Actions will appear here as users authenticate and interact with servers.
          </p>
        </div>
      ) : (
        <div className="bg-obsidian-900 border border-obsidian-700/80 rounded-xl overflow-hidden shadow-xl">
          <div className="overflow-x-auto">
            <table className="w-full text-left font-mono text-xs">
              <thead className="bg-obsidian-950/80 text-slate-400 border-b border-obsidian-800 uppercase tracking-wider">
                <tr>
                  <th className="px-6 py-3">Timestamp</th>
                  <th className="px-6 py-3">Actor</th>
                  <th className="px-6 py-3">Action</th>
                  <th className="px-6 py-3">Target</th>
                  <th className="px-6 py-3">Client IP</th>
                  <th className="px-6 py-3">Details</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-obsidian-800">
                {paginatedLogs.map((log) => (
                  <tr key={log.id} className="hover:bg-obsidian-850/50 transition-colors">
                    <td className="px-6 py-3 text-slate-400 whitespace-nowrap">
                      {new Date(log.timestamp).toLocaleString()}
                    </td>
                    <td className="px-6 py-3">
                      <span className="font-bold text-slate-200">{log.actor_name || 'Anonymous'}</span>
                      <span className="text-[10px] text-slate-500 ml-1.5 uppercase">({log.actor_type})</span>
                    </td>
                    <td className="px-6 py-3">
                      <span className="px-2 py-0.5 rounded bg-obsidian-950 border border-obsidian-700 text-emerald-400">
                        {log.action}
                      </span>
                    </td>
                    <td className="px-6 py-3 text-slate-300">
                      {log.target || '—'}
                    </td>
                    <td className="px-6 py-3 text-slate-400">
                      {log.client_ip || '—'}
                    </td>
                    <td className="px-6 py-3 text-slate-400 max-w-xs truncate" title={log.details}>
                      {log.details}
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
    </div>
  );
};
