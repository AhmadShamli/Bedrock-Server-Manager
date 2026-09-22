import React, { useEffect, useState } from 'react';
import { useParams } from 'react-router-dom';
import { Shield, KeyRound, Play, RefreshCw, AlertCircle, CheckCircle2, Wifi, Clock, Loader2 } from 'lucide-react';
import { api } from '../api/client';
import { KnockConfig } from '../types';

export const KnockPortal: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const [config, setConfig] = useState<KnockConfig | null>(null);
  const [gamertag, setGamertag] = useState('');
  const [passphrase, setPassphrase] = useState('');
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [activeLease, setActiveLease] = useState<{
    ip_address: string;
    expires_in_seconds: number;
    direct_launch_url: string;
    server_name: string;
    server_port: number;
    session_token?: string;
  } | null>(null);
  const [reconnecting, setReconnecting] = useState(false);
  const [heartbeatStatus, setHeartbeatStatus] = useState<string>('Active');

  useEffect(() => {
    if (!id) return;

    const init = async () => {
      try {
        const conf = await api.getKnockConfig(id);
        setConfig(conf);

        // Check if caller already has an active lease
        const status = await api.getKnockStatus(id);
        if (status.active) {
          setActiveLease({
            ip_address: status.ip_address,
            expires_in_seconds: status.expires_in_seconds || 3600,
            direct_launch_url: status.direct_launch_url || `minecraft://?addExternalServer=${conf.server_name}|${window.location.host}:${conf.port}`,
            server_name: conf.server_name,
            server_port: conf.port,
          });
        }
      } catch (err: any) {
        setError(err.message || 'Failed to connect to Knock Portal');
      } finally {
        setLoading(false);
      }
    };

    init();
  }, [id]);

  // Background Heartbeat for Mobile 4G/5G Roaming (Configurable, default 10s)
  useEffect(() => {
    if (!id || !activeLease) return;

    const intervalSec = config?.heartbeat_interval_seconds || 10;
    const interval = setInterval(async () => {
      try {
        const hb = await api.sendHeartbeat(id, activeLease.session_token);
        if (hb.ip_updated) {
          setActiveLease((prev) => prev ? { ...prev, ip_address: hb.ip_address } : null);
          setHeartbeatStatus(`Roaming handoff: IP updated to ${hb.ip_address}`);
        } else {
          setHeartbeatStatus('Synced');
        }
      } catch {
        setHeartbeatStatus('Disconnected');
      }
    }, intervalSec * 1000);

    return () => clearInterval(interval);
  }, [id, activeLease, config]);

  const handleKnock = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!id) return;

    setError(null);
    setSubmitting(true);

    try {
      const res = await api.knock(id, {
        gamertag: gamertag.trim(),
        passphrase: passphrase.trim(),
      });

      setActiveLease({
        ip_address: res.ip_address,
        expires_in_seconds: res.expires_in_seconds,
        direct_launch_url: res.direct_launch_url,
        server_name: res.server_name,
        server_port: res.server_port,
        session_token: res.session_token,
      });
    } catch (err: any) {
      setError(err.message || 'Authorization failed');
    } finally {
      setSubmitting(false);
    }
  };

  const handle1TapReconnect = async () => {
    if (!id) return;
    setReconnecting(true);
    setError(null);
    try {
      const hb = await api.sendHeartbeat(id, activeLease?.session_token);
      setActiveLease((prev) => prev ? { ...prev, ip_address: hb.ip_address } : null);
    } catch (err: any) {
      setError('Session expired. Please re-enter your knock credentials.');
      setActiveLease(null);
    } finally {
      setReconnecting(false);
    }
  };

  if (loading) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-obsidian-950 text-slate-300">
        <Loader2 className="w-8 h-8 animate-spin text-emerald-400 mb-2" />
      </div>
    );
  }

  return (
    <div className="min-h-screen flex items-center justify-center p-4 bg-obsidian-950 relative overflow-hidden">
      {/* Glow Effects */}
      <div className="absolute top-1/4 left-1/2 -translate-x-1/2 w-96 h-96 bg-emerald-500/10 rounded-full blur-3xl pointer-events-none"></div>

      <div className="w-full max-w-md bg-obsidian-900 border border-obsidian-700/80 rounded-2xl p-6 sm:p-8 shadow-2xl relative backdrop-blur-xl">
        {/* Header */}
        <div className="text-center mb-6">
          <div className="inline-flex items-center justify-center w-14 h-14 rounded-2xl bg-emerald-950/80 border border-emerald-500/40 text-emerald-400 mb-3 shadow-[0_0_25px_rgba(16,185,129,0.25)]">
            <Shield className="w-7 h-7" />
          </div>
          <h1 className="text-xl font-bold font-mono tracking-wider text-slate-100">
            {config?.server_name || 'BEDROCK SERVER'}
          </h1>
          <p className="text-xs text-slate-400 font-mono mt-1">
            Dynamic Port Gate Access Portal (UDP {config?.port || 19132})
          </p>
        </div>

        {error && (
          <div className="mb-6 p-3 rounded-xl bg-rose-950/40 border border-rose-500/50 flex items-start space-x-3 text-rose-300 text-xs">
            <AlertCircle className="w-4 h-4 flex-shrink-0 mt-0.5 text-rose-400" />
            <span>{error}</span>
          </div>
        )}

        {activeLease ? (
          /* UNLOCKED SCREEN */
          <div className="space-y-6">
            <div className="p-4 rounded-xl bg-emerald-950/30 border border-emerald-500/40 text-center">
              <div className="inline-flex items-center justify-center w-10 h-10 rounded-full bg-emerald-500/20 text-emerald-400 mb-2">
                <CheckCircle2 className="w-6 h-6" />
              </div>
              <h2 className="text-base font-bold font-mono text-emerald-400">ACCESS UNLOCKED</h2>
              <p className="text-xs text-slate-300 mt-1 font-mono">
                Your IP <span className="text-emerald-300 font-bold">{activeLease.ip_address}</span> is authorized on the firewall.
              </p>
            </div>

            {/* Direct Launch Button */}
            <a
              href={activeLease.direct_launch_url}
              className="w-full py-3.5 px-4 rounded-xl bg-emerald-500 hover:bg-emerald-400 text-slate-950 font-bold font-mono text-sm tracking-wider flex items-center justify-center space-x-2 transition-all shadow-[0_0_25px_rgba(16,185,129,0.4)] hover:shadow-[0_0_35px_rgba(16,185,129,0.6)]"
            >
              <Play className="w-5 h-5 fill-current" />
              <span>Launch Minecraft Bedrock</span>
            </a>

            {/* Mobile Roaming & Heartbeat indicator */}
            <div className="bg-obsidian-950 p-4 rounded-xl border border-obsidian-800 space-y-3 font-mono text-xs">
              <div className="flex items-center justify-between text-slate-400">
                <span className="flex items-center space-x-2">
                  <Wifi className="w-4 h-4 text-emerald-400 animate-pulse" />
                  <span>Mobile Roaming Guard</span>
                </span>
                <span className="text-emerald-400 text-[11px]">{heartbeatStatus}</span>
              </div>
              <div className="flex items-center justify-between text-slate-400">
                <span className="flex items-center space-x-2">
                  <Clock className="w-4 h-4 text-cyber-cyan" />
                  <span>Heartbeat Interval</span>
                </span>
                <span className="text-slate-200">{config?.heartbeat_interval_seconds || 10}s</span>
              </div>
            </div>

            {/* 1-Tap Reconnect Button */}
            <button
              onClick={handle1TapReconnect}
              disabled={reconnecting}
              className="w-full py-2.5 px-3 rounded-lg bg-obsidian-800 hover:bg-obsidian-750 border border-obsidian-700 text-slate-300 hover:text-emerald-400 font-mono text-xs flex items-center justify-center space-x-2 transition-colors"
            >
              <RefreshCw className={`w-3.5 h-3.5 ${reconnecting ? 'animate-spin text-emerald-400' : ''}`} />
              <span>🔄 1-Tap Update My IP (Network Roamed)</span>
            </button>
          </div>
        ) : (
          /* UNLOCK FORM */
          <form onSubmit={handleKnock} className="space-y-4">
            {(config?.port_gate_mode === 'gamertag' || config?.port_gate_mode === 'combined') && (
              <div>
                <label className="block text-xs font-mono text-slate-300 mb-1.5 uppercase tracking-wider">
                  Minecraft Gamertag
                </label>
                <input
                  type="text"
                  required
                  value={gamertag}
                  onChange={(e) => setGamertag(e.target.value)}
                  className="w-full px-3.5 py-2.5 rounded-lg bg-obsidian-950 border border-obsidian-700 focus:border-emerald-500 text-slate-100 font-mono text-sm"
                  placeholder="Steve"
                />
              </div>
            )}

            {(config?.port_gate_mode === 'passphrase' || config?.port_gate_mode === 'combined') && (
              <div>
                <label className="block text-xs font-mono text-slate-300 mb-1.5 uppercase tracking-wider">
                  Access Passphrase
                </label>
                <input
                  type="password"
                  required
                  value={passphrase}
                  onChange={(e) => setPassphrase(e.target.value)}
                  className="w-full px-3.5 py-2.5 rounded-lg bg-obsidian-950 border border-obsidian-700 focus:border-emerald-500 text-slate-100 font-mono text-sm"
                  placeholder="Enter knock secret..."
                />
              </div>
            )}

            <button
              type="submit"
              disabled={submitting}
              className="w-full py-3 px-4 rounded-lg bg-emerald-500 hover:bg-emerald-400 text-slate-950 font-bold font-mono uppercase tracking-wider transition-all shadow-[0_0_20px_rgba(16,185,129,0.3)] hover:shadow-[0_0_30px_rgba(16,185,129,0.5)] disabled:opacity-50 flex items-center justify-center space-x-2"
            >
              {submitting ? <Loader2 className="w-5 h-5 animate-spin" /> : (
                <>
                  <KeyRound className="w-4 h-4" />
                  <span>Unlock Firewall Access</span>
                </>
              )}
            </button>

            <p className="text-center text-[11px] text-slate-500 font-mono mt-3">
              Dynamic IP verification grants immediate access to UDP {config?.port || 19132}.
            </p>
          </form>
        )}
      </div>
    </div>
  );
};
