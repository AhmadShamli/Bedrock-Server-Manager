import React, { useEffect, useState } from 'react';
import { useParams } from 'react-router-dom';
import { Shield, KeyRound, Play, RefreshCw, AlertCircle, CheckCircle2, Wifi, Clock, Loader2, Copy, Check, Timer } from 'lucide-react';
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
    game_server_address?: string;
    session_token?: string;
    always_allowed?: boolean;
    rule_comment?: string;
  } | null>(null);
  const [reconnecting, setReconnecting] = useState(false);
  const [heartbeatStatus, setHeartbeatStatus] = useState<string>('Active');
  const [copiedField, setCopiedField] = useState<string | null>(null);

  // Live Countdown Timer states
  const [secondsRemaining, setSecondsRemaining] = useState<number | null>(null);
  const [initialLeaseDuration, setInitialLeaseDuration] = useState<number>(7200);

  const copyToClipboard = (text: string, fieldId: string) => {
    navigator.clipboard.writeText(text);
    setCopiedField(fieldId);
    setTimeout(() => setCopiedField(null), 2000);
  };

  const formatTimerDigital = (totalSec: number | null): string => {
    if (totalSec === null) return '--:--:--';
    if (totalSec <= 0) return '00:00:00';
    const hours = Math.floor(totalSec / 3600);
    const minutes = Math.floor((totalSec % 3600) / 60);
    const seconds = totalSec % 60;
    return `${hours.toString().padStart(2, '0')}:${minutes.toString().padStart(2, '0')}:${seconds.toString().padStart(2, '0')}`;
  };

  const formatTimerHuman = (totalSec: number | null): string => {
    if (totalSec === null) return '';
    if (totalSec <= 0) return 'Expired';
    const hours = Math.floor(totalSec / 3600);
    const minutes = Math.floor((totalSec % 3600) / 60);
    const seconds = totalSec % 60;
    if (hours > 0) {
      return `${hours}h ${minutes}m ${seconds}s`;
    }
    if (minutes > 0) {
      return `${minutes}m ${seconds}s`;
    }
    return `${seconds}s`;
  };

  const formatDurationFriendly = (sec?: number): string => {
    if (!sec || sec <= 0) return '2 hours';
    const hrs = Math.floor(sec / 3600);
    const mins = Math.floor((sec % 3600) / 60);
    if (hrs > 0 && mins > 0) return `${hrs}h ${mins}m`;
    if (hrs > 0) return `${hrs} hour${hrs > 1 ? 's' : ''}`;
    return `${mins} minute${mins > 1 ? 's' : ''}`;
  };

  useEffect(() => {
    if (!id) return;

    const init = async () => {
      try {
        const conf = await api.getKnockConfig(id);
        setConfig(conf);

        if (conf.is_banned) {
          setLoading(false);
          return;
        }

        // Check if caller already has an active lease or is permanently allowed
        try {
          const status = await api.getKnockStatus(id);
          if (status.active || conf.always_allowed) {
            const effectiveHost = status.game_server_address || conf.game_server_address || window.location.hostname;
            setActiveLease({
              ip_address: status.ip_address || conf.client_ip || '',
              expires_in_seconds: status.expires_in_seconds || 0,
              direct_launch_url: status.direct_launch_url || `minecraft://?addExternalServer=${encodeURIComponent(conf.server_name)}|${effectiveHost}:${conf.port}`,
              server_name: conf.server_name,
              server_port: conf.port,
              game_server_address: effectiveHost,
              always_allowed: status.always_allowed || conf.always_allowed,
              rule_comment: status.rule_comment || conf.rule_comment,
            });
          }
        } catch (statusErr: any) {
          // If status returned banned or error, ignore if config already handled it
        }
      } catch (err: any) {
        setError(err.message || 'Failed to connect to Knock Portal');
      } finally {
        setLoading(false);
      }
    };

    init();
  }, [id]);

  // Live Timer Countdown Effect (ticks every second for temporary leases)
  useEffect(() => {
    if (!activeLease || activeLease.always_allowed) {
      setSecondsRemaining(null);
      return;
    }

    const duration = activeLease.expires_in_seconds || config?.port_gate_timeout || 7200;
    setInitialLeaseDuration((prev) => Math.max(prev, duration));

    const expiryTime = Date.now() + duration * 1000;

    const tick = () => {
      const remaining = Math.max(0, Math.floor((expiryTime - Date.now()) / 1000));
      setSecondsRemaining(remaining);
      if (remaining <= 0) {
        setHeartbeatStatus('Lease Expired');
      }
    };

    tick();
    const interval = setInterval(tick, 1000);
    return () => clearInterval(interval);
  }, [activeLease?.expires_in_seconds, activeLease?.always_allowed, config?.port_gate_timeout]);

  // Background Heartbeat for Mobile 4G/5G Roaming (only for temporary leases)
  useEffect(() => {
    if (!id || !activeLease?.session_token || activeLease.always_allowed) return;

    const intervalSec = config?.heartbeat_interval_seconds || 10;
    const interval = setInterval(async () => {
      try {
        const hb = await api.sendHeartbeat(id, activeLease.session_token);
        setActiveLease((prev) => {
          if (!prev) return null;
          return {
            ...prev,
            ip_address: hb.ip_updated ? hb.ip_address : prev.ip_address,
            expires_in_seconds: typeof hb.expires_in_seconds === 'number' ? hb.expires_in_seconds : prev.expires_in_seconds,
          };
        });
        if (hb.ip_updated) {
          setHeartbeatStatus(`Roaming handoff: IP updated to ${hb.ip_address}`);
        } else {
          setHeartbeatStatus('Synced');
        }
      } catch {
        setHeartbeatStatus('Disconnected');
      }
    }, intervalSec * 1000);

    return () => clearInterval(interval);
  }, [id, activeLease?.session_token, activeLease?.always_allowed, config?.heartbeat_interval_seconds]);

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

      const effectiveHost = res.game_server_address || config?.game_server_address || window.location.hostname;
      setActiveLease({
        ip_address: res.ip_address,
        expires_in_seconds: res.expires_in_seconds,
        direct_launch_url: res.direct_launch_url,
        server_name: res.server_name,
        server_port: res.server_port,
        game_server_address: effectiveHost,
        session_token: res.session_token,
        always_allowed: res.always_allowed,
        rule_comment: res.rule_comment,
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
      setActiveLease((prev) => prev ? {
        ...prev,
        ip_address: hb.ip_address,
        expires_in_seconds: typeof hb.expires_in_seconds === 'number' ? hb.expires_in_seconds : prev.expires_in_seconds,
      } : null);
    } catch (err: any) {
      setError('Session expired. Please re-enter your knock credentials.');
      setActiveLease(null);
    } finally {
      setReconnecting(false);
    }
  };

  const gameServerHost = activeLease?.game_server_address || config?.game_server_address || window.location.hostname;
  const gameServerPort = activeLease?.server_port || config?.port || 19132;

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
            Dynamic Port Gate Access Portal ({gameServerHost}:{gameServerPort})
          </p>
        </div>

        {error && (
          <div className="mb-6 p-3 rounded-xl bg-rose-950/40 border border-rose-500/50 flex items-start space-x-3 text-rose-300 text-xs">
            <AlertCircle className="w-4 h-4 flex-shrink-0 mt-0.5 text-rose-400" />
            <span>{error}</span>
          </div>
        )}

        {config?.is_banned ? (
          /* BANNED IP SCREEN */
          <div className="space-y-6">
            <div className="p-5 rounded-xl bg-rose-950/40 border border-rose-500/50 text-center shadow-[0_0_25px_rgba(244,63,94,0.15)]">
              <div className="inline-flex items-center justify-center w-12 h-12 rounded-full bg-rose-500/20 text-rose-400 mb-3 border border-rose-500/30">
                <AlertCircle className="w-7 h-7" />
              </div>
              <div className="block mb-2">
                <span className="px-2.5 py-0.5 rounded-full bg-rose-500/20 text-rose-400 text-[10px] font-bold font-mono uppercase tracking-widest border border-rose-500/30">
                  Access Blocked
                </span>
              </div>
              <h2 className="text-base font-bold font-mono text-rose-300">YOUR IP ADDRESS IS BANNED</h2>
              <p className="text-xs text-slate-300 mt-2 font-mono leading-relaxed">
                Your IP address <span className="text-rose-400 font-bold bg-obsidian-950 px-2 py-0.5 rounded border border-rose-500/30">{config.client_ip}</span> is banned from accessing this server.
              </p>
              {config.ban_reason && (
                <div className="mt-3 p-3 bg-obsidian-950/80 rounded-lg border border-rose-500/20 text-left">
                  <span className="text-[10px] uppercase tracking-wider text-slate-400 block font-mono">Reason for Ban</span>
                  <p className="text-xs text-rose-300 font-mono mt-0.5">"{config.ban_reason}"</p>
                </div>
              )}
            </div>
          </div>
        ) : activeLease ? (
          activeLease.always_allowed ? (
            /* PERMANENT ACCESS UNLOCKED SCREEN */
            <div className="space-y-6">
              <div className="p-5 rounded-xl bg-emerald-950/40 border border-emerald-500/50 text-center shadow-[0_0_25px_rgba(16,185,129,0.15)]">
                <div className="inline-flex items-center justify-center w-12 h-12 rounded-full bg-emerald-500/20 text-emerald-400 mb-3 border border-emerald-500/30">
                  <CheckCircle2 className="w-7 h-7" />
                </div>
                <div className="block mb-2">
                  <span className="px-2.5 py-0.5 rounded-full bg-emerald-500/20 text-emerald-400 text-[10px] font-bold font-mono uppercase tracking-widest border border-emerald-500/30">
                    Always Allowed
                  </span>
                </div>
                <h2 className="text-base font-bold font-mono text-emerald-300">ACCESS ALREADY AUTHORIZED</h2>
                <p className="text-xs text-slate-300 mt-2 font-mono leading-relaxed">
                  Your IP address <span className="text-emerald-400 font-bold bg-obsidian-950 px-2 py-0.5 rounded border border-emerald-500/30">{activeLease.ip_address}</span> is on the permanent allowlist.
                </p>
                {activeLease.rule_comment && (
                  <p className="text-[11px] text-slate-400 mt-2 font-mono italic">
                    "{activeLease.rule_comment}"
                  </p>
                )}
                <div className="mt-3 text-[11px] text-emerald-400/80 font-mono">
                  No login or knock passphrase required to connect.
                </div>
              </div>

              {/* Direct Launch Button */}
              <a
                href={activeLease.direct_launch_url}
                className="w-full py-3.5 px-4 rounded-xl bg-emerald-500 hover:bg-emerald-400 text-slate-950 font-bold font-mono text-sm tracking-wider flex items-center justify-center space-x-2 transition-all shadow-[0_0_25px_rgba(16,185,129,0.4)] hover:shadow-[0_0_35px_rgba(16,185,129,0.6)]"
              >
                <Play className="w-5 h-5 fill-current" />
                <span>Launch Minecraft Bedrock</span>
              </a>

              {/* Server Connection Info */}
              <div className="bg-obsidian-950 p-4 rounded-xl border border-obsidian-800 space-y-2.5 font-mono text-xs">
                <div className="flex items-center justify-between text-slate-400">
                  <span>Server Address:</span>
                  <div className="flex items-center space-x-1.5">
                    <span className="text-slate-200 font-semibold">{gameServerHost}</span>
                    <button
                      type="button"
                      onClick={() => copyToClipboard(gameServerHost, 'always-host')}
                      className="text-slate-500 hover:text-emerald-400 p-1 rounded hover:bg-obsidian-800 transition-colors"
                      title="Copy server address"
                    >
                      {copiedField === 'always-host' ? <Check className="w-3.5 h-3.5 text-emerald-400" /> : <Copy className="w-3.5 h-3.5" />}
                    </button>
                  </div>
                </div>
                <div className="flex items-center justify-between text-slate-400">
                  <span>UDP Port:</span>
                  <div className="flex items-center space-x-1.5">
                    <span className="text-emerald-400 font-semibold">{gameServerPort}</span>
                    <button
                      type="button"
                      onClick={() => copyToClipboard(String(gameServerPort), 'always-port')}
                      className="text-slate-500 hover:text-emerald-400 p-1 rounded hover:bg-obsidian-800 transition-colors"
                      title="Copy port"
                    >
                      {copiedField === 'always-port' ? <Check className="w-3.5 h-3.5 text-emerald-400" /> : <Copy className="w-3.5 h-3.5" />}
                    </button>
                  </div>
                </div>
                <div className="flex items-center justify-between text-slate-400">
                  <span>Firewall Status:</span>
                  <span className="text-emerald-400 font-semibold flex items-center space-x-1.5">
                    <span className="w-2 h-2 rounded-full bg-emerald-400 animate-pulse"></span>
                    <span>Permanently Open</span>
                  </span>
                </div>
                <div className="flex items-center justify-between text-slate-400">
                  <span>Lease Timer:</span>
                  <span className="text-emerald-400 font-semibold flex items-center space-x-1.5">
                    <Timer className="w-3.5 h-3.5 text-emerald-400" />
                    <span>Permanent Whitelist</span>
                  </span>
                </div>
              </div>
            </div>
          ) : (
            /* TEMPORARY LEASE UNLOCKED SCREEN */
            <div className="space-y-5">
              <div className="p-4 rounded-xl bg-emerald-950/30 border border-emerald-500/40 text-center">
                <div className="inline-flex items-center justify-center w-10 h-10 rounded-full bg-emerald-500/20 text-emerald-400 mb-2">
                  <CheckCircle2 className="w-6 h-6" />
                </div>
                <h2 className="text-base font-bold font-mono text-emerald-400">ACCESS UNLOCKED</h2>
                <p className="text-xs text-slate-300 mt-1 font-mono">
                  Your IP <span className="text-emerald-300 font-bold">{activeLease.ip_address}</span> is authorized on the firewall.
                </p>
              </div>

              {/* Live Countdown Timer Card */}
              <div className={`p-4 rounded-xl border text-center transition-all ${
                secondsRemaining !== null && secondsRemaining <= 0
                  ? 'bg-rose-950/40 border-rose-500/50 shadow-[0_0_25px_rgba(244,63,94,0.2)]'
                  : secondsRemaining !== null && secondsRemaining < 300
                  ? 'bg-amber-950/40 border-amber-500/50 shadow-[0_0_25px_rgba(245,158,11,0.2)]'
                  : 'bg-emerald-950/40 border-emerald-500/40 shadow-[0_0_25px_rgba(16,185,129,0.15)]'
              }`}>
                <div className="flex items-center justify-center space-x-2 mb-1.5">
                  <Timer className={`w-4 h-4 ${
                    secondsRemaining !== null && secondsRemaining <= 0
                      ? 'text-rose-400'
                      : secondsRemaining !== null && secondsRemaining < 300
                      ? 'text-amber-400 animate-pulse'
                      : 'text-emerald-400 animate-pulse'
                  }`} />
                  <span className="text-[11px] font-mono uppercase tracking-widest text-slate-300 font-bold">
                    {secondsRemaining !== null && secondsRemaining <= 0
                      ? 'Firewall Lease Expired'
                      : 'Firewall Access Lease Timer'}
                  </span>
                </div>

                <div className={`font-mono text-3xl sm:text-4xl font-black tracking-wider my-2 tabular-nums ${
                  secondsRemaining !== null && secondsRemaining <= 0
                    ? 'text-rose-400'
                    : secondsRemaining !== null && secondsRemaining < 300
                    ? 'text-amber-400'
                    : 'text-emerald-400'
                }`}>
                  {formatTimerDigital(secondsRemaining)}
                </div>

                <div className="flex items-center justify-between text-[11px] font-mono text-slate-400 mt-2 px-1">
                  <span>Port Gating Active</span>
                  <span className={secondsRemaining !== null && secondsRemaining < 300 ? 'text-amber-300 font-bold' : 'text-slate-300'}>
                    {formatTimerHuman(secondsRemaining)}
                  </span>
                </div>

                {/* Animated Progress Bar */}
                {initialLeaseDuration > 0 && secondsRemaining !== null && (
                  <div className="w-full h-2 bg-obsidian-950 rounded-full mt-2.5 overflow-hidden border border-obsidian-800">
                    <div
                      className={`h-full transition-all duration-1000 rounded-full ${
                        secondsRemaining <= 0
                          ? 'bg-rose-500 w-full'
                          : secondsRemaining < 300
                          ? 'bg-amber-400'
                          : 'bg-emerald-400'
                      }`}
                      style={{
                        width: `${Math.min(100, Math.max(0, (secondsRemaining / initialLeaseDuration) * 100))}%`,
                      }}
                    />
                  </div>
                )}

                {/* If Expired, show Re-knock / Renew button */}
                {secondsRemaining !== null && secondsRemaining <= 0 && (
                  <button
                    type="button"
                    onClick={() => setActiveLease(null)}
                    className="w-full mt-3 py-2 px-3 rounded-lg bg-rose-600 hover:bg-rose-500 text-slate-950 font-bold font-mono text-xs flex items-center justify-center space-x-1.5 transition-colors"
                  >
                    <RefreshCw className="w-3.5 h-3.5" />
                    <span>Renew Access (Knock Again)</span>
                  </button>
                )}
              </div>

              {/* Direct Launch Button */}
              <a
                href={activeLease.direct_launch_url}
                className="w-full py-3.5 px-4 rounded-xl bg-emerald-500 hover:bg-emerald-400 text-slate-950 font-bold font-mono text-sm tracking-wider flex items-center justify-center space-x-2 transition-all shadow-[0_0_25px_rgba(16,185,129,0.4)] hover:shadow-[0_0_35px_rgba(16,185,129,0.6)]"
              >
                <Play className="w-5 h-5 fill-current" />
                <span>Launch Minecraft Bedrock</span>
              </a>

              {/* Server Connection Info */}
              <div className="bg-obsidian-950 p-4 rounded-xl border border-obsidian-800 space-y-2.5 font-mono text-xs">
                <div className="flex items-center justify-between text-slate-400">
                  <span>Server Address:</span>
                  <div className="flex items-center space-x-1.5">
                    <span className="text-slate-200 font-semibold">{gameServerHost}</span>
                    <button
                      type="button"
                      onClick={() => copyToClipboard(gameServerHost, 'temp-host')}
                      className="text-slate-500 hover:text-emerald-400 p-1 rounded hover:bg-obsidian-800 transition-colors"
                      title="Copy server address"
                    >
                      {copiedField === 'temp-host' ? <Check className="w-3.5 h-3.5 text-emerald-400" /> : <Copy className="w-3.5 h-3.5" />}
                    </button>
                  </div>
                </div>
                <div className="flex items-center justify-between text-slate-400">
                  <span>UDP Port:</span>
                  <div className="flex items-center space-x-1.5">
                    <span className="text-emerald-400 font-semibold">{gameServerPort}</span>
                    <button
                      type="button"
                      onClick={() => copyToClipboard(String(gameServerPort), 'temp-port')}
                      className="text-slate-500 hover:text-emerald-400 p-1 rounded hover:bg-obsidian-800 transition-colors"
                      title="Copy port"
                    >
                      {copiedField === 'temp-port' ? <Check className="w-3.5 h-3.5 text-emerald-400" /> : <Copy className="w-3.5 h-3.5" />}
                    </button>
                  </div>
                </div>
                <div className="flex items-center justify-between text-slate-400">
                  <span>Access Lease:</span>
                  <span className="text-emerald-400 font-semibold flex items-center space-x-1.5">
                    <span className="w-2 h-2 rounded-full bg-emerald-400 animate-pulse"></span>
                    <span>Unlocked for {activeLease.ip_address}</span>
                  </span>
                </div>
                <div className="flex items-center justify-between text-slate-400">
                  <span>Lease Timer:</span>
                  <span className={`font-semibold flex items-center space-x-1.5 tabular-nums ${
                    secondsRemaining !== null && secondsRemaining <= 0
                      ? 'text-rose-400'
                      : secondsRemaining !== null && secondsRemaining < 300
                      ? 'text-amber-400'
                      : 'text-emerald-400'
                  }`}>
                    <Clock className="w-3.5 h-3.5 inline" />
                    <span>{formatTimerHuman(secondsRemaining)}</span>
                  </span>
                </div>
              </div>

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
          )
        ) : (
          /* UNLOCK FORM */
          <form onSubmit={handleKnock} className="space-y-4">
            {/* Session Lease Timer Badge */}
            <div className="flex items-center justify-between p-2.5 rounded-lg bg-obsidian-950 border border-obsidian-800 text-[11px] font-mono text-slate-400">
              <span className="flex items-center gap-1.5">
                <Timer className="w-3.5 h-3.5 text-cyber-cyan" />
                <span>Session Lease Duration:</span>
              </span>
              <span className="text-emerald-400 font-semibold">
                {formatDurationFriendly(config?.port_gate_timeout || 7200)}
              </span>
            </div>
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
              Dynamic IP verification grants immediate access to {gameServerHost}:{gameServerPort}.
            </p>
          </form>
        )}

        {/* Footer */}
        <div className="mt-6 pt-4 border-t border-obsidian-800 text-center font-mono text-[11px] text-slate-500 flex items-center justify-center space-x-2">
          <span>&copy; {new Date().getFullYear()} Bedrock Server Manager</span>
          <span>•</span>
          <a
            href="https://github.com/AhmadShamli/Bedrock-Server-Manager"
            target="_blank"
            rel="noopener noreferrer"
            className="hover:text-emerald-400 transition-colors"
          >
            GitHub
          </a>
        </div>
      </div>
    </div>
  );
};
