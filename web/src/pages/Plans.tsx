import React, { useEffect, useState } from 'react';
import { api } from '../api/client';
import { Plan } from '../types';
import { 
  Layers, Plus, Edit2, Trash2, CheckCircle, 
  HardDrive, Users, Clock, AlertTriangle, X, Globe
} from 'lucide-react';

export const Plans: React.FC = () => {
  const [plans, setPlans] = useState<Plan[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [modalOpen, setModalOpen] = useState(false);
  const [editingPlan, setEditingPlan] = useState<Plan | null>(null);
  const [actionLoading, setActionLoading] = useState(false);

  // Form State
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [billingInterval, setBillingInterval] = useState('permanent');
  const [maxServers, setMaxServers] = useState(1);
  const [maxMemory, setMaxMemory] = useState('2G');
  const [maxCPU, setMaxCPU] = useState(2.0);
  const [maxBackups, setMaxBackups] = useState(3);
  const [maxDiskMB, setMaxDiskMB] = useState(5120);
  const [maxPlayerSlots, setMaxPlayerSlots] = useState(10);
  const [maxCollaborators, setMaxCollaborators] = useState(0);
  const [idleTimeoutMinutes, setIdleTimeoutMinutes] = useState(0);
  const [allowCustomSeed, setAllowCustomSeed] = useState(true);
  const [allowCustomPort, setAllowCustomPort] = useState(false);
  const [allowPreviewVersions, setAllowPreviewVersions] = useState(false);
  const [allowAddons, setAllowAddons] = useState(true);
  const [allowPortGateKeys, setAllowPortGateKeys] = useState(true);
  const [allowTasks, setAllowTasks] = useState(false);

  const fetchPlans = async () => {
    try {
      setLoading(true);
      setError(null);
      const data = await api.listPlans();
      setPlans(data);
    } catch (err: any) {
      setError(err.message || 'Failed to load plans');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchPlans();
  }, []);

  const openCreateModal = () => {
    setEditingPlan(null);
    setName('');
    setDescription('');
    setBillingInterval('permanent');
    setMaxServers(1);
    setMaxMemory('2G');
    setMaxCPU(2.0);
    setMaxBackups(3);
    setMaxDiskMB(5120);
    setMaxPlayerSlots(10);
    setMaxCollaborators(0);
    setIdleTimeoutMinutes(0);
    setAllowCustomSeed(true);
    setAllowCustomPort(false);
    setAllowPreviewVersions(false);
    setAllowAddons(true);
    setAllowPortGateKeys(true);
    setAllowTasks(false);
    setModalOpen(true);
  };

  const openEditModal = (p: Plan) => {
    setEditingPlan(p);
    setName(p.name);
    setDescription(p.description);
    setBillingInterval(p.billing_interval || 'permanent');
    setMaxServers(p.max_servers);
    setMaxMemory(p.max_memory);
    setMaxCPU(p.max_cpu);
    setMaxBackups(p.max_backups_per_server);
    setMaxDiskMB(p.max_disk_mb);
    setMaxPlayerSlots(p.max_player_slots);
    setMaxCollaborators(p.max_collaborators);
    setIdleTimeoutMinutes(p.idle_timeout_minutes);
    setAllowCustomSeed(p.allow_custom_seed);
    setAllowCustomPort(p.allow_custom_port);
    setAllowPreviewVersions(p.allow_preview_versions);
    setAllowAddons(p.allow_addons);
    setAllowPortGateKeys(p.allow_port_gate_keys);
    setAllowTasks(p.allow_tasks);
    setModalOpen(true);
  };

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim()) return;

    try {
      setActionLoading(true);
      const payload: Partial<Plan> = {
        name: name.trim(),
        description: description.trim(),
        billing_interval: billingInterval,
        max_servers: Number(maxServers),
        max_memory: maxMemory,
        max_cpu: Number(maxCPU),
        max_backups_per_server: Number(maxBackups),
        max_disk_mb: Number(maxDiskMB),
        max_player_slots: Number(maxPlayerSlots),
        max_collaborators: Number(maxCollaborators),
        idle_timeout_minutes: Number(idleTimeoutMinutes),
        allow_custom_seed: allowCustomSeed,
        allow_custom_port: allowCustomPort,
        allow_preview_versions: allowPreviewVersions,
        allow_addons: allowAddons,
        allow_port_gate_keys: allowPortGateKeys,
        allow_tasks: allowTasks,
      };

      if (editingPlan) {
        await api.updatePlan(editingPlan.id, payload);
      } else {
        await api.createPlan(payload);
      }

      setModalOpen(false);
      await fetchPlans();
    } catch (err: any) {
      alert(err.message || 'Operation failed');
    } finally {
      setActionLoading(false);
    }
  };

  const handleSetDefault = async (p: Plan) => {
    if (p.is_default) return;
    try {
      await api.setDefaultPlan(p.id);
      await fetchPlans();
    } catch (err: any) {
      alert(err.message || 'Failed to set default plan');
    }
  };

  const handleDelete = async (p: Plan) => {
    if (p.is_default) {
      alert('Cannot delete the default plan. Please set another plan as default first.');
      return;
    }
    if ((p.user_count || 0) > 0) {
      alert(`Cannot delete plan: ${p.user_count} active user(s) are currently assigned to it. Please reassign their plans first in Users management.`);
      return;
    }
    if (!confirm(`Are you sure you want to permanently delete plan "${p.name}"?`)) return;

    try {
      await api.deletePlan(p.id);
      await fetchPlans();
    } catch (err: any) {
      alert(err.message || 'Failed to delete plan');
    }
  };

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8 space-y-6">
      {/* Top Banner */}
      <div className="flex flex-col md:flex-row md:items-center md:justify-between gap-4 border-b border-obsidian-800 pb-5">
        <div>
          <div className="flex items-center space-x-2">
            <Layers className="w-7 h-7 text-emerald-400" />
            <h1 className="text-2xl font-bold text-slate-100 tracking-tight">User Deployment Plans & Limits</h1>
          </div>
          <p className="text-sm text-slate-400 mt-1">
            Configure resource quotas, computing capacities, and feature permissions for self-service server deployment.
          </p>
        </div>

        <button
          onClick={openCreateModal}
          className="flex items-center space-x-2 px-4 py-2 bg-emerald-600 hover:bg-emerald-500 text-white rounded-lg font-medium shadow-md shadow-emerald-950/40 transition-all self-start md:self-auto"
        >
          <Plus className="w-4 h-4" />
          <span>New Plan</span>
        </button>
      </div>

      {error && (
        <div className="p-4 rounded-lg bg-rose-950/40 border border-rose-800/60 text-rose-300 text-sm flex items-center gap-3">
          <AlertTriangle className="w-5 h-5 flex-shrink-0 text-rose-400" />
          <span>{error}</span>
        </div>
      )}

      {/* Plans Grid */}
      {loading ? (
        <div className="p-12 text-center text-slate-500">Loading plans...</div>
      ) : plans.length === 0 ? (
        <div className="p-12 text-center text-slate-500 border border-dashed border-obsidian-800 rounded-xl">
          No plans configured. Click "New Plan" to create one.
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {plans.map((p) => (
            <div
              key={p.id}
              className={`bg-obsidian-900 border rounded-xl p-5 flex flex-col justify-between transition-all ${
                p.is_default
                  ? 'border-emerald-500/50 shadow-[0_0_20px_rgba(16,185,129,0.08)]'
                  : 'border-obsidian-800 hover:border-obsidian-700'
              }`}
            >
              <div>
                {/* Header */}
                <div className="flex items-start justify-between gap-3 mb-3">
                  <div>
                    <div className="flex items-center gap-2">
                      <h3 className="text-lg font-bold text-slate-100">{p.name}</h3>
                      {p.is_default && (
                        <span className="text-[10px] font-semibold bg-emerald-500/20 text-emerald-400 border border-emerald-500/40 px-2 py-0.5 rounded-full uppercase tracking-wider">
                          Default
                        </span>
                      )}
                    </div>
                    <p className="text-xs text-slate-400 mt-1 line-clamp-2">{p.description || 'No description provided'}</p>
                  </div>

                  <div className="flex items-center space-x-1">
                    <button
                      onClick={() => openEditModal(p)}
                      title="Edit Plan"
                      className="p-1.5 rounded-lg text-slate-400 hover:text-slate-200 hover:bg-obsidian-800 transition-colors"
                    >
                      <Edit2 className="w-4 h-4" />
                    </button>
                    {!p.is_default && (
                      <button
                        onClick={() => handleDelete(p)}
                        title="Delete Plan"
                        className="p-1.5 rounded-lg text-slate-400 hover:text-rose-400 hover:bg-obsidian-800 transition-colors"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    )}
                  </div>
                </div>

                {/* Quotas & Specs */}
                <div className="grid grid-cols-2 gap-2 my-4 text-xs font-mono">
                  <div className="bg-obsidian-950 border border-obsidian-800/80 rounded-lg p-2.5">
                    <span className="text-slate-400 block text-[10px] uppercase font-sans">Server Limit</span>
                    <span className="text-emerald-400 font-bold text-sm">{p.max_servers}</span> Server(s)
                  </div>
                  <div className="bg-obsidian-950 border border-obsidian-800/80 rounded-lg p-2.5">
                    <span className="text-slate-400 block text-[10px] uppercase font-sans">Memory Limit</span>
                    <span className="text-sky-400 font-bold text-sm">{p.max_memory}</span> RAM
                  </div>
                  <div className="bg-obsidian-950 border border-obsidian-800/80 rounded-lg p-2.5">
                    <span className="text-slate-400 block text-[10px] uppercase font-sans">CPU Limit</span>
                    <span className="text-indigo-400 font-bold text-sm">{p.max_cpu}</span> Cores
                  </div>
                  <div className="bg-obsidian-950 border border-obsidian-800/80 rounded-lg p-2.5">
                    <span className="text-slate-400 block text-[10px] uppercase font-sans">Max Backups</span>
                    <span className="text-amber-400 font-bold text-sm">{p.max_backups_per_server}</span> Archives
                  </div>
                </div>

                {/* Feature Tags */}
                <div className="space-y-1.5 pt-2 border-t border-obsidian-800/80 text-xs">
                  <div className="flex items-center justify-between text-slate-300">
                    <span className="flex items-center gap-1.5 text-slate-400">
                      <Users className="w-3.5 h-3.5" /> Player Slots
                    </span>
                    <span className="font-mono text-slate-200">{p.max_player_slots}</span>
                  </div>
                  <div className="flex items-center justify-between text-slate-300">
                    <span className="flex items-center gap-1.5 text-slate-400">
                      <HardDrive className="w-3.5 h-3.5" /> Disk Quota
                    </span>
                    <span className="font-mono text-slate-200">{p.max_disk_mb > 0 ? `${(p.max_disk_mb / 1024).toFixed(1)} GB` : 'Unlimited'}</span>
                  </div>
                  <div className="flex items-center justify-between text-slate-300">
                    <span className="flex items-center gap-1.5 text-slate-400">
                      <Clock className="w-3.5 h-3.5" /> Inactivity Stop
                    </span>
                    <span className="font-mono text-slate-200">{p.idle_timeout_minutes > 0 ? `${p.idle_timeout_minutes}m` : '24/7 Runtime'}</span>
                  </div>
                  <div className="flex items-center justify-between text-slate-300">
                    <span className="flex items-center gap-1.5 text-slate-400">
                      <Globe className="w-3.5 h-3.5" /> Custom UDP Ports
                    </span>
                    <span className={`font-mono ${p.allow_custom_port ? 'text-emerald-400' : 'text-slate-500'}`}>
                      {p.allow_custom_port ? 'Allowed' : 'Auto-Allocated'}
                    </span>
                  </div>
                </div>
              </div>

              {/* Footer */}
              <div className="mt-5 pt-3 border-t border-obsidian-800 flex items-center justify-between">
                <div className="text-xs text-slate-400">
                  <span className="font-bold text-slate-200">{p.user_count || 0}</span> assigned user(s)
                </div>

                {!p.is_default && (
                  <button
                    onClick={() => handleSetDefault(p)}
                    className="text-xs text-slate-400 hover:text-emerald-400 flex items-center gap-1 transition-colors"
                  >
                    <CheckCircle className="w-3.5 h-3.5" />
                    <span>Set as Default</span>
                  </button>
                )}
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Create / Edit Plan Modal */}
      {modalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-sm overflow-y-auto">
          <div className="bg-obsidian-900 border border-obsidian-750 rounded-2xl w-full max-w-2xl my-8 p-6 shadow-2xl relative">
            <button
              onClick={() => setModalOpen(false)}
              className="absolute top-5 right-5 text-slate-400 hover:text-slate-200 p-1"
            >
              <X className="w-5 h-5" />
            </button>

            <h2 className="text-xl font-bold text-slate-100 flex items-center gap-2 mb-1">
              <Layers className="w-5 h-5 text-emerald-400" />
              <span>{editingPlan ? 'Edit Deployment Plan' : 'Create New Plan'}</span>
            </h2>
            <p className="text-xs text-slate-400 mb-6">
              Configure server limits and feature privileges enforced for users on this tier.
            </p>

            <form onSubmit={handleSave} className="space-y-4">
              {/* Name & Description */}
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <div>
                  <label className="block text-xs font-semibold text-slate-300 uppercase tracking-wider mb-1.5">
                    Plan Name *
                  </label>
                  <input
                    type="text"
                    required
                    value={name}
                    onChange={(e) => setName(e.target.value)}
                    placeholder="e.g. Starter Tier, Pro Realm"
                    className="w-full bg-obsidian-950 border border-obsidian-800 rounded-lg px-3 py-2 text-slate-100 text-sm focus:outline-none focus:border-emerald-500"
                  />
                </div>

                <div>
                  <label className="block text-xs font-semibold text-slate-300 uppercase tracking-wider mb-1.5">
                    Billing / Cycle
                  </label>
                  <select
                    value={billingInterval}
                    onChange={(e) => setBillingInterval(e.target.value)}
                    className="w-full bg-obsidian-950 border border-obsidian-800 rounded-lg px-3 py-2 text-slate-100 text-sm focus:outline-none focus:border-emerald-500"
                  >
                    <option value="permanent">Permanent / Free</option>
                    <option value="monthly">Monthly Subscription</option>
                    <option value="trial">Trial Period</option>
                  </select>
                </div>
              </div>

              <div>
                <label className="block text-xs font-semibold text-slate-300 uppercase tracking-wider mb-1.5">
                  Description
                </label>
                <input
                  type="text"
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                  placeholder="Summary for normal users"
                  className="w-full bg-obsidian-950 border border-obsidian-800 rounded-lg px-3 py-2 text-slate-100 text-sm focus:outline-none focus:border-emerald-500"
                />
              </div>

              {/* Resource Limits */}
              <div className="pt-2 border-t border-obsidian-800/80">
                <span className="text-xs font-bold text-slate-300 uppercase tracking-wider block mb-3">Compute & Resource Quotas</span>
                <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
                  <div>
                    <label className="block text-[11px] text-slate-400 mb-1">Max Servers</label>
                    <input
                      type="number"
                      min={1}
                      max={100}
                      value={maxServers}
                      onChange={(e) => setMaxServers(parseInt(e.target.value) || 1)}
                      className="w-full bg-obsidian-950 border border-obsidian-800 rounded-lg px-2.5 py-1.5 text-slate-100 text-sm focus:outline-none focus:border-emerald-500 font-mono"
                    />
                  </div>
                  <div>
                    <label className="block text-[11px] text-slate-400 mb-1">Max RAM</label>
                    <select
                      value={maxMemory}
                      onChange={(e) => setMaxMemory(e.target.value)}
                      className="w-full bg-obsidian-950 border border-obsidian-800 rounded-lg px-2.5 py-1.5 text-slate-100 text-sm focus:outline-none focus:border-emerald-500 font-mono"
                    >
                      <option value="1G">1 GB</option>
                      <option value="2G">2 GB</option>
                      <option value="4G">4 GB</option>
                      <option value="8G">8 GB</option>
                      <option value="16G">16 GB</option>
                    </select>
                  </div>
                  <div>
                    <label className="block text-[11px] text-slate-400 mb-1">Max CPU Cores</label>
                    <input
                      type="number"
                      step="0.5"
                      min="0.5"
                      max="32"
                      value={maxCPU}
                      onChange={(e) => setMaxCPU(parseFloat(e.target.value) || 2.0)}
                      className="w-full bg-obsidian-950 border border-obsidian-800 rounded-lg px-2.5 py-1.5 text-slate-100 text-sm focus:outline-none focus:border-emerald-500 font-mono"
                    />
                  </div>
                  <div>
                    <label className="block text-[11px] text-slate-400 mb-1">Max Backups</label>
                    <input
                      type="number"
                      min={1}
                      max={50}
                      value={maxBackups}
                      onChange={(e) => setMaxBackups(parseInt(e.target.value) || 3)}
                      className="w-full bg-obsidian-950 border border-obsidian-800 rounded-lg px-2.5 py-1.5 text-slate-100 text-sm focus:outline-none focus:border-emerald-500 font-mono"
                    />
                  </div>
                </div>
              </div>

              {/* Game & Operational Limits */}
              <div className="pt-2 border-t border-obsidian-800/80">
                <span className="text-xs font-bold text-slate-300 uppercase tracking-wider block mb-3">Gameplay & Host Limits</span>
                <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
                  <div>
                    <label className="block text-[11px] text-slate-400 mb-1">Player Slots</label>
                    <input
                      type="number"
                      min={1}
                      max={200}
                      value={maxPlayerSlots}
                      onChange={(e) => setMaxPlayerSlots(parseInt(e.target.value) || 10)}
                      className="w-full bg-obsidian-950 border border-obsidian-800 rounded-lg px-2.5 py-1.5 text-slate-100 text-sm focus:outline-none focus:border-emerald-500 font-mono"
                    />
                  </div>
                  <div>
                    <label className="block text-[11px] text-slate-400 mb-1">Disk Quota (MB)</label>
                    <input
                      type="number"
                      min={0}
                      step={512}
                      value={maxDiskMB}
                      onChange={(e) => setMaxDiskMB(parseInt(e.target.value) || 0)}
                      className="w-full bg-obsidian-950 border border-obsidian-800 rounded-lg px-2.5 py-1.5 text-slate-100 text-sm focus:outline-none focus:border-emerald-500 font-mono"
                    />
                  </div>
                  <div>
                    <label className="block text-[11px] text-slate-400 mb-1">Collaborators</label>
                    <input
                      type="number"
                      min={0}
                      max={10}
                      value={maxCollaborators}
                      onChange={(e) => setMaxCollaborators(parseInt(e.target.value) || 0)}
                      className="w-full bg-obsidian-950 border border-obsidian-800 rounded-lg px-2.5 py-1.5 text-slate-100 text-sm focus:outline-none focus:border-emerald-500 font-mono"
                    />
                  </div>
                  <div>
                    <label className="block text-[11px] text-slate-400 mb-1">Idle Stop (min)</label>
                    <input
                      type="number"
                      min={0}
                      step={15}
                      value={idleTimeoutMinutes}
                      onChange={(e) => setIdleTimeoutMinutes(parseInt(e.target.value) || 0)}
                      className="w-full bg-obsidian-950 border border-obsidian-800 rounded-lg px-2.5 py-1.5 text-slate-100 text-sm focus:outline-none focus:border-emerald-500 font-mono"
                    />
                  </div>
                </div>
              </div>

              {/* Feature Toggles */}
              <div className="pt-2 border-t border-obsidian-800/80">
                <span className="text-xs font-bold text-slate-300 uppercase tracking-wider block mb-2">Feature Toggles</span>
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-2 text-xs">
                  <label className="flex items-center space-x-2 bg-obsidian-950 border border-obsidian-800 p-2 rounded-lg cursor-pointer">
                    <input
                      type="checkbox"
                      checked={allowCustomPort}
                      onChange={(e) => setAllowCustomPort(e.target.checked)}
                      className="rounded text-emerald-500 focus:ring-0 bg-obsidian-900 border-obsidian-700"
                    />
                    <span className="text-slate-300">Allow Custom UDP Ports</span>
                  </label>

                  <label className="flex items-center space-x-2 bg-obsidian-950 border border-obsidian-800 p-2 rounded-lg cursor-pointer">
                    <input
                      type="checkbox"
                      checked={allowCustomSeed}
                      onChange={(e) => setAllowCustomSeed(e.target.checked)}
                      className="rounded text-emerald-500 focus:ring-0 bg-obsidian-900 border-obsidian-700"
                    />
                    <span className="text-slate-300">Allow Custom World Seed</span>
                  </label>

                  <label className="flex items-center space-x-2 bg-obsidian-950 border border-obsidian-800 p-2 rounded-lg cursor-pointer">
                    <input
                      type="checkbox"
                      checked={allowPreviewVersions}
                      onChange={(e) => setAllowPreviewVersions(e.target.checked)}
                      className="rounded text-emerald-500 focus:ring-0 bg-obsidian-900 border-obsidian-700"
                    />
                    <span className="text-slate-300">Allow Preview / Beta Builds</span>
                  </label>

                  <label className="flex items-center space-x-2 bg-obsidian-950 border border-obsidian-800 p-2 rounded-lg cursor-pointer">
                    <input
                      type="checkbox"
                      checked={allowAddons}
                      onChange={(e) => setAllowAddons(e.target.checked)}
                      className="rounded text-emerald-500 focus:ring-0 bg-obsidian-900 border-obsidian-700"
                    />
                    <span className="text-slate-300">Allow Addons & Behavior Packs</span>
                  </label>

                  <label className="flex items-center space-x-2 bg-obsidian-950 border border-obsidian-800 p-2 rounded-lg cursor-pointer">
                    <input
                      type="checkbox"
                      checked={allowPortGateKeys}
                      onChange={(e) => setAllowPortGateKeys(e.target.checked)}
                      className="rounded text-emerald-500 focus:ring-0 bg-obsidian-900 border-obsidian-700"
                    />
                    <span className="text-slate-300">Allow Port Gate Access Keys</span>
                  </label>

                  <label className="flex items-center space-x-2 bg-obsidian-950 border border-obsidian-800 p-2 rounded-lg cursor-pointer">
                    <input
                      type="checkbox"
                      checked={allowTasks}
                      onChange={(e) => setAllowTasks(e.target.checked)}
                      className="rounded text-emerald-500 focus:ring-0 bg-obsidian-900 border-obsidian-700"
                    />
                    <span className="text-slate-300">Allow Automated Cron Tasks</span>
                  </label>
                </div>
              </div>

              {/* Actions */}
              <div className="pt-4 border-t border-obsidian-800 flex justify-end space-x-3">
                <button
                  type="button"
                  onClick={() => setModalOpen(false)}
                  className="px-4 py-2 text-sm text-slate-400 hover:text-slate-200 transition-colors"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={actionLoading}
                  className="px-5 py-2 bg-emerald-600 hover:bg-emerald-500 disabled:opacity-50 text-white rounded-lg font-medium text-sm transition-colors shadow-md shadow-emerald-950/40"
                >
                  {actionLoading ? 'Saving...' : editingPlan ? 'Save Changes' : 'Create Plan'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
};
