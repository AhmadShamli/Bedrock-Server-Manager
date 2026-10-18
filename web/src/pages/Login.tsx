import React, { useState, useEffect, useRef } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import { KeyRound, AlertCircle, Loader2, MailCheck } from 'lucide-react';
import { api } from '../api/client';
import { User, TurnstileConfig } from '../types';
import { TurnstileWidget, TurnstileWidgetRef } from '../components/TurnstileWidget';

interface LoginProps {
  onLoginSuccess: (user: User) => void;
}

export const Login: React.FC<LoginProps> = ({ onLoginSuccess }) => {
  const navigate = useNavigate();
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [rememberMe, setRememberMe] = useState(true);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [unverified, setUnverified] = useState(false);
  const [resending, setResending] = useState(false);
  const [resendNotice, setResendNotice] = useState<string | null>(null);

  // Turnstile state
  const [turnstileConfig, setTurnstileConfig] = useState<TurnstileConfig | null>(null);
  const [turnstileToken, setTurnstileToken] = useState<string>('');
  const turnstileRef = useRef<TurnstileWidgetRef>(null);

  useEffect(() => {
    api.getTurnstileConfig()
      .then((cfg) => setTurnstileConfig(cfg))
      .catch(() => null);
  }, []);

  const isTurnstileRequired = !!(turnstileConfig?.enabled && turnstileConfig.login_enabled && turnstileConfig.site_key);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setUnverified(false);
    setResendNotice(null);

    if (isTurnstileRequired && !turnstileToken) {
      setError('Please complete the security challenge verification.');
      return;
    }

    setLoading(true);

    try {
      const res = await api.login({
        username,
        password,
        remember_me: rememberMe,
        turnstile_token: turnstileToken || undefined,
      });
      onLoginSuccess(res.user);
      navigate('/');
    } catch (err: any) {
      const msg = err.message || 'Login failed';
      setError(msg);
      if (msg.includes('verification is required') || msg.includes('EMAIL_NOT_VERIFIED')) {
        setUnverified(true);
      } else {
        setUnverified(false);
      }
      if (isTurnstileRequired) {
        setTurnstileToken('');
        turnstileRef.current?.reset();
      }
    } finally {
      setLoading(false);
    }
  };

  const handleResendVerification = async () => {
    if (!username.trim()) return;
    setResending(true);
    setResendNotice(null);
    try {
      const res = await api.resendVerification({ email_or_username: username.trim() });
      setResendNotice(res.message || 'Verification link sent to your email.');
    } catch (err: any) {
      setResendNotice(err.message || 'Failed to resend verification email.');
    } finally {
      setResending(false);
    }
  };

  return (
    <div className="min-h-[85vh] flex items-center justify-center px-4">
      <div className="w-full max-w-md bg-obsidian-900 border border-obsidian-700/80 rounded-2xl p-8 shadow-2xl relative overflow-hidden backdrop-blur-xl">
        <div className="absolute -top-24 -right-24 w-48 h-48 bg-emerald-500/10 rounded-full blur-3xl pointer-events-none"></div>

        <div className="text-center mb-8">
          <div className="inline-flex items-center justify-center w-14 h-14 rounded-xl bg-emerald-950/60 border border-emerald-500/40 text-emerald-400 mb-4 shadow-[0_0_20px_rgba(16,185,129,0.2)]">
            <KeyRound className="w-7 h-7" />
          </div>
          <h1 className="text-2xl font-bold font-mono tracking-wide text-slate-100">BSM ACCESS</h1>
          <p className="text-sm text-slate-400 mt-1">Bedrock Dedicated Server Manager</p>
        </div>

        {unverified ? (
          <div className="mb-6 p-4 rounded-xl bg-amber-950/40 border border-amber-500/50 space-y-3 text-amber-200 text-xs">
            <div className="flex items-start space-x-2.5">
              <AlertCircle className="w-4 h-4 flex-shrink-0 mt-0.5 text-amber-400" />
              <div>
                <p className="font-bold text-amber-300">Email Verification Required</p>
                <p className="mt-0.5 text-slate-300">
                  Your account requires email verification before logging in. Please check your inbox for the activation link.
                </p>
              </div>
            </div>

            {resendNotice && (
              <div className="p-2.5 rounded bg-emerald-950/60 border border-emerald-500/30 text-emerald-300">
                {resendNotice}
              </div>
            )}

            <button
              type="button"
              onClick={handleResendVerification}
              disabled={resending}
              className="w-full py-2 px-3 rounded bg-amber-900/60 hover:bg-amber-900 border border-amber-500/40 text-amber-200 font-mono text-xs font-bold transition-colors flex items-center justify-center gap-1.5 disabled:opacity-50"
            >
              {resending ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <MailCheck className="w-3.5 h-3.5" />}
              <span>Resend Verification Email</span>
            </button>
          </div>
        ) : error && (
          <div className="mb-6 p-3 rounded-lg bg-rose-950/40 border border-rose-500/50 flex items-start space-x-3 text-rose-300 text-sm">
            <AlertCircle className="w-5 h-5 flex-shrink-0 mt-0.5 text-rose-400" />
            <span>{error}</span>
          </div>
        )}

        <form onSubmit={handleSubmit} className="space-y-5">
          <div>
            <label className="block text-xs font-mono font-medium text-slate-300 mb-1.5 uppercase tracking-wider">
              Username
            </label>
            <input
              type="text"
              required
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              className="w-full px-4 py-2.5 rounded-lg bg-obsidian-950 border border-obsidian-700 focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500 text-slate-100 placeholder-slate-500 font-mono text-sm transition-all"
              placeholder="admin"
            />
          </div>

          <div>
            <label className="block text-xs font-mono font-medium text-slate-300 mb-1.5 uppercase tracking-wider">
              Password
            </label>
            <input
              type="password"
              required
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className="w-full px-4 py-2.5 rounded-lg bg-obsidian-950 border border-obsidian-700 focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500 text-slate-100 placeholder-slate-500 font-mono text-sm transition-all"
              placeholder="••••••••••••"
            />
          </div>

          <div className="flex items-center justify-between text-xs">
            <label className="flex items-center space-x-2 text-slate-400 cursor-pointer">
              <input
                type="checkbox"
                checked={rememberMe}
                onChange={(e) => setRememberMe(e.target.checked)}
                className="rounded bg-obsidian-950 border-obsidian-700 text-emerald-500 focus:ring-emerald-500/30"
              />
              <span>Remember session</span>
            </label>
            <Link to="/forgot-password" className="text-emerald-400 hover:underline font-medium">
              Forgot Password?
            </Link>
          </div>

          {isTurnstileRequired && (
            <TurnstileWidget
              ref={turnstileRef}
              siteKey={turnstileConfig!.site_key}
              action="login"
              onVerify={(token) => setTurnstileToken(token)}
              onExpire={() => setTurnstileToken('')}
              onError={() => setTurnstileToken('')}
            />
          )}

          <button
            type="submit"
            disabled={loading || (isTurnstileRequired && !turnstileToken)}
            className="w-full py-3 px-4 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-slate-950 font-bold font-mono uppercase tracking-wider transition-all shadow-[0_0_20px_rgba(16,185,129,0.3)] hover:shadow-[0_0_25px_rgba(16,185,129,0.5)] disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center space-x-2"
          >
            {loading ? <Loader2 className="w-5 h-5 animate-spin" /> : <span>Authenticate</span>}
          </button>
        </form>

        <div className="mt-6 text-center text-xs text-slate-400">
          Need an account?{' '}
          <Link to="/register" className="text-emerald-400 hover:underline font-medium">
            Register here
          </Link>
        </div>
      </div>
    </div>
  );
};
