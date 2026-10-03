import React, { useState, useEffect } from 'react';
import { api } from '../api/client';
import { Preset, SeedPreset, UserPlanStatus, User } from '../types';
import { PopularSeedPicker } from './PopularSeedPicker';
import {
  Plus,
  Zap,
  Wand2,
  X,
  Server as ServerIcon,
  Shield,
  Cpu,
  Gamepad2,
  Compass,
  ArrowRight,
  ArrowLeft,
  CheckCircle2,
  Sparkles,
  Loader2,
  AlertTriangle,
  Network,
} from 'lucide-react';

interface DeployModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSuccess: () => void;
  userPlan?: UserPlanStatus | null;
  user?: User | null;
}

type DeployMethod = 'quick' | 'wizard';

const parseMemoryMb = (mem: string | undefined): number => {
  if (!mem) return 8192;
  const upper = mem.toUpperCase().trim();
  if (upper.endsWith('G')) {
    return (parseFloat(upper) || 2) * 1024;
  }
  if (upper.endsWith('M') || upper.endsWith('MB')) {
    return parseFloat(upper) || 2048;
  }
  return 8192;
};

export const DeployModal: React.FC<DeployModalProps> = ({ isOpen, onClose, onSuccess, userPlan, user }) => {
  const [method, setMethod] = useState<DeployMethod>(() => {
    return (localStorage.getItem('bsm_deploy_method') as DeployMethod) || 'wizard';
  });

  const isAdmin = user?.role === 'admin';
  const isQuotaFull = Boolean(userPlan && userPlan.usage.servers_count >= userPlan.usage.servers_max);
  const maxRamMb = userPlan?.plan?.max_memory ? parseMemoryMb(userPlan.plan.max_memory) : 8192;
  const maxCpu = userPlan?.plan?.max_cpu || 8.0;
  const allowCustomPort = userPlan ? userPlan.plan.allow_custom_port : true;
  const allowCustomSeed = userPlan ? userPlan.plan.allow_custom_seed : true;
  const allowPortGate = userPlan ? userPlan.plan.allow_port_gate_keys : true;

  const ramOptions = [
    { value: '1G', label: '1 GB', fullLabel: '1 GB (Lightweight / 2-3 players)', mb: 1024 },
    { value: '2G', label: '2 GB', fullLabel: '2 GB (Recommended standard)', mb: 2048 },
    { value: '4G', label: '4 GB', fullLabel: '4 GB (Medium worlds & Addons)', mb: 4096 },
    { value: '8G', label: '8 GB', fullLabel: '8 GB (Heavy loads / Mega realm)', mb: 8192 },
  ].filter((o) => o.mb <= maxRamMb || o.mb === 1024);

  const cpuOptions = [
    { value: 1.0, label: '1.0 Core' },
    { value: 2.0, label: '2.0 Cores (Recommended)' },
    { value: 4.0, label: '4.0 Cores' },
    { value: 8.0, label: '8.0 Cores' },
  ].filter((o) => o.value <= maxCpu || o.value === 1.0);

  // Wizard current step (1 to 5)
  const [step, setStep] = useState(1);

  // Form State
  const [serverName, setServerName] = useState('');
  const [serverId, setServerId] = useState('');
  const [port, setPort] = useState(19132);
  const [mode, setMode] = useState('survival');
  const [difficulty, setDifficulty] = useState('normal');
  const [memLimit, setMemLimit] = useState('2G');
  const [cpuLimit, setCpuLimit] = useState<number>(2.0);
  const [autostartOnBoot, setAutostartOnBoot] = useState(true);
  const [portGate, setPortGate] = useState(false);
  const [portGateMode, setPortGateMode] = useState<'gamertag' | 'passphrase' | 'combined'>('passphrase');
  const [seed, setSeed] = useState('');
  const [selectedSeedPreset, setSelectedSeedPreset] = useState<SeedPreset | null>(null);
  const [gameServerAddress, setGameServerAddress] = useState('');
  const [networkMode, setNetworkMode] = useState<string>('host');
  const [availableNetworks, setAvailableNetworks] = useState<string[]>(['bridge', 'host']);

  // Auxiliary state
  const [presets, setPresets] = useState<Preset[]>([]);
  const [selectedPresetId, setSelectedPresetId] = useState<string>('survival');
  const [showSeedPickerDrawer, setShowSeedPickerDrawer] = useState(false);
  const [seedTypeSelection, setSeedTypeSelection] = useState<'popular' | 'custom' | 'random'>('popular');
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Load presets & port suggestion on open and apply plan restrictions
  useEffect(() => {
    if (!isOpen) return;

    api.suggestPorts()
      .then((res) => {
        if (res && res.port) setPort(res.port);
      })
      .catch(() => {});

    api.listPresets()
      .then((data) => {
        if (data && data.length > 0) setPresets(data);
      })
      .catch(() => {});

    if (isAdmin) {
      api.listNetworks()
        .then((res) => {
          if (res && Array.isArray(res.networks) && res.networks.length > 0) {
            setAvailableNetworks(res.networks);
            if (res.default) setNetworkMode(res.default);
          }
        })
        .catch(() => {});
    }

    if (userPlan) {
      if (!ramOptions.some((r) => r.value === memLimit)) {
        setMemLimit(ramOptions[ramOptions.length - 1].value);
      }
      if (cpuLimit > maxCpu) {
        setCpuLimit(cpuOptions[cpuOptions.length - 1].value);
      }
      if (!allowPortGate) {
        setPortGate(false);
      }
      if (!allowCustomSeed) {
        setSeed('');
        setSelectedSeedPreset(null);
      }
    }
  }, [isOpen, userPlan]);

  const handleMethodChange = (newMethod: DeployMethod) => {
    setMethod(newMethod);
    localStorage.setItem('bsm_deploy_method', newMethod);
  };

  const handleApplyPreset = (p: Preset) => {
    setSelectedPresetId(p.id);
    if (p.mode) setMode(p.mode);
    if (p.difficulty) setDifficulty(p.difficulty);
    if (!serverName) {
      setServerName(p.name);
      setServerId(p.id + '-' + Math.floor(100 + Math.random() * 900));
    }
  };

  const handleSelectSeed = (newSeed: string, presetObj?: SeedPreset) => {
    setSeed(newSeed);
    setSelectedSeedPreset(presetObj || null);
    if (newSeed) {
      setSeedTypeSelection('popular');
    } else {
      setSeedTypeSelection('random');
    }
  };

  const handleSubmit = async (e?: React.FormEvent) => {
    if (e) e.preventDefault();
    setError(null);

    const trimmedId = serverId.trim().toLowerCase().replace(/[^a-z0-9-]/g, '-');
    const trimmedName = serverName.trim();

    if (!trimmedId || !trimmedName) {
      setError('Server ID and Server Name are required.');
      return;
    }

    setCreating(true);
    try {
      await api.createServer({
        id: trimmedId,
        name: trimmedName,
        seed: seed.trim() || undefined,
        game_server_address: gameServerAddress.trim() || undefined,
        port: Number(port),
        portv6: Number(port) + 1,
        mode,
        difficulty,
        memory_limit: memLimit,
        cpu_limit: Number(cpuLimit),
        autostart_on_boot: autostartOnBoot,
        port_gate_enabled: portGate,
        port_gate_mode: portGateMode,
        port_gate_timeout: 7200,
        network_mode: isAdmin ? networkMode : 'host',
      });

      onSuccess();
      onClose();
    } catch (err: any) {
      setError(err.message || 'Failed to deploy instance');
    } finally {
      setCreating(false);
    }
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 bg-black/80 backdrop-blur-sm flex items-center justify-center p-3 sm:p-4 overflow-y-auto">
      <div className="bg-obsidian-900 border border-obsidian-700/80 rounded-2xl max-w-3xl w-full shadow-2xl overflow-hidden flex flex-col my-auto max-h-[92vh]">
        {/* Modal Top Bar */}
        <div className="p-4 sm:p-5 border-b border-obsidian-800 bg-obsidian-950/70 flex flex-col sm:flex-row sm:items-center justify-between gap-3">
          <div className="flex items-center space-x-3">
            <div className="w-9 h-9 rounded-xl bg-emerald-950/80 border border-emerald-500/40 flex items-center justify-center text-emerald-400">
              <ServerIcon className="w-5 h-5" />
            </div>
            <div>
              <h3 className="text-base font-bold font-mono text-slate-100 flex items-center space-x-2">
                <span>DEPLOY BEDROCK INSTANCE</span>
              </h3>
              <p className="text-xs text-slate-400">
                {method === 'wizard'
                  ? `Step-by-step guided configuration (Step ${step} of 5)`
                  : 'Fast direct single-page instance provisioning'}
              </p>
            </div>
          </div>

          <div className="flex items-center space-x-2">
            {/* Deploy Method Switcher */}
            <div className="p-1 rounded-xl bg-obsidian-900 border border-obsidian-800 flex items-center space-x-1">
              <button
                type="button"
                onClick={() => handleMethodChange('quick')}
                className={`px-3 py-1.5 rounded-lg font-mono text-xs font-semibold flex items-center space-x-1.5 transition-all ${
                  method === 'quick'
                    ? 'bg-emerald-600 text-slate-950 shadow-sm'
                    : 'text-slate-400 hover:text-slate-200'
                }`}
              >
                <Zap className="w-3.5 h-3.5" />
                <span>Quick Deploy</span>
              </button>

              <button
                type="button"
                onClick={() => handleMethodChange('wizard')}
                className={`px-3 py-1.5 rounded-lg font-mono text-xs font-semibold flex items-center space-x-1.5 transition-all ${
                  method === 'wizard'
                    ? 'bg-emerald-600 text-slate-950 shadow-sm'
                    : 'text-slate-400 hover:text-slate-200'
                }`}
              >
                <Wand2 className="w-3.5 h-3.5" />
                <span>Guided Wizard</span>
              </button>
            </div>

            <button
              onClick={onClose}
              className="p-1.5 rounded-lg hover:bg-obsidian-800 text-slate-400 hover:text-slate-200 transition-colors"
            >
              <X className="w-5 h-5" />
            </button>
          </div>
        </div>

        {error && (
          <div className="px-5 py-3 bg-rose-950/40 border-b border-rose-500/30 text-rose-300 text-xs font-mono flex items-center justify-between">
            <span>{error}</span>
            <button onClick={() => setError(null)} className="text-rose-400 hover:text-rose-200">
              <X className="w-4 h-4" />
            </button>
          </div>
        )}

        {isQuotaFull && userPlan && (
          <div className="px-5 py-3 bg-amber-950/40 border-b border-amber-500/30 text-amber-300 text-xs font-mono flex items-center space-x-2.5">
            <AlertTriangle className="w-4 h-4 text-amber-400 shrink-0" />
            <span>
              <strong>Deployment Quota Reached:</strong> Your current plan (<strong>{userPlan.plan.name}</strong>) allows up to {userPlan.usage.servers_max} servers ({userPlan.usage.servers_count} deployed). Delete an existing server to deploy a new one.
            </span>
          </div>
        )}

        {/* Modal Body */}
        <div className="p-4 sm:p-6 overflow-y-auto flex-1">
          {/* ================= METHOD 1: GUIDED WIZARD ================= */}
          {method === 'wizard' && (
            <div className="space-y-6">
              {/* Stepper Progress Bar */}
              <div className="grid grid-cols-5 gap-2 pb-2">
                {[
                  { num: 1, title: 'Basics & Style' },
                  { num: 2, title: 'World & Seeds' },
                  { num: 3, title: 'Resources' },
                  { num: 4, title: 'Port Gate' },
                  { num: 5, title: 'Review & Launch' },
                ].map((s) => (
                  <button
                    key={s.num}
                    type="button"
                    onClick={() => {
                      if (s.num < step || (serverName && serverId)) setStep(s.num);
                    }}
                    className={`text-left p-2 rounded-lg border transition-all ${
                      step === s.num
                        ? 'bg-emerald-950/40 border-emerald-500/50 text-emerald-400'
                        : step > s.num
                        ? 'bg-obsidian-950/80 border-obsidian-800 text-slate-300'
                        : 'bg-obsidian-950/30 border-obsidian-800/40 text-slate-600'
                    }`}
                  >
                    <div className="flex items-center space-x-1.5">
                      <span
                        className={`w-4 h-4 rounded-full text-[10px] font-mono flex items-center justify-center font-bold ${
                          step === s.num
                            ? 'bg-emerald-500 text-slate-950'
                            : step > s.num
                            ? 'bg-emerald-900/60 text-emerald-400'
                            : 'bg-obsidian-800 text-slate-500'
                        }`}
                      >
                        {step > s.num ? '✓' : s.num}
                      </span>
                      <span className="text-[11px] font-mono font-medium truncate">{s.title}</span>
                    </div>
                  </button>
                ))}
              </div>

              {/* STEP 1: BASICS & STYLE */}
              {step === 1 && (
                <div className="space-y-4">
                  <div className="bg-obsidian-950/60 border border-obsidian-800 p-4 rounded-xl">
                    <h4 className="font-mono text-sm font-bold text-slate-200 mb-1 flex items-center space-x-2">
                      <Gamepad2 className="w-4 h-4 text-emerald-400" />
                      <span>Instance Identity & Gameplay Style</span>
                    </h4>
                    <p className="text-xs text-slate-400 mb-4">
                      Give your server a friendly name, container slug, and select an initial gameplay template.
                    </p>

                    <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                      <div>
                        <label className="block text-xs font-mono text-slate-300 mb-1">
                          Server Name <span className="text-emerald-400">*</span>
                        </label>
                        <input
                          type="text"
                          required
                          value={serverName}
                          onChange={(e) => {
                            setServerName(e.target.value);
                            if (!serverId || serverId === serverName.toLowerCase().replace(/[^a-z0-9]/g, '-')) {
                              setServerId(e.target.value.toLowerCase().replace(/[^a-z0-9]/g, '-'));
                            }
                          }}
                          className="w-full px-3.5 py-2.5 rounded-lg bg-obsidian-900 border border-obsidian-700 text-slate-100 font-mono text-sm focus:border-emerald-500"
                          placeholder="My Bedrock Realm"
                        />
                      </div>

                      <div>
                        <label className="block text-xs font-mono text-slate-300 mb-1">
                          Server Slug (Unique ID) <span className="text-emerald-400">*</span>
                        </label>
                        <input
                          type="text"
                          required
                          value={serverId}
                          onChange={(e) => setServerId(e.target.value.toLowerCase().replace(/[^a-z0-9-]/g, '-'))}
                          className="w-full px-3.5 py-2.5 rounded-lg bg-obsidian-900 border border-obsidian-700 text-slate-100 font-mono text-sm focus:border-emerald-500"
                          placeholder="my-bedrock-realm"
                        />
                      </div>
                    </div>
                  </div>

                  {/* Preset Templates */}
                  <div>
                    <label className="block text-xs font-mono text-slate-300 mb-2">
                      Choose Starting Preset Template:
                    </label>
                    <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                      {presets.map((p) => {
                        const isSelected = selectedPresetId === p.id;
                        return (
                          <div
                            key={p.id}
                            onClick={() => handleApplyPreset(p)}
                            className={`p-3.5 rounded-xl border text-left cursor-pointer transition-all ${
                              isSelected
                                ? 'bg-emerald-950/40 border-emerald-500 shadow-sm ring-1 ring-emerald-500/40'
                                : 'bg-obsidian-950/60 border-obsidian-800 hover:border-obsidian-700'
                            }`}
                          >
                            <div className="flex items-center justify-between mb-1">
                              <span className="font-mono text-xs font-bold text-slate-200">
                                {p.name}
                              </span>
                              {isSelected && (
                                <span className="text-[10px] font-mono text-emerald-400 font-bold">
                                  ✓ Applied
                                </span>
                              )}
                            </div>
                            <p className="text-[11px] text-slate-400 leading-relaxed mb-2">
                              {p.description}
                            </p>
                            <div className="flex items-center space-x-2 text-[10px] font-mono text-slate-500">
                              <span className="px-1.5 py-0.5 rounded bg-obsidian-900 border border-obsidian-800 capitalize">
                                {p.mode}
                              </span>
                              <span className="px-1.5 py-0.5 rounded bg-obsidian-900 border border-obsidian-800 capitalize">
                                {p.difficulty}
                              </span>
                            </div>
                          </div>
                        );
                      })}
                    </div>
                  </div>

                  {/* Manual Overrides for Mode & Difficulty */}
                  <div className="grid grid-cols-2 gap-3 pt-2">
                    <div>
                      <label className="block text-xs font-mono text-slate-300 mb-1">Game Mode</label>
                      <select
                        value={mode}
                        onChange={(e) => setMode(e.target.value)}
                        className="w-full px-3 py-2 rounded-lg bg-obsidian-950 border border-obsidian-700 text-slate-100 font-mono text-xs focus:border-emerald-500"
                      >
                        <option value="survival">Survival</option>
                        <option value="creative">Creative</option>
                        <option value="adventure">Adventure</option>
                      </select>
                    </div>
                    <div>
                      <label className="block text-xs font-mono text-slate-300 mb-1">Difficulty</label>
                      <select
                        value={difficulty}
                        onChange={(e) => setDifficulty(e.target.value)}
                        className="w-full px-3 py-2 rounded-lg bg-obsidian-950 border border-obsidian-700 text-slate-100 font-mono text-xs focus:border-emerald-500"
                      >
                        <option value="peaceful">Peaceful</option>
                        <option value="easy">Easy</option>
                        <option value="normal">Normal</option>
                        <option value="hard">Hard</option>
                      </select>
                    </div>
                  </div>
                </div>
              )}

              {/* STEP 2: WORLD & SEED SELECTION */}
              {step === 2 && (
                <div className="space-y-4">
                  {!allowCustomSeed ? (
                    <div className="p-8 bg-obsidian-950/60 border border-obsidian-800 rounded-xl text-center">
                      <Compass className="w-10 h-10 text-emerald-500/70 mx-auto mb-3" />
                      <h4 className="font-mono text-sm font-bold text-slate-200">Random World Generation Only</h4>
                      <p className="text-xs text-slate-400 max-w-md mx-auto mt-2 font-mono">
                        Your plan limits custom terrain seeds. The Bedrock engine will generate a brand new randomized world seed when the container starts.
                      </p>
                    </div>
                  ) : (
                    <>
                    <div className="bg-obsidian-950/60 border border-obsidian-800 p-4 rounded-xl">
                    <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2 mb-2">
                      <div>
                        <h4 className="font-mono text-sm font-bold text-slate-200 flex items-center space-x-2">
                          <Compass className="w-4 h-4 text-emerald-400" />
                          <span>World Generation & Seed Selection</span>
                        </h4>
                        <p className="text-xs text-slate-400">
                          Choose how your Minecraft world terrain generates: pick from curated popular seeds or use random generation.
                        </p>
                      </div>

                      {/* Seed Selector Tabs */}
                      <div className="p-1 rounded-lg bg-obsidian-900 border border-obsidian-800 flex items-center space-x-1 self-start sm:self-auto">
                        <button
                          type="button"
                          onClick={() => setSeedTypeSelection('popular')}
                          className={`px-2.5 py-1 rounded text-xs font-mono transition-colors ${
                            seedTypeSelection === 'popular'
                              ? 'bg-emerald-500/20 text-emerald-300 font-bold border border-emerald-500/40'
                              : 'text-slate-400 hover:text-slate-200'
                          }`}
                        >
                          Popular Seeds
                        </button>
                        <button
                          type="button"
                          onClick={() => {
                            setSeedTypeSelection('custom');
                            setSelectedSeedPreset(null);
                          }}
                          className={`px-2.5 py-1 rounded text-xs font-mono transition-colors ${
                            seedTypeSelection === 'custom'
                              ? 'bg-emerald-500/20 text-emerald-300 font-bold border border-emerald-500/40'
                              : 'text-slate-400 hover:text-slate-200'
                          }`}
                        >
                          Custom Seed
                        </button>
                        <button
                          type="button"
                          onClick={() => {
                            setSeedTypeSelection('random');
                            setSeed('');
                            setSelectedSeedPreset(null);
                          }}
                          className={`px-2.5 py-1 rounded text-xs font-mono transition-colors ${
                            seedTypeSelection === 'random'
                              ? 'bg-emerald-500/20 text-emerald-300 font-bold border border-emerald-500/40'
                              : 'text-slate-400 hover:text-slate-200'
                          }`}
                        >
                          Random (Blank)
                        </button>
                      </div>
                    </div>

                    {/* Active Selected Seed Summary Banner */}
                    <div className="mt-3 p-3 rounded-xl bg-obsidian-900 border border-obsidian-800 flex items-center justify-between">
                      <div className="flex items-center space-x-2.5">
                        <Sparkles className="w-4 h-4 text-emerald-400 flex-shrink-0" />
                        <div>
                          <div className="text-xs font-mono font-bold text-slate-200 flex items-center space-x-2">
                            <span>Selected Seed:</span>
                            {seed ? (
                              <code className="text-emerald-400 font-bold px-1.5 py-0.5 rounded bg-obsidian-950 border border-obsidian-800">
                                {seed}
                              </code>
                            ) : (
                              <span className="text-amber-400 font-normal">Pure Random Generation (Seed is blank)</span>
                            )}
                          </div>
                          {selectedSeedPreset && (
                            <p className="text-[11px] text-slate-400 mt-0.5">
                              {selectedSeedPreset.name} • {selectedSeedPreset.category}
                            </p>
                          )}
                        </div>
                      </div>

                      {seed && (
                        <button
                          type="button"
                          onClick={() => handleSelectSeed('')}
                          className="text-xs font-mono text-slate-500 hover:text-rose-400 transition-colors"
                        >
                          Clear
                        </button>
                      )}
                    </div>
                  </div>

                  {/* Seed Tab Content */}
                  {seedTypeSelection === 'popular' && (
                    <div className="border border-obsidian-800 rounded-xl bg-obsidian-950/40 p-3">
                      <PopularSeedPicker
                        selectedSeed={seed}
                        onSelectSeed={handleSelectSeed}
                        inline={true}
                      />
                    </div>
                  )}

                  {seedTypeSelection === 'custom' && (
                    <div className="p-4 bg-obsidian-950/60 border border-obsidian-800 rounded-xl space-y-3">
                      <div>
                        <label className="block text-xs font-mono text-slate-300 mb-1">
                          Custom Minecraft World Seed
                        </label>
                        <input
                          type="text"
                          value={seed}
                          onChange={(e) => {
                            setSeed(e.target.value);
                            setSelectedSeedPreset(null);
                          }}
                          placeholder="e.g. -8219986470354173872 or 123456789"
                          className="w-full px-3.5 py-2.5 rounded-lg bg-obsidian-900 border border-obsidian-700 text-slate-100 font-mono text-sm focus:border-emerald-500"
                        />
                        <p className="text-[11px] text-slate-400 mt-1.5 font-mono">
                          Supports numeric 64-bit integer seeds or alphanumeric strings.
                        </p>
                      </div>
                    </div>
                  )}

                  {seedTypeSelection === 'random' && (
                    <div className="p-6 text-center bg-obsidian-950/60 border border-obsidian-800 rounded-xl space-y-2">
                      <Sparkles className="w-8 h-8 text-amber-400 mx-auto" />
                      <h4 className="font-mono text-sm font-bold text-slate-200">
                        Minecraft Default Random Generation
                      </h4>
                      <p className="text-xs text-slate-400 max-w-md mx-auto">
                        Leaving the seed blank allows the Bedrock Dedicated Server engine to generate a brand new pseudo-random seed on world launch.
                      </p>
                    </div>
                  )}
                    </>
                  )}
                </div>
              )}

              {/* STEP 3: RESOURCES & PERFORMANCE */}
              {step === 3 && (
                <div className="space-y-4">
                  <div className="bg-obsidian-950/60 border border-obsidian-800 p-4 rounded-xl">
                    <h4 className="font-mono text-sm font-bold text-slate-200 mb-1 flex items-center space-x-2">
                      <Cpu className="w-4 h-4 text-emerald-400" />
                      <span>Host Port Allocation & Container Resource Limits</span>
                    </h4>
                    <p className="text-xs text-slate-400 mb-4">
                      Isolated resource governance ensures fair CPU and RAM distribution without impacting other instances.
                    </p>

                    <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                      <div>
                        <label className="block text-xs font-mono text-slate-300 mb-1 flex items-center justify-between">
                          <span>UDP Port (IPv4)</span>
                          <span className="text-[10px] text-emerald-400">IPv6 Port: {Number(port) + 1}</span>
                        </label>
                        <input
                          type="number"
                          required
                          disabled={!allowCustomPort}
                          value={port}
                          onChange={(e) => setPort(Number(e.target.value))}
                          className={`w-full px-3.5 py-2.5 rounded-lg border font-mono text-sm focus:border-emerald-500 ${
                            !allowCustomPort
                              ? 'bg-obsidian-950 border-obsidian-800 text-slate-400 cursor-not-allowed'
                              : 'bg-obsidian-900 border-obsidian-700 text-slate-100'
                          }`}
                        />
                        <p className="text-[11px] text-slate-500 mt-1 font-mono">
                          {!allowCustomPort ? 'Auto-allocated per plan policy.' : 'Auto-suggested next free port pair.'}
                        </p>
                      </div>

                      <div>
                        <label className="block text-xs font-mono text-slate-300 mb-1 flex items-center justify-between">
                          <span>RAM Memory Capping</span>
                          {userPlan && (
                            <span className="text-[10px] text-emerald-400">Plan Cap: {userPlan.plan.max_memory}</span>
                          )}
                        </label>
                        <select
                          value={memLimit}
                          onChange={(e) => setMemLimit(e.target.value)}
                          className="w-full px-3.5 py-2.5 rounded-lg bg-obsidian-900 border border-obsidian-700 text-slate-100 font-mono text-sm focus:border-emerald-500"
                        >
                          {ramOptions.map((opt) => (
                            <option key={opt.value} value={opt.value}>
                              {opt.fullLabel}
                            </option>
                          ))}
                        </select>
                      </div>

                      <div>
                        <label className="block text-xs font-mono text-slate-300 mb-1 flex items-center justify-between">
                          <span>CPU Allocation</span>
                          {userPlan && (
                            <span className="text-[10px] text-emerald-400">Plan Cap: {userPlan.plan.max_cpu} Cores</span>
                          )}
                        </label>
                        <select
                          value={cpuLimit}
                          onChange={(e) => setCpuLimit(Number(e.target.value))}
                          className="w-full px-3.5 py-2.5 rounded-lg bg-obsidian-900 border border-obsidian-700 text-slate-100 font-mono text-sm focus:border-emerald-500"
                        >
                          {cpuOptions.map((opt) => (
                            <option key={opt.value} value={opt.value}>
                              {opt.label}
                            </option>
                          ))}
                        </select>
                      </div>

                      <div className="flex items-center">
                        <label className="flex items-center space-x-2.5 text-xs font-mono text-slate-200 cursor-pointer p-3 rounded-lg bg-obsidian-900 border border-obsidian-800 w-full">
                          <input
                            type="checkbox"
                            checked={autostartOnBoot}
                            onChange={(e) => setAutostartOnBoot(e.target.checked)}
                            className="rounded bg-obsidian-950 border-obsidian-700 text-emerald-500"
                          />
                          <div>
                            <span className="font-bold">Autostart on Boot</span>
                            <p className="text-[10px] text-slate-400">Launch container when host daemon boots</p>
                          </div>
                        </label>
                      </div>

                      {isAdmin && (
                        <div className="p-3 rounded-lg bg-obsidian-900 border border-obsidian-800 space-y-1.5">
                          <label className="block text-xs font-mono text-slate-300 flex items-center justify-between">
                            <span className="flex items-center gap-1.5 font-bold">
                              <Network className="w-3.5 h-3.5 text-emerald-400" />
                              <span>Deployment Network</span>
                            </span>
                            <span className="text-[10px] text-emerald-400 font-normal">Admin Option</span>
                          </label>
                          <select
                            value={networkMode}
                            onChange={(e) => setNetworkMode(e.target.value)}
                            className="w-full px-3 py-2 rounded-lg bg-obsidian-950 border border-obsidian-700 text-slate-100 font-mono text-xs focus:border-emerald-500"
                          >
                            <option value="host">host (Default - Direct Host Network & Direct UDP Port Binding)</option>
                            <option value="bridge">bridge (Virtual Bridge & Port Mapping)</option>
                            {availableNetworks
                              .filter((n) => n !== 'bridge' && n !== 'host')
                              .map((net) => (
                                <option key={net} value={net}>
                                  {net} (Custom Docker Network)
                                </option>
                              ))}
                          </select>
                          <p className="text-[10px] text-slate-400 leading-relaxed">
                            {networkMode === 'host'
                              ? 'Host Mode: Binds UDP directly to host network interface. Eliminates bridge forwarding latency and resolves UFW forwarding blocks.'
                              : 'Bridge Mode: Container gets isolated virtual IP. Traffic is forwarded via Docker NAT proxy.'}
                          </p>
                        </div>
                      )}
                    </div>
                  </div>
                </div>
              )}

              {/* STEP 4: PORT GATE & SECURITY */}
              {step === 4 && (
                <div className="space-y-4">
                  <div className="bg-obsidian-950/60 border border-obsidian-800 p-4 rounded-xl">
                    <h4 className="font-mono text-sm font-bold text-slate-200 mb-1 flex items-center space-x-2">
                      <Shield className="w-4 h-4 text-emerald-400" />
                      <span>Dynamic Port Gate & Stealth Firewall</span>
                    </h4>
                    <p className="text-xs text-slate-400 mb-4">
                      Keep your Bedrock port completely closed to public scanners and DDoS bots. Players unlock temporary firewall leases via the web Knock Portal.
                    </p>

                    <div className="space-y-3">
                      <label className={`flex items-center space-x-3 p-3.5 rounded-xl border ${
                        !allowPortGate ? 'bg-obsidian-950 border-obsidian-800 opacity-60 cursor-not-allowed' : 'bg-obsidian-900 border-obsidian-800 cursor-pointer'
                      }`}>
                        <input
                          type="checkbox"
                          disabled={!allowPortGate}
                          checked={portGate}
                          onChange={(e) => setPortGate(e.target.checked)}
                          className="rounded bg-obsidian-950 border-obsidian-700 text-emerald-500 w-4 h-4"
                        />
                        <div>
                          <span className="font-mono text-xs font-bold text-slate-200">
                            Enable Port Gate (Zero-Trust Stealth Firewall)
                            {!allowPortGate && <span className="text-amber-400 text-[10px] ml-2 font-normal">(Unavailable on current plan)</span>}
                          </span>
                          <p className="text-[11px] text-slate-400">
                            {!allowPortGate
                              ? 'Your current plan does not include the Dynamic Port Gate stealth feature.'
                              : `UDP port ${port} is dropped by default until knocked via /knock/${serverId || 'id'}`}
                          </p>
                        </div>
                      </label>

                      {portGate && (
                        <div className="p-3.5 bg-obsidian-900/80 border border-obsidian-800 rounded-xl space-y-3">
                          <div>
                            <label className="block text-xs font-mono text-slate-300 mb-1">
                              Verification Mode
                            </label>
                            <select
                              value={portGateMode}
                              onChange={(e) => setPortGateMode(e.target.value as any)}
                              className="w-full px-3 py-2 rounded-lg bg-obsidian-950 border border-obsidian-700 text-slate-200 text-xs font-mono"
                            >
                              <option value="passphrase">Passphrase / Access Key</option>
                              <option value="gamertag">Gamertag Allowlist Check</option>
                              <option value="combined">Combined (Passphrase + Gamertag)</option>
                            </select>
                          </div>
                        </div>
                      )}

                      <div>
                        <label className="block text-xs font-mono text-slate-300 mb-1 flex items-center justify-between">
                          <span>Custom Game Server Address (Knock Host Override)</span>
                          <span className="text-[10px] text-slate-500">Optional</span>
                        </label>
                        <input
                          type="text"
                          value={gameServerAddress}
                          onChange={(e) => setGameServerAddress(e.target.value)}
                          placeholder={`Optional (defaults to ${window.location.hostname})`}
                          className="w-full px-3.5 py-2.5 rounded-lg bg-obsidian-900 border border-obsidian-700 text-slate-100 font-mono text-sm focus:border-emerald-500"
                        />
                        <p className="text-[11px] text-slate-500 mt-1 font-mono">
                          Specify a custom domain (e.g. play.example.com) displayed on the Knock Portal.
                        </p>
                      </div>
                    </div>
                  </div>
                </div>
              )}

              {/* STEP 5: REVIEW & LAUNCH */}
              {step === 5 && (
                <div className="space-y-4">
                  <div className="bg-obsidian-950/60 border border-obsidian-800 p-4 rounded-xl">
                    <h4 className="font-mono text-sm font-bold text-slate-200 mb-1 flex items-center space-x-2">
                      <CheckCircle2 className="w-4 h-4 text-emerald-400" />
                      <span>Configuration Summary & Confirmation</span>
                    </h4>
                    <p className="text-xs text-slate-400 mb-4">
                      Review all parameters below before creating your containerized Bedrock instance.
                    </p>

                    <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 text-xs font-mono">
                      <div className="p-3 rounded-lg bg-obsidian-900 border border-obsidian-800">
                        <span className="text-slate-500 block mb-0.5">Instance Name:</span>
                        <span className="text-slate-100 font-bold">{serverName || 'Untitled'}</span>
                      </div>

                      <div className="p-3 rounded-lg bg-obsidian-900 border border-obsidian-800">
                        <span className="text-slate-500 block mb-0.5">Server ID (Slug):</span>
                        <span className="text-emerald-400 font-bold">{serverId || 'none'}</span>
                      </div>

                      <div className="p-3 rounded-lg bg-obsidian-900 border border-obsidian-800">
                        <span className="text-slate-500 block mb-0.5">UDP Ports:</span>
                        <span className="text-slate-200 font-bold">IPv4: {port} • IPv6: {Number(port) + 1}</span>
                      </div>

                      <div className="p-3 rounded-lg bg-obsidian-900 border border-obsidian-800">
                        <span className="text-slate-500 block mb-0.5">Gameplay Mode & Difficulty:</span>
                        <span className="text-slate-200 capitalize font-bold">{mode} ({difficulty})</span>
                      </div>

                      <div className="p-3 rounded-lg bg-obsidian-900 border border-obsidian-800">
                        <span className="text-slate-500 block mb-0.5">Resource Allocations:</span>
                        <span className="text-slate-200 font-bold">{memLimit} RAM • {cpuLimit} CPU Cores</span>
                      </div>

                      <div className="p-3 rounded-lg bg-obsidian-900 border border-obsidian-800">
                        <span className="text-slate-500 block mb-0.5">Port Gate Security:</span>
                        <span className={`font-bold ${portGate ? 'text-emerald-400' : 'text-slate-400'}`}>
                          {portGate ? `Enabled (${portGateMode})` : 'Disabled (Public Open)'}
                        </span>
                      </div>

                      {isAdmin && (
                        <div className="col-span-full p-3 rounded-lg bg-obsidian-900 border border-obsidian-800 flex items-center justify-between">
                          <span className="text-slate-500">Deployment Network Mode:</span>
                          <span className="text-emerald-400 font-bold uppercase">{networkMode}</span>
                        </div>
                      )}

                      <div className="col-span-full p-3.5 rounded-lg bg-obsidian-900 border border-obsidian-800">
                        <span className="text-slate-500 block mb-1">World Generation Seed:</span>
                        {seed ? (
                          <div>
                            <code className="text-emerald-400 font-bold bg-obsidian-950 px-2 py-0.5 rounded border border-obsidian-800">
                              {seed}
                            </code>
                            {selectedSeedPreset && (
                              <div className="mt-2 text-[11px] text-slate-300">
                                <span className="font-bold text-emerald-300">{selectedSeedPreset.name}</span>
                                <p className="text-slate-400 mt-0.5">{selectedSeedPreset.description}</p>
                              </div>
                            )}
                          </div>
                        ) : (
                          <span className="text-amber-400">Random Generation (Blank seed)</span>
                        )}
                      </div>
                    </div>
                  </div>
                </div>
              )}

              {/* Wizard Navigation Buttons */}
              <div className="flex items-center justify-between pt-4 border-t border-obsidian-800">
                <button
                  type="button"
                  disabled={step === 1}
                  onClick={() => setStep((s) => Math.max(1, s - 1))}
                  className="px-4 py-2 rounded-lg bg-obsidian-800 hover:bg-obsidian-700 disabled:opacity-40 text-slate-300 font-mono text-xs flex items-center space-x-1.5 transition-colors"
                >
                  <ArrowLeft className="w-3.5 h-3.5" />
                  <span>Back</span>
                </button>

                <div className="flex items-center space-x-3">
                  <button
                    type="button"
                    onClick={onClose}
                    className="px-4 py-2 rounded-lg bg-transparent hover:bg-obsidian-800 text-slate-400 hover:text-slate-200 font-mono text-xs transition-colors"
                  >
                    Cancel
                  </button>

                  {step < 5 ? (
                    <button
                      type="button"
                      disabled={step === 1 && (!serverName.trim() || !serverId.trim())}
                      onClick={() => setStep((s) => Math.min(5, s + 1))}
                      className="px-4 py-2 rounded-lg bg-emerald-600 hover:bg-emerald-500 disabled:opacity-50 text-slate-950 font-bold font-mono text-xs flex items-center space-x-1.5 transition-all shadow-[0_0_12px_rgba(16,185,129,0.3)]"
                    >
                      <span>Continue</span>
                      <ArrowRight className="w-3.5 h-3.5" />
                    </button>
                  ) : (
                    <button
                      type="button"
                      disabled={creating || isQuotaFull}
                      onClick={() => handleSubmit()}
                      className="px-5 py-2.5 rounded-lg bg-emerald-500 hover:bg-emerald-400 disabled:opacity-40 disabled:cursor-not-allowed text-slate-950 font-black font-mono text-xs flex items-center space-x-2 transition-all shadow-[0_0_20px_rgba(16,185,129,0.4)]"
                    >
                      {creating ? (
                        <>
                          <Loader2 className="w-4 h-4 animate-spin" />
                          <span>Provisioning Container...</span>
                        </>
                      ) : isQuotaFull ? (
                        <>
                          <AlertTriangle className="w-4 h-4 text-amber-950" />
                          <span>Quota Full ({userPlan?.usage.servers_count}/{userPlan?.usage.servers_max})</span>
                        </>
                      ) : (
                        <>
                          <Plus className="w-4 h-4" />
                          <span>Deploy Instance Now</span>
                        </>
                      )}
                    </button>
                  )}
                </div>
              </div>
            </div>
          )}

          {/* ================= METHOD 2: QUICK DEPLOY ================= */}
          {method === 'quick' && (
            <form onSubmit={handleSubmit} className="space-y-4">
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <div>
                  <label className="block text-xs font-mono text-slate-300 mb-1">
                    Server Name <span className="text-emerald-400">*</span>
                  </label>
                  <input
                    type="text"
                    required
                    value={serverName}
                    onChange={(e) => {
                      setServerName(e.target.value);
                      if (!serverId || serverId === serverName.toLowerCase().replace(/[^a-z0-9]/g, '-')) {
                        setServerId(e.target.value.toLowerCase().replace(/[^a-z0-9]/g, '-'));
                      }
                    }}
                    className="w-full px-3 py-2 rounded-lg bg-obsidian-950 border border-obsidian-700 text-slate-100 font-mono text-sm focus:border-emerald-500"
                    placeholder="Survival Realm"
                  />
                </div>

                <div>
                  <label className="block text-xs font-mono text-slate-300 mb-1">
                    Server ID (Slug) <span className="text-emerald-400">*</span>
                  </label>
                  <input
                    type="text"
                    required
                    value={serverId}
                    onChange={(e) => setServerId(e.target.value.toLowerCase().replace(/[^a-z0-9-]/g, '-'))}
                    className="w-full px-3 py-2 rounded-lg bg-obsidian-950 border border-obsidian-700 text-slate-100 font-mono text-sm focus:border-emerald-500"
                    placeholder="survival-realm"
                  />
                </div>
              </div>

              <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
                <div>
                  <label className="block text-xs font-mono text-slate-300 mb-1 flex items-center justify-between">
                    <span>UDP Port</span>
                    <span className="text-[10px] text-emerald-400">IPv6: {Number(port) + 1}</span>
                  </label>
                  <input
                    type="number"
                    required
                    disabled={!allowCustomPort}
                    value={port}
                    onChange={(e) => setPort(Number(e.target.value))}
                    className={`w-full px-3 py-2 rounded-lg border font-mono text-sm focus:border-emerald-500 ${
                      !allowCustomPort
                        ? 'bg-obsidian-950 border-obsidian-800 text-slate-400 cursor-not-allowed'
                        : 'bg-obsidian-950 border-obsidian-700 text-slate-100'
                    }`}
                  />
                  {!allowCustomPort && (
                    <span className="text-[10px] text-slate-500 font-mono block mt-0.5">Auto-allocated</span>
                  )}
                </div>

                <div>
                  <label className="block text-xs font-mono text-slate-300 mb-1">Game Mode</label>
                  <select
                    value={mode}
                    onChange={(e) => setMode(e.target.value)}
                    className="w-full px-3 py-2 rounded-lg bg-obsidian-950 border border-obsidian-700 text-slate-100 font-mono text-sm focus:border-emerald-500"
                  >
                    <option value="survival">Survival</option>
                    <option value="creative">Creative</option>
                    <option value="adventure">Adventure</option>
                  </select>
                </div>

                <div>
                  <label className="block text-xs font-mono text-slate-300 mb-1">Difficulty</label>
                  <select
                    value={difficulty}
                    onChange={(e) => setDifficulty(e.target.value)}
                    className="w-full px-3 py-2 rounded-lg bg-obsidian-950 border border-obsidian-700 text-slate-100 font-mono text-sm focus:border-emerald-500"
                  >
                    <option value="peaceful">Peaceful</option>
                    <option value="easy">Easy</option>
                    <option value="normal">Normal</option>
                    <option value="hard">Hard</option>
                  </select>
                </div>
              </div>

              {/* World Seed Input with Popular Seed Picker trigger */}
              {!allowCustomSeed ? (
                <div className="p-3 bg-obsidian-950 border border-obsidian-800 rounded-xl text-center">
                  <span className="text-xs text-slate-400 font-mono">
                    World seed will be randomly chosen at launch per plan restrictions.
                  </span>
                </div>
              ) : (
                <div className="p-3 bg-obsidian-950 border border-obsidian-800 rounded-xl space-y-2">
                  <div className="flex items-center justify-between">
                    <label className="block text-xs font-mono text-slate-300 flex items-center space-x-1.5">
                      <Compass className="w-3.5 h-3.5 text-emerald-400" />
                      <span>World Seed</span>
                    </label>
                    <button
                      type="button"
                      onClick={() => setShowSeedPickerDrawer(!showSeedPickerDrawer)}
                      className="text-[11px] font-mono text-emerald-400 hover:text-emerald-300 flex items-center space-x-1 bg-emerald-500/10 px-2 py-0.5 rounded border border-emerald-500/30 transition-colors"
                    >
                      <Sparkles className="w-3 h-3 text-amber-400" />
                      <span>{showSeedPickerDrawer ? 'Hide Popular Seeds' : '✨ Browse Popular Seeds'}</span>
                    </button>
                  </div>

                  <div className="flex items-center space-x-2">
                    <input
                      type="text"
                      value={seed}
                      onChange={(e) => {
                        setSeed(e.target.value);
                        setSelectedSeedPreset(null);
                      }}
                      className="flex-1 px-3 py-2 rounded-lg bg-obsidian-900 border border-obsidian-700 text-slate-100 font-mono text-sm focus:border-emerald-500"
                      placeholder="Leave blank for random world seed"
                    />
                    {seed && (
                      <button
                        type="button"
                        onClick={() => handleSelectSeed('')}
                        className="px-2.5 py-2 rounded-lg bg-obsidian-900 border border-obsidian-700 text-xs font-mono text-slate-400 hover:text-rose-400"
                      >
                        Clear
                      </button>
                    )}
                  </div>

                  {selectedSeedPreset && (
                    <div className="text-[11px] font-mono text-slate-400 flex items-center space-x-2">
                      <span className="text-emerald-400 font-bold">{selectedSeedPreset.name}</span>
                      <span>•</span>
                      <span>{selectedSeedPreset.category}</span>
                    </div>
                  )}

                  {/* Inline Drawer for Popular Seeds in Quick Mode */}
                  {showSeedPickerDrawer && (
                    <div className="mt-3 pt-3 border-t border-obsidian-800">
                      <PopularSeedPicker
                        selectedSeed={seed}
                        onSelectSeed={(s, p) => {
                          handleSelectSeed(s, p);
                          setShowSeedPickerDrawer(false);
                        }}
                        inline={true}
                      />
                    </div>
                  )}
                </div>
              )}

              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="block text-xs font-mono text-slate-300 mb-1 flex items-center justify-between">
                    <span>RAM Capping</span>
                    {userPlan && <span className="text-[10px] text-emerald-400">{userPlan.plan.max_memory}</span>}
                  </label>
                  <select
                    value={memLimit}
                    onChange={(e) => setMemLimit(e.target.value)}
                    className="w-full px-3 py-2 rounded-lg bg-obsidian-950 border border-obsidian-700 text-slate-100 font-mono text-sm focus:border-emerald-500"
                  >
                    {ramOptions.map((opt) => (
                      <option key={opt.value} value={opt.value}>
                        {opt.label}
                      </option>
                    ))}
                  </select>
                </div>
                <div>
                  <label className="block text-xs font-mono text-slate-300 mb-1 flex items-center justify-between">
                    <span>CPU Allocation</span>
                    {userPlan && <span className="text-[10px] text-emerald-400">{userPlan.plan.max_cpu} Cores</span>}
                  </label>
                  <select
                    value={cpuLimit}
                    onChange={(e) => setCpuLimit(Number(e.target.value))}
                    className="w-full px-3 py-2 rounded-lg bg-obsidian-950 border border-obsidian-700 text-slate-100 font-mono text-sm focus:border-emerald-500"
                  >
                    {cpuOptions.map((opt) => (
                      <option key={opt.value} value={opt.value}>
                        {opt.label}
                      </option>
                    ))}
                  </select>
                </div>
              </div>

              <div>
                <label className="block text-xs font-mono text-slate-300 mb-1 flex items-center justify-between">
                  <span>Game Server Address (Knock Host)</span>
                  <span className="text-[10px] text-slate-500">Optional override</span>
                </label>
                <input
                  type="text"
                  value={gameServerAddress}
                  onChange={(e) => setGameServerAddress(e.target.value)}
                  className="w-full px-3 py-2 rounded-lg bg-obsidian-950 border border-obsidian-700 text-slate-100 font-mono text-sm focus:border-emerald-500"
                  placeholder={`Optional (defaults to ${window.location.hostname})`}
                />
              </div>

              <div className="p-3 bg-obsidian-950 border border-obsidian-800 rounded-lg space-y-2.5">
                <label className="flex items-center space-x-2 text-xs font-mono text-slate-200 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={autostartOnBoot}
                    onChange={(e) => setAutostartOnBoot(e.target.checked)}
                    className="rounded bg-obsidian-900 border-obsidian-700 text-emerald-500"
                  />
                  <span>Autostart on Daemon Boot</span>
                </label>

                <label className={`flex items-center space-x-2 text-xs font-mono text-slate-200 ${
                  !allowPortGate ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer'
                }`}>
                  <input
                    type="checkbox"
                    disabled={!allowPortGate}
                    checked={portGate}
                    onChange={(e) => setPortGate(e.target.checked)}
                    className="rounded bg-obsidian-900 border-obsidian-700 text-emerald-500"
                  />
                  <span>Enable Dynamic Port Gating (Firewall Block) {!allowPortGate && '(Unavailable on plan)'}</span>
                </label>

                {portGate && (
                  <div className="pt-2">
                    <label className="block text-[11px] font-mono text-slate-400 mb-1">Verification Mode</label>
                    <select
                      value={portGateMode}
                      onChange={(e) => setPortGateMode(e.target.value as any)}
                      className="w-full px-2.5 py-1.5 rounded bg-obsidian-900 border border-obsidian-700 text-slate-200 text-xs font-mono"
                    >
                      <option value="passphrase">Passphrase / Knock Key</option>
                      <option value="gamertag">Gamertag Allowlist</option>
                      <option value="combined">Combined (Passphrase + Gamertag)</option>
                    </select>
                  </div>
                )}

                {isAdmin && (
                  <div className="pt-2 border-t border-obsidian-800/80 space-y-1">
                    <label className="block text-[11px] font-mono text-slate-300 flex items-center justify-between">
                      <span className="flex items-center gap-1.5 font-bold">
                        <Network className="w-3.5 h-3.5 text-emerald-400" />
                        <span>Deployment Network</span>
                      </span>
                      <span className="text-[10px] text-emerald-400 font-normal">Admin Option</span>
                    </label>
                    <select
                      value={networkMode}
                      onChange={(e) => setNetworkMode(e.target.value)}
                      className="w-full px-2.5 py-1.5 rounded bg-obsidian-900 border border-obsidian-700 text-slate-100 font-mono text-xs focus:border-emerald-500"
                    >
                      <option value="host">host (Default - Direct Host Network & Direct UDP Port Binding)</option>
                      <option value="bridge">bridge (Virtual Bridge & Port Mapping)</option>
                      {availableNetworks
                        .filter((n) => n !== 'bridge' && n !== 'host')
                        .map((net) => (
                          <option key={net} value={net}>
                            {net} (Custom Docker Network)
                          </option>
                        ))}
                    </select>
                    <p className="text-[10px] text-slate-400">
                      {networkMode === 'host'
                        ? 'Host Mode: Direct host UDP binding. Resolves bridge/UFW forwarding conflicts.'
                        : 'Bridge Mode: Isolated virtual bridge network with NAT forwarding.'}
                    </p>
                  </div>
                )}
              </div>

              <div className="flex items-center justify-end space-x-3 pt-4 border-t border-obsidian-800">
                <button
                  type="button"
                  onClick={onClose}
                  className="px-4 py-2 rounded-lg bg-obsidian-800 hover:bg-obsidian-700 text-slate-300 font-mono text-xs"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={creating || isQuotaFull}
                  className="px-4 py-2 rounded-lg bg-emerald-600 hover:bg-emerald-500 disabled:opacity-40 disabled:cursor-not-allowed text-slate-950 font-bold font-mono text-xs flex items-center space-x-1.5"
                >
                  {creating ? (
                    <Loader2 className="w-4 h-4 animate-spin" />
                  ) : isQuotaFull ? (
                    <span>Quota Reached</span>
                  ) : (
                    <span>Confirm Deploy</span>
                  )}
                </button>
              </div>
            </form>
          )}
        </div>
      </div>
    </div>
  );
};
