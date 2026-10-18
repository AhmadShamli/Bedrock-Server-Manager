import React, { useState, useEffect, useRef } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import { UserPlus, AlertCircle, Loader2 } from 'lucide-react';
import { api } from '../api/client';
import { User, TurnstileConfig } from '../types';
import { TurnstileWidget, TurnstileWidgetRef } from '../components/TurnstileWidget';

interface RegisterProps {
  onRegisterSuccess: (user: User) => void;
}

export const Register: React.FC<RegisterProps> = ({ onRegisterSuccess }) => {
  const navigate = useNavigate();
  const [username, setUsername] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Turnstile state
  const [turnstileConfig, setTurnstileConfig] = useState<TurnstileConfig | null>(null);
  const [turnstileToken, setTurnstileToken] = useState<string>('');
  const turnstileRef = useRef<TurnstileWidgetRef>(null);

  useEffect(() => {
    api.getTurnstileConfig()
      .then((cfg) => setTurnstileConfig(cfg))
      .catch(() => null);
  }, []);

  const isTurnstileRequired = !!(turnstileConfig?.enabled && turnstileConfig.register_enabled && turnstileConfig.site_key);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    if (password !== confirmPassword) {
      setError('Passwords do not match');
      return;
    }

    if (password.length < 8) {
      setError('Password must be at least 8 characters long');
      return;
    }

    if (isTurnstileRequired && !turnstileToken) {
      setError('Please complete the security challenge verification.');
      return;
    }

    setLoading(true);

    try {
      const res = await api.register({
        username: username.trim(),
        email: email.trim(),
        password,
        turnstile_token: turnstileToken || undefined,
      });
      onRegisterSuccess(res.user);
      navigate('/');
    } catch (err: any) {
      setError(err.message || 'Registration failed');
      if (isTurnstileRequired) {
        setTurnstileToken('');
        turnstileRef.current?.reset();
      }
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-[85vh] flex items-center justify-center px-4">
      <div className="w-full max-w-md bg-obsidian-900 border border-obsidian-700/80 rounded-2xl p-8 shadow-2xl relative overflow-hidden backdrop-blur-xl">
        <div className="absolute -top-24 -right-24 w-48 h-48 bg-emerald-500/10 rounded-full blur-3xl pointer-events-none"></div>

        <div className="text-center mb-8">
          <div className="inline-flex items-center justify-center w-14 h-14 rounded-xl bg-emerald-950/60 border border-emerald-500/40 text-emerald-400 mb-4 shadow-[0_0_20px_rgba(16,185,129,0.2)]">
            <UserPlus className="w-7 h-7" />
          </div>
          <h1 className="text-2xl font-bold font-mono tracking-wide text-slate-100">JOIN BSM</h1>
          <p className="text-sm text-slate-400 mt-1">Create an account to deploy your game servers</p>
        </div>

        {error && (
          <div className="mb-6 p-3 rounded-lg bg-rose-950/40 border border-rose-500/50 flex items-start space-x-3 text-rose-300 text-sm">
            <AlertCircle className="w-5 h-5 flex-shrink-0 mt-0.5 text-rose-400" />
            <span>{error}</span>
          </div>
        )}

        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-xs font-mono font-medium text-slate-300 mb-1.5 uppercase tracking-wider">
              Username *
            </label>
            <input
              type="text"
              required
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              className="w-full px-4 py-2.5 rounded-lg bg-obsidian-950 border border-obsidian-700 focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500 text-slate-100 placeholder-slate-500 font-mono text-sm transition-all"
              placeholder="e.g. steve"
            />
          </div>

          <div>
            <label className="block text-xs font-mono font-medium text-slate-300 mb-1.5 uppercase tracking-wider">
              Email (Optional)
            </label>
            <input
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              className="w-full px-4 py-2.5 rounded-lg bg-obsidian-950 border border-obsidian-700 focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500 text-slate-100 placeholder-slate-500 font-mono text-sm transition-all"
              placeholder="player@example.com"
            />
          </div>

          <div>
            <label className="block text-xs font-mono font-medium text-slate-300 mb-1.5 uppercase tracking-wider">
              Password *
            </label>
            <input
              type="password"
              required
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className="w-full px-4 py-2.5 rounded-lg bg-obsidian-950 border border-obsidian-700 focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500 text-slate-100 placeholder-slate-500 font-mono text-sm transition-all"
              placeholder="At least 8 characters"
            />
          </div>

          <div>
            <label className="block text-xs font-mono font-medium text-slate-300 mb-1.5 uppercase tracking-wider">
              Confirm Password *
            </label>
            <input
              type="password"
              required
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              className="w-full px-4 py-2.5 rounded-lg bg-obsidian-950 border border-obsidian-700 focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500 text-slate-100 placeholder-slate-500 font-mono text-sm transition-all"
              placeholder="••••••••••••"
            />
          </div>

          {isTurnstileRequired && (
            <TurnstileWidget
              ref={turnstileRef}
              siteKey={turnstileConfig!.site_key}
              action="register"
              onVerify={(token) => setTurnstileToken(token)}
              onExpire={() => setTurnstileToken('')}
              onError={() => setTurnstileToken('')}
            />
          )}

          <button
            type="submit"
            disabled={loading || (isTurnstileRequired && !turnstileToken)}
            className="w-full mt-2 py-3 px-4 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-slate-950 font-bold font-mono uppercase tracking-wider transition-all shadow-[0_0_20px_rgba(16,185,129,0.3)] hover:shadow-[0_0_25px_rgba(16,185,129,0.5)] disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center space-x-2"
          >
            {loading ? <Loader2 className="w-5 h-5 animate-spin" /> : <span>Create Account</span>}
          </button>
        </form>

        <div className="mt-6 text-center text-xs text-slate-400">
          Already have an account?{' '}
          <Link to="/login" className="text-emerald-400 hover:underline font-medium">
            Sign In
          </Link>
        </div>
      </div>
    </div>
  );
};
