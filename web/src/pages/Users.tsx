import React, { useState, useEffect } from 'react';
import { Shield, UserPlus, Key, Trash2, Server, Check, Loader2, AlertCircle, Clock, Save, Layers, Edit3, UserCheck, ToggleLeft, ToggleRight } from 'lucide-react';
import { api } from '../api/client';
import { User, Server as ServerType, Plan } from '../types';
import { Pagination } from '../components/Pagination';
import { usePagination } from '../hooks/usePagination';

interface UsersPageProps {
  currentUser: User;
}

export const UsersPage: React.FC<UsersPageProps> = ({ currentUser }) => {
  const [users, setUsers] = useState<User[]>([]);
  const [servers, setServers] = useState<ServerType[]>([]);
  const [plans, setPlans] = useState<Plan[]>([]);
  const [userServerMap, setUserServerMap] = useState<Record<number, string[]>>({});
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const {
    currentPage,
    pageSize,
    totalItems,
    paginatedItems: paginatedUsers,
    setCurrentPage,
    setPageSize,
  } = usePagination(users, 10);

  // System Settings state
  const [heartbeatSec, setHeartbeatSec] = useState(10);
  const [allowRegistration, setAllowRegistration] = useState(false);
  const [curseforgeApiKey, setCurseforgeApiKey] = useState('');
  const [curseForgeConfigured, setCurseForgeConfigured] = useState(false);
  const [savingSettings, setSavingSettings] = useState(false);
  const [settingsSaved, setSettingsSaved] = useState(false);

  // Add User modal state
  const [showAddModal, setShowAddModal] = useState(false);
  const [newUsername, setNewUsername] = useState('');
  const [newEmail, setNewEmail] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [newRole, setNewRole] = useState<'admin' | 'operator' | 'user'>('user');
  const [newPlanId, setNewPlanId] = useState<number | undefined>(undefined);
  const [creatingUser, setCreatingUser] = useState(false);

  // Edit User Plan / Role modal state
  const [editPlanUser, setEditPlanUser] = useState<User | null>(null);
  const [editRole, setEditRole] = useState<'admin' | 'operator' | 'user'>('user');
  const [editEmail, setEditEmail] = useState('');
  const [editPlanId, setEditPlanId] = useState<number | null>(null);
  const [savingPlanUser, setSavingPlanUser] = useState(false);

  // Reset Password modal state
  const [pwResetUser, setPwResetUser] = useState<User | null>(null);
  const [resetPasswordVal, setResetPasswordVal] = useState('');
  const [resettingPw, setResettingPw] = useState(false);

  // Server Access modal state (for operators)
  const [accessUser, setAccessUser] = useState<User | null>(null);
  const [selectedServerIds, setSelectedServerIds] = useState<string[]>([]);
  const [savingAccess, setSavingAccess] = useState(false);

  const loadData = async () => {
    try {
      setLoading(true);
      setError(null);
      const [uList, sList, pList, settingsRes] = await Promise.all([
        api.listUsers(),
        api.listServers(),
        api.listPlans(),
        api.getSettings(),
      ]);
      const safeUsers = Array.isArray(uList) ? uList : [];
      setUsers(safeUsers);
      setServers(Array.isArray(sList) ? sList : []);
      setPlans(Array.isArray(pList) ? pList : []);

      if (settingsRes && settingsRes['heartbeat_interval_seconds']) {
        const val = parseInt(settingsRes['heartbeat_interval_seconds'], 10);
        if (!isNaN(val) && val > 0) setHeartbeatSec(val);
      }

      if (settingsRes && settingsRes['allow_registration']) {
        setAllowRegistration(settingsRes['allow_registration'] === 'true' || settingsRes['allow_registration'] === '1');
      }

      if (settingsRes && settingsRes['curseforge_api_key']) {
        setCurseForgeConfigured(true);
      }

      // Default plan selection for modal
      const def = pList.find((p) => p.is_default);
      if (def) {
        setNewPlanId(def.id);
      }

      // Fetch server access for each operator user
      const accessMap: Record<number, string[]> = {};
      await Promise.all(
        safeUsers.map(async (u) => {
          if (u.role === 'operator') {
            try {
              const srvs = await api.getUserServerAccess(u.id);
              accessMap[u.id] = srvs;
            } catch {
              accessMap[u.id] = [];
            }
          }
        })
      );
      setUserServerMap(accessMap);
    } catch (err: any) {
      setError(err.message || 'Failed to load user management data');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadData();
  }, []);

  const handleCreateUser = async (e: React.FormEvent) => {
    e.preventDefault();
    setCreatingUser(true);
    try {
      await api.createUser({
        username: newUsername.trim(),
        email: newEmail.trim(),
        password: newPassword,
        role: newRole,
        plan_id: newRole === 'user' ? newPlanId : undefined,
      });
      setShowAddModal(false);
      setNewUsername('');
      setNewEmail('');
      setNewPassword('');
      setNewRole('user');
      loadData();
    } catch (err: any) {
      alert(err.message || 'Failed to create user');
    } finally {
      setCreatingUser(false);
    }
  };

  const handleOpenEditPlanModal = (u: User) => {
    setEditPlanUser(u);
    setEditRole(u.role);
    setEditEmail(u.email || '');
    setEditPlanId(u.plan_id || null);
  };

  const handleSavePlanUser = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!editPlanUser) return;
    setSavingPlanUser(true);
    try {
      await api.updateUser(editPlanUser.id, {
        role: editRole,
        email: editEmail.trim(),
        plan_id: editRole === 'user' ? editPlanId : null,
      });
      setEditPlanUser(null);
      await loadData();
    } catch (err: any) {
      alert(err.message || 'Failed to update user');
    } finally {
      setSavingPlanUser(false);
    }
  };

  const handleResetPassword = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!pwResetUser) return;
    setResettingPw(true);
    try {
      await api.updateUserPassword(pwResetUser.id, resetPasswordVal);
      setPwResetUser(null);
      setResetPasswordVal('');
      alert(`Password updated successfully for ${pwResetUser.username}!`);
    } catch (err: any) {
      alert(err.message || 'Failed to reset password');
    } finally {
      setResettingPw(false);
    }
  };

  const handleOpenAccessModal = (u: User) => {
    setAccessUser(u);
    setSelectedServerIds(userServerMap[u.id] || []);
  };

  const handleSaveAccess = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!accessUser) return;
    setSavingAccess(true);
    try {
      await api.updateUserServerAccess(accessUser.id, selectedServerIds);
      setUserServerMap((prev) => ({ ...prev, [accessUser.id]: selectedServerIds }));
      setAccessUser(null);
    } catch (err: any) {
      alert(err.message || 'Failed to update server access');
    } finally {
      setSavingAccess(false);
    }
  };

  const handleDeleteUser = async (u: User) => {
    if (u.id === currentUser.id) {
      alert('You cannot delete your own account!');
      return;
    }
    if (!confirm(`Are you sure you want to delete user "${u.username}"?`)) return;
    try {
      await api.deleteUser(u.id);
      loadData();
    } catch (err: any) {
      alert(err.message || 'Failed to delete user');
    }
  };

  const handleSaveSettings = async (e: React.FormEvent) => {
    e.preventDefault();
    setSavingSettings(true);
    setSettingsSaved(false);
    try {
      const updates: Promise<any>[] = [
        api.updateSetting('heartbeat_interval_seconds', heartbeatSec.toString()),
        api.updateSetting('allow_registration', allowRegistration ? 'true' : 'false'),
      ];
      if (curseforgeApiKey.trim()) {
        updates.push(api.updateMarketplaceConfig(curseforgeApiKey.trim()));
      }
      await Promise.all(updates);
      if (curseforgeApiKey.trim()) {
        setCurseForgeConfigured(true);
        setCurseforgeApiKey('');
      }
      setSettingsSaved(true);
      setTimeout(() => setSettingsSaved(false), 3000);
    } catch (err: any) {
      alert(err.message || 'Failed to update settings');
    } finally {
      setSavingSettings(false);
    }
  };

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8 space-y-8">
      {/* HEADER */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4 pb-6 border-b border-obsidian-800">
        <div>
          <h1 className="text-2xl font-mono font-bold text-slate-100 flex items-center gap-3">
            <Shield className="w-6 h-6 text-emerald-400" />
            <span>User & Access Management</span>
          </h1>
          <p className="text-sm font-mono text-slate-400 mt-1">
            Provision users, assign deployment plans, configure operator scopes, and manage portal access policies.
          </p>
        </div>

        <button
          onClick={() => setShowAddModal(true)}
          className="px-4 py-2.5 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-slate-950 font-bold font-mono text-xs flex items-center space-x-2 shadow-[0_0_15px_rgba(16,185,129,0.2)]"
        >
          <UserPlus className="w-4 h-4" />
          <span>+ Add Account</span>
        </button>
      </div>

      {error && (
        <div className="p-4 rounded-xl bg-rose-950/40 border border-rose-800 text-rose-300 flex items-center space-x-3 text-sm font-mono">
          <AlertCircle className="w-5 h-5 shrink-0" />
          <span>{error}</span>
        </div>
      )}

      {/* USERS TABLE */}
      <div className="bg-obsidian-900 border border-obsidian-700/80 rounded-xl overflow-hidden shadow-xl">
        <div className="p-5 border-b border-obsidian-800 flex items-center justify-between">
          <h3 className="font-mono text-base font-bold text-slate-100 flex items-center gap-2">
            <span>Registered Accounts</span>
            <span className="text-xs px-2 py-0.5 rounded-full bg-emerald-500/10 text-emerald-400 border border-emerald-500/30">
              {users.length}
            </span>
          </h3>
        </div>

        {loading ? (
          <div className="flex flex-col items-center justify-center py-16 text-slate-400 font-mono text-sm">
            <Loader2 className="w-6 h-6 animate-spin text-emerald-400 mb-2" />
            <span>Loading accounts...</span>
          </div>
        ) : (
          <div>
            <div className="overflow-x-auto w-full max-w-full">
              <table className="w-full text-left font-mono text-xs min-w-[700px]">
                <thead className="bg-obsidian-950/80 text-slate-400 border-b border-obsidian-800 uppercase">
                  <tr>
                    <th className="px-6 py-3.5">User</th>
                    <th className="px-6 py-3.5">Role</th>
                    <th className="px-6 py-3.5">Assigned Plan</th>
                    <th className="px-6 py-3.5">Server Scope</th>
                    <th className="px-6 py-3.5">Created At</th>
                    <th className="px-6 py-3.5 text-right">Actions</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-obsidian-800">
                  {paginatedUsers.map((u) => {
                    const isSelf = u.id === currentUser.id;
                    const assigned = userServerMap[u.id] || [];
                    return (
                      <tr key={u.id} className="hover:bg-obsidian-850/50 transition-colors">
                        <td className="px-6 py-4">
                          <div className="flex items-center space-x-2.5">
                            <div className="w-7 h-7 rounded-lg bg-obsidian-800 border border-obsidian-700 flex items-center justify-center text-slate-300 font-bold">
                              {u.username.charAt(0).toUpperCase()}
                            </div>
                            <div>
                              <div className="flex items-center gap-1.5">
                                <span className="font-bold text-slate-100">{u.username}</span>
                                {isSelf && <span className="text-[10px] text-emerald-400 font-bold">(You)</span>}
                              </div>
                              {u.email && <span className="text-[10px] text-slate-500 block">{u.email}</span>}
                            </div>
                          </div>
                        </td>

                        <td className="px-6 py-4">
                          <span
                            className={`px-2 py-0.5 rounded text-[10px] font-bold uppercase tracking-wider ${
                              u.role === 'admin'
                                ? 'bg-purple-500/10 text-purple-400 border border-purple-500/30'
                                : u.role === 'operator'
                                ? 'bg-cyber-cyan/10 text-cyber-cyan border border-cyber-cyan/30'
                                : 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/30'
                            }`}
                          >
                            {u.role}
                          </span>
                        </td>

                        <td className="px-6 py-4">
                          {u.role === 'user' ? (
                            <div className="flex items-center gap-2">
                              <span className="px-2 py-0.5 rounded bg-emerald-500/10 text-emerald-300 border border-emerald-500/30 font-semibold text-[10px]">
                                {u.plan_name || 'Default Plan'}
                              </span>
                              <button
                                onClick={() => handleOpenEditPlanModal(u)}
                                title="Change Plan / Role"
                                className="p-1 rounded bg-obsidian-800 text-slate-400 hover:text-slate-200"
                              >
                                <Edit3 className="w-3 h-3" />
                              </button>
                            </div>
                          ) : (
                            <span className="text-slate-500 italic">N/A ({u.role})</span>
                          )}
                        </td>

                        <td className="px-6 py-4 text-slate-300">
                          {u.role === 'admin' ? (
                            <span className="text-slate-400 italic">All Servers (Admin)</span>
                          ) : u.role === 'operator' ? (
                            <div className="flex items-center gap-2">
                              <span className="text-slate-300 font-bold">
                                {assigned.length} {assigned.length === 1 ? 'server' : 'servers'}
                              </span>
                              <button
                                onClick={() => handleOpenAccessModal(u)}
                                className="px-2 py-0.5 rounded bg-obsidian-800 hover:bg-obsidian-700 text-slate-300 hover:text-emerald-400 text-[10px] border border-obsidian-700 transition-colors"
                              >
                                Configure
                              </button>
                            </div>
                          ) : (
                            <span className="text-slate-400 italic">Owned Self-Deployed</span>
                          )}
                        </td>

                        <td className="px-6 py-4 text-slate-400">
                          {new Date(u.created_at).toLocaleDateString()}
                        </td>

                        <td className="px-6 py-4 text-right space-x-2">
                          <button
                            onClick={() => handleOpenEditPlanModal(u)}
                            title="Edit Role & Plan"
                            className="p-1.5 rounded bg-obsidian-800 text-slate-300 hover:bg-emerald-600 hover:text-slate-950 transition-colors"
                          >
                            <UserCheck className="w-3.5 h-3.5" />
                          </button>
                          <button
                            onClick={() => {
                              setPwResetUser(u);
                              setResetPasswordVal('');
                            }}
                            title="Reset Password"
                            className="p-1.5 rounded bg-obsidian-800 text-slate-300 hover:bg-emerald-600 hover:text-slate-950 transition-colors"
                          >
                            <Key className="w-3.5 h-3.5" />
                          </button>
                          <button
                            onClick={() => handleDeleteUser(u)}
                            disabled={isSelf}
                            title={isSelf ? 'Cannot delete your own account' : 'Delete User'}
                            className="p-1.5 rounded bg-rose-600/20 text-rose-400 hover:bg-rose-600 hover:text-slate-950 transition-colors disabled:opacity-30 disabled:cursor-not-allowed"
                          >
                            <Trash2 className="w-3.5 h-3.5" />
                          </button>
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
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
      </div>

      {/* SYSTEM REGISTRATION & PORTAL SETTINGS CARD */}
      <div className="bg-obsidian-900 border border-obsidian-700/80 rounded-xl p-6 space-y-6">
        <div>
          <h3 className="font-mono text-base font-bold text-slate-100 mb-1 flex items-center gap-2">
            <Clock className="w-4 h-4 text-emerald-400" />
            <span>Registration & Portal Access Policies</span>
          </h3>
          <p className="text-xs text-slate-400 font-mono">
            Configure open self-registration for normal users and dynamic firewall heartbeat intervals.
          </p>
        </div>

        {settingsSaved && (
          <div className="p-3 rounded-lg bg-emerald-950/40 border border-emerald-500/40 text-emerald-300 font-mono text-xs flex items-center space-x-2">
            <Check className="w-4 h-4 text-emerald-400 shrink-0" />
            <span>Settings saved successfully!</span>
          </div>
        )}

        <form onSubmit={handleSaveSettings} className="space-y-4 font-mono text-xs">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-6 p-4 rounded-xl bg-obsidian-950 border border-obsidian-800">
            {/* Registration toggle */}
            <div>
              <div className="flex items-center justify-between mb-1.5">
                <span className="font-bold text-slate-200">Public Self-Registration</span>
                <button
                  type="button"
                  onClick={() => setAllowRegistration(!allowRegistration)}
                  className="text-emerald-400 focus:outline-none"
                >
                  {allowRegistration ? (
                    <ToggleRight className="w-6 h-6 text-emerald-400" />
                  ) : (
                    <ToggleLeft className="w-6 h-6 text-slate-600" />
                  )}
                </button>
              </div>
              <p className="text-[11px] text-slate-400">
                {allowRegistration
                  ? 'Enabled: Visitors can register accounts at /register and receive the Default Plan.'
                  : 'Disabled: New user accounts can only be created by administrators.'}
              </p>
            </div>

            {/* Heartbeat seconds */}
            <div>
              <label className="block text-slate-200 font-bold mb-1.5">
                Heartbeat Ping Interval (seconds)
              </label>
              <input
                type="number"
                min="3"
                max="120"
                value={heartbeatSec}
                onChange={(e) => setHeartbeatSec(parseInt(e.target.value) || 10)}
                className="w-full px-3 py-1.5 bg-obsidian-900 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
              />
              <p className="text-[11px] text-slate-400 mt-1">
                Interval for client IP renewal during network roaming.
              </p>
            </div>

            {/* CurseForge API Key */}
            <div className="md:col-span-2 border-t border-obsidian-800/80 pt-4">
              <div className="flex items-center justify-between mb-1.5">
                <span className="font-bold text-slate-200">CurseForge API Key (Eternal API)</span>
                {curseForgeConfigured && (
                  <span className="text-[10px] text-emerald-400 bg-emerald-500/10 px-2 py-0.5 rounded border border-emerald-500/30">
                    Configured
                  </span>
                )}
              </div>
              <input
                type="password"
                placeholder={curseForgeConfigured ? '•••••••••••••••• (Leave blank to keep existing key)' : 'Enter CurseForge API Key'}
                value={curseforgeApiKey}
                onChange={(e) => setCurseforgeApiKey(e.target.value)}
                className="w-full px-3 py-1.5 bg-obsidian-900 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
              />
              <p className="text-[11px] text-slate-400 mt-1">
                Required for in-app CurseForge marketplace search &amp; 1-click install. Register a free API key at console.curseforge.com.
              </p>
            </div>
          </div>

          <div className="flex justify-end">
            <button
              type="submit"
              disabled={savingSettings}
              className="px-5 py-2 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-slate-950 font-bold flex items-center space-x-1.5 transition-colors"
            >
              {savingSettings ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <Save className="w-3.5 h-3.5" />}
              <span>Save System Settings</span>
            </button>
          </div>
        </form>
      </div>

      {/* CREATE USER MODAL */}
      {showAddModal && (
        <div className="fixed inset-0 bg-black/80 backdrop-blur-sm z-50 flex items-center justify-center p-4">
          <div className="bg-obsidian-900 border border-obsidian-700 rounded-xl max-w-md w-full p-6 shadow-2xl">
            <h3 className="text-base font-mono font-bold text-slate-100 mb-4 flex items-center gap-2">
              <UserPlus className="w-4 h-4 text-emerald-400" />
              <span>Create Account</span>
            </h3>

            <form onSubmit={handleCreateUser} className="space-y-4 font-mono text-xs">
              <div>
                <label className="block text-slate-300 mb-1 font-bold">Username *</label>
                <input
                  type="text"
                  required
                  placeholder="e.g. gamer_steve"
                  value={newUsername}
                  onChange={(e) => setNewUsername(e.target.value)}
                  className="w-full px-3 py-2 bg-obsidian-950 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
                />
              </div>

              <div>
                <label className="block text-slate-300 mb-1 font-bold">Email (Optional)</label>
                <input
                  type="email"
                  placeholder="user@example.com"
                  value={newEmail}
                  onChange={(e) => setNewEmail(e.target.value)}
                  className="w-full px-3 py-2 bg-obsidian-950 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
                />
              </div>

              <div>
                <label className="block text-slate-300 mb-1 font-bold">Password * (min 8 characters)</label>
                <input
                  type="password"
                  required
                  minLength={8}
                  placeholder="••••••••"
                  value={newPassword}
                  onChange={(e) => setNewPassword(e.target.value)}
                  className="w-full px-3 py-2 bg-obsidian-950 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
                />
              </div>

              <div>
                <label className="block text-slate-300 mb-1 font-bold">Role</label>
                <select
                  value={newRole}
                  onChange={(e) => setNewRole(e.target.value as any)}
                  className="w-full px-3 py-2 bg-obsidian-950 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
                >
                  <option value="user">Normal User (Deploy servers under plan quotas)</option>
                  <option value="operator">Operator (Scoped to assigned servers)</option>
                  <option value="admin">Administrator (Full root access)</option>
                </select>
              </div>

              {newRole === 'user' && (
                <div>
                  <label className="block text-slate-300 mb-1 font-bold">Assigned Plan</label>
                  <select
                    value={newPlanId || ''}
                    onChange={(e) => setNewPlanId(Number(e.target.value) || undefined)}
                    className="w-full px-3 py-2 bg-obsidian-950 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
                  >
                    {plans.map((p) => (
                      <option key={p.id} value={p.id}>
                        {p.name} {p.is_default ? '(Default)' : ''} - {p.max_servers} Server(s), {p.max_memory} RAM
                      </option>
                    ))}
                  </select>
                </div>
              )}

              <div className="flex justify-end space-x-3 pt-3 border-t border-obsidian-800">
                <button
                  type="button"
                  onClick={() => setShowAddModal(false)}
                  className="px-4 py-2 rounded-lg bg-obsidian-800 text-slate-300 hover:bg-obsidian-700"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={creatingUser}
                  className="px-4 py-2 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-slate-950 font-bold"
                >
                  {creatingUser ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <span>Create Account</span>}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* EDIT USER ROLE & PLAN MODAL */}
      {editPlanUser && (
        <div className="fixed inset-0 bg-black/80 backdrop-blur-sm z-50 flex items-center justify-center p-4">
          <div className="bg-obsidian-900 border border-obsidian-700 rounded-xl max-w-md w-full p-6 shadow-2xl">
            <h3 className="text-base font-mono font-bold text-slate-100 mb-2 flex items-center gap-2">
              <Layers className="w-4 h-4 text-emerald-400" />
              <span>Edit User: {editPlanUser.username}</span>
            </h3>
            <p className="text-xs text-slate-400 font-mono mb-4">
              Reassign role, plan tiers, or update user details.
            </p>

            <form onSubmit={handleSavePlanUser} className="space-y-4 font-mono text-xs">
              <div>
                <label className="block text-slate-300 mb-1 font-bold">Email</label>
                <input
                  type="email"
                  value={editEmail}
                  onChange={(e) => setEditEmail(e.target.value)}
                  placeholder="user@example.com"
                  className="w-full px-3 py-2 bg-obsidian-950 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
                />
              </div>

              <div>
                <label className="block text-slate-300 mb-1 font-bold">Role</label>
                <select
                  value={editRole}
                  onChange={(e) => setEditRole(e.target.value as any)}
                  className="w-full px-3 py-2 bg-obsidian-950 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
                >
                  <option value="user">Normal User (Deploy servers under plan quotas)</option>
                  <option value="operator">Operator (Scoped to assigned servers)</option>
                  <option value="admin">Administrator (Full root access)</option>
                </select>
              </div>

              {editRole === 'user' && (
                <div>
                  <label className="block text-slate-300 mb-1 font-bold">Assigned Deployment Plan</label>
                  <select
                    value={editPlanId || ''}
                    onChange={(e) => setEditPlanId(Number(e.target.value) || null)}
                    className="w-full px-3 py-2 bg-obsidian-950 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
                  >
                    {plans.map((p) => (
                      <option key={p.id} value={p.id}>
                        {p.name} {p.is_default ? '(Default)' : ''} - {p.max_servers} Server(s), {p.max_memory} RAM
                      </option>
                    ))}
                  </select>
                </div>
              )}

              <div className="flex justify-end space-x-3 pt-3 border-t border-obsidian-800">
                <button
                  type="button"
                  onClick={() => setEditPlanUser(null)}
                  className="px-4 py-2 rounded-lg bg-obsidian-800 text-slate-300 hover:bg-obsidian-700"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={savingPlanUser}
                  className="px-4 py-2 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-slate-950 font-bold"
                >
                  {savingPlanUser ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <span>Save Changes</span>}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* RESET PASSWORD MODAL */}
      {pwResetUser && (
        <div className="fixed inset-0 bg-black/80 backdrop-blur-sm z-50 flex items-center justify-center p-4">
          <div className="bg-obsidian-900 border border-obsidian-700 rounded-xl max-w-md w-full p-6 shadow-2xl">
            <h3 className="text-base font-mono font-bold text-slate-100 mb-2 flex items-center gap-2">
              <Key className="w-4 h-4 text-emerald-400" />
              <span>Reset Password: {pwResetUser.username}</span>
            </h3>
            <p className="text-xs text-slate-400 font-mono mb-4">
              Enter a new secure password for this account.
            </p>

            <form onSubmit={handleResetPassword} className="space-y-4 font-mono text-xs">
              <div>
                <label className="block text-slate-300 mb-1 font-bold">New Password</label>
                <input
                  type="password"
                  required
                  minLength={8}
                  placeholder="••••••••"
                  value={resetPasswordVal}
                  onChange={(e) => setResetPasswordVal(e.target.value)}
                  className="w-full px-3 py-2 bg-obsidian-950 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
                />
              </div>

              <div className="flex justify-end space-x-3 pt-3 border-t border-obsidian-800">
                <button
                  type="button"
                  onClick={() => setPwResetUser(null)}
                  className="px-4 py-2 rounded-lg bg-obsidian-800 text-slate-300 hover:bg-obsidian-700"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={resettingPw}
                  className="px-4 py-2 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-slate-950 font-bold"
                >
                  {resettingPw ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <span>Update Password</span>}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* CONFIGURE SERVER ACCESS MODAL */}
      {accessUser && (
        <div className="fixed inset-0 bg-black/80 backdrop-blur-sm z-50 flex items-center justify-center p-4">
          <div className="bg-obsidian-900 border border-obsidian-700 rounded-xl max-w-lg w-full p-6 shadow-2xl">
            <h3 className="text-base font-mono font-bold text-slate-100 mb-2 flex items-center gap-2">
              <Server className="w-4 h-4 text-emerald-400" />
              <span>Assigned Servers: {accessUser.username}</span>
            </h3>
            <p className="text-xs text-slate-400 font-mono mb-4">
              Select which servers operator <strong>{accessUser.username}</strong> can view and manage.
            </p>

            <form onSubmit={handleSaveAccess} className="space-y-4 font-mono text-xs">
              <div className="max-h-60 overflow-y-auto space-y-2 border border-obsidian-800 rounded-lg p-3 bg-obsidian-950">
                {servers.length === 0 ? (
                  <div className="text-slate-500 text-center py-4">No servers available</div>
                ) : (
                  servers.map((s) => {
                    const isChecked = selectedServerIds.includes(s.id);
                    return (
                      <label
                        key={s.id}
                        className={`flex items-center justify-between p-2.5 rounded-lg border transition-colors cursor-pointer ${
                          isChecked
                            ? 'bg-emerald-950/20 border-emerald-500/40 text-slate-100'
                            : 'bg-obsidian-900 border-obsidian-800 text-slate-400 hover:border-obsidian-700'
                        }`}
                      >
                        <div className="flex items-center space-x-2.5">
                          <input
                            type="checkbox"
                            checked={isChecked}
                            onChange={(e) => {
                              if (e.target.checked) {
                                setSelectedServerIds([...selectedServerIds, s.id]);
                              } else {
                                setSelectedServerIds(selectedServerIds.filter((id) => id !== s.id));
                              }
                            }}
                            className="rounded bg-obsidian-950 border-obsidian-700 text-emerald-500 focus:ring-emerald-500/20"
                          />
                          <div>
                            <span className="font-bold block">{s.name}</span>
                            <span className="text-[10px] text-slate-500">{s.id} • Port {s.port}</span>
                          </div>
                        </div>
                        <span
                          className={`text-[10px] uppercase font-bold px-1.5 py-0.5 rounded ${
                            s.status === 'running'
                              ? 'bg-emerald-500/10 text-emerald-400'
                              : 'bg-slate-800 text-slate-400'
                          }`}
                        >
                          {s.status}
                        </span>
                      </label>
                    );
                  })
                )}
              </div>

              <div className="flex justify-end space-x-3 pt-3 border-t border-obsidian-800">
                <button
                  type="button"
                  onClick={() => setAccessUser(null)}
                  className="px-4 py-2 rounded-lg bg-obsidian-800 text-slate-300 hover:bg-obsidian-700"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={savingAccess}
                  className="px-4 py-2 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-slate-950 font-bold"
                >
                  {savingAccess ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <span>Save Access</span>}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
};
