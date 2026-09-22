import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Sparkles, ShieldCheck, AlertCircle, Loader2 } from 'lucide-react';
import { api } from '../api/client';
import { User } from '../types';

interface SetupProps {
  onSetupSuccess: (user: User) => void;
}

export const Setup: React.FC<SetupProps> = ({ onSetupSuccess }) => {
  const navigate = useNavigate();
  const [username, setUsername] = useState('admin');
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    if (password.length < 8) {
      setError('Password must be at least 8 characters');
      return;
    }

    if (password !== confirmPassword) {
      setError('Passwords do not match');
      return;
    }

    setLoading(true);
    try {
      const res = await api.setup({ username, password });
      onSetupSuccess(res.user);
      navigate('/');
    } catch (err: any) {
      setError(err.message || 'Setup failed');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-[85vh] flex items-center justify-center px-4">
      <div className="w-full max-w-lg bg-obsidian-900 border border-emerald-500/40 rounded-2xl p-8 shadow-[0_0_40px_rgba(16,185,129,0.15)] relative overflow-hidden backdrop-blur-xl">
        <div className="text-center mb-8">
          <div className="inline-flex items-center justify-center w-16 h-16 rounded-2xl bg-emerald-950 border border-emerald-500/50 text-emerald-400 mb-4 shadow-[0_0_25px_rgba(16,185,129,0.3)]">
            <Sparkles className="w-8 h-8" />
          </div>
          <h1 className="text-2xl font-bold font-mono tracking-wider text-slate-100">INITIAL ONBOARDING</h1>
          <p className="text-sm text-slate-400 mt-1">Configure your master administrator account to get started.</p>
        </div>

        {error && (
          <div className="mb-6 p-3 rounded-lg bg-rose-950/40 border border-rose-500/50 flex items-start space-x-3 text-rose-300 text-sm">
            <AlertCircle className="w-5 h-5 flex-shrink-0 mt-0.5 text-rose-400" />
            <span>{error}</span>
          </div>
        )}

        <form onSubmit={handleSubmit} className="space-y-5">
          <div>
            <label className="block text-xs font-mono font-medium text-slate-300 mb-1.5 uppercase tracking-wider">
              Administrator Username
            </label>
            <input
              type="text"
              required
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              className="w-full px-4 py-2.5 rounded-lg bg-obsidian-950 border border-obsidian-700 focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500 text-slate-100 font-mono text-sm"
              placeholder="admin"
            />
          </div>

          <div>
            <label className="block text-xs font-mono font-medium text-slate-300 mb-1.5 uppercase tracking-wider">
              Master Password (min 8 chars)
            </label>
            <input
              type="password"
              required
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className="w-full px-4 py-2.5 rounded-lg bg-obsidian-950 border border-obsidian-700 focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500 text-slate-100 font-mono text-sm"
              placeholder="••••••••••••"
            />
          </div>

          <div>
            <label className="block text-xs font-mono font-medium text-slate-300 mb-1.5 uppercase tracking-wider">
              Confirm Password
            </label>
            <input
              type="password"
              required
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              className="w-full px-4 py-2.5 rounded-lg bg-obsidian-950 border border-obsidian-700 focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500 text-slate-100 font-mono text-sm"
              placeholder="••••••••••••"
            />
          </div>

          <button
            type="submit"
            disabled={loading}
            className="w-full py-3 px-4 rounded-lg bg-emerald-500 hover:bg-emerald-400 text-slate-950 font-bold font-mono uppercase tracking-wider transition-all shadow-[0_0_20px_rgba(16,185,129,0.3)] hover:shadow-[0_0_30px_rgba(16,185,129,0.5)] disabled:opacity-50 flex items-center justify-center space-x-2"
          >
            {loading ? <Loader2 className="w-5 h-5 animate-spin" /> : (
              <>
                <ShieldCheck className="w-5 h-5" />
                <span>Initialize Platform</span>
              </>
            )}
          </button>
        </form>
      </div>
    </div>
  );
};
