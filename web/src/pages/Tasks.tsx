import React, { useState, useEffect } from 'react';
import { Clock, Play, Trash2, Plus, Server, AlertCircle, RefreshCw, Pencil } from 'lucide-react';
import { api } from '../api/client';
import { Task, Server as ServerType } from '../types';
import { Pagination } from '../components/Pagination';
import { usePagination } from '../hooks/usePagination';

export const Tasks: React.FC = () => {
  const [tasks, setTasks] = useState<Task[]>([]);
  const [servers, setServers] = useState<ServerType[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [showCreateModal, setShowCreateModal] = useState(false);

  const {
    currentPage,
    pageSize,
    totalItems,
    paginatedItems: paginatedTasks,
    setCurrentPage,
    setPageSize,
  } = usePagination(tasks, 10);

  // Form state
  const [name, setName] = useState('');
  const [serverId, setServerId] = useState('');
  const [cronExpr, setCronExpr] = useState('0 4 * * *');
  const [action, setAction] = useState('backup');
  const [payload, setPayload] = useState('');

  // Edit task state
  const [editingTask, setEditingTask] = useState<Task | null>(null);
  const [editName, setEditName] = useState('');
  const [editServerId, setEditServerId] = useState('');
  const [editCronExpr, setEditCronExpr] = useState('');
  const [editAction, setEditAction] = useState('backup');
  const [editPayload, setEditPayload] = useState('');

  const openEditModal = (t: Task) => {
    setEditingTask(t);
    setEditName(t.name);
    setEditServerId(t.server_id || '');
    setEditCronExpr(t.cron_expr);
    setEditAction(t.action);
    setEditPayload(t.payload || '');
  };

  const handleUpdateTask = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!editingTask) return;
    try {
      await api.updateTask(editingTask.id, {
        name: editName,
        server_id: editServerId || undefined,
        cron_expr: editCronExpr,
        action: editAction,
        payload: editPayload,
      });
      setEditingTask(null);
      loadData();
    } catch (err: any) {
      alert(err.message || 'Failed to update task');
    }
  };

  const loadData = async () => {
    try {
      setLoading(true);
      const [tList, sList] = await Promise.all([
        api.listTasks(),
        api.listServers(),
      ]);
      setTasks(Array.isArray(tList) ? tList : []);
      setServers(Array.isArray(sList) ? sList : []);
      if ((sList || []).length > 0 && !serverId) {
        setServerId(sList[0].id);
      }
    } catch (err: any) {
      setError(err.message || 'Failed to load tasks');
      setTasks([]);
      setServers([]);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadData();
  }, []);

  const handleCreateTask = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      await api.createTask({
        name,
        server_id: serverId || undefined,
        cron_expr: cronExpr,
        action,
        payload,
      });
      setShowCreateModal(false);
      setName('');
      setPayload('');
      loadData();
    } catch (err: any) {
      alert(err.message || 'Failed to create task');
    }
  };

  const handleToggle = async (id: number) => {
    try {
      await api.toggleTask(id);
      loadData();
    } catch (err: any) {
      alert(err.message || 'Failed to toggle task');
    }
  };

  const handleDelete = async (id: number) => {
    if (!confirm('Are you sure you want to delete this scheduled task?')) return;
    try {
      await api.deleteTask(id);
      loadData();
    } catch (err: any) {
      alert(err.message || 'Failed to delete task');
    }
  };

  const handleRunNow = async (id: number) => {
    try {
      await api.runTaskNow(id);
      alert('Task triggered successfully in the background');
      setTimeout(loadData, 1000);
    } catch (err: any) {
      alert(err.message || 'Failed to execute task');
    }
  };

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
      <div className="flex items-center justify-between mb-8 pb-4 border-b border-obsidian-800">
        <div>
          <h1 className="text-2xl font-mono font-bold text-slate-100 flex items-center gap-3">
            <Clock className="w-6 h-6 text-emerald-400" />
            <span>Automated Task Scheduler</span>
          </h1>
          <p className="text-sm font-mono text-slate-400 mt-1">
            Configure automated cron tasks for hot backups, server restarts with chat countdowns, and timed commands.
          </p>
        </div>

        <div className="flex items-center space-x-3">
          <button
            onClick={loadData}
            className="p-2 rounded-lg bg-obsidian-900 border border-obsidian-700 text-slate-400 hover:text-slate-200"
            title="Refresh"
          >
            <RefreshCw className="w-4 h-4" />
          </button>
          <button
            onClick={() => setShowCreateModal(true)}
            className="px-4 py-2 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-slate-950 font-mono text-sm font-bold flex items-center space-x-2"
          >
            <Plus className="w-4 h-4" />
            <span>New Task</span>
          </button>
        </div>
      </div>

      {error && (
        <div className="mb-6 p-4 rounded-lg bg-rose-950/40 border border-rose-800 text-rose-300 flex items-center space-x-3 text-sm font-mono">
          <AlertCircle className="w-5 h-5 flex-shrink-0" />
          <span>{error}</span>
        </div>
      )}

      {loading ? (
        <div className="text-center py-12 text-slate-400 font-mono text-sm">
          Loading scheduled tasks...
        </div>
      ) : tasks.length === 0 ? (
        <div className="text-center py-16 bg-obsidian-900 border border-obsidian-800 rounded-xl">
          <Clock className="w-12 h-12 text-slate-600 mx-auto mb-3" />
          <h3 className="font-mono text-base font-bold text-slate-200">No Automated Tasks Configured</h3>
          <p className="text-xs font-mono text-slate-400 mt-1 max-w-sm mx-auto">
            Schedule recurring routines like 4:00 AM daily hot backups or graceful server restarts.
          </p>
          <button
            onClick={() => setShowCreateModal(true)}
            className="mt-4 px-4 py-2 rounded-lg bg-emerald-600/20 hover:bg-emerald-600/30 border border-emerald-500/40 text-emerald-400 font-mono text-xs font-bold"
          >
            Create First Task
          </button>
        </div>
      ) : (
        <div className="bg-obsidian-900 border border-obsidian-700/80 rounded-xl overflow-hidden shadow-xl">
          <table className="w-full text-left font-mono text-xs">
            <thead className="bg-obsidian-950/80 text-slate-400 border-b border-obsidian-800 uppercase tracking-wider">
              <tr>
                <th className="px-6 py-4">Status</th>
                <th className="px-6 py-4">Task Name</th>
                <th className="px-6 py-4">Target Server</th>
                <th className="px-6 py-4">Cron Schedule</th>
                <th className="px-6 py-4">Action</th>
                <th className="px-6 py-4">Next Run</th>
                <th className="px-6 py-4 text-right">Controls</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-obsidian-800">
              {paginatedTasks.map((task) => {
                const srv = (servers || []).find((s) => s.id === task.server_id);
                return (
                  <tr key={task.id} className="hover:bg-obsidian-850/50 transition-colors">
                    <td className="px-6 py-4">
                      <button
                        onClick={() => handleToggle(task.id)}
                        className={`inline-flex items-center px-2 py-0.5 rounded text-[11px] font-bold border transition-colors ${
                          task.enabled
                            ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/30 hover:bg-emerald-500/20'
                            : 'bg-slate-800 text-slate-400 border-slate-700 hover:bg-slate-700'
                        }`}
                      >
                        {task.enabled ? 'ENABLED' : 'PAUSED'}
                      </button>
                    </td>
                    <td className="px-6 py-4 font-bold text-slate-200">
                      {task.name}
                    </td>
                    <td className="px-6 py-4 text-slate-300">
                      {srv ? (
                        <div className="flex items-center space-x-1.5">
                          <Server className="w-3.5 h-3.5 text-emerald-400" />
                          <span>{srv.name}</span>
                        </div>
                      ) : (
                        <span className="text-slate-500">Global</span>
                      )}
                    </td>
                    <td className="px-6 py-4">
                      <span className="px-2 py-1 bg-obsidian-950 rounded border border-obsidian-700 text-emerald-300">
                        {task.cron_expr}
                      </span>
                    </td>
                    <td className="px-6 py-4 capitalize text-slate-300">
                      <span className="px-2 py-0.5 rounded bg-obsidian-800 border border-obsidian-700">
                        {task.action}
                      </span>
                    </td>
                    <td className="px-6 py-4 text-slate-400">
                      {task.next_run ? new Date(task.next_run).toLocaleString() : 'Not scheduled'}
                    </td>
                    <td className="px-6 py-4 text-right space-x-2">
                      <button
                        onClick={() => handleRunNow(task.id)}
                        title="Run Immediately"
                        className="p-1.5 rounded bg-emerald-600/20 text-emerald-400 hover:bg-emerald-600 hover:text-slate-950 transition-colors"
                      >
                        <Play className="w-3.5 h-3.5" />
                      </button>
                      <button
                        onClick={() => openEditModal(task)}
                        title="Edit Task"
                        className="p-1.5 rounded bg-obsidian-800 text-slate-300 hover:bg-emerald-600 hover:text-slate-950 transition-colors"
                      >
                        <Pencil className="w-3.5 h-3.5" />
                      </button>
                      <button
                        onClick={() => handleDelete(task.id)}
                        title="Delete Task"
                        className="p-1.5 rounded bg-rose-600/20 text-rose-400 hover:bg-rose-600 hover:text-slate-950 transition-colors"
                      >
                        <Trash2 className="w-3.5 h-3.5" />
                      </button>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
          <Pagination
            currentPage={currentPage}
            totalItems={totalItems}
            pageSize={pageSize}
            onPageChange={setCurrentPage}
            onPageSizeChange={setPageSize}
            pageSizeOptions={[5, 10, 20, 50]}
          />
        </div>
      )}

      {/* CREATE TASK MODAL */}
      {showCreateModal && (
        <div className="fixed inset-0 bg-black/80 backdrop-blur-sm z-50 flex items-center justify-center p-4">
          <div className="bg-obsidian-900 border border-obsidian-700 rounded-xl max-w-md w-full p-6 shadow-2xl">
            <h3 className="text-lg font-mono font-bold text-slate-100 mb-4 flex items-center gap-2">
              <Clock className="w-5 h-5 text-emerald-400" />
              <span>Create Scheduled Task</span>
            </h3>

            <form onSubmit={handleCreateTask} className="space-y-4 font-mono text-xs">
              <div>
                <label className="block text-slate-300 mb-1 font-bold">Task Name</label>
                <input
                  type="text"
                  required
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="e.g. Daily Hot Backup"
                  className="w-full px-3 py-2 bg-obsidian-950 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
                />
              </div>

              <div>
                <label className="block text-slate-300 mb-1 font-bold">Target Server</label>
                <select
                  value={serverId}
                  onChange={(e) => setServerId(e.target.value)}
                  className="w-full px-3 py-2 bg-obsidian-950 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
                >
                  <option value="">Global (All Servers / System)</option>
                  {(servers || []).map((s) => (
                    <option key={s.id} value={s.id}>
                      {s.name} ({s.port})
                    </option>
                  ))}
                </select>
              </div>

              <div>
                <label className="block text-slate-300 mb-1 font-bold">Action</label>
                <select
                  value={action}
                  onChange={(e) => setAction(e.target.value)}
                  className="w-full px-3 py-2 bg-obsidian-950 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
                >
                  <option value="backup">Zero-Downtime Hot Backup</option>
                  <option value="restart">Graceful Restart (with player countdown)</option>
                  <option value="command">Console Command</option>
                </select>
              </div>

              {action === 'command' && (
                <div>
                  <label className="block text-slate-300 mb-1 font-bold">Console Command</label>
                  <input
                    type="text"
                    required
                    value={payload}
                    onChange={(e) => setPayload(e.target.value)}
                    placeholder="e.g. say [ALERT] Daily cleaning in progress"
                    className="w-full px-3 py-2 bg-obsidian-950 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
                  />
                </div>
              )}

              <div>
                <label className="block text-slate-300 mb-1 font-bold">Cron Expression</label>
                <input
                  type="text"
                  required
                  value={cronExpr}
                  onChange={(e) => setCronExpr(e.target.value)}
                  placeholder="0 4 * * *"
                  className="w-full px-3 py-2 bg-obsidian-950 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
                />
                <div className="flex gap-2 mt-1.5 text-[10px] text-slate-400">
                  <button
                    type="button"
                    onClick={() => setCronExpr('0 * * * *')}
                    className="hover:text-emerald-400"
                  >
                    Every hour
                  </button>
                  <span>•</span>
                  <button
                    type="button"
                    onClick={() => setCronExpr('0 4 * * *')}
                    className="hover:text-emerald-400"
                  >
                    Daily at 4 AM
                  </button>
                  <span>•</span>
                  <button
                    type="button"
                    onClick={() => setCronExpr('0 0 * * 0')}
                    className="hover:text-emerald-400"
                  >
                    Weekly on Sun
                  </button>
                </div>
              </div>

              <div className="flex justify-end space-x-3 pt-4 border-t border-obsidian-800">
                <button
                  type="button"
                  onClick={() => setShowCreateModal(false)}
                  className="px-4 py-2 rounded-lg bg-obsidian-800 text-slate-300 hover:bg-obsidian-700"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  className="px-4 py-2 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-slate-950 font-bold"
                >
                  Schedule Task
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* EDIT TASK MODAL */}
      {editingTask && (
        <div className="fixed inset-0 bg-black/80 backdrop-blur-sm z-50 flex items-center justify-center p-4">
          <div className="bg-obsidian-900 border border-obsidian-700 rounded-xl max-w-md w-full p-6 shadow-2xl">
            <h3 className="text-lg font-mono font-bold text-slate-100 mb-4 flex items-center gap-2">
              <Pencil className="w-5 h-5 text-emerald-400" />
              <span>Edit Scheduled Task</span>
            </h3>

            <form onSubmit={handleUpdateTask} className="space-y-4 font-mono text-xs">
              <div>
                <label className="block text-slate-300 mb-1 font-bold">Task Name</label>
                <input
                  type="text"
                  required
                  value={editName}
                  onChange={(e) => setEditName(e.target.value)}
                  className="w-full px-3 py-2 bg-obsidian-950 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
                />
              </div>

              <div>
                <label className="block text-slate-300 mb-1 font-bold">Target Server</label>
                <select
                  value={editServerId}
                  onChange={(e) => setEditServerId(e.target.value)}
                  className="w-full px-3 py-2 bg-obsidian-950 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
                >
                  <option value="">Global (All Servers / System)</option>
                  {(servers || []).map((s) => (
                    <option key={s.id} value={s.id}>
                      {s.name} ({s.port})
                    </option>
                  ))}
                </select>
              </div>

              <div>
                <label className="block text-slate-300 mb-1 font-bold">Action</label>
                <select
                  value={editAction}
                  onChange={(e) => setEditAction(e.target.value)}
                  className="w-full px-3 py-2 bg-obsidian-950 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
                >
                  <option value="backup">Zero-Downtime Hot Backup</option>
                  <option value="restart">Graceful Restart (with player countdown)</option>
                  <option value="command">Console Command</option>
                </select>
              </div>

              {editAction === 'command' && (
                <div>
                  <label className="block text-slate-300 mb-1 font-bold">Console Command</label>
                  <input
                    type="text"
                    required
                    value={editPayload}
                    onChange={(e) => setEditPayload(e.target.value)}
                    placeholder="e.g. say [ALERT] Daily cleaning in progress"
                    className="w-full px-3 py-2 bg-obsidian-950 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
                  />
                </div>
              )}

              <div>
                <label className="block text-slate-300 mb-1 font-bold">Cron Expression</label>
                <input
                  type="text"
                  required
                  value={editCronExpr}
                  onChange={(e) => setEditCronExpr(e.target.value)}
                  className="w-full px-3 py-2 bg-obsidian-950 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
                />
              </div>

              <div className="flex justify-end space-x-3 pt-4 border-t border-obsidian-800">
                <button
                  type="button"
                  onClick={() => setEditingTask(null)}
                  className="px-4 py-2 rounded-lg bg-obsidian-800 text-slate-300 hover:bg-obsidian-700"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  className="px-4 py-2 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-slate-950 font-bold"
                >
                  Save Changes
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
};
