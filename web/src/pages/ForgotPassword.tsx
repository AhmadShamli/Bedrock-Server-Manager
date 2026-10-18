import React, { useState, useEffect, useRef } from 'react';
import { Link } from 'react-router-dom';
import { Mail, AlertCircle, CheckCircle2, Loader2, ArrowLeft } from 'lucide-react';
import { api } from '../api/client';
import { TurnstileConfig } from '../types';
import { TurnstileWidget, TurnstileWidgetRef } from '../components/TurnstileWidget';

export const ForgotPassword: React.FC = () => {
  const [emailOrUsername, setEmailOrUsername] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [submitted, setSubmitted] = useState(false);

  // Turnstile state
  const [turnstileConfig, setTurnstileConfig] = useState<TurnstileConfig | null>(null);
  const [turnstileToken, setTurnstileToken] = useState<string>('');
  const turnstileRef = useRef<TurnstileWidgetRef>(null);

  useEffect(() => {
    api.getTurnstileConfig()
      .then((cfg) => setTurnstileConfig(cfg))
      .catch(() => null);
  }, []);

  const isTurnstileRequired = !!(turnstileConfig?.enabled && turnstileConfig.site_key);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    if (isTurnstileRequired && !turnstileToken) {
      setError('Please complete the security challenge verification.');
      return;
    }

    setLoading(true);

    try {
      await api.forgotPassword({
        email_or_username: emailOrUsername.trim(),
        turnstile_token: turnstileToken || undefined,
      });
      setSubmitted(true);
    } catch (err: any) {
      setError(err.message || 'Failed to request password reset');
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
            <Mail className="w-7 h-7" />
          </div>
          <h1 className="text-2xl font-bold font-mono tracking-wide text-slate-100">FORGOT PASSWORD</h1>
          <p className="text-sm text-slate-400 mt-1">Enter your details to receive recovery instructions</p>
        </div>

        {error && (
          <div className="mb-6 p-3 rounded-lg bg-rose-950/40 border border-rose-500/50 flex items-start space-x-3 text-rose-300 text-sm">
            <AlertCircle className="w-5 h-5 flex-shrink-0 mt-0.5 text-rose-400" />
            <span>{error}</span>
          </div>
        )}

        {submitted ? (
          <div className="space-y-6 text-center">
            <div className="p-4 rounded-xl bg-emerald-950/40 border border-emerald-500/40 text-emerald-300 text-sm flex flex-col items-center gap-3">
              <CheckCircle2 className="w-8 h-8 text-emerald-400" />
              <p className="leading-relaxed">
                If an account matches that username or email address, password reset instructions have been sent. Please check your inbox and spam folder.
              </p>
            </div>
            <Link
              to="/login"
              className="inline-flex items-center justify-center gap-2 w-full py-2.5 px-4 rounded-lg bg-obsidian-800 hover:bg-obsidian-750 text-slate-200 font-mono text-sm border border-obsidian-700 transition-colors"
            >
              <ArrowLeft className="w-4 h-4" />
              <span>Return to Login</span>
            </Link>
          </div>
        ) : (
          <form onSubmit={handleSubmit} className="space-y-5">
            <div>
              <label className="block text-xs font-mono font-medium text-slate-300 mb-1.5 uppercase tracking-wider">
                Username or Registered Email
              </label>
              <input
                type="text"
                required
                value={emailOrUsername}
                onChange={(e) => setEmailOrUsername(e.target.value)}
                className="w-full px-4 py-2.5 rounded-lg bg-obsidian-950 border border-obsidian-700 focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500 text-slate-100 placeholder-slate-500 font-mono text-sm transition-all"
                placeholder="admin or user@example.com"
              />
            </div>

            {isTurnstileRequired && (
              <TurnstileWidget
                ref={turnstileRef}
                siteKey={turnstileConfig!.site_key}
                action="password_reset"
                onVerify={(token) => setTurnstileToken(token)}
                onExpire={() => setTurnstileToken('')}
                onError={() => setTurnstileToken('')}
              />
            )}

            <button
              type="submit"
              disabled={loading || !emailOrUsername.trim() || (isTurnstileRequired && !turnstileToken)}
              className="w-full py-3 px-4 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-slate-950 font-bold font-mono uppercase tracking-wider transition-all shadow-[0_0_20px_rgba(16,185,129,0.3)] hover:shadow-[0_0_25px_rgba(16,185,129,0.5)] disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center space-x-2"
            >
              {loading ? <Loader2 className="w-5 h-5 animate-spin" /> : <span>Send Reset Instructions</span>}
            </button>

            <div className="text-center pt-2">
              <Link to="/login" className="inline-flex items-center gap-1.5 text-xs text-slate-400 hover:text-emerald-400 transition-colors">
                <ArrowLeft className="w-3.5 h-3.5" />
                <span>Back to Login</span>
              </Link>
            </div>
          </form>
        )}
      </div>
    </div>
  );
};
