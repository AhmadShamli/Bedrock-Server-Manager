import React, { useState, useEffect } from 'react';
import { useSearchParams, Link } from 'react-router-dom';
import { CheckCircle2, AlertCircle, Loader2, MailCheck, ArrowLeft, Send } from 'lucide-react';
import { api } from '../api/client';

export const VerifyEmail: React.FC = () => {
  const [searchParams] = useSearchParams();
  const token = searchParams.get('token') || '';

  const [loading, setLoading] = useState(!!token);
  const [success, setSuccess] = useState(false);
  const [error, setError] = useState<string | null>(token ? null : 'No verification token provided.');

  // Resend state
  const [resendEmail, setResendEmail] = useState('');
  const [resending, setResending] = useState(false);
  const [resendMessage, setResendMessage] = useState<string | null>(null);

  useEffect(() => {
    if (!token) return;

    let mounted = true;
    api.verifyEmail(token)
      .then(() => {
        if (mounted) {
          setSuccess(true);
          setError(null);
        }
      })
      .catch((err: any) => {
        if (mounted) {
          setError(err.message || 'Verification link is invalid or has expired.');
          setSuccess(false);
        }
      })
      .finally(() => {
        if (mounted) setLoading(false);
      });

    return () => {
      mounted = false;
    };
  }, [token]);

  const handleResend = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!resendEmail.trim()) return;

    setResending(true);
    setResendMessage(null);

    try {
      const res = await api.resendVerification({ email_or_username: resendEmail.trim() });
      setResendMessage(res.message || 'A new verification link has been sent to your email.');
    } catch (err: any) {
      setResendMessage(err.message || 'Failed to resend verification link.');
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
            <MailCheck className="w-7 h-7" />
          </div>
          <h1 className="text-2xl font-bold font-mono tracking-wide text-slate-100">EMAIL VERIFICATION</h1>
          <p className="text-sm text-slate-400 mt-1">Bedrock Dedicated Server Manager</p>
        </div>

        {loading && (
          <div className="py-12 flex flex-col items-center justify-center gap-4 text-center">
            <Loader2 className="w-10 h-10 animate-spin text-emerald-400" />
            <p className="font-mono text-sm text-slate-300">Verifying your email address...</p>
          </div>
        )}

        {!loading && success && (
          <div className="space-y-6 text-center">
            <div className="p-4 rounded-xl bg-emerald-950/40 border border-emerald-500/40 text-emerald-300 text-sm flex flex-col items-center gap-3">
              <CheckCircle2 className="w-10 h-10 text-emerald-400" />
              <p className="font-bold text-base text-slate-100">Email Address Verified!</p>
              <p className="text-xs text-slate-300 leading-relaxed">
                Your email address has been confirmed and your account is now fully active. You may proceed to the management dashboard.
              </p>
            </div>
            <Link
              to="/login"
              className="inline-flex items-center justify-center w-full py-3 px-4 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-slate-950 font-bold font-mono text-sm transition-all shadow-[0_0_20px_rgba(16,185,129,0.3)]"
            >
              Continue to Login
            </Link>
          </div>
        )}

        {!loading && !success && (
          <div className="space-y-6">
            <div className="p-3.5 rounded-lg bg-rose-950/40 border border-rose-500/50 flex items-start space-x-3 text-rose-300 text-sm">
              <AlertCircle className="w-5 h-5 flex-shrink-0 mt-0.5 text-rose-400" />
              <span>{error}</span>
            </div>

            <div className="pt-2 border-t border-obsidian-800">
              <h3 className="text-xs font-mono font-bold text-slate-300 mb-2 uppercase tracking-wider">
                Need a new verification link?
              </h3>
              <p className="text-xs text-slate-400 mb-4">
                Enter your username or email address below to receive another confirmation link:
              </p>

              {resendMessage && (
                <div className="mb-4 p-3 rounded-lg bg-emerald-950/40 border border-emerald-500/30 text-emerald-300 text-xs">
                  {resendMessage}
                </div>
              )}

              <form onSubmit={handleResend} className="space-y-4">
                <input
                  type="text"
                  required
                  value={resendEmail}
                  onChange={(e) => setResendEmail(e.target.value)}
                  className="w-full px-4 py-2.5 rounded-lg bg-obsidian-950 border border-obsidian-700 focus:border-emerald-500 text-slate-100 placeholder-slate-500 font-mono text-sm"
                  placeholder="Username or registered email"
                />

                <button
                  type="submit"
                  disabled={resending || !resendEmail.trim()}
                  className="w-full py-2.5 px-4 rounded-lg bg-obsidian-800 hover:bg-obsidian-750 border border-obsidian-700 text-slate-200 font-bold font-mono text-xs flex items-center justify-center gap-2 transition-colors disabled:opacity-50"
                >
                  {resending ? <Loader2 className="w-4 h-4 animate-spin" /> : <Send className="w-4 h-4 text-emerald-400" />}
                  <span>Resend Verification Link</span>
                </button>
              </form>
            </div>

            <div className="text-center pt-2">
              <Link to="/login" className="inline-flex items-center gap-1.5 text-xs text-slate-400 hover:text-emerald-400 transition-colors">
                <ArrowLeft className="w-3.5 h-3.5" />
                <span>Return to Login</span>
              </Link>
            </div>
          </div>
        )}
      </div>
    </div>
  );
};
