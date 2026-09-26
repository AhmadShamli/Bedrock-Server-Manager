import React, { useState, useEffect } from 'react';
import { Shield, UserPlus, Key, Trash2, Server, Check, Loader2, AlertCircle, Clock, Save } from 'lucide-react';
import { api } from '../api/client';
import { User, Server as ServerType } from '../types';
import { Pagination } from '../components/Pagination';
import { usePagination } from '../hooks/usePagination';

interface UsersPageProps {
  currentUser: User;
}

export const UsersPage: React.FC<UsersPageProps> = ({ currentUser }) => {
  const [users, setUsers] = useState<User[]>([]);
  const [servers, setServers] = useState<ServerType[]>([]);
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
  const [savingSettings, setSavingSettings] = useState(false);
  const [settingsSaved, setSettingsSaved] = useState(false);

  // Add User modal state
  const [showAddModal, setShowAddModal] = useState(false);
  const [newUsername, setNewUsername] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [newRole, setNewRole] = useState<'admin' | 'operator'>('operator');
  const [creatingUser, setCreatingUser] = useState(false);

  // Reset Password modal state
  const [pwResetUser, setPwResetUser] = useState<User | null>(null);
  const [resetPasswordVal, setResetPasswordVal] = useState('');
  const [resettingPw, setResettingPw] = useState(false);

  // Server Access modal state
  const [accessUser, setAccessUser] = useState<User | null>(null);
  const [selectedServerIds, setSelectedServerIds] = useState<string[]>([]);
  const [savingAccess, setSavingAccess] = useState(false);

  const loadData = async () => {
    try {
      setLoading(true);
      setError(null);
      const [uList, sList, settingsRes] = await Promise.all([
        api.listUsers(),
        api.listServers(),
        api.getSettings(),
      ]);
      const safeUsers = Array.isArray(uList) ? uList : [];
      setUsers(safeUsers);
      setServers(Array.isArray(sList) ? sList : []);

      if (settingsRes && settingsRes['heartbeat_interval_seconds']) {
        const val = parseInt(settingsRes['heartbeat_interval_seconds'], 10);
        if (!isNaN(val) && val > 0) setHeartbeatSec(val);
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
        password: newPassword,
        role: newRole,
      });
      setShowAddModal(false);
      setNewUsername('');
      setNewPassword('');
      setNewRole('operator');
      loadData();
    } catch (err: any) {
      alert(err.message || 'Failed to create user');
    } finally {
      setCreatingUser(false);
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

  const handleSaveHeartbeatSetting = async (e: React.FormEvent) => {
    e.preventDefault();
    setSavingSettings(true);
    setSettingsSaved(false);
    try {
      await api.updateSetting('heartbeat_interval_seconds', heartbeatSec.toString());
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
            Provision dashboard operators, manage server permissions, reset credentials, and configure global portal settings.
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
            <span>System Accounts</span>
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
            <div className="overflow-x-auto">
              <table className="w-full text-left font-mono text-xs">
                <thead className="bg-obsidian-950/80 text-slate-400 border-b border-obsidian-800 uppercase">
                  <tr>
                    <th className="px-6 py-3.5">User</th>
                    <th className="px-6 py-3.5">Role</th>
                    <th className="px-6 py-3.5">Assigned Server Access</th>
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
                              <span className="font-bold text-slate-100">{u.username}</span>
                              {isSelf && <span className="ml-2 text-[10px] text-emerald-400 font-bold">(You)</span>}
                            </div>
                          </div>
                        </td>
                        <td className="px-6 py-4">
                          <span
                            className={`px-2 py-0.5 rounded text-[10px] font-bold uppercase tracking-wider ${
                              u.role === 'admin'
                                ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/30'
                                : 'bg-cyber-cyan/10 text-cyber-cyan border border-cyber-cyan/30'
                            }`}
                          >
                            {u.role}
                          </span>
                        </td>
                        <td className="px-6 py-4 text-slate-300">
                          {u.role === 'admin' ? (
                            <span className="text-slate-400 italic">All Servers (Full Admin Access)</span>
                          ) : (
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
                          )}
                        </td>
                        <td className="px-6 py-4 text-slate-400">
                          {new Date(u.created_at).toLocaleDateString()}
                        </td>
                        <td className="px-6 py-4 text-right space-x-2">
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

      {/* SYSTEM SETTINGS CARD */}
      <div className="bg-obsidian-900 border border-obsidian-700/80 rounded-xl p-6">
        <h3 className="font-mono text-base font-bold text-slate-100 mb-2 flex items-center gap-2">
          <Clock className="w-4 h-4 text-cyber-cyan" />
          <span>Knock Portal & Mobile Roaming Policy</span>
        </h3>
        <p className="text-xs text-slate-400 font-mono mb-4">
          Configure how frequently mobile clients send background heartbeat pings to retain their dynamic firewall lease during roaming between cell networks.
        </p>

        {settingsSaved && (
          <div className="mb-4 p-3 rounded-lg bg-emerald-950/40 border border-emerald-500/40 text-emerald-300 font-mono text-xs flex items-center space-x-2">
            <Check className="w-4 h-4 text-emerald-400 shrink-0" />
            <span>Settings saved successfully!</span>
          </div>
        )}

        <form onSubmit={handleSaveHeartbeatSetting} className="flex flex-col sm:flex-row items-start sm:items-center gap-3 font-mono text-xs">
          <div>
            <label className="block text-slate-400 mb-1 text-[11px] font-bold">
              Heartbeat Timer Interval (seconds)
            </label>
            <input
              type="number"
              min="3"
              max="120"
              value={heartbeatSec}
              onChange={(e) => setHeartbeatSec(parseInt(e.target.value) || 10)}
              className="w-48 px-3 py-2 bg-obsidian-950 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
            />
          </div>

          <button
            type="submit"
            disabled={savingSettings}
            className="sm:mt-5 px-4 py-2 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-slate-950 font-bold flex items-center space-x-1.5 transition-colors"
          >
            {savingSettings ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <Save className="w-3.5 h-3.5" />}
            <span>Save Setting</span>
          </button>
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
                <label className="block text-slate-300 mb-1 font-bold">Username</label>
                <input
                  type="text"
                  required
                  placeholder="e.g. realm_moderator"
                  value={newUsername}
                  onChange={(e) => setNewUsername(e.target.value)}
                  className="w-full px-3 py-2 bg-obsidian-950 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
                />
              </div>

              <div>
                <label className="block text-slate-300 mb-1 font-bold">Password (min 8 characters)</label>
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
                  <option value="operator">Operator (Scoped to assigned servers)</option>
                  <option value="admin">Administrator (Full root access)</option>
                </select>
              </div>

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
                  placeholder="Min 8 characters"
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
          <div className="bg-obsidian-900 border border-obsidian-700 rounded-xl max-w-md w-full p-6 shadow-2xl">
            <h3 className="text-base font-mono font-bold text-slate-100 mb-1 flex items-center gap-2">
              <Server className="w-4 h-4 text-emerald-400" />
              <span>Server Permissions: {accessUser.username}</span>
            </h3>
            <p className="text-xs text-slate-400 font-mono mb-4">
              Select which Bedrock Dedicated Server instances this operator can manage.
            </p>

            <form onSubmit={handleSaveAccess} className="space-y-4 font-mono text-xs">
              <div className="max-h-60 overflow-y-auto space-y-2 p-3 bg-obsidian-950 border border-obsidian-800 rounded-lg">
                {servers.length === 0 ? (
                  <p className="text-slate-500 italic">No servers currently deployed.</p>
                ) : (
                  servers.map((s) => {
                    const isChecked = selectedServerIds.includes(s.id);
                    return (
                      <label
                        key={s.id}
                        className={`flex items-center justify-between p-2 rounded cursor-pointer transition-colors ${
                          isChecked ? 'bg-emerald-950/40 border border-emerald-500/40' : 'hover:bg-obsidian-900 border border-transparent'
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
                            className="rounded bg-obsidian-900 border-obsidian-700 text-emerald-500 focus:ring-emerald-500"
                          />
                          <div>
                            <span className="text-slate-200 font-bold block">{s.name}</span>
                            <span className="text-slate-500 text-[10px]">ID: {s.id} • UDP :{s.port}</span>
                          </div>
                        </div>
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
                  {savingAccess ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <span>Save Permissions</span>}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
};
