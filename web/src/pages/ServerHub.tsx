import React, { useEffect, useState, useRef } from 'react';
import { useParams, Link, useNavigate } from 'react-router-dom';
import {
  ArrowLeft, Play, Square, RefreshCw, Shield, Terminal, Settings, ExternalLink,
  HardDrive, Cpu, AlertTriangle, Loader2, Send, Users, MessageSquare, Copy,
  Download, UserPlus, ShieldAlert, Check, Lock, Unlock, Package, Archive,
  Upload, Trash2, Share2, Crown, Globe, UserMinus, Key, KeyRound, Compass,
  ChevronDown, ChevronRight, Plus, Activity, UserCog, Ban, X, Sparkles
} from 'lucide-react';
import { api } from '../api/client';
import { Server, User, Backup, AddonPack, PortGateLease, PortGateKey, GlobalPlayer, MetricsData, PortGateAllowRule, BannedPlayer } from '../types';
import { Pagination } from '../components/Pagination';
import { usePagination } from '../hooks/usePagination';
import { TelemetryCharts } from '../components/TelemetryCharts';

interface ServerHubProps {
  user: User;
}

export const ServerHub: React.FC<ServerHubProps> = ({ user }) => {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [server, setServer] = useState<Server | null>(null);
  const [loading, setLoading] = useState(true);
  const [activeTab, setActiveTab] = useState<'overview' | 'players' | 'portgate' | 'backups' | 'addons' | 'settings' | 'actions'>('overview');
  const [error, setError] = useState<string | null>(null);
  const [actionLoading, setActionLoading] = useState(false);

  // Port Gate Leases & Access Keys state
  const [leases, setLeases] = useState<PortGateLease[]>([]);
  const [accessKeys, setAccessKeys] = useState<PortGateKey[]>([]);
  const [showManualLeaseModal, setShowManualLeaseModal] = useState(false);
  const [manualIP, setManualIP] = useState('');
  const [manualGamertag, setManualGamertag] = useState('');
  const [manualDuration, setManualDuration] = useState(60);
  const [manualComment, setManualComment] = useState('');

  const [showNewKeyModal, setShowNewKeyModal] = useState(false);
  const [newKeyLabel, setNewKeyLabel] = useState('');
  const [newKeyPassphrase, setNewKeyPassphrase] = useState('');
  const [newKeyMaxUses, setNewKeyMaxUses] = useState(0);
  const [newKeyDurationMinutes, setNewKeyDurationMinutes] = useState(120);
  const [createdKeySecret, setCreatedKeySecret] = useState<string | null>(null);
  const [copiedSecret, setCopiedSecret] = useState(false);
  const [copiedSeed, setCopiedSeed] = useState(false);

  // Port Gate Permanent Allowlist state
  const [allowRules, setAllowRules] = useState<PortGateAllowRule[]>([]);
  const [showAddAllowModal, setShowAddAllowModal] = useState(false);
  const [newAllowIP, setNewAllowIP] = useState('');
  const [newAllowScope, setNewAllowScope] = useState<'server' | 'global'>('server');
  const [newAllowComment, setNewAllowComment] = useState('');
  const [allowSubmitting, setAllowSubmitting] = useState(false);
  const [detectedClientIP, setDetectedClientIP] = useState<string | null>(null);

  // Server Configuration Edit state
  const [editServerName, setEditServerName] = useState('');
  const [editServerVersion, setEditServerVersion] = useState('latest');
  const [editServerPort, setEditServerPort] = useState(19132);
  const [editServerPortV6, setEditServerPortV6] = useState(19133);
  const [editServerMode, setEditServerMode] = useState('survival');
  const [editServerDifficulty, setEditServerDifficulty] = useState('normal');
  const [editServerSeed, setEditServerSeed] = useState('');
  const [editServerMemLimit, setEditServerMemLimit] = useState('2G');
  const [editServerCpuLimit, setEditServerCpuLimit] = useState(2.0);
  const [editServerAutostart, setEditServerAutostart] = useState(false);
  const [editServerPortGate, setEditServerPortGate] = useState(false);
  const [editServerPortGateMode, setEditServerPortGateMode] = useState<'gamertag' | 'passphrase' | 'combined'>('passphrase');
  const [editServerPortGateTimeout, setEditServerPortGateTimeout] = useState(7200);
  const [editGameServerAddress, setEditGameServerAddress] = useState('');
  const [serverUpdating, setServerUpdating] = useState(false);
  const [serverUpdatedMsg, setServerUpdatedMsg] = useState<string | null>(null);
  const [propertyFilter, setPropertyFilter] = useState('');

  // Server Deletion state
  const [showDeleteServerModal, setShowDeleteServerModal] = useState(false);
  const [deleteConfirmText, setDeleteConfirmText] = useState('');
  const [serverDeleting, setServerDeleting] = useState(false);

  // Backups state
  const [backupsList, setBackupsList] = useState<Backup[]>([]);
  const [backupLoading, setBackupLoading] = useState(false);

  // Addons state
  const [addonsList, setAddonsList] = useState<AddonPack[]>([]);
  const [addonLoading, setAddonLoading] = useState(false);

  // Console WebSocket state
  const [logs, setLogs] = useState<string[]>([]);
  const [command, setCommand] = useState('');
  const [wsConnected, setWsConnected] = useState(false);
  const [terminalOpen, setTerminalOpen] = useState(false);
  const wsRef = useRef<WebSocket | null>(null);
  const consoleBottomRef = useRef<HTMLDivElement | null>(null);

  // Live Stats state
  const [stats, setStats] = useState<{ cpu_percent: number; ram_bytes: number; player_count: number } | null>(null);

  // Historical Telemetry Metrics state
  const [metrics, setMetrics] = useState<MetricsData | null>(null);
  const [metricsRange, setMetricsRange] = useState<'15m' | '1h' | '6h' | '24h'>('1h');
  const [metricsLoading, setMetricsLoading] = useState(false);

  // Player Hub & Chat state
  const [onlinePlayers, setOnlinePlayers] = useState<Array<{ gamertag: string; xuid: string; joined_at: string; is_op?: boolean; permission?: string }>>([]);
  const [chatFeed, setChatFeed] = useState<Array<{ gamertag: string; message: string; timestamp: string }>>([]);
  const [broadcastMsg, setBroadcastMsg] = useState('');
  const [broadcastTarget, setBroadcastTarget] = useState('');
  const [serverBans, setServerBans] = useState<BannedPlayer[]>([]);
  const [serverBansLoading, setServerBansLoading] = useState(false);

  // Configuration state
  const [properties, setProperties] = useState<Record<string, string>>({});
  const [propKeys, setPropKeys] = useState<string[]>([]);
  const [propUninitialized, setPropUninitialized] = useState(false);
  const [propPending, setPropPending] = useState(false);
  const [allowlist, setAllowlist] = useState<Array<{ name: string; xuid?: string; ignoresPlayerLimit: boolean }>>([]);
  const [newAllowlistPlayer, setNewAllowlistPlayer] = useState('');
  const [permissions, setPermissions] = useState<Array<{ permission: string; xuid: string }>>([]);
  const [newPermXuid, setNewPermXuid] = useState('');
  const [newPermRole, setNewPermRole] = useState<'operator' | 'member' | 'visitor'>('operator');
  const [configSaving, setConfigSaving] = useState(false);
  const [configSaved, setConfigSaved] = useState(false);
  const [globalPlayers, setGlobalPlayers] = useState<GlobalPlayer[]>([]);
  const [syncingGlobal, setSyncingGlobal] = useState(false);
  const [syncReportMsg, setSyncReportMsg] = useState<string | null>(null);

  // Clone & Export & Sync state
  const [cloneId, setCloneId] = useState('');
  const [cloneName, setCloneName] = useState('');
  const [cloning, setCloning] = useState(false);
  const [allServers, setAllServers] = useState<Server[]>([]);
  const [selectedTargetIds, setSelectedTargetIds] = useState<string[]>([]);
  const [copyAllowlist, setCopyAllowlist] = useState(true);
  const [copyPermissions, setCopyPermissions] = useState(true);
  const [copyProperties, setCopyProperties] = useState(false);
  const [copyMode, setCopyMode] = useState<'merge' | 'replace'>('merge');
  const [copying, setCopying] = useState(false);
  const [copyStatusMsg, setCopyStatusMsg] = useState<{ text: string; isError: boolean } | null>(null);

  // Version update info
  const [updateInfo, setUpdateInfo] = useState<{ current_version: string; latest_version: string; update_available: boolean; release_url: string } | null>(null);

  // Player Moderation & Management Modal state
  const [selectedPlayer, setSelectedPlayer] = useState<{
    gamertag: string;
    xuid: string;
    is_op?: boolean;
    ip_address?: string;
  } | null>(null);
  const [playerModalRole, setPlayerModalRole] = useState<'member' | 'operator'>('member');
  const [playerBanScope, setPlayerBanScope] = useState<'instance' | 'global'>('instance');
  const [playerBanReason, setPlayerBanReason] = useState('Banned by administrator');
  const [playerBanIP, setPlayerBanIP] = useState(true);
  const [playerBanLoading, setPlayerBanLoading] = useState(false);
  const [playerGlobalLoading, setPlayerGlobalLoading] = useState(false);

  // Pagination hooks for tables
  const {
    currentPage: leasesPage,
    pageSize: leasesPageSize,
    totalItems: totalLeases,
    paginatedItems: paginatedLeases,
    setCurrentPage: setLeasesPage,
    setPageSize: setLeasesPageSize,
  } = usePagination(leases, 10);

  const {
    currentPage: keysPage,
    pageSize: keysPageSize,
    totalItems: totalKeys,
    paginatedItems: paginatedKeys,
    setCurrentPage: setKeysPage,
    setPageSize: setKeysPageSize,
  } = usePagination(accessKeys, 10);

  const {
    currentPage: allowRulesPage,
    pageSize: allowRulesPageSize,
    totalItems: totalAllowRules,
    paginatedItems: paginatedAllowRules,
    setCurrentPage: setAllowRulesPage,
    setPageSize: setAllowRulesPageSize,
  } = usePagination(allowRules, 10);

  const {
    currentPage: backupsPage,
    pageSize: backupsPageSize,
    totalItems: totalBackups,
    paginatedItems: paginatedBackups,
    setCurrentPage: setBackupsPage,
    setPageSize: setBackupsPageSize,
  } = usePagination(backupsList, 10);

  const {
    currentPage: allowlistPage,
    pageSize: allowlistPageSize,
    totalItems: totalAllowlist,
    paginatedItems: paginatedAllowlist,
    setCurrentPage: setAllowlistPage,
    setPageSize: setAllowlistPageSize,
  } = usePagination(allowlist, 10);

  const {
    currentPage: permissionsPage,
    pageSize: permissionsPageSize,
    totalItems: totalPermissions,
    paginatedItems: paginatedPermissions,
    setCurrentPage: setPermissionsPage,
    setPageSize: setPermissionsPageSize,
  } = usePagination(permissions, 10);

  const fetchServer = async () => {
    if (!id) return;
    try {
      const data = await api.getServer(id);
      setServer(data);
    } catch (err: any) {
      setError(err.message || 'Server not found');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchServer();
  }, [id]);

  // Connect WebSocket for live logs (only when terminal panel is open)
  useEffect(() => {
    if (!id || activeTab !== 'overview' || !terminalOpen) {
      // Close any existing connection when terminal is collapsed or tab changes
      if (wsRef.current) {
        wsRef.current.close();
        wsRef.current = null;
        setWsConnected(false);
      }
      return;
    }

    // Reset logs so new connection displays fresh history without duplication
    setLogs([]);

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const token = localStorage.getItem('bsm_token') || '';
    const wsUrl = `${protocol}//${window.location.host}/api/servers/${id}/console/ws?token=${encodeURIComponent(token)}`;

    const ws = new WebSocket(wsUrl);
    wsRef.current = ws;

    ws.onopen = () => setWsConnected(true);
    ws.onmessage = (evt) => {
      try {
        const msg = JSON.parse(evt.data);
        if (msg.type === 'history' || msg.type === 'log') {
          setLogs((prev) => [...prev, msg.payload]);
        } else if (msg.type === 'error') {
          setLogs((prev) => [...prev, `[ERROR] ${msg.payload}`]);
        }
      } catch {
        setLogs((prev) => [...prev, evt.data]);
      }
    };
    ws.onclose = () => setWsConnected(false);

    return () => {
      ws.close();
      wsRef.current = null;
    };
  }, [id, activeTab, terminalOpen]);

  // Auto-scroll console
  useEffect(() => {
    consoleBottomRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [logs]);

  // Live Stats Poller
  useEffect(() => {
    if (!id || server?.status !== 'running') {
      setStats(null);
      return;
    }
    const fetchLiveStats = async () => {
      try {
        const res = await api.getStats(id);
        setStats(res);
      } catch {}
    };
    fetchLiveStats();
    const interval = setInterval(fetchLiveStats, 2000);
    return () => clearInterval(interval);
  }, [id, server?.status]);

  const fetchMetrics = async (range: '15m' | '1h' | '6h' | '24h' = metricsRange, silent = false) => {
    if (!id) return;
    if (!silent) setMetricsLoading(true);
    try {
      const data = await api.getMetrics(id, range);
      setMetrics(data);
    } catch {
      // Quietly ignore telemetry fetch errors
    } finally {
      if (!silent) setMetricsLoading(false);
    }
  };

  // Historical Telemetry Poller
  useEffect(() => {
    if (!id || activeTab !== 'overview') return;
    fetchMetrics(metricsRange);

    const intervalTime = server?.status === 'running' ? 8000 : 30000;
    const interval = setInterval(() => {
      fetchMetrics(metricsRange, true);
    }, intervalTime);

    return () => clearInterval(interval);
  }, [id, activeTab, metricsRange, server?.status]);

  // Periodic player list & chat feed poller while on Players tab
  useEffect(() => {
    if (!id || activeTab !== 'players' || server?.status !== 'running') return;
    const interval = setInterval(async () => {
      try {
        const chat = await api.getChat(id);
        setChatFeed(Array.isArray(chat) ? chat : []);
        const pl = await api.getPlayers(id);
        setOnlinePlayers(Array.isArray(pl?.online_players) ? pl.online_players : []);
      } catch {}
    }, 3000);
    return () => clearInterval(interval);
  }, [id, activeTab, server?.status]);

  // Fetch tab data when active tab switches
  useEffect(() => {
    if (!id) return;
    if (activeTab === 'overview') {
      if (server?.status === 'running') {
        api.getPlayers(id).then((res) => setOnlinePlayers(Array.isArray(res?.online_players) ? res.online_players : [])).catch(() => {});
      }
    } else if (activeTab === 'players') {
      api.getPlayers(id).then((res) => setOnlinePlayers(Array.isArray(res?.online_players) ? res.online_players : [])).catch(() => {});
      api.getPermissions(id).then((res) => setPermissions(Array.isArray(res) ? res : [])).catch(() => {});
      api.getChat(id).then((res) => setChatFeed(Array.isArray(res) ? res : [])).catch(() => {});
      api.listPlayerBans(id).then((res) => setServerBans(Array.isArray(res) ? res : [])).catch(() => {});
    } else if (activeTab === 'portgate') {
      api.listLeases(id).then((res) => setLeases(Array.isArray(res) ? res : [])).catch(() => {});
      api.listAccessKeys(id).then((res) => setAccessKeys(Array.isArray(res) ? res : [])).catch(() => {});
      api.listPortGateAllowRules(id).then((res) => setAllowRules(Array.isArray(res) ? res : [])).catch(() => {});
      api.getKnockConfig(id).then((cfg) => {
        if (cfg.client_ip) setDetectedClientIP(cfg.client_ip);
      }).catch(() => {});
    } else if (activeTab === 'backups') {
      api.listBackups(id).then((res) => setBackupsList(Array.isArray(res) ? res : [])).catch(() => {});
    } else if (activeTab === 'addons') {
      api.listAddons(id).then((res) => setAddonsList(Array.isArray(res) ? res : [])).catch(() => {});
    } else if (activeTab === 'settings') {
      api.getProperties(id).then((res) => {
        setProperties(res?.properties || {});
        setPropKeys(Array.isArray(res?.keys) ? res.keys : Object.keys(res?.properties || {}));
        setPropUninitialized(!!res?.uninitialized);
        setPropPending(!!res?.pending);
      }).catch(() => {});
      api.getAllowlist(id).then((res) => setAllowlist(Array.isArray(res) ? res : [])).catch(() => {});
      api.getPermissions(id).then((res) => setPermissions(Array.isArray(res) ? res : [])).catch(() => {});
      api.listGlobalPlayers().then((res) => setGlobalPlayers(Array.isArray(res) ? res : [])).catch(() => {});
    } else if (activeTab === 'actions') {
      api.checkUpdates(server?.version || 'latest').then((res) => setUpdateInfo(res)).catch(() => {});
      api.listServers().then((servers) => setAllServers(Array.isArray(servers) ? servers : [])).catch(() => {});
    }
  }, [id, activeTab, server?.version]);

  useEffect(() => {
    if (server) {
      setEditServerName(server.name);
      setEditServerVersion(server.version);
      setEditServerPort(server.port);
      setEditServerPortV6(server.portv6);
      setEditServerMode(server.mode);
      setEditServerDifficulty(server.difficulty);
      setEditServerSeed(server.seed || '');
      setEditServerMemLimit(server.memory_limit);
      setEditServerCpuLimit(server.cpu_limit || 2.0);
      setEditServerAutostart(server.autostart_on_boot);
      setEditServerPortGate(server.port_gate_enabled);
      setEditServerPortGateMode(server.port_gate_mode);
      setEditServerPortGateTimeout(server.port_gate_timeout);
      setEditGameServerAddress(server.game_server_address || '');
    }
  }, [server]);

  const handleCreateAccessKey = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!id) return;
    try {
      const res = await api.createAccessKey(id, {
        label: newKeyLabel.trim(),
        passphrase: newKeyPassphrase.trim() || undefined,
        max_uses: Number(newKeyMaxUses),
        lease_duration_seconds: Number(newKeyDurationMinutes) * 60,
      });
      setCreatedKeySecret(res.plaintext_passphrase || newKeyPassphrase.trim());
      setNewKeyLabel('');
      setNewKeyPassphrase('');
      setNewKeyMaxUses(0);
      const updatedKeys = await api.listAccessKeys(id);
      setAccessKeys(Array.isArray(updatedKeys) ? updatedKeys : []);
    } catch (err: any) {
      alert(err.message || 'Failed to create access key');
    }
  };

  const handleDeleteAccessKey = async (keyId: number) => {
    if (!id || !confirm('Are you sure you want to revoke and delete this access key?')) return;
    try {
      await api.deleteAccessKey(id, keyId);
      const updatedKeys = await api.listAccessKeys(id);
      setAccessKeys(Array.isArray(updatedKeys) ? updatedKeys : []);
    } catch (err: any) {
      alert(err.message || 'Failed to delete access key');
    }
  };

  const handleCreateAllowRule = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!id || !newAllowIP.trim()) return;
    setAllowSubmitting(true);
    try {
      await api.createPortGateAllowRule({
        ip_or_subnet: newAllowIP.trim(),
        comment: newAllowComment.trim(),
        is_global: newAllowScope === 'global',
        server_id: newAllowScope === 'global' ? null : id,
      }, id);
      setShowAddAllowModal(false);
      setNewAllowIP('');
      setNewAllowComment('');
      setNewAllowScope('server');
      const updated = await api.listPortGateAllowRules(id);
      setAllowRules(Array.isArray(updated) ? updated : []);
    } catch (err: any) {
      alert(err.message || 'Failed to add permanent allow rule');
    } finally {
      setAllowSubmitting(false);
    }
  };

  const handleDeleteAllowRule = async (ruleId: number) => {
    if (!id || !confirm('Are you sure you want to remove this permanent allow rule? The firewall rule will be deleted.')) return;
    try {
      await api.deletePortGateAllowRule(ruleId, id);
      const updated = await api.listPortGateAllowRules(id);
      setAllowRules(Array.isArray(updated) ? updated : []);
    } catch (err: any) {
      alert(err.message || 'Failed to delete allow rule');
    }
  };

  const handleUpdateServerSettings = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!id) return;
    setServerUpdating(true);
    setServerUpdatedMsg(null);
    try {
      const updated = await api.updateServer(id, {
        name: editServerName,
        version: editServerVersion.trim() || undefined,
        seed: editServerSeed.trim(),
        port: Number(editServerPort),
        portv6: Number(editServerPortV6),
        mode: editServerMode,
        difficulty: editServerDifficulty,
        memory_limit: editServerMemLimit,
        cpu_limit: Number(editServerCpuLimit),
        autostart_on_boot: editServerAutostart,
        port_gate_enabled: editServerPortGate,
        port_gate_mode: editServerPortGateMode,
        port_gate_timeout: Number(editServerPortGateTimeout),
        game_server_address: editGameServerAddress.trim(),
      });
      setServer((prev) => (prev ? { ...prev, ...updated, status: prev.status } : null));
      setServerUpdatedMsg('Server instance configuration updated successfully!');
      setTimeout(() => setServerUpdatedMsg(null), 4000);
    } catch (err: any) {
      alert(err.message || 'Failed to update server configuration');
    } finally {
      setServerUpdating(false);
    }
  };

  const handleDeleteServerInstance = async () => {
    if (!id) return;
    if (deleteConfirmText.trim() !== id) {
      alert(`Please type "${id}" to confirm deletion.`);
      return;
    }
    setServerDeleting(true);
    try {
      await api.deleteServer(id);
      navigate('/');
    } catch (err: any) {
      alert(err.message || 'Failed to delete server');
      setServerDeleting(false);
    }
  };

  const handleStart = async () => {
    if (!id) return;
    setActionLoading(true);
    try {
      await api.startServer(id);
      await fetchServer();
    } catch (err: any) {
      alert(err.message || 'Failed to start server');
    } finally {
      setActionLoading(false);
    }
  };

  const handleStop = async () => {
    if (!id) return;
    setActionLoading(true);
    try {
      await api.stopServer(id);
      await fetchServer();
    } catch (err: any) {
      alert(err.message || 'Failed to stop server');
    } finally {
      setActionLoading(false);
    }
  };

  const handleRestart = async () => {
    if (!id) return;
    setActionLoading(true);
    try {
      await api.restartServer(id);
      await fetchServer();
    } catch (err: any) {
      alert(err.message || 'Failed to restart server');
    } finally {
      setActionLoading(false);
    }
  };

  const handleSendCommand = (e: React.FormEvent) => {
    e.preventDefault();
    const cmd = command.trim();
    if (!cmd) return;

    if (server?.status !== 'running') {
      setLogs((prev) => [...prev, `[SYSTEM] Server is currently offline. Start the server before sending commands.`]);
      setCommand('');
      return;
    }

    setCommand('');

    if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({ type: 'command', payload: cmd }));
    } else if (id) {
      api.sendCommand(id, cmd).catch((err: any) => {
        setLogs((prev) => [...prev, `[ERROR] ${err.message || 'Failed to send command'}`]);
      });
    }
  };

  const handleBroadcast = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!id || !broadcastMsg.trim()) return;
    try {
      await api.broadcast(id, broadcastMsg.trim(), broadcastTarget.trim() || undefined);
      setBroadcastMsg('');
      setBroadcastTarget('');
      // Refresh chat feed immediately
      const chat = await api.getChat(id);
      setChatFeed(Array.isArray(chat) ? chat : []);
    } catch (err: any) {
      alert(err.message || 'Failed to broadcast');
    }
  };

  const handleKick = async (gt: string) => {
    if (!id) return;
    if (!confirm(`Are you sure you want to kick ${gt}?`)) return;
    try {
      await api.kickPlayer(id, gt);
      const res = await api.getPlayers(id);
      setOnlinePlayers(Array.isArray(res?.online_players) ? res.online_players : []);
    } catch (err: any) {
      alert(err.message || 'Failed to kick player');
    }
  };

  const handleOp = async (gt: string) => {
    if (!id) return;
    try {
      setOnlinePlayers((prev) =>
        prev.map((p) => (p.gamertag === gt ? { ...p, is_op: true, permission: 'operator' } : p))
      );
      await api.opPlayer(id, gt);
      const [resPl, resPerm] = await Promise.allSettled([
        api.getPlayers(id),
        api.getPermissions(id),
      ]);
      if (resPl.status === 'fulfilled' && Array.isArray(resPl.value?.online_players)) {
        setOnlinePlayers(resPl.value.online_players);
      }
      if (resPerm.status === 'fulfilled' && Array.isArray(resPerm.value)) {
        setPermissions(resPerm.value);
      }
    } catch (err: any) {
      const res = await api.getPlayers(id).catch(() => null);
      if (res?.online_players) setOnlinePlayers(res.online_players);
      alert(err.message || 'Failed to op player');
    }
  };

  const handleDeop = async (gt: string) => {
    if (!id) return;
    if (!confirm(`Are you sure you want to revoke operator status from ${gt}?`)) return;
    try {
      setOnlinePlayers((prev) =>
        prev.map((p) => (p.gamertag === gt ? { ...p, is_op: false, permission: 'member' } : p))
      );
      await api.deopPlayer(id, gt);
      const [resPl, resPerm] = await Promise.allSettled([
        api.getPlayers(id),
        api.getPermissions(id),
      ]);
      if (resPl.status === 'fulfilled' && Array.isArray(resPl.value?.online_players)) {
        setOnlinePlayers(resPl.value.online_players);
      }
      if (resPerm.status === 'fulfilled' && Array.isArray(resPerm.value)) {
        setPermissions(resPerm.value);
      }
    } catch (err: any) {
      const res = await api.getPlayers(id).catch(() => null);
      if (res?.online_players) setOnlinePlayers(res.online_players);
      alert(err.message || 'Failed to deop player');
    }
  };

  const handleOpenPlayerModal = (p: { gamertag: string; xuid: string; is_op?: boolean }) => {
    const matchedLease = leases.find(
      (l) => l.gamertag && l.gamertag.toLowerCase() === p.gamertag.toLowerCase()
    );
    setSelectedPlayer({
      ...p,
      ip_address: matchedLease?.ip_address,
    });
    setPlayerModalRole(p.is_op ? 'operator' : 'member');
    setPlayerBanScope('instance');
    setPlayerBanReason('Banned by administrator');
    setPlayerBanIP(Boolean(matchedLease?.ip_address));
  };

  const handleAddModalPlayerToGlobal = async () => {
    if (!id || !selectedPlayer) return;
    setPlayerGlobalLoading(true);
    try {
      await api.promotePlayerToGlobal(id, {
        name: selectedPlayer.gamertag,
        xuid: selectedPlayer.xuid || '',
        permission: playerModalRole,
        is_allowlisted: true,
      });
      const newGps = await api.listGlobalPlayers();
      setGlobalPlayers(Array.isArray(newGps) ? newGps : []);
      alert(`Player '${selectedPlayer.gamertag}' added to Global Players as ${playerModalRole}!`);
    } catch (err: any) {
      alert(err.message || 'Failed to add player to Global Players');
    } finally {
      setPlayerGlobalLoading(false);
    }
  };

  const handleBanModalPlayer = async () => {
    if (!id || !selectedPlayer) return;
    const confirmMsg =
      playerBanScope === 'global'
        ? `Are you sure you want to GLOBALLY ban '${selectedPlayer.gamertag}' from all instances?`
        : `Are you sure you want to ban '${selectedPlayer.gamertag}' from this server instance?`;
    if (!confirm(confirmMsg)) return;

    setPlayerBanLoading(true);
    try {
      await api.banPlayer(id, {
        gamertag: selectedPlayer.gamertag,
        xuid: selectedPlayer.xuid || '',
        scope: playerBanScope,
        reason: playerBanReason,
        ban_ip: playerBanIP,
        ip_address: selectedPlayer.ip_address,
      });

      alert(`Player '${selectedPlayer.gamertag}' has been banned (${playerBanScope}) and kicked!`);
      setSelectedPlayer(null);

      // Refresh online players, allowlist, leases, and bans
      const [resPl, resAl, resLeases, resBans] = await Promise.allSettled([
        api.getPlayers(id),
        api.getAllowlist(id),
        api.listLeases(id),
        api.listPlayerBans(id),
      ]);
      if (resPl.status === 'fulfilled' && Array.isArray(resPl.value?.online_players)) {
        setOnlinePlayers(resPl.value.online_players);
      }
      if (resAl.status === 'fulfilled' && Array.isArray(resAl.value)) {
        setAllowlist(resAl.value);
      }
      if (resLeases.status === 'fulfilled' && Array.isArray(resLeases.value)) {
        setLeases(resLeases.value);
      }
      if (resBans.status === 'fulfilled' && Array.isArray(resBans.value)) {
        setServerBans(resBans.value);
      }
      if (playerBanScope === 'global') {
        const newGps = await api.listGlobalPlayers().catch(() => []);
        setGlobalPlayers(Array.isArray(newGps) ? newGps : []);
      }
    } catch (err: any) {
      alert(err.message || 'Failed to ban player');
    } finally {
      setPlayerBanLoading(false);
    }
  };

  const handleUnbanServerPlayer = async (ban: BannedPlayer) => {
    if (!id) return;
    if (!confirm(`Are you sure you want to unban '${ban.gamertag}'?`)) return;
    try {
      await api.unbanPlayer(id, { id: ban.id });
      alert(`Player '${ban.gamertag}' has been unbanned.`);
      const updated = await api.listPlayerBans(id);
      setServerBans(Array.isArray(updated) ? updated : []);
    } catch (err: any) {
      alert(err.message || 'Failed to unban player');
    }
  };

  const handleSaveProperties = async () => {
    if (!id) return;
    setConfigSaving(true);
    try {
      const res: any = await api.updateProperties(id, properties, propKeys);
      if (res?.pending) {
        setPropPending(true);
      }
      if (properties && properties['level-seed'] !== undefined) {
        setServer((prev) => (prev ? { ...prev, seed: properties['level-seed'] } : null));
        setEditServerSeed(properties['level-seed']);
      }
      setConfigSaved(true);
      setTimeout(() => setConfigSaved(false), 2500);
    } catch (err: any) {
      alert(err.message || 'Failed to save properties');
    } finally {
      setConfigSaving(false);
    }
  };

  const handleAddAllowlistPlayer = async () => {
    if (!id || !newAllowlistPlayer.trim()) return;
    const updated = [...(allowlist || []), { name: newAllowlistPlayer.trim(), ignoresPlayerLimit: false }];
    try {
      await api.updateAllowlist(id, updated);
      setAllowlist(updated);
      setNewAllowlistPlayer('');
    } catch (err: any) {
      alert(err.message || 'Failed to update allowlist');
    }
  };

  const handleRemoveAllowlistPlayer = async (name: string) => {
    if (!id) return;
    const updated = (allowlist || []).filter((p) => p.name !== name);
    try {
      await api.updateAllowlist(id, updated);
      setAllowlist(updated);
    } catch (err: any) {
      alert(err.message || 'Failed to update allowlist');
    }
  };

  const handleAddPermission = async () => {
    if (!id || !newPermXuid.trim()) return;
    const cleanXuid = newPermXuid.trim();
    const updated = [...(permissions || []).filter((p) => p.xuid !== cleanXuid), { permission: newPermRole, xuid: cleanXuid }];
    try {
      await api.updatePermissions(id, updated);
      setPermissions(updated);
      setNewPermXuid('');
    } catch (err: any) {
      alert(err.message || 'Failed to update permissions');
    }
  };

  const handleRemovePermission = async (xuid: string) => {
    if (!id) return;
    const updated = (permissions || []).filter((p) => p.xuid !== xuid);
    try {
      await api.updatePermissions(id, updated);
      setPermissions(updated);
    } catch (err: any) {
      alert(err.message || 'Failed to update permissions');
    }
  };

  const handleSyncGlobalNow = async () => {
    if (!id) return;
    setSyncingGlobal(true);
    setSyncReportMsg(null);
    try {
      const rep = await api.syncServerGlobal(id);
      const allowCount = Array.isArray(rep?.allowlist_added) ? rep.allowlist_added.length : 0;
      const permCount = Array.isArray(rep?.permissions_updated) ? rep.permissions_updated.length : 0;
      setSyncReportMsg(`Synced with Global: Added ${allowCount} players to allowlist, updated ${permCount} permissions.`);
      setTimeout(() => setSyncReportMsg(null), 5000);
      const [newAl, newPerms, newGps] = await Promise.all([
        api.getAllowlist(id),
        api.getPermissions(id),
        api.listGlobalPlayers(),
      ]);
      setAllowlist(Array.isArray(newAl) ? newAl : []);
      setPermissions(Array.isArray(newPerms) ? newPerms : []);
      setGlobalPlayers(Array.isArray(newGps) ? newGps : []);
    } catch (err: any) {
      alert(err.message || 'Failed to sync with global');
    } finally {
      setSyncingGlobal(false);
    }
  };

  const handlePromoteToGlobal = async (playerName: string, playerXuid?: string, role = 'member', isAllowlisted = true) => {
    if (!id) return;
    try {
      await api.promotePlayerToGlobal(id, {
        name: playerName,
        xuid: playerXuid || '',
        permission: role,
        is_allowlisted: isAllowlisted,
      });
      alert(`Player '${playerName}' successfully promoted to Global access list!`);
      const newGps = await api.listGlobalPlayers();
      setGlobalPlayers(Array.isArray(newGps) ? newGps : []);
    } catch (err: any) {
      alert(err.message || 'Failed to promote player to global');
    }
  };

  const handleRemoveFromGlobal = async (playerName: string) => {
    if (!confirm(`Remove '${playerName}' from Global list? They will remain on this server as a local-only entry.`)) {
      return;
    }
    try {
      await api.removeGlobalPlayerByName(playerName);
      const newGps = await api.listGlobalPlayers();
      setGlobalPlayers(Array.isArray(newGps) ? newGps : []);
    } catch (err: any) {
      alert(err.message || 'Failed to remove player from global');
    }
  };

  const handleCopyConfigs = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!id) return;
    if (selectedTargetIds.length === 0) {
      alert('Please select at least one target server');
      return;
    }
    if (!copyAllowlist && !copyPermissions && !copyProperties) {
      alert('Please select at least one configuration type (allowlist, permissions, or properties)');
      return;
    }

    setCopying(true);
    setCopyStatusMsg(null);
    try {
      const res = await api.copyConfigs(id, {
        target_server_ids: selectedTargetIds,
        copy_allowlist: copyAllowlist,
        copy_permissions: copyPermissions,
        copy_properties: copyProperties,
        mode: copyMode,
      });

      const results = Array.isArray(res?.results) ? res.results : [];
      const successCount = results.filter((r) => r.success).length;
      const failCount = results.filter((r) => !r.success).length;
      let text = `Successfully synced configs to ${successCount} server(s).`;
      if (failCount > 0) {
        text += ` (${failCount} target(s) reported an error)`;
      }
      setCopyStatusMsg({ text, isError: failCount > 0 && successCount === 0 });
      setSelectedTargetIds([]);
    } catch (err: any) {
      setCopyStatusMsg({ text: err.message || 'Failed to sync configurations', isError: true });
    } finally {
      setCopying(false);
    }
  };

  const handleClone = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!id || !cloneId || !cloneName) return;
    setCloning(true);
    try {
      await api.cloneServer(id, cloneId, cloneName);
      alert(`Successfully cloned server to ${cloneName} (${cloneId})!`);
      setCloneId('');
      setCloneName('');
    } catch (err: any) {
      alert(err.message || 'Failed to clone server');
    } finally {
      setCloning(false);
    }
  };

  if (loading) {
    return (
      <div className="flex flex-col items-center justify-center py-24 text-slate-400">
        <Loader2 className="w-8 h-8 animate-spin text-emerald-400 mb-3" />
        <span className="font-mono text-sm">Loading server configuration...</span>
      </div>
    );
  }

  if (error || !server) {
    return (
      <div className="max-w-4xl mx-auto px-4 py-12">
        <div className="p-4 rounded-xl bg-rose-950/40 border border-rose-500/40 text-rose-300 flex items-center space-x-3">
          <AlertTriangle className="w-5 h-5 text-rose-400 flex-shrink-0" />
          <span>{error || 'Server not found'}</span>
        </div>
        <Link to="/" className="mt-4 inline-flex items-center space-x-2 text-emerald-400 hover:underline font-mono text-sm">
          <ArrowLeft className="w-4 h-4" />
          <span>Back to Dashboard</span>
        </Link>
      </div>
    );
  }

  // Leases handlers
  const handleRevokeLease = async (leaseId: number) => {
    if (!id) return;
    try {
      await api.revokeLease(id, leaseId);
      const updated = await api.listLeases(id);
      setLeases(updated);
    } catch (err: any) {
      alert(err.message || 'Failed to revoke lease');
    }
  };

  const handleCreateManualLease = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!id || !manualIP) return;
    try {
      await api.createManualLease(id, {
        ip_address: manualIP,
        gamertag: manualGamertag,
        duration_minutes: manualDuration,
        comment: manualComment,
      });
      setShowManualLeaseModal(false);
      setManualIP('');
      setManualGamertag('');
      setManualComment('');
      const updated = await api.listLeases(id);
      setLeases(updated);
    } catch (err: any) {
      alert(err.message || 'Failed to grant lease');
    }
  };

  // Backups handlers
  const handleCreateBackup = async () => {
    if (!id) return;
    setBackupLoading(true);
    try {
      await api.createBackup(id);
      const updated = await api.listBackups(id);
      setBackupsList(updated);
    } catch (err: any) {
      alert(err.message || 'Failed to create backup');
    } finally {
      setBackupLoading(false);
    }
  };

  const handleToggleBackupLock = async (backupId: number) => {
    if (!id) return;
    try {
      await api.toggleBackupLock(id, backupId);
      const updated = await api.listBackups(id);
      setBackupsList(updated);
    } catch (err: any) {
      alert(err.message || 'Failed to toggle lock');
    }
  };

  const handleDeleteBackup = async (backupId: number) => {
    if (!id) return;
    if (!confirm('Are you sure you want to delete this backup?')) return;
    try {
      await api.deleteBackup(id, backupId);
      const updated = await api.listBackups(id);
      setBackupsList(updated);
    } catch (err: any) {
      alert(err.message || 'Failed to delete backup');
    }
  };

  const handleRestoreBackup = async (backupId: number) => {
    if (!id) return;
    if (server?.status === 'running') {
      alert('Server must be stopped before restoring a backup.');
      return;
    }
    if (!confirm('Restoring will replace the current world state. Continue?')) return;
    try {
      await api.restoreBackup(id, backupId);
      alert('Backup restored successfully!');
    } catch (err: any) {
      alert(err.message || 'Failed to restore backup');
    }
  };

  const handleImportWorld = async (e: React.ChangeEvent<HTMLInputElement>) => {
    if (!id || !e.target.files || e.target.files.length === 0) return;
    if (server?.status === 'running') {
      alert('Server must be stopped before importing a new world.');
      return;
    }
    const file = e.target.files[0];
    try {
      await api.importWorld(id, file);
      alert('World imported successfully!');
    } catch (err: any) {
      alert(err.message || 'Failed to import world');
    }
  };

  // Addon handlers
  const handleInstallAddon = async (e: React.ChangeEvent<HTMLInputElement>) => {
    if (!id || !e.target.files || e.target.files.length === 0) return;
    const file = e.target.files[0];
    setAddonLoading(true);
    try {
      await api.installAddon(id, file);
      const updated = await api.listAddons(id);
      setAddonsList(updated);
      alert('Addon installed successfully!');
    } catch (err: any) {
      alert(err.message || 'Failed to install addon');
    } finally {
      setAddonLoading(false);
    }
  };

  const handleDeleteAddon = async (packType: string, folder: string) => {
    if (!id) return;
    if (!confirm(`Delete addon "${folder}"?`)) return;
    try {
      await api.deleteAddon(id, packType, folder);
      const updated = await api.listAddons(id);
      setAddonsList(updated);
    } catch (err: any) {
      alert(err.message || 'Failed to delete addon');
    }
  };

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
      {/* Header */}
      <div className="flex flex-col md:flex-row items-start md:items-center justify-between gap-4 mb-6">
        <div className="flex items-center space-x-4">
          <Link
            to="/"
            className="p-2 rounded-lg bg-obsidian-900 border border-obsidian-700 text-slate-400 hover:text-emerald-400 transition-colors"
          >
            <ArrowLeft className="w-4 h-4" />
          </Link>
          <div>
            <div className="flex items-center space-x-3">
              <h1 className="text-2xl font-bold font-mono text-slate-100">{server.name}</h1>
              <span
                className={`text-[11px] font-mono px-2 py-0.5 rounded-full border uppercase ${
                  server.status === 'running'
                    ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/30'
                    : server.status === 'crashed'
                    ? 'bg-rose-500/10 text-rose-400 border-rose-500/30'
                    : 'bg-slate-800 text-slate-400 border-slate-700'
                }`}
              >
                {server.status}
              </span>
            </div>
            <div className="text-xs text-slate-400 font-mono mt-0.5 flex flex-wrap items-center gap-x-2">
              <span>ID: {server.id}</span>
              <span>•</span>
              <span>UDP: {server.port}</span>
              <span>•</span>
              <span>v{server.version}</span>
              {server.seed && (
                <>
                  <span>•</span>
                  <span className="text-emerald-400/90 font-medium">Seed: {server.seed}</span>
                </>
              )}
            </div>
          </div>
        </div>

        <div className="flex items-center space-x-3">
          {server.port_gate_enabled && (
            <Link
              to={`/knock/${server.id}`}
              target="_blank"
              className="px-3 py-2 rounded-lg bg-emerald-950 border border-emerald-500/40 text-emerald-400 hover:bg-emerald-900/50 font-mono text-xs flex items-center space-x-1.5 transition-all shadow-[0_0_10px_rgba(16,185,129,0.15)]"
            >
              <Shield className="w-3.5 h-3.5" />
              <span>Knock Portal</span>
              <ExternalLink className="w-3 h-3 ml-0.5" />
            </Link>
          )}

          {server.status === 'running' ? (
            <>
              <button
                onClick={handleRestart}
                disabled={actionLoading}
                className="px-3 py-2 rounded-lg bg-obsidian-850 hover:bg-obsidian-800 border border-obsidian-700 text-slate-200 font-mono text-xs flex items-center space-x-1.5"
              >
                <RefreshCw className={`w-3.5 h-3.5 text-cyber-cyan ${actionLoading ? 'animate-spin' : ''}`} />
                <span>Restart</span>
              </button>
              <button
                onClick={handleStop}
                disabled={actionLoading}
                className="px-3 py-2 rounded-lg bg-rose-950/40 hover:bg-rose-900/60 border border-rose-500/40 text-rose-300 font-mono text-xs flex items-center space-x-1.5"
              >
                <Square className="w-3.5 h-3.5 fill-current text-rose-400" />
                <span>Stop</span>
              </button>
            </>
          ) : (
            <button
              onClick={handleStart}
              disabled={actionLoading}
              className="px-4 py-2 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-slate-950 font-mono text-xs font-bold flex items-center space-x-1.5 shadow-[0_0_15px_rgba(16,185,129,0.3)]"
            >
              {actionLoading ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <Play className="w-3.5 h-3.5 fill-current" />}
              <span>Start Server</span>
            </button>
          )}
        </div>
      </div>

      {/* Tabs */}
      <div className="flex border-b border-obsidian-700/80 mb-6 font-mono text-xs overflow-x-auto">
        <button
          onClick={() => setActiveTab('overview')}
          className={`pb-3 px-4 border-b-2 font-medium flex items-center space-x-2 whitespace-nowrap transition-colors ${
            activeTab === 'overview'
              ? 'border-emerald-500 text-emerald-400'
              : 'border-transparent text-slate-400 hover:text-slate-200'
          }`}
        >
          <Activity className="w-3.5 h-3.5" />
          <span>Instance Overview</span>
        </button>

        <button
          onClick={() => setActiveTab('players')}
          className={`pb-3 px-4 border-b-2 font-medium flex items-center space-x-2 whitespace-nowrap transition-colors ${
            activeTab === 'players'
              ? 'border-emerald-500 text-emerald-400'
              : 'border-transparent text-slate-400 hover:text-slate-200'
          }`}
        >
          <Users className="w-3.5 h-3.5" />
          <span>Players & Chat</span>
        </button>

        <button
          onClick={() => setActiveTab('portgate')}
          className={`pb-3 px-4 border-b-2 font-medium flex items-center space-x-2 whitespace-nowrap transition-colors ${
            activeTab === 'portgate'
              ? 'border-emerald-500 text-emerald-400'
              : 'border-transparent text-slate-400 hover:text-slate-200'
          }`}
        >
          <Shield className="w-3.5 h-3.5" />
          <span>Port Gate & Leases</span>
        </button>

        <button
          onClick={() => setActiveTab('backups')}
          className={`pb-3 px-4 border-b-2 font-medium flex items-center space-x-2 whitespace-nowrap transition-colors ${
            activeTab === 'backups'
              ? 'border-emerald-500 text-emerald-400'
              : 'border-transparent text-slate-400 hover:text-slate-200'
          }`}
        >
          <Archive className="w-3.5 h-3.5" />
          <span>Backups & Worlds</span>
        </button>

        <button
          onClick={() => setActiveTab('addons')}
          className={`pb-3 px-4 border-b-2 font-medium flex items-center space-x-2 whitespace-nowrap transition-colors ${
            activeTab === 'addons'
              ? 'border-emerald-500 text-emerald-400'
              : 'border-transparent text-slate-400 hover:text-slate-200'
          }`}
        >
          <Package className="w-3.5 h-3.5" />
          <span>Addons & Packs</span>
        </button>

        <button
          onClick={() => setActiveTab('settings')}
          className={`pb-3 px-4 border-b-2 font-medium flex items-center space-x-2 whitespace-nowrap transition-colors ${
            activeTab === 'settings'
              ? 'border-emerald-500 text-emerald-400'
              : 'border-transparent text-slate-400 hover:text-slate-200'
          }`}
        >
          <Settings className="w-3.5 h-3.5" />
          <span>Configuration Editor</span>
        </button>

        <button
          onClick={() => setActiveTab('actions')}
          className={`pb-3 px-4 border-b-2 font-medium flex items-center space-x-2 whitespace-nowrap transition-colors ${
            activeTab === 'actions'
              ? 'border-emerald-500 text-emerald-400'
              : 'border-transparent text-slate-400 hover:text-slate-200'
          }`}
        >
          <Copy className="w-3.5 h-3.5" />
          <span>Clone & Export</span>
        </button>
      </div>

      {/* OVERVIEW TAB */}
      {activeTab === 'overview' && (
        <div className="space-y-6">
          {/* Top: 3-column Instance Summary Cards */}
          <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
            {/* World Information & Seed */}
            <div className="bg-obsidian-900 border border-obsidian-700/80 rounded-xl p-5 flex flex-col justify-between">
              <div>
                <div className="flex items-center justify-between mb-3">
                  <h3 className="font-mono text-sm font-bold text-slate-200 flex items-center gap-2">
                    <Compass className="w-4 h-4 text-emerald-400" />
                    <span>World & Environment</span>
                  </h3>
                  {server.seed && (
                    <button
                      onClick={() => {
                        navigator.clipboard.writeText(server.seed || '');
                        setCopiedSeed(true);
                        setTimeout(() => setCopiedSeed(false), 2000);
                      }}
                      title="Copy world seed"
                      className="px-2 py-0.5 rounded bg-obsidian-950 border border-obsidian-800 hover:border-emerald-500/40 text-slate-400 hover:text-emerald-400 text-[10px] font-mono flex items-center gap-1 transition-colors"
                    >
                      {copiedSeed ? <Check className="w-3 h-3 text-emerald-400" /> : <Copy className="w-3 h-3" />}
                      <span>{copiedSeed ? 'Copied' : 'Copy'}</span>
                    </button>
                  )}
                </div>
                <div className="space-y-3 font-mono text-xs">
                  <div className="flex justify-between items-center pb-2 border-b border-obsidian-800">
                    <span className="text-slate-400">World Seed</span>
                    {server.seed ? (
                      <span className="text-emerald-400 font-bold font-mono tracking-wide select-all">
                        {server.seed}
                      </span>
                    ) : (
                      <span className="text-slate-500 italic">Random / Default</span>
                    )}
                  </div>
                  <div className="flex justify-between items-center pb-2 border-b border-obsidian-800">
                    <span className="text-slate-400">Game Mode</span>
                    <span className="text-slate-200 capitalize">{server.mode}</span>
                  </div>
                  <div className="flex justify-between items-center">
                    <span className="text-slate-400">Difficulty</span>
                    <span className="text-slate-200 capitalize">{server.difficulty}</span>
                  </div>
                </div>
              </div>
            </div>

            {/* Hardware Limits */}
            <div className="bg-obsidian-900 border border-obsidian-700/80 rounded-xl p-5 flex flex-col justify-between">
              <div>
                <h3 className="font-mono text-sm font-bold text-slate-200 mb-3 flex items-center gap-2">
                  <HardDrive className="w-4 h-4 text-emerald-400" />
                  <span>Hardware Caps</span>
                </h3>
                <div className="space-y-3 font-mono text-xs">
                  <div className="flex justify-between items-center pb-2 border-b border-obsidian-800">
                    <span className="text-slate-400">Memory Cap</span>
                    <span className="text-slate-200 font-semibold">{server.memory_limit}</span>
                  </div>
                  <div className="flex justify-between items-center pb-2 border-b border-obsidian-800">
                    <span className="text-slate-400">CPU Allocation</span>
                    <span className="text-slate-200 font-semibold">{server.cpu_limit} Cores</span>
                  </div>
                  <div className="flex justify-between items-center">
                    <span className="text-slate-400">Autostart on Boot</span>
                    <span className={server.autostart_on_boot ? "text-emerald-400 font-semibold" : "text-slate-500"}>
                      {server.autostart_on_boot ? "Enabled" : "Disabled"}
                    </span>
                  </div>
                </div>
              </div>
            </div>

            {/* Live Telemetry & Status */}
            <div className="bg-obsidian-900 border border-obsidian-700/80 rounded-xl p-5 flex flex-col justify-between">
              <div>
                <div className="flex items-center justify-between mb-3">
                  <h3 className="font-mono text-sm font-bold text-slate-200 flex items-center gap-2">
                    <Cpu className="w-4 h-4 text-cyber-cyan" />
                    <span>Live Telemetry</span>
                  </h3>
                  <span
                    className={`text-[10px] font-mono px-2 py-0.5 rounded-full border uppercase ${
                      server.status === 'running'
                        ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/30'
                        : server.status === 'crashed'
                        ? 'bg-rose-500/10 text-rose-400 border-rose-500/30'
                        : 'bg-slate-800 text-slate-400 border-slate-700'
                    }`}
                  >
                    {server.status}
                  </span>
                </div>
                <div className="space-y-3 font-mono text-xs">
                  <div className="flex justify-between items-center pb-2 border-b border-obsidian-800">
                    <span className="text-slate-400">Current CPU</span>
                    <span className={stats ? "text-emerald-400 font-bold" : "text-slate-500"}>
                      {stats ? `${stats.cpu_percent.toFixed(1)}%` : '—'}
                    </span>
                  </div>
                  <div className="flex justify-between items-center pb-2 border-b border-obsidian-800">
                    <span className="text-slate-400">Current RAM</span>
                    <span className={stats ? "text-slate-200 font-semibold" : "text-slate-500"}>
                      {stats ? `${(stats.ram_bytes / (1024 * 1024)).toFixed(0)} MB` : '—'}
                    </span>
                  </div>
                  <div className="flex justify-between items-center">
                    <span className="text-slate-400">Online Players</span>
                    <span className={server.status === 'running' ? "text-cyber-cyan font-bold" : "text-slate-500"}>
                      {server.status === 'running'
                        ? (stats ? stats.player_count : (onlinePlayers.length > 0 ? onlinePlayers.length : 0))
                        : 0}
                    </span>
                  </div>
                </div>
              </div>
            </div>
          </div>

          {/* Middle: Telemetry Resource & Player Charts */}
          <TelemetryCharts
            metrics={metrics}
            loading={metricsLoading}
            timeRange={metricsRange}
            onTimeRangeChange={(r) => {
              setMetricsRange(r);
              fetchMetrics(r);
            }}
            serverStatus={server.status}
          />

          {/* Bottom: Collapsible Interactive BDS Terminal */}
          <div className="bg-obsidian-900 border border-obsidian-700/80 rounded-xl flex flex-col overflow-hidden transition-all duration-200">
            {/* Collapsible Terminal Header Bar */}
            <button
              onClick={() => setTerminalOpen((prev) => !prev)}
              className="w-full flex items-center justify-between p-4 sm:p-5 hover:bg-obsidian-850/50 transition-colors cursor-pointer select-none text-left"
            >
              <h3 className="font-mono text-sm font-bold text-slate-200 flex items-center gap-2">
                {terminalOpen ? (
                  <ChevronDown className="w-4 h-4 text-emerald-400" />
                ) : (
                  <ChevronRight className="w-4 h-4 text-slate-400" />
                )}
                <Terminal className="w-4 h-4 text-emerald-400" />
                <span>Interactive BDS Terminal</span>
              </h3>
              <div className="flex items-center space-x-2 text-[11px] font-mono">
                <span className={`w-2 h-2 rounded-full ${wsConnected ? 'bg-emerald-400 animate-pulse' : 'bg-slate-600'}`}></span>
                <span className="text-slate-400">
                  {terminalOpen
                    ? wsConnected ? 'Connected' : 'Connecting...'
                    : wsConnected ? 'Connected' : 'Offline'}
                </span>
                {!terminalOpen && logs.length > 0 && (
                  <span className="ml-2 px-1.5 py-0.5 rounded bg-obsidian-800 border border-obsidian-700 text-slate-400 text-[10px]">
                    {logs.length} lines
                  </span>
                )}
                <span className="hidden sm:inline text-slate-500 text-[10px] ml-2">
                  {terminalOpen ? '(Click to collapse)' : '(Click to expand)'}
                </span>
              </div>
            </button>

            {/* Expanded Terminal Content */}
            {terminalOpen && (
              <div className="flex flex-col px-5 pb-5 h-[480px]">
                <div className="flex-1 bg-obsidian-950 rounded-lg p-4 font-mono text-xs text-slate-300 overflow-y-auto border border-obsidian-800 space-y-1">
                  {(logs || []).length === 0 ? (
                    <p className="text-slate-600 italic">No console logs received yet...</p>
                  ) : (
                    (logs || []).map((line, idx) => (
                      <div key={idx} className="leading-relaxed hover:bg-obsidian-900/60 px-1 rounded">
                        {line}
                      </div>
                    ))
                  )}
                  <div ref={consoleBottomRef} />
                </div>

                <form onSubmit={handleSendCommand} className="mt-3 flex items-center gap-2">
                  <input
                    type="text"
                    value={command}
                    onChange={(e) => setCommand(e.target.value)}
                    placeholder="Enter BDS console command (e.g. say Hello, op Steve, time set day)..."
                    className="flex-1 px-3 py-2 rounded-lg bg-obsidian-950 border border-obsidian-700 text-slate-100 font-mono text-xs focus:border-emerald-500"
                  />
                  <button
                    type="submit"
                    className="px-3.5 py-2 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-slate-950 font-bold font-mono text-xs flex items-center space-x-1"
                  >
                    <Send className="w-3.5 h-3.5" />
                    <span>Send</span>
                  </button>
                </form>
              </div>
            )}
          </div>
        </div>
      )}

      {/* PLAYERS & CHAT TAB */}
      {activeTab === 'players' && (
        <div className="space-y-6">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
          {/* Online Players */}
          <div className="bg-obsidian-900 border border-obsidian-700/80 rounded-xl p-5 flex flex-col h-[520px]">
            <h3 className="font-mono text-sm font-bold text-slate-200 mb-3 flex items-center justify-between">
              <span className="flex items-center gap-2">
                <Users className="w-4 h-4 text-emerald-400" />
                <span>Online Players ({(onlinePlayers || []).length})</span>
              </span>
            </h3>

            <div className="flex-1 bg-obsidian-950 rounded-lg p-3 overflow-y-auto border border-obsidian-800 space-y-2 font-mono text-xs">
              {(onlinePlayers || []).length === 0 ? (
                <p className="text-slate-600 italic">No players online right now.</p>
              ) : (
                (onlinePlayers || []).map((p) => {
                  const isOp = !!p.is_op || (permissions || []).some(
                    (perm) => (perm.xuid === p.xuid || (p.gamertag && perm.xuid?.toLowerCase() === p.gamertag.toLowerCase())) && perm.permission === 'operator'
                  );

                  return (
                    <div key={p.gamertag} className="p-2.5 bg-obsidian-900 border border-obsidian-700/80 rounded-lg flex items-center justify-between">
                      <div>
                        <div className="flex items-center space-x-2">
                          <span className="font-bold text-slate-100">{p.gamertag}</span>
                          {isOp && (
                            <span className="px-1.5 py-0.2 rounded bg-amber-500/20 text-amber-300 border border-amber-500/40 text-[10px] font-bold">
                              OP
                            </span>
                          )}
                        </div>
                        <div className="text-[10px] text-slate-500">XUID: {p.xuid || 'N/A'}</div>
                      </div>
                      <div className="flex items-center space-x-2">
                        {isOp ? (
                          <button
                            type="button"
                            onClick={() => handleDeop(p.gamertag)}
                            title="Demote from Operator (DeOP)"
                            className="px-2.5 py-1 bg-amber-950/50 hover:bg-amber-900/80 text-amber-300 hover:text-amber-100 border border-amber-600/50 rounded text-[11px] font-semibold transition-colors"
                          >
                            DeOP
                          </button>
                        ) : (
                          <button
                            type="button"
                            onClick={() => handleOp(p.gamertag)}
                            title="Promote to Operator (OP)"
                            className="px-2.5 py-1 bg-obsidian-800 hover:bg-emerald-950 hover:text-emerald-400 text-slate-300 rounded text-[11px] font-semibold transition-colors"
                          >
                            OP
                          </button>
                        )}
                        <button
                          type="button"
                          onClick={() => handleKick(p.gamertag)}
                          title="Kick Player"
                          className="px-2.5 py-1 bg-obsidian-800 hover:bg-rose-950 hover:text-rose-400 text-slate-300 rounded text-[11px] font-semibold transition-colors"
                        >
                          Kick
                        </button>
                        <button
                          type="button"
                          onClick={() => handleOpenPlayerModal(p)}
                          title="Player Moderation & Actions"
                          className="px-2.5 py-1 bg-obsidian-800 hover:bg-cyan-950/80 hover:text-cyan-300 text-slate-300 border border-obsidian-700/80 hover:border-cyan-500/40 rounded text-[11px] font-semibold transition-colors flex items-center space-x-1"
                        >
                          <UserCog className="w-3.5 h-3.5" />
                          <span>Manage</span>
                        </button>
                      </div>
                    </div>
                  );
                })
              )}
            </div>

            {/* Broadcast Box */}
            <form onSubmit={handleBroadcast} className="mt-4 pt-3 border-t border-obsidian-800 space-y-2">
              <label className="block text-[11px] font-mono text-slate-400 uppercase">In-Game Broadcast</label>
              <div className="flex gap-2">
                <input
                  type="text"
                  value={broadcastMsg}
                  onChange={(e) => setBroadcastMsg(e.target.value)}
                  placeholder="Broadcast message to server chat..."
                  className="flex-1 px-3 py-2 bg-obsidian-950 border border-obsidian-700 rounded-lg text-slate-100 font-mono text-xs focus:border-emerald-500"
                />
                <button
                  type="submit"
                  className="px-3.5 py-2 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-slate-950 font-bold font-mono text-xs flex items-center space-x-1"
                >
                  <Send className="w-3.5 h-3.5" />
                  <span>Send</span>
                </button>
              </div>
            </form>
          </div>

          {/* Live In-Game Chat Feed */}
          <div className="bg-obsidian-900 border border-obsidian-700/80 rounded-xl p-5 flex flex-col h-[520px]">
            <h3 className="font-mono text-sm font-bold text-slate-200 mb-3 flex items-center gap-2">
              <MessageSquare className="w-4 h-4 text-cyber-cyan" />
              <span>In-Game Chat Feed</span>
            </h3>

            <div className="flex-1 bg-obsidian-950 rounded-lg p-3 overflow-y-auto border border-obsidian-800 space-y-2 font-mono text-xs">
              {(chatFeed || []).length === 0 ? (
                <p className="text-slate-600 italic">No chat messages captured yet.</p>
              ) : (
                (chatFeed || []).map((msg, i) => (
                  <div key={i} className="p-2 rounded bg-obsidian-900/60 border border-obsidian-800 flex items-start space-x-2">
                    <span className="text-emerald-400 font-bold">&lt;{msg.gamertag}&gt;</span>
                    <span className="text-slate-200">{msg.message}</span>
                  </div>
                ))
              )}
            </div>
          </div>
        </div>

        {/* Banned Players Panel */}
        <div className="bg-obsidian-900 border border-obsidian-700/80 rounded-xl p-5">
          <div className="flex items-center justify-between mb-4">
            <h3 className="font-mono text-sm font-bold text-slate-200 flex items-center gap-2">
              <Ban className="w-4 h-4 text-rose-400" />
              <span>Banned Players ({(serverBans || []).length})</span>
            </h3>
            <button
              type="button"
              disabled={serverBansLoading}
              onClick={async () => {
                if (!id) return;
                setServerBansLoading(true);
                try {
                  const res = await api.listPlayerBans(id);
                  setServerBans(Array.isArray(res) ? res : []);
                } finally {
                  setServerBansLoading(false);
                }
              }}
              title="Refresh bans"
              className="p-1 rounded text-slate-400 hover:text-slate-200 hover:bg-obsidian-800 transition-colors"
            >
              {serverBansLoading ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <RefreshCw className="w-3.5 h-3.5" />}
            </button>
          </div>

          <div className="overflow-x-auto">
            <table className="w-full text-left font-mono text-xs">
              <thead className="bg-obsidian-950/80 border-b border-obsidian-800 text-slate-400 uppercase text-[10px] tracking-wider">
                <tr>
                  <th className="px-4 py-2.5">Gamertag</th>
                  <th className="px-4 py-2.5">Ban Scope</th>
                  <th className="px-4 py-2.5">Reason</th>
                  <th className="px-4 py-2.5">Banned By</th>
                  <th className="px-4 py-2.5">Banned Date</th>
                  <th className="px-4 py-2.5 text-right">Action</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-obsidian-800/60 text-slate-300">
                {(serverBans || []).length === 0 ? (
                  <tr>
                    <td colSpan={6} className="px-4 py-6 text-center text-slate-500 italic">
                      No players banned on this server.
                    </td>
                  </tr>
                ) : (
                  (serverBans || []).map((b) => (
                    <tr key={b.id} className="hover:bg-obsidian-850/60 transition-colors">
                      <td className="px-4 py-3 font-bold text-rose-300 flex items-center gap-2">
                        <Ban className="w-3.5 h-3.5 text-rose-400" />
                        <span>{b.gamertag}</span>
                      </td>
                      <td className="px-4 py-3">
                        {!b.server_id ? (
                          <span className="px-1.5 py-0.5 rounded text-[10px] font-bold uppercase bg-rose-950/80 text-rose-300 border border-rose-600/50">
                            Global Ban
                          </span>
                        ) : (
                          <span className="px-1.5 py-0.5 rounded text-[10px] font-mono bg-amber-950/50 text-amber-300 border border-amber-600/40">
                            Instance Ban
                          </span>
                        )}
                      </td>
                      <td className="px-4 py-3 text-slate-300">{b.reason || 'Banned by administrator'}</td>
                      <td className="px-4 py-3 text-slate-400">{b.banned_by || 'Admin'}</td>
                      <td className="px-4 py-3 text-slate-500 text-[11px]">
                        {b.created_at ? new Date(b.created_at).toLocaleDateString() : 'N/A'}
                      </td>
                      <td className="px-4 py-3 text-right">
                        <button
                          type="button"
                          onClick={() => handleUnbanServerPlayer(b)}
                          className="px-2.5 py-1 rounded bg-emerald-950/60 hover:bg-emerald-900/80 text-emerald-300 hover:text-emerald-100 border border-emerald-600/40 text-[11px] font-bold transition-colors"
                        >
                          Unban
                        </button>
                      </td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          </div>
        </div>
      </div>
      )}

      {/* PORT GATE TAB */}
      {activeTab === 'portgate' && (
        <div className="space-y-6">
          <div className="bg-obsidian-900 border border-obsidian-700/80 rounded-xl p-6">
            <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 mb-6">
              <div>
                <h3 className="font-mono text-base font-bold text-slate-100 flex items-center gap-2">
                  <Shield className="w-4 h-4 text-emerald-400" />
                  <span>Dynamic Firewall Port Gate</span>
                </h3>
                <p className="text-xs text-slate-400 mt-1 font-mono">
                  Port {server.port}/udp is dynamically granted to verified players.
                </p>
              </div>
              <div className="flex items-center space-x-3">
                <button
                  onClick={() => setShowManualLeaseModal(true)}
                  className="px-3 py-1.5 rounded-lg bg-obsidian-800 hover:bg-obsidian-700 text-slate-200 border border-obsidian-600 font-mono text-xs font-bold flex items-center space-x-1.5"
                >
                  <UserPlus className="w-3.5 h-3.5 text-emerald-400" />
                  <span>Manual IP Grant</span>
                </button>
                <Link
                  to={`/knock/${server.id}`}
                  target="_blank"
                  className="px-3 py-1.5 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-slate-950 font-mono text-xs font-bold flex items-center space-x-1.5"
                >
                  <span>Test Unlock Portal</span>
                  <ExternalLink className="w-3 h-3" />
                </Link>
              </div>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-3 mb-4 font-mono text-xs">
              <div className="p-3 bg-obsidian-950 border border-obsidian-800 rounded-lg flex items-center justify-between">
                <div>
                  <span className="text-[10px] text-slate-500 uppercase tracking-wider block">Game Server Address</span>
                  <span className="text-slate-200 font-bold">{server.game_server_address || `${window.location.hostname} (default)`}</span>
                </div>
                <button
                  type="button"
                  onClick={() => setActiveTab('settings')}
                  className="text-[11px] text-emerald-400 hover:underline"
                >
                  Configure
                </button>
              </div>
              <div className="p-3 bg-obsidian-950 border border-obsidian-800 rounded-lg flex items-center justify-between">
                <div>
                  <span className="text-[10px] text-slate-500 uppercase tracking-wider block">Knock Portal URL</span>
                  <span className="text-slate-300 truncate max-w-[180px] sm:max-w-[240px] block">{`${window.location.origin}/knock/${server.id}`}</span>
                </div>
                <button
                  type="button"
                  onClick={() => {
                    navigator.clipboard.writeText(`${window.location.origin}/knock/${server.id}`);
                    alert('Knock Portal URL copied to clipboard!');
                  }}
                  className="px-2 py-1 rounded bg-obsidian-800 hover:bg-obsidian-750 text-slate-300 text-[10px] font-bold shrink-0 ml-2"
                >
                  Copy
                </button>
              </div>
            </div>

            <div className="p-4 bg-obsidian-950 border border-obsidian-800 rounded-lg text-xs font-mono text-slate-300 mb-6">
              <p className="text-emerald-400 mb-1 font-bold">Mobile Roaming Handoff Ready</p>
              <p className="text-slate-400">
                When players authenticate via the Knock Portal, their browser keeps a lightweight background heartbeat (default: 10s).
                If their cellular IP changes while moving between cell towers or networks, BSM automatically detects the change and hot-swaps the firewall rule.
              </p>
            </div>

            <h4 className="font-mono text-xs font-bold uppercase tracking-wider text-slate-400 mb-3">
              Active Client IP Leases ({leases.length})
            </h4>

            {leases.length === 0 ? (
              <div className="text-center py-8 bg-obsidian-950/60 border border-obsidian-800 rounded-lg text-slate-500 font-mono text-xs">
                No active firewall grants at this moment. Players must authenticate via the Knock Portal to unlock UDP port access.
              </div>
            ) : (
              <div className="border border-obsidian-800 rounded-lg overflow-hidden">
                <table className="w-full text-left font-mono text-xs">
                  <thead className="bg-obsidian-950 text-slate-400 border-b border-obsidian-800 uppercase">
                    <tr>
                      <th className="px-4 py-2.5">IP Address</th>
                      <th className="px-4 py-2.5">Gamertag</th>
                      <th className="px-4 py-2.5">Knock Method</th>
                      <th className="px-4 py-2.5">Granted At</th>
                      <th className="px-4 py-2.5">Expires At</th>
                      <th className="px-4 py-2.5 text-right">Action</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-obsidian-800 bg-obsidian-950/40">
                    {paginatedLeases.map((lease) => (
                      <tr key={lease.id} className="hover:bg-obsidian-800/40">
                        <td className="px-4 py-2.5 text-emerald-400 font-bold">{lease.ip_address}</td>
                        <td className="px-4 py-2.5 text-slate-300">{lease.gamertag || '—'}</td>
                        <td className="px-4 py-2.5 text-slate-400 capitalize">{lease.knock_method}</td>
                        <td className="px-4 py-2.5 text-slate-400">{new Date(lease.granted_at).toLocaleTimeString()}</td>
                        <td className="px-4 py-2.5 text-slate-400">{new Date(lease.expires_at).toLocaleTimeString()}</td>
                        <td className="px-4 py-2.5 text-right">
                          <button
                            onClick={() => handleRevokeLease(lease.id)}
                            className="px-2 py-1 rounded bg-rose-950/60 hover:bg-rose-900 border border-rose-800 text-rose-300 text-[10px] font-bold"
                          >
                            Revoke
                          </button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
                <Pagination
                  currentPage={leasesPage}
                  totalItems={totalLeases}
                  pageSize={leasesPageSize}
                  onPageChange={setLeasesPage}
                  onPageSizeChange={setLeasesPageSize}
                  pageSizeOptions={[5, 10, 25, 50]}
                />
              </div>
            )}
          </div>

          {/* MANUAL LEASE MODAL */}
          {showManualLeaseModal && (
            <div className="fixed inset-0 bg-black/80 backdrop-blur-sm z-50 flex items-center justify-center p-4">
              <div className="bg-obsidian-900 border border-obsidian-700 rounded-xl max-w-md w-full p-6 shadow-2xl">
                <h3 className="text-base font-mono font-bold text-slate-100 mb-4 flex items-center gap-2">
                  <UserPlus className="w-4 h-4 text-emerald-400" />
                  <span>Grant Manual IP Bypass</span>
                </h3>
                <form onSubmit={handleCreateManualLease} className="space-y-4 font-mono text-xs">
                  <div>
                    <label className="block text-slate-300 mb-1 font-bold">Client IP Address</label>
                    <input
                      type="text"
                      required
                      placeholder="e.g. 198.51.100.24"
                      value={manualIP}
                      onChange={(e) => setManualIP(e.target.value)}
                      className="w-full px-3 py-2 bg-obsidian-950 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
                    />
                  </div>
                  <div>
                    <label className="block text-slate-300 mb-1 font-bold">Gamertag (optional)</label>
                    <input
                      type="text"
                      placeholder="e.g. DiamondMiner99"
                      value={manualGamertag}
                      onChange={(e) => setManualGamertag(e.target.value)}
                      className="w-full px-3 py-2 bg-obsidian-950 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
                    />
                  </div>
                  <div>
                    <label className="block text-slate-300 mb-1 font-bold">Duration (minutes)</label>
                    <input
                      type="number"
                      min="1"
                      value={manualDuration}
                      onChange={(e) => setManualDuration(parseInt(e.target.value) || 60)}
                      className="w-full px-3 py-2 bg-obsidian-950 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
                    />
                  </div>
                  <div>
                    <label className="block text-slate-300 mb-1 font-bold">Comment / Note</label>
                    <input
                      type="text"
                      placeholder="Reason for manual bypass"
                      value={manualComment}
                      onChange={(e) => setManualComment(e.target.value)}
                      className="w-full px-3 py-2 bg-obsidian-950 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
                    />
                  </div>
                  <div className="flex justify-end space-x-3 pt-3 border-t border-obsidian-800">
                    <button
                      type="button"
                      onClick={() => setShowManualLeaseModal(false)}
                      className="px-4 py-2 rounded-lg bg-obsidian-800 text-slate-300 hover:bg-obsidian-700"
                    >
                      Cancel
                    </button>
                    <button
                      type="submit"
                      className="px-4 py-2 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-slate-950 font-bold"
                    >
                      Apply Firewall Rule
                    </button>
                  </div>
                </form>
              </div>
            </div>
          )}

          {/* ACCESS KEYS / PASSPHRASES CARD */}
          <div className="bg-obsidian-900 border border-obsidian-700/80 rounded-xl p-6">
            <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 mb-6">
              <div>
                <h3 className="font-mono text-base font-bold text-slate-100 flex items-center gap-2">
                  <Key className="w-4 h-4 text-emerald-400" />
                  <span>Port Gate Access Passphrases & Keys</span>
                </h3>
                <p className="text-xs text-slate-400 mt-1 font-mono">
                  Manage visitor passphrases and community keys for unlocking UDP port access via the Knock Portal.
                </p>
              </div>
              <button
                onClick={() => {
                  setCreatedKeySecret(null);
                  setShowNewKeyModal(true);
                }}
                className="px-3 py-1.5 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-slate-950 font-mono text-xs font-bold flex items-center space-x-1.5 shrink-0 shadow-[0_0_10px_rgba(16,185,129,0.2)]"
              >
                <KeyRound className="w-3.5 h-3.5" />
                <span>+ Create Access Passphrase</span>
              </button>
            </div>

            {accessKeys.length === 0 ? (
              <div className="text-center py-8 bg-obsidian-950/60 border border-obsidian-800 rounded-lg text-slate-500 font-mono text-xs">
                No access passphrases configured for this server. Create a key to let players knock using a shared or personal passphrase.
              </div>
            ) : (
              <div className="border border-obsidian-800 rounded-lg overflow-hidden">
                <table className="w-full text-left font-mono text-xs">
                  <thead className="bg-obsidian-950 text-slate-400 border-b border-obsidian-800 uppercase">
                    <tr>
                      <th className="px-4 py-2.5">Label</th>
                      <th className="px-4 py-2.5">Passphrase Prefix</th>
                      <th className="px-4 py-2.5">Uses / Limit</th>
                      <th className="px-4 py-2.5">Grant Duration</th>
                      <th className="px-4 py-2.5">Status</th>
                      <th className="px-4 py-2.5 text-right">Action</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-obsidian-800 bg-obsidian-950/40">
                    {paginatedKeys.map((k) => (
                      <tr key={k.id} className="hover:bg-obsidian-800/40">
                        <td className="px-4 py-2.5 text-slate-200 font-bold">{k.label}</td>
                        <td className="px-4 py-2.5 text-emerald-400">
                          <code>{k.key_prefix}••••••</code>
                        </td>
                        <td className="px-4 py-2.5 text-slate-300">
                          {k.used_count} / {k.max_uses === 0 ? '∞' : k.max_uses}
                        </td>
                        <td className="px-4 py-2.5 text-slate-400">
                          {Math.round(k.lease_duration_seconds / 60)} min
                        </td>
                        <td className="px-4 py-2.5">
                          <span
                            className={`px-1.5 py-0.5 rounded text-[10px] font-bold uppercase ${
                              k.is_active ? 'bg-emerald-500/10 text-emerald-400' : 'bg-rose-500/10 text-rose-400'
                            }`}
                          >
                            {k.is_active ? 'Active' : 'Inactive'}
                          </span>
                        </td>
                        <td className="px-4 py-2.5 text-right">
                          <button
                            onClick={() => handleDeleteAccessKey(k.id)}
                            className="px-2 py-1 rounded bg-rose-950/60 hover:bg-rose-900 border border-rose-800 text-rose-300 text-[10px] font-bold"
                          >
                            Revoke
                          </button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
                <Pagination
                  currentPage={keysPage}
                  totalItems={totalKeys}
                  pageSize={keysPageSize}
                  onPageChange={setKeysPage}
                  onPageSizeChange={setKeysPageSize}
                  pageSizeOptions={[5, 10, 25, 50]}
                />
              </div>
            )}
          </div>

          {/* NEW ACCESS KEY MODAL */}
          {showNewKeyModal && (
            <div className="fixed inset-0 bg-black/80 backdrop-blur-sm z-50 flex items-center justify-center p-4">
              <div className="bg-obsidian-900 border border-obsidian-700 rounded-xl max-w-md w-full p-6 shadow-2xl">
                <h3 className="text-base font-mono font-bold text-slate-100 mb-4 flex items-center gap-2">
                  <KeyRound className="w-4 h-4 text-emerald-400" />
                  <span>Create Port Gate Passphrase</span>
                </h3>

                {createdKeySecret ? (
                  <div className="space-y-4 font-mono text-xs">
                    <div className="p-4 bg-emerald-950/40 border border-emerald-500/40 rounded-xl space-y-2">
                      <p className="text-emerald-300 font-bold">Passphrase Key Generated!</p>
                      <p className="text-slate-400 text-[11px]">
                        Copy and share this passphrase with players. For security, it will not be displayed again.
                      </p>
                      <div className="flex items-center justify-between p-2.5 bg-obsidian-950 border border-emerald-500/30 rounded-lg">
                        <code className="text-emerald-400 font-bold text-sm select-all">{createdKeySecret}</code>
                        <button
                          type="button"
                          onClick={() => {
                            navigator.clipboard.writeText(createdKeySecret);
                            setCopiedSecret(true);
                            setTimeout(() => setCopiedSecret(false), 2000);
                          }}
                          className="px-2.5 py-1 rounded bg-emerald-600 hover:bg-emerald-500 text-slate-950 font-bold text-[11px] flex items-center gap-1"
                        >
                          {copiedSecret ? <Check className="w-3 h-3" /> : <Copy className="w-3 h-3" />}
                          <span>{copiedSecret ? 'Copied' : 'Copy'}</span>
                        </button>
                      </div>
                    </div>

                    <div className="flex justify-end pt-2">
                      <button
                        type="button"
                        onClick={() => {
                          setShowNewKeyModal(false);
                          setCreatedKeySecret(null);
                        }}
                        className="px-4 py-2 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-slate-950 font-bold"
                      >
                        Done
                      </button>
                    </div>
                  </div>
                ) : (
                  <form onSubmit={handleCreateAccessKey} className="space-y-4 font-mono text-xs">
                    <div>
                      <label className="block text-slate-300 mb-1 font-bold">Key Label / Description</label>
                      <input
                        type="text"
                        required
                        placeholder="e.g. Discord Community Key"
                        value={newKeyLabel}
                        onChange={(e) => setNewKeyLabel(e.target.value)}
                        className="w-full px-3 py-2 bg-obsidian-950 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
                      />
                    </div>
                    <div>
                      <label className="block text-slate-300 mb-1 font-bold">Custom Passphrase (optional)</label>
                      <input
                        type="text"
                        placeholder="Leave blank to auto-generate secure key"
                        value={newKeyPassphrase}
                        onChange={(e) => setNewKeyPassphrase(e.target.value)}
                        className="w-full px-3 py-2 bg-obsidian-950 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
                      />
                    </div>
                    <div className="grid grid-cols-2 gap-3">
                      <div>
                        <label className="block text-slate-300 mb-1 font-bold">Max Uses (0 = unlimited)</label>
                        <input
                          type="number"
                          min="0"
                          value={newKeyMaxUses}
                          onChange={(e) => setNewKeyMaxUses(parseInt(e.target.value) || 0)}
                          className="w-full px-3 py-2 bg-obsidian-950 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
                        />
                      </div>
                      <div>
                        <label className="block text-slate-300 mb-1 font-bold">Duration (minutes)</label>
                        <input
                          type="number"
                          min="5"
                          value={newKeyDurationMinutes}
                          onChange={(e) => setNewKeyDurationMinutes(parseInt(e.target.value) || 120)}
                          className="w-full px-3 py-2 bg-obsidian-950 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
                        />
                      </div>
                    </div>
                    <div className="flex justify-end space-x-3 pt-3 border-t border-obsidian-800">
                      <button
                        type="button"
                        onClick={() => setShowNewKeyModal(false)}
                        className="px-4 py-2 rounded-lg bg-obsidian-800 text-slate-300 hover:bg-obsidian-700"
                      >
                        Cancel
                      </button>
                      <button
                        type="submit"
                        className="px-4 py-2 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-slate-950 font-bold"
                      >
                        Generate Key
                      </button>
                    </div>
                  </form>
                )}
              </div>
            </div>
          )}

          {/* ALWAYS-ALLOWED IPS & SUBNETS (PERMANENT WHITELIST) CARD */}
          <div className="bg-obsidian-900 border border-obsidian-700/80 rounded-xl p-6">
            <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 mb-6">
              <div>
                <h3 className="font-mono text-base font-bold text-slate-100 flex items-center gap-2">
                  <Globe className="w-4 h-4 text-emerald-400" />
                  <span>Always-Allowed IPs & Subnets (Permanent Whitelist)</span>
                </h3>
                <p className="text-xs text-slate-400 mt-1 font-mono">
                  IP addresses and CIDR subnets that are permanently authorized on the firewall. Players from these networks can connect directly without logging in or using a knock passphrase.
                </p>
              </div>
              <div className="flex items-center gap-2 shrink-0">
                <Link
                  to="/portgate"
                  className="px-3 py-1.5 rounded-lg bg-obsidian-800 hover:bg-obsidian-750 border border-obsidian-700 text-slate-300 font-mono text-xs flex items-center space-x-1.5 transition-colors"
                >
                  <Shield className="w-3.5 h-3.5 text-emerald-400" />
                  <span>Port Gate Manager</span>
                </Link>
                <button
                  onClick={() => {
                    setNewAllowIP(detectedClientIP || '');
                    setNewAllowComment('');
                    setNewAllowScope('server');
                    setShowAddAllowModal(true);
                  }}
                  className="px-3 py-1.5 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-slate-950 font-mono text-xs font-bold flex items-center space-x-1.5 shadow-[0_0_10px_rgba(16,185,129,0.2)]"
                >
                  <Plus className="w-3.5 h-3.5" />
                  <span>+ Add Allowed IP / Range</span>
                </button>
              </div>
            </div>

            {allowRules.length === 0 ? (
              <div className="text-center py-8 bg-obsidian-950/60 border border-obsidian-800 rounded-lg text-slate-500 font-mono text-xs">
                No permanent IP allowlist rules configured. Add trusted client IPs or LAN/VPN subnets (e.g. 192.168.1.0/24) for zero-friction access.
              </div>
            ) : (
              <div className="border border-obsidian-800 rounded-lg overflow-hidden">
                <table className="w-full text-left font-mono text-xs">
                  <thead className="bg-obsidian-950 text-slate-400 border-b border-obsidian-800 uppercase">
                    <tr>
                      <th className="px-4 py-2.5">IP / CIDR Range</th>
                      <th className="px-4 py-2.5">Scope</th>
                      <th className="px-4 py-2.5">Description / Note</th>
                      <th className="px-4 py-2.5">Added Date</th>
                      <th className="px-4 py-2.5 text-right">Action</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-obsidian-800 bg-obsidian-950/40">
                    {paginatedAllowRules.map((rule) => {
                      const isGlobal = !rule.server_id;
                      return (
                        <tr key={rule.id} className="hover:bg-obsidian-800/40">
                          <td className="px-4 py-2.5 text-emerald-400 font-bold font-mono">
                            {rule.ip_or_subnet}
                          </td>
                          <td className="px-4 py-2.5">
                            {isGlobal ? (
                              <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-bold bg-sky-500/10 text-sky-400 border border-sky-500/30">
                                <Globe className="w-3 h-3" />
                                <span>All Instances (Global)</span>
                              </span>
                            ) : (
                              <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-bold bg-emerald-500/10 text-emerald-400 border border-emerald-500/30">
                                <Shield className="w-3 h-3" />
                                <span>This Instance Only</span>
                              </span>
                            )}
                          </td>
                          <td className="px-4 py-2.5 text-slate-300">
                            {rule.comment || '—'}
                          </td>
                          <td className="px-4 py-2.5 text-slate-400">
                            {new Date(rule.created_at).toLocaleString()}
                          </td>
                          <td className="px-4 py-2.5 text-right">
                            <button
                              onClick={() => handleDeleteAllowRule(rule.id)}
                              className="px-2 py-1 rounded bg-rose-950/60 hover:bg-rose-900 border border-rose-800 text-rose-300 text-[10px] font-bold"
                            >
                              Delete
                            </button>
                          </td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
                <Pagination
                  currentPage={allowRulesPage}
                  totalItems={totalAllowRules}
                  pageSize={allowRulesPageSize}
                  onPageChange={setAllowRulesPage}
                  onPageSizeChange={setAllowRulesPageSize}
                  pageSizeOptions={[5, 10, 25, 50]}
                />
              </div>
            )}
          </div>

          {/* ADD ALLOW RULE MODAL */}
          {showAddAllowModal && (
            <div className="fixed inset-0 bg-black/80 backdrop-blur-sm z-50 flex items-center justify-center p-4">
              <div className="bg-obsidian-900 border border-obsidian-700 rounded-xl max-w-md w-full p-6 shadow-2xl">
                <h3 className="text-base font-mono font-bold text-slate-100 mb-4 flex items-center gap-2">
                  <Globe className="w-4 h-4 text-emerald-400" />
                  <span>Add Permanent Allowed IP / Range</span>
                </h3>
                <form onSubmit={handleCreateAllowRule} className="space-y-4 font-mono text-xs">
                  <div>
                    <div className="flex items-center justify-between mb-1">
                      <label className="text-slate-300 font-bold">IP Address or CIDR Range</label>
                      {detectedClientIP && (
                        <button
                          type="button"
                          onClick={() => setNewAllowIP(detectedClientIP)}
                          className="text-[11px] text-emerald-400 hover:text-emerald-300 underline"
                        >
                          Use My IP ({detectedClientIP})
                        </button>
                      )}
                    </div>
                    <input
                      type="text"
                      required
                      placeholder="e.g. 192.168.1.100 or 10.0.0.0/24"
                      value={newAllowIP}
                      onChange={(e) => setNewAllowIP(e.target.value)}
                      className="w-full px-3 py-2 bg-obsidian-950 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
                    />
                    <p className="text-[11px] text-slate-500 mt-1">
                      Supports single IPv4/IPv6 or CIDR subnets (e.g. <code>192.168.1.0/24</code>).
                    </p>
                  </div>

                  <div>
                    <label className="block text-slate-300 mb-1 font-bold">Scope</label>
                    <div className="grid grid-cols-2 gap-2">
                      <button
                        type="button"
                        onClick={() => setNewAllowScope('server')}
                        className={`p-2.5 rounded-lg border text-left transition-colors ${
                          newAllowScope === 'server'
                            ? 'bg-emerald-950/60 border-emerald-500 text-emerald-300'
                            : 'bg-obsidian-950 border-obsidian-800 text-slate-400 hover:border-obsidian-700'
                        }`}
                      >
                        <div className="font-bold flex items-center gap-1">
                          <Shield className="w-3.5 h-3.5" />
                          <span>This Instance</span>
                        </div>
                        <div className="text-[10px] text-slate-400 mt-0.5">UDP {server.port} only</div>
                      </button>

                      <button
                        type="button"
                        onClick={() => setNewAllowScope('global')}
                        className={`p-2.5 rounded-lg border text-left transition-colors ${
                          newAllowScope === 'global'
                            ? 'bg-sky-950/60 border-sky-500 text-sky-300'
                            : 'bg-obsidian-950 border-obsidian-800 text-slate-400 hover:border-obsidian-700'
                        }`}
                      >
                        <div className="font-bold flex items-center gap-1">
                          <Globe className="w-3.5 h-3.5" />
                          <span>All Instances</span>
                        </div>
                        <div className="text-[10px] text-slate-400 mt-0.5">Global across all servers</div>
                      </button>
                    </div>
                  </div>

                  <div>
                    <label className="block text-slate-300 mb-1 font-bold">Comment / Note (optional)</label>
                    <input
                      type="text"
                      placeholder="e.g. Home Office, LAN Subnet, Admin Mobile"
                      value={newAllowComment}
                      onChange={(e) => setNewAllowComment(e.target.value)}
                      className="w-full px-3 py-2 bg-obsidian-950 border border-obsidian-700 rounded-lg text-slate-200 focus:outline-none focus:border-emerald-500"
                    />
                  </div>

                  <div className="flex justify-end space-x-3 pt-3 border-t border-obsidian-800">
                    <button
                      type="button"
                      onClick={() => setShowAddAllowModal(false)}
                      className="px-4 py-2 rounded-lg bg-obsidian-800 text-slate-300 hover:bg-obsidian-700"
                    >
                      Cancel
                    </button>
                    <button
                      type="submit"
                      disabled={allowSubmitting}
                      className="px-4 py-2 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-slate-950 font-bold flex items-center gap-1.5"
                    >
                      {allowSubmitting ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : null}
                      <span>Save Rule</span>
                    </button>
                  </div>
                </form>
              </div>
            </div>
          )}
        </div>
      )}

      {/* BACKUPS & WORLDS TAB */}
      {activeTab === 'backups' && (
        <div className="space-y-6">
          <div className="bg-obsidian-900 border border-obsidian-700/80 rounded-xl p-6">
            <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 mb-6">
              <div>
                <h3 className="font-mono text-base font-bold text-slate-100 flex items-center gap-2">
                  <Archive className="w-4 h-4 text-emerald-400" />
                  <span>Zero-Downtime Hot Backups</span>
                </h3>
                <p className="text-xs text-slate-400 mt-1 font-mono">
                  LevelDB safe snapshots using Bedrock's save hold/resume protocol without taking the server offline.
                </p>
              </div>

              <div className="flex items-center flex-wrap gap-2">
                <label className="cursor-pointer px-3 py-1.5 rounded-lg bg-obsidian-800 hover:bg-obsidian-700 text-slate-200 border border-obsidian-600 font-mono text-xs font-bold flex items-center space-x-1.5">
                  <Upload className="w-3.5 h-3.5 text-emerald-400" />
                  <span>Import World</span>
                  <input
                    type="file"
                    accept=".mcworld,.zip"
                    onChange={handleImportWorld}
                    className="hidden"
                  />
                </label>

                <a
                  href={api.exportWorldUrl(server.id)}
                  download
                  className="px-3 py-1.5 rounded-lg bg-obsidian-800 hover:bg-obsidian-700 text-slate-200 border border-obsidian-600 font-mono text-xs font-bold flex items-center space-x-1.5"
                >
                  <Download className="w-3.5 h-3.5 text-emerald-400" />
                  <span>Export .mcworld</span>
                </a>

                <button
                  onClick={handleCreateBackup}
                  disabled={backupLoading}
                  className="px-4 py-1.5 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-slate-950 font-mono text-xs font-bold flex items-center space-x-1.5 disabled:opacity-50"
                >
                  {backupLoading ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <Play className="w-3.5 h-3.5" />}
                  <span>Hot Backup Now</span>
                </button>
              </div>
            </div>

            <h4 className="font-mono text-xs font-bold uppercase tracking-wider text-slate-400 mb-3">
              Stored World Archives ({backupsList.length})
            </h4>

            {backupsList.length === 0 ? (
              <div className="text-center py-10 bg-obsidian-950/60 border border-obsidian-800 rounded-lg text-slate-500 font-mono text-xs">
                No backups recorded yet. Click "Hot Backup Now" to create an instant LevelDB snapshot.
              </div>
            ) : (
              <div className="border border-obsidian-800 rounded-lg overflow-hidden">
                <table className="w-full text-left font-mono text-xs">
                  <thead className="bg-obsidian-950 text-slate-400 border-b border-obsidian-800 uppercase">
                    <tr>
                      <th className="px-4 py-2.5">Pin</th>
                      <th className="px-4 py-2.5">Archive File</th>
                      <th className="px-4 py-2.5">Size</th>
                      <th className="px-4 py-2.5">Type</th>
                      <th className="px-4 py-2.5">Timestamp</th>
                      <th className="px-4 py-2.5 text-right">Actions</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-obsidian-800 bg-obsidian-950/40">
                    {paginatedBackups.map((b) => (
                      <tr key={b.id} className="hover:bg-obsidian-800/40">
                        <td className="px-4 py-2.5">
                          <button
                            onClick={() => handleToggleBackupLock(b.id)}
                            title={b.is_locked ? 'Pinned/Locked against retention purge' : 'Click to pin/lock'}
                            className={`p-1 rounded hover:bg-obsidian-800 transition-colors ${
                              b.is_locked ? 'text-amber-400' : 'text-slate-600 hover:text-slate-400'
                            }`}
                          >
                            {b.is_locked ? <Lock className="w-3.5 h-3.5" /> : <Unlock className="w-3.5 h-3.5" />}
                          </button>
                        </td>
                        <td className="px-4 py-2.5 font-bold text-slate-200">{b.filename}</td>
                        <td className="px-4 py-2.5 text-slate-300">
                          {(b.size_bytes / (1024 * 1024)).toFixed(2)} MB
                        </td>
                        <td className="px-4 py-2.5 capitalize text-slate-400">
                          <span className="px-2 py-0.5 rounded bg-obsidian-800 border border-obsidian-700">
                            {b.type}
                          </span>
                        </td>
                        <td className="px-4 py-2.5 text-slate-400">
                          {new Date(b.created_at).toLocaleString()}
                        </td>
                        <td className="px-4 py-2.5 text-right space-x-2">
                          <a
                            href={api.downloadBackupUrl(server.id, b.id)}
                            download
                            title="Download Archive"
                            className="inline-block p-1.5 rounded bg-obsidian-800 hover:bg-obsidian-700 text-slate-300 transition-colors"
                          >
                            <Download className="w-3.5 h-3.5" />
                          </a>
                          <button
                            onClick={() => handleRestoreBackup(b.id)}
                            title="Restore to Server"
                            className="p-1.5 rounded bg-amber-500/20 text-amber-400 hover:bg-amber-500 hover:text-slate-950 transition-colors"
                          >
                            <RefreshCw className="w-3.5 h-3.5" />
                          </button>
                          <button
                            onClick={() => handleDeleteBackup(b.id)}
                            disabled={b.is_locked}
                            title={b.is_locked ? 'Locked backup cannot be deleted' : 'Delete Backup'}
                            className="p-1.5 rounded bg-rose-600/20 text-rose-400 hover:bg-rose-600 hover:text-slate-950 transition-colors disabled:opacity-30 disabled:cursor-not-allowed"
                          >
                            <Trash2 className="w-3.5 h-3.5" />
                          </button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
                <Pagination
                  currentPage={backupsPage}
                  totalItems={totalBackups}
                  pageSize={backupsPageSize}
                  onPageChange={setBackupsPage}
                  onPageSizeChange={setBackupsPageSize}
                  pageSizeOptions={[5, 10, 25, 50]}
                />
              </div>
            )}
          </div>
        </div>
      )}

      {/* ADDONS & PACKS TAB */}
      {activeTab === 'addons' && (
        <div className="space-y-6">
          <div className="bg-obsidian-900 border border-obsidian-700/80 rounded-xl p-6">
            <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 mb-6">
              <div>
                <h3 className="font-mono text-base font-bold text-slate-100 flex items-center gap-2">
                  <Package className="w-4 h-4 text-emerald-400" />
                  <span>Bedrock Addons & Resource Packs</span>
                </h3>
                <p className="text-xs text-slate-400 mt-1 font-mono">
                  Upload .mcpack or .zip archives to install behavior and texture packs into the server.
                </p>
              </div>

              <div>
                <label className="cursor-pointer px-4 py-2 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-slate-950 font-mono text-xs font-bold flex items-center space-x-2">
                  {addonLoading ? <Loader2 className="w-4 h-4 animate-spin" /> : <Upload className="w-4 h-4" />}
                  <span>Install .mcpack / .zip</span>
                  <input
                    type="file"
                    accept=".mcpack,.mcaddon,.zip"
                    onChange={handleInstallAddon}
                    disabled={addonLoading}
                    className="hidden"
                  />
                </label>
              </div>
            </div>

            <h4 className="font-mono text-xs font-bold uppercase tracking-wider text-slate-400 mb-3">
              Installed Packs ({(addonsList || []).length})
            </h4>

            {(addonsList || []).length === 0 ? (
              <div className="text-center py-10 bg-obsidian-950/60 border border-obsidian-800 rounded-lg text-slate-500 font-mono text-xs">
                No custom addons installed yet. Click "Install .mcpack / .zip" to add behavior or texture packs.
              </div>
            ) : (
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                {(addonsList || []).map((pack) => (
                  <div
                    key={`${pack.type}-${pack.folder}`}
                    className="bg-obsidian-950 border border-obsidian-800 rounded-lg p-4 flex flex-col justify-between"
                  >
                    <div>
                      <div className="flex items-center justify-between mb-2">
                        <span
                          className={`text-[10px] font-mono px-2 py-0.5 rounded uppercase font-bold border ${
                            pack.type === 'behavior'
                              ? 'bg-purple-500/10 text-purple-400 border-purple-500/30'
                              : 'bg-cyan-500/10 text-cyan-400 border-cyan-500/30'
                          }`}
                        >
                          {pack.type === 'behavior' ? 'Behavior Pack' : 'Resource Pack'}
                        </span>
                        <span className="text-[11px] font-mono text-slate-400">v{pack.version}</span>
                      </div>
                      <h5 className="font-mono font-bold text-sm text-slate-200">{pack.name}</h5>
                      <p className="font-mono text-xs text-slate-400 mt-1 line-clamp-2">
                        {pack.description || 'No description provided.'}
                      </p>
                    </div>

                    <div className="mt-4 pt-3 border-t border-obsidian-800/80 flex items-center justify-between font-mono text-[11px] text-slate-500">
                      <span>Folder: {pack.folder}</span>
                      <button
                        onClick={() => handleDeleteAddon(pack.type, pack.folder)}
                        className="text-rose-400 hover:text-rose-300 flex items-center space-x-1"
                      >
                        <Trash2 className="w-3.5 h-3.5" />
                        <span>Remove</span>
                      </button>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>
      )}

      {/* CONFIGURATION EDITOR TAB */}
      {activeTab === 'settings' && (
        <div className="bg-obsidian-900 border border-obsidian-700/80 rounded-xl p-6 max-w-4xl space-y-8">
          {/* SERVER INSTANCE CONFIGURATION */}
          {user.role === 'admin' && (
            <div className="border-b border-obsidian-800 pb-8">
            <div className="flex items-center justify-between mb-4">
              <div>
                <h3 className="font-mono text-base font-bold text-slate-100 flex items-center gap-2">
                  <Settings className="w-4 h-4 text-emerald-400" />
                  <span>Server Instance Configuration</span>
                </h3>
                <p className="text-xs text-slate-400 font-mono mt-0.5">
                  Update server metadata, memory limits, autostart, and port gating policy.
                </p>
              </div>

              <button
                onClick={handleUpdateServerSettings}
                disabled={serverUpdating}
                className="px-4 py-2 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-slate-950 font-bold font-mono text-xs flex items-center space-x-1.5 shrink-0"
              >
                {serverUpdating ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <Check className="w-3.5 h-3.5" />}
                <span>Save Server Settings</span>
              </button>
            </div>

            {serverUpdatedMsg && (
              <div className="mb-4 p-3 rounded-xl bg-emerald-950/40 border border-emerald-500/40 text-emerald-300 font-mono text-xs flex items-center space-x-2">
                <Check className="w-4 h-4 text-emerald-400 shrink-0" />
                <span>{serverUpdatedMsg}</span>
              </div>
            )}

            <form onSubmit={handleUpdateServerSettings} className="space-y-4 font-mono text-xs bg-obsidian-950 p-4 rounded-xl border border-obsidian-800">
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div>
                  <label className="block text-slate-400 mb-1 font-bold">Server Name</label>
                  <input
                    type="text"
                    required
                    value={editServerName}
                    onChange={(e) => setEditServerName(e.target.value)}
                    className="w-full px-3 py-2 rounded bg-obsidian-900 border border-obsidian-700 text-slate-100 text-xs focus:border-emerald-500"
                  />
                </div>

                <div>
                  <label className="block text-slate-400 mb-1 font-bold">BDS Version</label>
                  <input
                    type="text"
                    required
                    value={editServerVersion}
                    onChange={(e) => setEditServerVersion(e.target.value)}
                    placeholder="latest"
                    className="w-full px-3 py-2 rounded bg-obsidian-900 border border-obsidian-700 text-slate-100 text-xs focus:border-emerald-500"
                  />
                </div>
              </div>

              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div>
                  <label className="block text-slate-400 mb-1 font-bold">UDP Port (IPv4)</label>
                  <input
                    type="number"
                    required
                    min="1025"
                    max="65535"
                    value={editServerPort}
                    onChange={(e) => setEditServerPort(parseInt(e.target.value) || 19132)}
                    className="w-full px-3 py-2 rounded bg-obsidian-900 border border-obsidian-700 text-slate-100 text-xs focus:border-emerald-500"
                  />
                </div>

                <div>
                  <label className="block text-slate-400 mb-1 font-bold">UDP Port (IPv6)</label>
                  <input
                    type="number"
                    required
                    min="1025"
                    max="65535"
                    value={editServerPortV6}
                    onChange={(e) => setEditServerPortV6(parseInt(e.target.value) || 19133)}
                    className="w-full px-3 py-2 rounded bg-obsidian-900 border border-obsidian-700 text-slate-100 text-xs focus:border-emerald-500"
                  />
                </div>
              </div>

              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div>
                  <label className="block text-slate-400 mb-1 font-bold">RAM Limit</label>
                  <select
                    value={editServerMemLimit}
                    onChange={(e) => setEditServerMemLimit(e.target.value)}
                    className="w-full px-3 py-2 rounded bg-obsidian-900 border border-obsidian-700 text-slate-100 text-xs focus:border-emerald-500"
                  >
                    <option value="1G">1 GB</option>
                    <option value="2G">2 GB (Recommended)</option>
                    <option value="4G">4 GB</option>
                    <option value="8G">8 GB</option>
                  </select>
                </div>

                <div>
                  <label className="block text-slate-400 mb-1 font-bold">CPU Core Limit</label>
                  <select
                    value={editServerCpuLimit}
                    onChange={(e) => setEditServerCpuLimit(parseFloat(e.target.value) || 2.0)}
                    className="w-full px-3 py-2 rounded bg-obsidian-900 border border-obsidian-700 text-slate-100 text-xs focus:border-emerald-500"
                  >
                    <option value={1.0}>1.0 Core</option>
                    <option value={2.0}>2.0 Cores (Recommended)</option>
                    <option value={4.0}>4.0 Cores</option>
                    <option value={8.0}>8.0 Cores</option>
                  </select>
                </div>
              </div>

              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div>
                  <label className="block text-slate-400 mb-1 font-bold">Game Mode</label>
                  <select
                    value={editServerMode}
                    onChange={(e) => setEditServerMode(e.target.value)}
                    className="w-full px-3 py-2 rounded bg-obsidian-900 border border-obsidian-700 text-slate-100 text-xs focus:border-emerald-500"
                  >
                    <option value="survival">Survival</option>
                    <option value="creative">Creative</option>
                    <option value="adventure">Adventure</option>
                  </select>
                </div>

                <div>
                  <label className="block text-slate-400 mb-1 font-bold">Difficulty</label>
                  <select
                    value={editServerDifficulty}
                    onChange={(e) => setEditServerDifficulty(e.target.value)}
                    className="w-full px-3 py-2 rounded bg-obsidian-900 border border-obsidian-700 text-slate-100 text-xs focus:border-emerald-500"
                  >
                    <option value="peaceful">Peaceful</option>
                    <option value="easy">Easy</option>
                    <option value="normal">Normal</option>
                    <option value="hard">Hard</option>
                  </select>
                </div>
              </div>

              <div>
                <label className="block text-slate-400 mb-1 font-bold flex items-center justify-between">
                  <span>World Seed</span>
                  <span className="text-[10px] text-slate-500 font-normal">Seed for world generation. Changes apply to newly generated chunks.</span>
                </label>
                <input
                  type="text"
                  value={editServerSeed}
                  onChange={(e) => setEditServerSeed(e.target.value)}
                  placeholder="e.g. 123456789 or custom-seed (leave empty for random seed)"
                  className="w-full px-3 py-2 rounded bg-obsidian-900 border border-obsidian-700 text-slate-100 text-xs focus:border-emerald-500 font-mono"
                />
              </div>

              <div>
                <label className="block text-slate-400 mb-1 font-bold flex items-center justify-between">
                  <span>Game Server Address (Knock Portal Hostname / IP)</span>
                  <span className="text-[10px] text-slate-500 font-normal">Optional: e.g. play.example.com or server public IP. Defaults to panel host.</span>
                </label>
                <input
                  type="text"
                  value={editGameServerAddress}
                  onChange={(e) => setEditGameServerAddress(e.target.value)}
                  placeholder={`Optional (defaults to ${window.location.hostname})`}
                  className="w-full px-3 py-2 rounded bg-obsidian-900 border border-obsidian-700 text-slate-100 text-xs focus:border-emerald-500 font-mono"
                />
              </div>

              <div className="pt-2 border-t border-obsidian-800/60 grid grid-cols-1 md:grid-cols-2 gap-4 items-center">
                <label className="flex items-center space-x-2.5 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={editServerAutostart}
                    onChange={(e) => setEditServerAutostart(e.target.checked)}
                    className="rounded bg-obsidian-900 border-obsidian-700 text-emerald-500 focus:ring-emerald-500"
                  />
                  <div>
                    <span className="text-slate-200 font-bold block">Autostart on Daemon Boot</span>
                    <span className="text-[10px] text-slate-400">Launch this server automatically when system restarts</span>
                  </div>
                </label>

                <label className="flex items-center space-x-2.5 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={editServerPortGate}
                    onChange={(e) => setEditServerPortGate(e.target.checked)}
                    className="rounded bg-obsidian-900 border-obsidian-700 text-emerald-500 focus:ring-emerald-500"
                  />
                  <div>
                    <span className="text-slate-200 font-bold block">Dynamic Port Gating</span>
                    <span className="text-[10px] text-slate-400">Enforce firewall knock before allowing UDP connections</span>
                  </div>
                </label>
              </div>

              {editServerPortGate && (
                <div className="p-3 bg-obsidian-900/60 border border-obsidian-800 rounded-lg grid grid-cols-1 sm:grid-cols-2 gap-3 mt-2">
                  <div>
                    <label className="block text-slate-400 mb-1 text-[11px] font-bold">Knock Verification Mode</label>
                    <select
                      value={editServerPortGateMode}
                      onChange={(e) => setEditServerPortGateMode(e.target.value as any)}
                      className="w-full px-2.5 py-1.5 rounded bg-obsidian-950 border border-obsidian-700 text-slate-200 text-xs"
                    >
                      <option value="passphrase">Passphrase / Access Key</option>
                      <option value="gamertag">Gamertag Allowlist</option>
                      <option value="combined">Combined (Passphrase + Gamertag)</option>
                    </select>
                  </div>
                  <div>
                    <label className="block text-slate-400 mb-1 text-[11px] font-bold">Lease Timeout (seconds)</label>
                    <input
                      type="number"
                      min="60"
                      value={editServerPortGateTimeout}
                      onChange={(e) => setEditServerPortGateTimeout(parseInt(e.target.value) || 7200)}
                      className="w-full px-2.5 py-1.5 rounded bg-obsidian-950 border border-obsidian-700 text-slate-200 text-xs"
                    />
                  </div>
                </div>
              )}
            </form>
          </div>
          )}

          {/* server.properties form */}
          <div>
            <div className="flex items-center justify-between mb-4">
              <div>
                <h3 className="font-mono text-base font-bold text-slate-100 flex items-center gap-2">
                  <Settings className="w-4 h-4 text-emerald-400" />
                  <span>server.properties Editor</span>
                  <span className="text-xs px-2 py-0.5 rounded-full bg-emerald-500/10 text-emerald-400 border border-emerald-500/30 font-normal">
                    {propKeys.length} properties
                  </span>
                  {propUninitialized && (
                    <span className="text-[10px] px-2 py-0.5 rounded-full bg-amber-500/10 text-amber-400 border border-amber-500/30 font-bold">
                      {propPending ? 'Pending Pre-Boot Changes' : 'Pre-Boot Mode'}
                    </span>
                  )}
                </h3>
                <p className="text-xs text-slate-400 font-mono mt-0.5">
                  Configure world seeds, view distance, tick distance, and dedicated server flags.
                </p>
              </div>

              <button
                onClick={handleSaveProperties}
                disabled={configSaving || propKeys.length === 0}
                className="px-4 py-2 rounded-lg bg-emerald-600 hover:bg-emerald-500 disabled:opacity-40 disabled:hover:bg-emerald-600 text-slate-950 font-bold font-mono text-xs flex items-center space-x-1.5 shrink-0"
              >
                {configSaving ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : configSaved ? <Check className="w-3.5 h-3.5" /> : null}
                <span>{configSaved ? (propPending ? 'Saved (Pending Boot)!' : 'Saved!') : propUninitialized ? 'Save Pre-Boot Config' : 'Save Properties'}</span>
              </button>
            </div>

            {propUninitialized && (
              <div className="mb-4 p-3.5 rounded-xl bg-amber-950/30 border border-amber-500/30 text-amber-300 font-mono text-xs flex items-start space-x-2.5">
                <Sparkles className="w-4 h-4 text-amber-400 mt-0.5 shrink-0" />
                <div className="space-y-1">
                  <span className="font-bold flex items-center gap-1.5">
                    <span>{propPending ? 'Pre-Boot Configuration Saved' : 'Pre-Boot Configuration Mode'}</span>
                  </span>
                  <p className="text-[11px] text-amber-400/90 leading-relaxed">
                    {propPending
                      ? 'Your custom properties are queued. When you launch this instance, the official Minecraft BDS template will unpack, your customized properties will be merged, and the container will restart automatically if changes were made.'
                      : 'You can configure any property before launching. On first start, the server engine will unpack the official template and automatically merge your settings.'}
                  </p>
                </div>
              </div>
            )}

            {propKeys.length === 0 ? (
              <div className="p-8 text-center bg-obsidian-950 rounded-xl border border-obsidian-800 font-mono space-y-2">
                <Settings className="w-8 h-8 text-slate-600 mx-auto" />
                <h4 className="text-sm font-bold text-slate-300">Properties Not Available</h4>
                <p className="text-xs text-slate-400 max-w-md mx-auto leading-relaxed">
                  Start this server instance once to allow the Minecraft Bedrock engine to automatically download and unpack the official version-specific <code className="text-emerald-400 font-bold">server.properties</code> template.
                </p>
              </div>
            ) : (
              <>
                <div className="mb-3">
                  <input
                    type="text"
                    value={propertyFilter}
                    onChange={(e) => setPropertyFilter(e.target.value)}
                    placeholder="Filter properties (e.g. seed, cheats, view-distance, max-players)..."
                    className="w-full px-3 py-2 rounded-lg bg-obsidian-950 border border-obsidian-800 text-slate-200 text-xs font-mono placeholder:text-slate-500 focus:border-emerald-500 focus:outline-none"
                  />
                </div>

                <div className="grid grid-cols-1 md:grid-cols-2 gap-4 font-mono text-xs bg-obsidian-950 p-4 rounded-xl border border-obsidian-800 max-h-[520px] overflow-y-auto">
                  {(propKeys || [])
                    .filter((key) => key && key.toLowerCase().includes(propertyFilter.toLowerCase()))
                    .map((key) => (
                      <div key={key}>
                        <label className="block text-slate-400 mb-1 text-[11px] font-bold">{key}</label>
                        <input
                          type="text"
                          value={(properties && properties[key]) || ''}
                          onChange={(e) => setProperties({ ...(properties || {}), [key]: e.target.value })}
                          className="w-full px-3 py-1.5 rounded bg-obsidian-900 border border-obsidian-700 text-slate-100 text-xs focus:border-emerald-500"
                        />
                      </div>
                    ))}
                </div>
              </>
            )}
          </div>

          {/* Multi-level Global Sync Notification */}
          {syncReportMsg && (
            <div className="p-3 rounded-xl bg-emerald-950/40 border border-emerald-500/40 text-emerald-300 font-mono text-xs flex items-center space-x-2">
              <Check className="w-4 h-4 text-emerald-400 shrink-0" />
              <span>{syncReportMsg}</span>
            </div>
          )}

          {/* allowlist.json manager */}
          <div>
            <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 mb-3">
              <div>
                <h3 className="font-mono text-base font-bold text-slate-100 flex items-center gap-2">
                  <ShieldAlert className="w-4 h-4 text-cyber-cyan" />
                  <span>Allowlist Manager (allowlist.json)</span>
                </h3>
                <p className="text-xs text-slate-400 font-mono mt-0.5">
                  Merge and manage players permitted on this server. Global entries sync automatically across instances.
                </p>
              </div>

              <button
                onClick={handleSyncGlobalNow}
                disabled={syncingGlobal}
                title="Pull and merge global allowlist and permissions"
                className="px-3 py-1.5 rounded-lg bg-obsidian-950 hover:bg-obsidian-850 border border-obsidian-700 text-slate-300 hover:text-emerald-400 font-mono text-xs font-bold flex items-center space-x-1.5 transition-colors disabled:opacity-50 shrink-0"
              >
                {syncingGlobal ? <Loader2 className="w-3.5 h-3.5 animate-spin text-emerald-400" /> : <Globe className="w-3.5 h-3.5 text-emerald-400" />}
                <span>Sync from Global</span>
              </button>
            </div>

            <div className="bg-obsidian-950 p-4 rounded-xl border border-obsidian-800 space-y-3 font-mono text-xs">
              <div className="flex gap-2">
                <input
                  type="text"
                  value={newAllowlistPlayer}
                  onChange={(e) => setNewAllowlistPlayer(e.target.value)}
                  placeholder="Enter player Gamertag..."
                  className="flex-1 px-3 py-1.5 rounded bg-obsidian-900 border border-obsidian-700 text-slate-100 text-xs"
                />
                <button
                  onClick={handleAddAllowlistPlayer}
                  className="px-3 py-1.5 rounded bg-emerald-600 hover:bg-emerald-500 text-slate-950 font-bold flex items-center space-x-1"
                >
                  <UserPlus className="w-3.5 h-3.5" />
                  <span>Add Player</span>
                </button>
              </div>

              <div className="space-y-1.5 pt-2">
                {(allowlist || []).length === 0 ? (
                  <p className="text-slate-600 italic">No players in allowlist.</p>
                ) : (
                  <>
                    <div className="space-y-1.5">
                      {paginatedAllowlist.map((p) => {
                        const isGlobal = (globalPlayers || []).some(
                          (gp) => gp && (((gp.name || '').toLowerCase() === (p.name || '').toLowerCase()) || (p.xuid && gp.xuid === p.xuid)) && gp.is_allowlisted
                        );
                        const matchingPerm = (permissions || []).find(
                          (perm) => perm && ((p.xuid && perm.xuid === p.xuid) || (perm.xuid || '').toLowerCase() === (p.name || '').toLowerCase())
                        );
                        const effectiveRole = matchingPerm ? matchingPerm.permission : 'member';

                        return (
                          <div key={p.name} className="flex flex-col sm:flex-row sm:items-center justify-between gap-2 p-2.5 rounded bg-obsidian-900 border border-obsidian-800">
                            <div className="flex items-center space-x-2.5">
                              <span className="text-slate-200 font-bold">{p.name}</span>
                              {p.xuid && <span className="text-slate-500 text-[10px]">({p.xuid})</span>}
                              {isGlobal ? (
                                <span className="px-1.5 py-0.5 rounded text-[10px] font-mono font-bold bg-emerald-500/10 text-emerald-400 border border-emerald-500/30 flex items-center gap-1">
                                  <Globe className="w-2.5 h-2.5" /> GLOBAL
                                </span>
                              ) : (
                                <span className="px-1.5 py-0.5 rounded text-[10px] font-mono text-slate-500 bg-obsidian-950 border border-obsidian-800">
                                  LOCAL ONLY
                                </span>
                              )}
                            </div>

                            <div className="flex items-center space-x-3 text-xs">
                              {isGlobal ? (
                                <button
                                  onClick={() => handleRemoveFromGlobal(p.name)}
                                  title="Remove from Global Access List (remains on this server)"
                                  className="text-amber-400 hover:text-amber-300 hover:underline text-[11px] flex items-center gap-1"
                                >
                                  <UserMinus className="w-3 h-3" />
                                  <span>Demote to Local</span>
                                </button>
                              ) : (
                                <button
                                  onClick={() => handlePromoteToGlobal(p.name, p.xuid, effectiveRole, true)}
                                  title="Promote to Global Access List (syncs across all servers)"
                                  className="text-emerald-400 hover:text-emerald-300 hover:underline text-[11px] flex items-center gap-1"
                                >
                                  <Globe className="w-3 h-3" />
                                  <span>Promote to Global</span>
                                </button>
                              )}

                              <span className="text-obsidian-700">|</span>

                              <button
                                onClick={() => handleRemoveAllowlistPlayer(p.name)}
                                className="text-rose-400 hover:underline text-[11px]"
                              >
                                Remove
                              </button>
                            </div>
                          </div>
                        );
                      })}
                    </div>
                    <Pagination
                      currentPage={allowlistPage}
                      totalItems={totalAllowlist}
                      pageSize={allowlistPageSize}
                      onPageChange={setAllowlistPage}
                      onPageSizeChange={setAllowlistPageSize}
                      pageSizeOptions={[5, 10, 20, 50]}
                      className="rounded-lg mt-2"
                    />
                  </>
                )}
              </div>
            </div>
          </div>

          {/* permissions.json manager */}
          <div>
            <h3 className="font-mono text-base font-bold text-slate-100 mb-1 flex items-center gap-2">
              <Crown className="w-4 h-4 text-amber-400" />
              <span>Operators & Player Permissions (permissions.json)</span>
            </h3>
            <p className="text-xs text-slate-400 font-mono mb-3">
              Configure operator and member roles for BDS. Operators gain access to in-game admin commands.
            </p>

            <div className="bg-obsidian-950 p-4 rounded-xl border border-obsidian-800 space-y-3 font-mono text-xs">
              <div className="flex flex-col sm:flex-row gap-2">
                <input
                  type="text"
                  value={newPermXuid}
                  onChange={(e) => setNewPermXuid(e.target.value)}
                  placeholder="Enter Player XUID or Gamertag..."
                  className="flex-1 px-3 py-1.5 rounded bg-obsidian-900 border border-obsidian-700 text-slate-100 text-xs"
                />
                <select
                  value={newPermRole}
                  onChange={(e) => setNewPermRole(e.target.value as any)}
                  className="px-3 py-1.5 rounded bg-obsidian-900 border border-obsidian-700 text-slate-100 text-xs"
                >
                  <option value="operator">Operator (Op)</option>
                  <option value="member">Member</option>
                  <option value="visitor">Visitor</option>
                </select>
                <button
                  onClick={handleAddPermission}
                  className="px-3 py-1.5 rounded bg-amber-500 hover:bg-amber-400 text-slate-950 font-bold flex items-center space-x-1"
                >
                  <Crown className="w-3.5 h-3.5" />
                  <span>Set Role</span>
                </button>
              </div>

              <div className="space-y-1.5 pt-2">
                {(permissions || []).length === 0 ? (
                  <p className="text-slate-600 italic">No custom permissions configured (all players use default member permission).</p>
                ) : (
                  <>
                    <div className="space-y-1.5">
                      {paginatedPermissions.map((p) => {
                        const matchingAllow = (allowlist || []).find(
                          (al) => al && ((al.xuid && al.xuid === p.xuid) || (al.name || '').toLowerCase() === (p.xuid || '').toLowerCase())
                        );
                        const playerName = matchingAllow ? matchingAllow.name : p.xuid;
                        const isGlobalOp = (globalPlayers || []).some(
                          (gp) => gp && (gp.xuid === p.xuid || (gp.name || '').toLowerCase() === (playerName || '').toLowerCase()) && gp.permission === p.permission
                        );

                        return (
                          <div key={p.xuid} className="flex flex-col sm:flex-row sm:items-center justify-between gap-2 p-2.5 rounded bg-obsidian-900 border border-obsidian-800">
                            <div className="flex items-center space-x-2.5">
                              <span
                                className={`text-[10px] font-mono uppercase px-2 py-0.5 rounded font-bold border flex items-center gap-1 ${
                                  p.permission === 'operator'
                                    ? 'bg-amber-500/10 text-amber-400 border-amber-500/30'
                                    : p.permission === 'member'
                                    ? 'bg-purple-500/10 text-purple-400 border-purple-500/30'
                                    : 'bg-slate-500/10 text-slate-400 border-slate-500/30'
                                }`}
                              >
                                {p.permission === 'operator' && <Crown className="w-3 h-3" />}
                                <span>{p.permission}</span>
                              </span>
                              <span className="text-slate-200 font-bold">{playerName}</span>
                              {matchingAllow && <span className="text-slate-500 text-[10px]">({p.xuid})</span>}
                              {isGlobalOp ? (
                                <span className="px-1.5 py-0.5 rounded text-[10px] font-mono font-bold bg-emerald-500/10 text-emerald-400 border border-emerald-500/30 flex items-center gap-1">
                                  <Globe className="w-2.5 h-2.5" /> GLOBAL
                                </span>
                              ) : (
                                <span className="px-1.5 py-0.5 rounded text-[10px] font-mono text-slate-500 bg-obsidian-950 border border-obsidian-800">
                                  LOCAL ONLY
                                </span>
                              )}
                            </div>

                            <div className="flex items-center space-x-3 text-xs">
                              {isGlobalOp ? (
                                <button
                                  onClick={() => handleRemoveFromGlobal(playerName)}
                                  title="Demote from Global role"
                                  className="text-amber-400 hover:text-amber-300 hover:underline text-[11px] flex items-center gap-1"
                                >
                                  <UserMinus className="w-3 h-3" />
                                  <span>Demote from Global</span>
                                </button>
                              ) : (
                                <button
                                  onClick={() => handlePromoteToGlobal(playerName, p.xuid, p.permission, matchingAllow ? true : false)}
                                  title="Promote this operator role to Global"
                                  className="text-emerald-400 hover:text-emerald-300 hover:underline text-[11px] flex items-center gap-1"
                                >
                                  <Globe className="w-3 h-3" />
                                  <span>Promote to Global Op</span>
                                </button>
                              )}

                              <span className="text-obsidian-700">|</span>

                              <button
                                onClick={() => handleRemovePermission(p.xuid)}
                                className="text-rose-400 hover:underline text-[11px]"
                              >
                                Remove
                              </button>
                            </div>
                          </div>
                        );
                      })}
                    </div>
                    <Pagination
                      currentPage={permissionsPage}
                      totalItems={totalPermissions}
                      pageSize={permissionsPageSize}
                      onPageChange={setPermissionsPage}
                      onPageSizeChange={setPermissionsPageSize}
                      pageSizeOptions={[5, 10, 20, 50]}
                      className="rounded-lg mt-2"
                    />
                  </>
                )}
              </div>
            </div>
          </div>
        </div>
      )}

      {/* CLONE & EXPORT TAB */}
      {activeTab === 'actions' && (
        <div className="bg-obsidian-900 border border-obsidian-700/80 rounded-xl p-6 max-w-2xl space-y-8">
          {/* Update Banner */}
          {updateInfo && (
            <div className="p-4 bg-obsidian-950 border border-obsidian-800 rounded-xl font-mono text-xs space-y-2">
              <div className="flex items-center justify-between">
                <span className="text-slate-400">Current BDS Version:</span>
                <span className="text-slate-200 font-bold">{updateInfo.current_version}</span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-slate-400">Latest Available:</span>
                <span className="text-emerald-400 font-bold">{updateInfo.latest_version}</span>
              </div>
              {updateInfo.update_available && (
                <div className="pt-2">
                  <a
                    href={updateInfo.release_url}
                    target="_blank"
                    className="inline-flex items-center space-x-1 text-emerald-400 hover:underline text-xs"
                  >
                    <span>View Release Notes</span>
                    <ExternalLink className="w-3 h-3" />
                  </a>
                </div>
              )}
            </div>
          )}

          {/* 1-Click Clone Form */}
          {user.role === 'admin' && (
            <div>
              <h3 className="font-mono text-base font-bold text-slate-100 mb-2 flex items-center gap-2">
                <Copy className="w-4 h-4 text-emerald-400" />
                <span>1-Click Server Cloning</span>
              </h3>
              <p className="text-xs text-slate-400 font-mono mb-4">
                Duplicate all worlds, configurations, and packs. An unused UDP port is automatically assigned.
              </p>

              <form onSubmit={handleClone} className="space-y-3 font-mono text-xs">
                <div>
                  <label className="block text-slate-400 mb-1">Cloned Server Name</label>
                  <input
                    type="text"
                    required
                    value={cloneName}
                    onChange={(e) => {
                      setCloneName(e.target.value);
                      if (!cloneId) setCloneId(e.target.value.toLowerCase().replace(/[^a-z0-9]/g, '-'));
                    }}
                    placeholder="Survival Clone"
                    className="w-full px-3 py-2 rounded bg-obsidian-950 border border-obsidian-700 text-slate-100 text-xs"
                  />
                </div>

                <div>
                  <label className="block text-slate-400 mb-1">Cloned Server ID</label>
                  <input
                    type="text"
                    required
                    value={cloneId}
                    onChange={(e) => setCloneId(e.target.value)}
                    placeholder="survival-clone"
                    className="w-full px-3 py-2 rounded bg-obsidian-950 border border-obsidian-700 text-slate-100 text-xs"
                  />
                </div>

                <button
                  type="submit"
                  disabled={cloning}
                  className="px-4 py-2.5 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-slate-950 font-bold font-mono text-xs flex items-center space-x-2"
                >
                  {cloning ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <Copy className="w-3.5 h-3.5" />}
                  <span>Clone Instance</span>
                </button>
              </form>
            </div>
          )}

          {/* Export Bundle */}
          <div className="pt-6 border-t border-obsidian-800">
            <h3 className="font-mono text-base font-bold text-slate-100 mb-2 flex items-center gap-2">
              <Download className="w-4 h-4 text-cyber-cyan" />
              <span>Full Server Export</span>
            </h3>
            <p className="text-xs text-slate-400 font-mono mb-4">
              Download a complete .zip archive containing all worlds, packs, and configuration files for migration or safe-keeping.
            </p>

            <a
              href={api.getExportUrl(server.id)}
              download
              className="inline-flex items-center space-x-2 px-4 py-2.5 rounded-lg bg-obsidian-800 hover:bg-obsidian-750 border border-obsidian-700 text-slate-200 hover:text-emerald-400 font-mono text-xs font-bold transition-colors"
            >
              <Download className="w-4 h-4" />
              <span>Download .zip Bundle</span>
            </a>
          </div>

          {/* Sync & Copy Configs to Another Server */}
          {user.role === 'admin' && (
            <div className="pt-6 border-t border-obsidian-800">
              <h3 className="font-mono text-base font-bold text-slate-100 mb-2 flex items-center gap-2">
                <Share2 className="w-4 h-4 text-emerald-400" />
                <span>Sync & Copy Configs to Another Server</span>
              </h3>
              <p className="text-xs text-slate-400 font-mono mb-4">
                Selectively copy allowlists (<code className="text-emerald-400">allowlist.json</code>), operator permissions (<code className="text-amber-400">permissions.json</code>), or server properties to existing servers. Target network ports and world identities are strictly preserved.
              </p>

              {copyStatusMsg && (
                <div
                  className={`p-3 rounded-lg font-mono text-xs mb-4 border ${
                    copyStatusMsg.isError
                      ? 'bg-rose-500/10 border-rose-500/30 text-rose-300'
                      : 'bg-emerald-500/10 border-emerald-500/30 text-emerald-300'
                  }`}
                >
                  {copyStatusMsg.text}
                </div>
              )}

              <form onSubmit={handleCopyConfigs} className="space-y-4 font-mono text-xs">
                {/* Target Servers Selector */}
                <div>
                  <label className="block text-slate-400 mb-1.5 font-bold">Select Target Server(s):</label>
                  {(allServers || []).filter((s) => s && s.id !== server?.id).length === 0 ? (
                    <p className="text-slate-600 italic bg-obsidian-950 p-3 rounded-lg border border-obsidian-800">
                      No other servers found. Create or clone another server first to sync configs between them.
                    </p>
                  ) : (
                    <div className="space-y-1.5 max-h-48 overflow-y-auto p-2 bg-obsidian-950 rounded-lg border border-obsidian-800">
                      {(allServers || [])
                        .filter((s) => s && s.id !== server?.id)
                        .map((s) => {
                          const isSelected = (selectedTargetIds || []).includes(s.id);
                          return (
                            <label
                              key={s.id}
                              className={`flex items-center justify-between p-2 rounded cursor-pointer transition-colors ${
                                isSelected ? 'bg-emerald-950/40 border border-emerald-500/40' : 'hover:bg-obsidian-900 border border-transparent'
                              }`}
                            >
                              <div className="flex items-center space-x-2.5">
                                <input
                                  type="checkbox"
                                  checked={isSelected}
                                  onChange={(e) => {
                                    if (e.target.checked) {
                                      setSelectedTargetIds([...(selectedTargetIds || []), s.id]);
                                    } else {
                                      setSelectedTargetIds((selectedTargetIds || []).filter((tid) => tid !== s.id));
                                    }
                                  }}
                                  className="rounded bg-obsidian-900 border-obsidian-700 text-emerald-500 focus:ring-emerald-500"
                                />
                                <div>
                                  <span className="text-slate-200 font-bold">{s.name}</span>
                                  <span className="text-slate-500 text-[10px] ml-2">({s.id} • UDP :{s.port})</span>
                                </div>
                              </div>
                              <span
                                className={`text-[10px] px-1.5 py-0.5 rounded font-bold uppercase ${
                                  s.status === 'running'
                                    ? 'bg-emerald-500/10 text-emerald-400'
                                    : 'bg-slate-500/10 text-slate-400'
                                }`}
                              >
                                {s.status}
                              </span>
                            </label>
                          );
                        })}
                    </div>
                  )}
                </div>

                {/* Items to copy */}
                <div>
                  <label className="block text-slate-400 mb-1.5 font-bold">Configurations to Copy:</label>
                  <div className="grid grid-cols-1 sm:grid-cols-3 gap-2 bg-obsidian-950 p-3 rounded-lg border border-obsidian-800">
                    <label className="flex items-center space-x-2 cursor-pointer">
                      <input
                        type="checkbox"
                        checked={copyAllowlist}
                        onChange={(e) => setCopyAllowlist(e.target.checked)}
                        className="rounded bg-obsidian-900 border-obsidian-700 text-emerald-500 focus:ring-emerald-500"
                      />
                      <span className="text-slate-200 text-xs">Allowlist</span>
                    </label>

                    <label className="flex items-center space-x-2 cursor-pointer">
                      <input
                        type="checkbox"
                        checked={copyPermissions}
                        onChange={(e) => setCopyPermissions(e.target.checked)}
                        className="rounded bg-obsidian-900 border-obsidian-700 text-emerald-500 focus:ring-emerald-500"
                      />
                      <span className="text-slate-200 text-xs">Ops & Permissions</span>
                    </label>

                    <label className="flex items-center space-x-2 cursor-pointer">
                      <input
                        type="checkbox"
                        checked={copyProperties}
                        onChange={(e) => setCopyProperties(e.target.checked)}
                        className="rounded bg-obsidian-900 border-obsidian-700 text-emerald-500 focus:ring-emerald-500"
                      />
                      <span className="text-slate-200 text-xs">server.properties</span>
                    </label>
                  </div>
                </div>

                {/* Mode */}
                <div>
                  <label className="block text-slate-400 mb-1.5 font-bold">Sync Mode:</label>
                  <div className="flex flex-col sm:flex-row gap-3 bg-obsidian-950 p-3 rounded-lg border border-obsidian-800">
                    <label className="flex items-center space-x-2 cursor-pointer">
                      <input
                        type="radio"
                        name="copyMode"
                        value="merge"
                        checked={copyMode === 'merge'}
                        onChange={() => setCopyMode('merge')}
                        className="text-emerald-500 bg-obsidian-900 border-obsidian-700 focus:ring-emerald-500"
                      />
                      <div>
                        <span className="text-slate-200 font-bold block">Merge</span>
                        <span className="text-[10px] text-slate-400">Preserve existing entries on target and append new</span>
                      </div>
                    </label>

                    <label className="flex items-center space-x-2 cursor-pointer">
                      <input
                        type="radio"
                        name="copyMode"
                        value="replace"
                        checked={copyMode === 'replace'}
                        onChange={() => setCopyMode('replace')}
                        className="text-emerald-500 bg-obsidian-900 border-obsidian-700 focus:ring-emerald-500"
                      />
                      <div>
                        <span className="text-slate-200 font-bold block">Replace / Overwrite</span>
                        <span className="text-[10px] text-slate-400">Overwrite target files completely</span>
                      </div>
                    </label>
                  </div>
                </div>

                <button
                  type="submit"
                  disabled={copying || selectedTargetIds.length === 0}
                  className="px-4 py-2.5 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-slate-950 font-bold font-mono text-xs flex items-center space-x-2 disabled:opacity-40 disabled:cursor-not-allowed"
                >
                  {copying ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <Share2 className="w-3.5 h-3.5" />}
                  <span>Sync Configurations</span>
                </button>
              </form>
            </div>
          )}

          {/* DANGER ZONE - DELETE SERVER */}
          {user.role === 'admin' && (
            <div className="pt-6 border-t border-rose-900/40">
              <div className="bg-rose-950/20 border border-rose-800/40 rounded-xl p-5 space-y-3">
                <h3 className="font-mono text-base font-bold text-rose-300 flex items-center gap-2">
                  <AlertTriangle className="w-4 h-4 text-rose-400" />
                  <span>Danger Zone: Delete Server Instance</span>
                </h3>
                <p className="text-xs text-slate-400 font-mono">
                  Irrevocably deletes this server container, its database record, and configuration. Be sure to export or backup worlds beforehand.
                </p>
                <button
                  type="button"
                  onClick={() => {
                    setDeleteConfirmText('');
                    setShowDeleteServerModal(true);
                  }}
                  className="px-4 py-2 rounded-lg bg-rose-600/20 hover:bg-rose-600 text-rose-300 hover:text-slate-950 font-bold font-mono text-xs border border-rose-500/40 flex items-center space-x-1.5 transition-colors"
                >
                  <Trash2 className="w-3.5 h-3.5" />
                  <span>Delete Server Instance</span>
                </button>
              </div>
            </div>
          )}

          {/* DELETE SERVER CONFIRMATION MODAL */}
          {showDeleteServerModal && (
            <div className="fixed inset-0 bg-black/80 backdrop-blur-sm z-50 flex items-center justify-center p-4">
              <div className="bg-obsidian-900 border border-rose-700/80 rounded-xl max-w-md w-full p-6 shadow-2xl">
                <h3 className="text-base font-mono font-bold text-rose-400 mb-2 flex items-center gap-2">
                  <AlertTriangle className="w-5 h-5" />
                  <span>Confirm Server Deletion</span>
                </h3>
                <p className="text-xs text-slate-300 font-mono mb-4">
                  This action is permanent and cannot be undone. To confirm deletion of <strong className="text-white font-bold">{server.name}</strong>, type its ID (<code className="text-rose-400 font-bold">{server.id}</code>) below:
                </p>

                <input
                  type="text"
                  value={deleteConfirmText}
                  onChange={(e) => setDeleteConfirmText(e.target.value)}
                  placeholder={`Type "${server.id}" to confirm`}
                  className="w-full px-3 py-2 bg-obsidian-950 border border-rose-800 rounded-lg text-rose-200 font-mono text-xs focus:outline-none focus:border-rose-500 mb-4"
                />

                <div className="flex justify-end space-x-3">
                  <button
                    type="button"
                    onClick={() => setShowDeleteServerModal(false)}
                    className="px-4 py-2 rounded-lg bg-obsidian-800 text-slate-300 hover:bg-obsidian-700 font-mono text-xs"
                  >
                    Cancel
                  </button>
                  <button
                    type="button"
                    disabled={deleteConfirmText.trim() !== server.id || serverDeleting}
                    onClick={handleDeleteServerInstance}
                    className="px-4 py-2 rounded-lg bg-rose-600 hover:bg-rose-500 text-white font-bold font-mono text-xs flex items-center space-x-1.5 disabled:opacity-40 disabled:cursor-not-allowed"
                  >
                    {serverDeleting ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <Trash2 className="w-3.5 h-3.5" />}
                    <span>Delete Permanently</span>
                  </button>
                </div>
              </div>
            </div>
          )}
        </div>
      )}

      {/* PLAYER MODERATION & MANAGEMENT MODAL */}
      {selectedPlayer && (
        <div className="fixed inset-0 bg-black/80 backdrop-blur-sm z-50 flex items-center justify-center p-4">
          <div className="bg-obsidian-900 border border-obsidian-700 rounded-2xl max-w-lg w-full p-6 shadow-2xl relative space-y-5">
            {/* Header */}
            <div className="flex items-center justify-between border-b border-obsidian-800 pb-3">
              <div className="flex items-center space-x-2.5">
                <div className="p-2 rounded-lg bg-cyan-950/60 border border-cyan-500/40 text-cyan-400">
                  <UserCog className="w-5 h-5" />
                </div>
                <div>
                  <h3 className="font-mono text-base font-bold text-slate-100">Player Moderation</h3>
                  <p className="text-xs text-slate-400 font-mono">Global player promotion & ban enforcement</p>
                </div>
              </div>
              <button
                type="button"
                onClick={() => setSelectedPlayer(null)}
                className="p-1.5 rounded-lg text-slate-400 hover:text-slate-100 hover:bg-obsidian-800 transition-colors"
              >
                <X className="w-5 h-5" />
              </button>
            </div>

            {/* Target Player Card */}
            <div className="bg-obsidian-950 border border-obsidian-800 rounded-xl p-4 flex flex-col sm:flex-row sm:items-center justify-between gap-3">
              <div>
                <div className="flex items-center space-x-2">
                  <span className="font-mono font-bold text-sm text-slate-100">{selectedPlayer.gamertag}</span>
                  {selectedPlayer.is_op ? (
                    <span className="px-1.5 py-0.5 rounded text-[10px] font-mono font-semibold bg-emerald-950/80 text-emerald-400 border border-emerald-600/40">
                      OPERATOR
                    </span>
                  ) : (
                    <span className="px-1.5 py-0.5 rounded text-[10px] font-mono text-slate-400 bg-obsidian-800 border border-obsidian-700">
                      MEMBER
                    </span>
                  )}
                </div>
                <div className="text-[11px] font-mono text-slate-500 mt-0.5">
                  XUID: {selectedPlayer.xuid || 'N/A'}
                </div>
              </div>
              {selectedPlayer.ip_address && (
                <div className="text-right">
                  <span className="text-[10px] font-mono text-slate-500 block uppercase">Port Gate Lease IP</span>
                  <span className="font-mono text-xs text-cyan-400 bg-cyan-950/40 px-2 py-0.5 rounded border border-cyan-800/40">
                    {selectedPlayer.ip_address}
                  </span>
                </div>
              )}
            </div>

            {/* SECTION 1: GLOBAL PLAYERS */}
            <div className="bg-obsidian-850/60 border border-obsidian-800 rounded-xl p-4 space-y-3">
              <div className="flex items-center space-x-2 text-xs font-mono font-semibold text-slate-200">
                <Globe className="w-4 h-4 text-emerald-400" />
                <span>Global Player Roster</span>
              </div>
              <p className="text-[11px] text-slate-400 font-mono">
                Adding to Global Players syncs this player's allowlist entry and permission across all existing and newly provisioned server instances.
              </p>

              {globalPlayers.some((g) => g.name.toLowerCase() === selectedPlayer.gamertag.toLowerCase()) ? (
                <div className="flex items-center justify-between p-2.5 rounded-lg bg-emerald-950/20 border border-emerald-800/40 text-emerald-300 text-xs font-mono">
                  <div className="flex items-center space-x-2">
                    <Check className="w-4 h-4 text-emerald-400" />
                    <span>Already on Global Player Roster</span>
                  </div>
                  <span className="text-[11px] text-emerald-400 font-bold uppercase">
                    {globalPlayers.find((g) => g.name.toLowerCase() === selectedPlayer.gamertag.toLowerCase())?.permission || 'MEMBER'}
                  </span>
                </div>
              ) : (
                <div className="flex items-center space-x-3 pt-1">
                  <div className="flex items-center space-x-2 flex-1">
                    <label className="text-xs font-mono text-slate-400">Role:</label>
                    <select
                      value={playerModalRole}
                      onChange={(e) => setPlayerModalRole(e.target.value as 'member' | 'operator')}
                      className="px-2.5 py-1.5 bg-obsidian-950 border border-obsidian-700 rounded-lg text-slate-200 font-mono text-xs focus:border-emerald-500"
                    >
                      <option value="member">Member</option>
                      <option value="operator">Operator (OP)</option>
                    </select>
                  </div>
                  <button
                    type="button"
                    disabled={playerGlobalLoading}
                    onClick={handleAddModalPlayerToGlobal}
                    className="px-3.5 py-1.5 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-slate-950 font-bold font-mono text-xs flex items-center space-x-1.5 transition-colors disabled:opacity-40"
                  >
                    {playerGlobalLoading ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <UserPlus className="w-3.5 h-3.5" />}
                    <span>Add to Global</span>
                  </button>
                </div>
              )}
            </div>

            {/* SECTION 2: BAN ENFORCEMENT */}
            <div className="bg-rose-950/20 border border-rose-900/40 rounded-xl p-4 space-y-3">
              <div className="flex items-center space-x-2 text-xs font-mono font-bold text-rose-300">
                <Ban className="w-4 h-4 text-rose-400" />
                <span>Ban Enforcement</span>
              </div>
              <p className="text-[11px] text-slate-400 font-mono">
                Banning immediately disconnects the player, strips them from the allowlist, and automatically ejects them whenever they attempt to reconnect.
              </p>

              {/* Scope Selection */}
              <div>
                <label className="block text-[11px] font-mono text-slate-400 mb-1.5">Ban Scope</label>
                <div className="grid grid-cols-2 gap-2">
                  <button
                    type="button"
                    onClick={() => setPlayerBanScope('instance')}
                    className={`px-3 py-2 rounded-lg border text-left transition-colors font-mono text-xs ${
                      playerBanScope === 'instance'
                        ? 'bg-rose-900/40 border-rose-500 text-rose-200 font-semibold'
                        : 'bg-obsidian-950 border-obsidian-800 text-slate-400 hover:border-obsidian-700'
                    }`}
                  >
                    <div className="font-bold">Current Instance</div>
                    <div className="text-[10px] text-slate-400 mt-0.5">Only this server</div>
                  </button>
                  <button
                    type="button"
                    onClick={() => setPlayerBanScope('global')}
                    className={`px-3 py-2 rounded-lg border text-left transition-colors font-mono text-xs ${
                      playerBanScope === 'global'
                        ? 'bg-rose-900/40 border-rose-500 text-rose-200 font-semibold'
                        : 'bg-obsidian-950 border-obsidian-800 text-slate-400 hover:border-obsidian-700'
                    }`}
                  >
                    <div className="font-bold text-rose-300">Global Ban</div>
                    <div className="text-[10px] text-slate-400 mt-0.5">All servers & global roster</div>
                  </button>
                </div>
              </div>

              {/* Ban Reason */}
              <div>
                <label className="block text-[11px] font-mono text-slate-400 mb-1">Reason for Ban</label>
                <input
                  type="text"
                  value={playerBanReason}
                  onChange={(e) => setPlayerBanReason(e.target.value)}
                  placeholder="e.g. Griefing, unauthorized modifications"
                  className="w-full px-3 py-1.5 bg-obsidian-950 border border-obsidian-700 rounded-lg text-slate-100 font-mono text-xs focus:border-rose-500"
                />
              </div>

              {/* Firewall IP Ban Checkbox */}
              {selectedPlayer.ip_address && (
                <label className="flex items-center space-x-2.5 pt-1 cursor-pointer select-none">
                  <input
                    type="checkbox"
                    checked={playerBanIP}
                    onChange={(e) => setPlayerBanIP(e.target.checked)}
                    className="rounded bg-obsidian-950 border-obsidian-700 text-rose-600 focus:ring-rose-500 h-4 w-4"
                  />
                  <div className="text-[11px] font-mono text-slate-300">
                    <span>Block IP Address in Port Gate: </span>
                    <code className="text-rose-400 font-bold">{selectedPlayer.ip_address}</code>
                  </div>
                </label>
              )}

              {/* Ban Button */}
              <div className="pt-2 flex justify-end">
                <button
                  type="button"
                  disabled={playerBanLoading}
                  onClick={handleBanModalPlayer}
                  className="px-4 py-2 rounded-lg bg-rose-600 hover:bg-rose-500 text-white font-bold font-mono text-xs flex items-center space-x-2 shadow-lg shadow-rose-950/50 transition-colors disabled:opacity-40"
                >
                  {playerBanLoading ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <Ban className="w-3.5 h-3.5" />}
                  <span>{playerBanScope === 'global' ? 'Execute Global Ban' : 'Ban From Instance'}</span>
                </button>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};
