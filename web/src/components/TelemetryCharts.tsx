import React, { useState, useId } from 'react';
import { Cpu, HardDrive, Users, Activity, Clock } from 'lucide-react';
import { MetricsData, MetricPoint } from '../types';

interface TelemetryChartsProps {
  metrics: MetricsData | null;
  loading: boolean;
  timeRange: '15m' | '1h' | '6h' | '24h';
  onTimeRangeChange: (range: '15m' | '1h' | '6h' | '24h') => void;
  serverStatus: string;
}

interface MiniChartProps {
  title: string;
  icon: React.ReactNode;
  data: number[];
  timestamps: string[];
  unit: string;
  color: 'emerald' | 'cyan' | 'amber';
  threshold?: number;
  thresholdLabel?: string;
  currentVal: number;
  currentLabel?: string;
  peakVal: number;
  avgVal: number;
  formatValue?: (val: number) => string;
  formatTooltip?: (val: number, idx: number) => string;
}

const formatBytes = (bytes: number): string => {
  if (bytes <= 0) return '0 MB';
  const mb = bytes / (1024 * 1024);
  if (mb >= 1024) {
    return `${(mb / 1024).toFixed(2)} GB`;
  }
  return `${mb.toFixed(0)} MB`;
};

const formatTimeLabel = (isoString: string): string => {
  try {
    const d = new Date(isoString);
    if (isNaN(d.getTime())) return '';
    return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
  } catch {
    return '';
  }
};

const SingleChart: React.FC<MiniChartProps> = ({
  title,
  icon,
  data,
  timestamps,
  color,
  threshold,
  thresholdLabel,
  currentVal,
  currentLabel,
  peakVal,
  avgVal,
  formatValue = (v) => `${v.toFixed(1)}`,
  formatTooltip,
}) => {
  const [hoverIdx, setHoverIdx] = useState<number | null>(null);
  const gradientId = useId();

  const width = 520;
  const height = 150;
  const padLeft = 38;
  const padRight = 16;
  const padTop = 16;
  const padBottom = 26;

  const chartW = width - padLeft - padRight;
  const chartH = height - padTop - padBottom;

  // Color mappings
  const colorMap = {
    emerald: {
      stroke: '#10b981',
      fillStart: 'rgba(16, 185, 129, 0.28)',
      fillEnd: 'rgba(16, 185, 129, 0.01)',
      text: 'text-emerald-400',
      badgeBg: 'bg-emerald-500/10 text-emerald-400 border-emerald-500/30',
      dot: 'bg-emerald-400',
    },
    cyan: {
      stroke: '#06b6d4',
      fillStart: 'rgba(6, 182, 212, 0.28)',
      fillEnd: 'rgba(6, 182, 212, 0.01)',
      text: 'text-cyber-cyan',
      badgeBg: 'bg-cyan-500/10 text-cyan-400 border-cyan-500/30',
      dot: 'bg-cyan-400',
    },
    amber: {
      stroke: '#f59e0b',
      fillStart: 'rgba(245, 158, 11, 0.28)',
      fillEnd: 'rgba(245, 158, 11, 0.01)',
      text: 'text-amber-400',
      badgeBg: 'bg-amber-500/10 text-amber-400 border-amber-500/30',
      dot: 'bg-amber-400',
    },
  }[color];

  // Calculate points
  const pointsData = data.length === 1 ? [data[0], data[0]] : data;
  const timestampsData = timestamps.length === 1 ? [timestamps[0], timestamps[0]] : timestamps;

  const maxVal = Math.max(...pointsData, threshold || 0, 1);
  const minVal = 0;
  const valRange = maxVal - minVal || 1;

  const coords = pointsData.map((val, idx) => {
    const x = padLeft + (idx / Math.max(pointsData.length - 1, 1)) * chartW;
    const y = padTop + (1 - (val - minVal) / valRange) * chartH;
    return { x, y, val };
  });

  // Construct SVG path with smooth bezier curves
  let linePath = '';
  let areaPath = '';

  if (coords.length > 0) {
    linePath = `M ${coords[0].x.toFixed(1)} ${coords[0].y.toFixed(1)}`;
    for (let i = 0; i < coords.length - 1; i++) {
      const p0 = coords[i];
      const p1 = coords[i + 1];
      const cx = (p0.x + p1.x) / 2;
      linePath += ` C ${cx.toFixed(1)} ${p0.y.toFixed(1)}, ${cx.toFixed(1)} ${p1.y.toFixed(1)}, ${p1.x.toFixed(1)} ${p1.y.toFixed(1)}`;
    }
    const lastX = coords[coords.length - 1].x;
    const bottomY = padTop + chartH;
    areaPath = `${linePath} L ${lastX.toFixed(1)} ${bottomY} L ${coords[0].x.toFixed(1)} ${bottomY} Z`;
  }

  // Threshold Y coordinate
  const thresholdY = threshold !== undefined ? padTop + (1 - (threshold - minVal) / valRange) * chartH : null;

  // Mouse hover event handler
  const handleMouseMove = (e: React.MouseEvent<SVGSVGElement>) => {
    if (coords.length === 0) return;
    const rect = e.currentTarget.getBoundingClientRect();
    const mouseX = ((e.clientX - rect.left) / rect.width) * width;
    const clampedX = Math.max(padLeft, Math.min(width - padRight, mouseX));
    const ratio = (clampedX - padLeft) / chartW;
    const nearestIdx = Math.round(ratio * (pointsData.length - 1));
    setHoverIdx(Math.max(0, Math.min(pointsData.length - 1, nearestIdx)));
  };

  const activeCoord = hoverIdx !== null && coords[hoverIdx] ? coords[hoverIdx] : null;

  return (
    <div className="bg-obsidian-900 border border-obsidian-700/80 rounded-xl p-4 flex flex-col justify-between shadow-lg relative group">
      {/* Card Header */}
      <div className="flex items-start justify-between mb-2">
        <div className="flex items-center gap-2">
          <div className="p-1.5 rounded-lg bg-obsidian-950 border border-obsidian-800 text-slate-300">
            {icon}
          </div>
          <div>
            <h4 className="font-mono text-xs font-bold text-slate-200">{title}</h4>
            <div className="text-[10px] text-slate-400 font-mono flex items-center gap-2 mt-0.5">
              <span>Avg: {formatValue(avgVal)}</span>
              <span>•</span>
              <span>Peak: {formatValue(peakVal)}</span>
            </div>
          </div>
        </div>

        <div className="text-right">
          <div className={`font-mono text-sm font-bold ${colorMap.text}`}>
            {currentLabel ? currentLabel : formatValue(currentVal)}
          </div>
          <span className="text-[10px] text-slate-500 font-mono">Current</span>
        </div>
      </div>

      {/* SVG Chart */}
      <div className="relative w-full h-[145px]">
        {coords.length === 0 ? (
          <div className="h-full flex items-center justify-center font-mono text-xs text-slate-500 italic bg-obsidian-950/40 rounded-lg border border-obsidian-800/40">
            No telemetry samples recorded yet
          </div>
        ) : (
          <svg
            viewBox={`0 0 ${width} ${height}`}
            className="w-full h-full overflow-visible select-none cursor-crosshair"
            onMouseMove={handleMouseMove}
            onMouseLeave={() => setHoverIdx(null)}
          >
            <defs>
              <linearGradient id={gradientId} x1="0" y1="0" x2="0" y2="1">
                <stop offset="0%" stopColor={colorMap.fillStart} />
                <stop offset="100%" stopColor={colorMap.fillEnd} />
              </linearGradient>
            </defs>

            {/* Horizontal Grid lines */}
            {[0, 0.5, 1].map((pct) => {
              const y = padTop + pct * chartH;
              const val = maxVal - pct * valRange;
              return (
                <g key={pct}>
                  <line
                    x1={padLeft}
                    y1={y}
                    x2={width - padRight}
                    y2={y}
                    stroke="#1e293b"
                    strokeDasharray="2 3"
                    strokeWidth="1"
                  />
                  <text
                    x={padLeft - 4}
                    y={y + 3}
                    textAnchor="end"
                    fill="#64748b"
                    fontSize="9"
                    fontFamily="monospace"
                  >
                    {formatValue(val)}
                  </text>
                </g>
              );
            })}

            {/* Threshold Line (Memory Limit or Max Players) */}
            {thresholdY !== null && thresholdY >= padTop && thresholdY <= padTop + chartH && (
              <g>
                <line
                  x1={padLeft}
                  y1={thresholdY}
                  x2={width - padRight}
                  y2={thresholdY}
                  stroke="#ef4444"
                  strokeDasharray="4 3"
                  strokeWidth="1.2"
                />
                <text
                  x={width - padRight}
                  y={thresholdY - 3}
                  textAnchor="end"
                  fill="#f87171"
                  fontSize="8.5"
                  fontFamily="monospace"
                >
                  {thresholdLabel || `Cap: ${formatValue(threshold!)}`}
                </text>
              </g>
            )}

            {/* Area Fill */}
            {areaPath && <path d={areaPath} fill={`url(#${gradientId})`} />}

            {/* Line Stroke */}
            {linePath && (
              <path
                d={linePath}
                fill="none"
                stroke={colorMap.stroke}
                strokeWidth="2"
                strokeLinecap="round"
                strokeLinejoin="round"
              />
            )}

            {/* Time Axis Labels */}
            {timestampsData.length > 0 && (
              <>
                <text
                  x={padLeft}
                  y={height - 8}
                  textAnchor="start"
                  fill="#64748b"
                  fontSize="8.5"
                  fontFamily="monospace"
                >
                  {formatTimeLabel(timestampsData[0])}
                </text>
                <text
                  x={width - padRight}
                  y={height - 8}
                  textAnchor="end"
                  fill="#64748b"
                  fontSize="8.5"
                  fontFamily="monospace"
                >
                  {formatTimeLabel(timestampsData[timestampsData.length - 1])}
                </text>
              </>
            )}

            {/* Hover Cursor and Crosshair */}
            {activeCoord && hoverIdx !== null && (
              <g>
                <line
                  x1={activeCoord.x}
                  y1={padTop}
                  x2={activeCoord.x}
                  y2={padTop + chartH}
                  stroke="#94a3b8"
                  strokeDasharray="2 2"
                  strokeWidth="1"
                />
                <circle
                  cx={activeCoord.x}
                  cy={activeCoord.y}
                  r="4.5"
                  fill={colorMap.stroke}
                  stroke="#0f172a"
                  strokeWidth="2"
                />
              </g>
            )}
          </svg>
        )}

        {/* Hover Tooltip Overlay */}
        {activeCoord && hoverIdx !== null && timestampsData[hoverIdx] && (
          <div
            className="absolute z-20 pointer-events-none -top-1 bg-obsidian-950/95 border border-obsidian-700/90 text-slate-100 font-mono text-[10px] px-2.5 py-1.5 rounded-md shadow-xl backdrop-blur-md transition-all -translate-x-1/2 whitespace-nowrap"
            style={{
              left: `${(activeCoord.x / width) * 100}%`,
            }}
          >
            <div className="text-slate-400 text-[9px] mb-0.5">
              {formatTimeLabel(timestampsData[hoverIdx])}
            </div>
            <div className={`font-bold ${colorMap.text}`}>
              {formatTooltip
                ? formatTooltip(activeCoord.val, hoverIdx)
                : `${title}: ${formatValue(activeCoord.val)}`}
            </div>
          </div>
        )}
      </div>
    </div>
  );
};

export const TelemetryCharts: React.FC<TelemetryChartsProps> = ({
  metrics,
  timeRange,
  onTimeRangeChange,
  serverStatus,
}) => {
  const series: MetricPoint[] = metrics?.series || [];

  // Extract data arrays
  const timestamps = series.map((s) => s.timestamp);
  const cpuData = series.map((s) => s.cpu_percent);
  const ramData = series.map((s) => s.ram_bytes);
  const playersData = series.map((s) => s.active_players);

  // CPU Stats
  const currentCpu = metrics?.current ? metrics.current.cpu_percent : cpuData[cpuData.length - 1] || 0;
  const peakCpu = cpuData.length > 0 ? Math.max(...cpuData) : 0;
  const avgCpu = cpuData.length > 0 ? cpuData.reduce((a, b) => a + b, 0) / cpuData.length : 0;

  // RAM Stats
  const currentRam = metrics?.current ? metrics.current.ram_bytes : ramData[ramData.length - 1] || 0;
  const peakRam = ramData.length > 0 ? Math.max(...ramData) : 0;
  const avgRam = ramData.length > 0 ? ramData.reduce((a, b) => a + b, 0) / ramData.length : 0;
  const memoryLimitBytes = metrics?.memory_limit_bytes || 2 * 1024 * 1024 * 1024;

  // Player Stats
  const maxPlayers = metrics?.max_players || 10;
  const totalAllowlist = metrics?.total_allowlist || 0;
  const currentPlayers = metrics?.current
    ? metrics.current.player_count
    : playersData[playersData.length - 1] || 0;
  const peakPlayers = playersData.length > 0 ? Math.max(...playersData) : 0;
  const avgPlayers = playersData.length > 0 ? playersData.reduce((a, b) => a + b, 0) / playersData.length : 0;

  return (
    <div className="bg-obsidian-950/80 border border-obsidian-800 rounded-2xl p-5 shadow-2xl">
      {/* Section Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 mb-5 border-b border-obsidian-800/80 pb-4">
        <div>
          <h3 className="font-mono text-base font-bold text-slate-100 flex items-center gap-2">
            <Activity className="w-4 h-4 text-emerald-400" />
            <span>Instance Telemetry & Performance Analytics</span>
            <span
              className={`text-[10px] font-mono px-2 py-0.5 rounded-full border uppercase ${
                serverStatus === 'running'
                  ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/30'
                  : 'bg-slate-800 text-slate-400 border-slate-700'
              }`}
            >
              {serverStatus === 'running' ? 'Live Telemetry' : 'Offline'}
            </span>
          </h3>
          <p className="text-xs text-slate-400 font-mono mt-0.5">
            Real-time monitoring of CPU allocation, RAM utilization, and active/capacity player trends.
          </p>
        </div>

        {/* Time Range Selector */}
        <div className="flex items-center space-x-1 bg-obsidian-900 border border-obsidian-750 p-1 rounded-lg shrink-0 font-mono text-xs">
          <Clock className="w-3.5 h-3.5 text-slate-500 ml-1.5 mr-1" />
          {(['15m', '1h', '6h', '24h'] as const).map((r) => (
            <button
              key={r}
              onClick={() => onTimeRangeChange(r)}
              className={`px-2.5 py-1 rounded text-[11px] font-bold transition-all ${
                timeRange === r
                  ? 'bg-emerald-600 text-slate-950 shadow-[0_0_8px_rgba(16,185,129,0.3)]'
                  : 'text-slate-400 hover:text-slate-200 hover:bg-obsidian-800'
              }`}
            >
              {r}
            </button>
          ))}
        </div>
      </div>

      {/* 3 Metric Charts Grid */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-5">
        {/* 1. CPU Usage Chart */}
        <SingleChart
          title="CPU Utilization"
          icon={<Cpu className="w-4 h-4 text-emerald-400" />}
          data={cpuData}
          timestamps={timestamps}
          unit="%"
          color="emerald"
          currentVal={currentCpu}
          currentLabel={`${currentCpu.toFixed(1)}%`}
          peakVal={peakCpu}
          avgVal={avgCpu}
          formatValue={(v) => `${v.toFixed(1)}%`}
          formatTooltip={(v) => `CPU: ${v.toFixed(1)}% (${metrics?.cpu_limit || 2.0} Cores)`}
        />

        {/* 2. RAM Usage Chart */}
        <SingleChart
          title="Memory (RAM) Usage"
          icon={<HardDrive className="w-4 h-4 text-cyan-400" />}
          data={ramData}
          timestamps={timestamps}
          unit="MB"
          color="cyan"
          threshold={memoryLimitBytes}
          thresholdLabel={`Cap: ${formatBytes(memoryLimitBytes)}`}
          currentVal={currentRam}
          currentLabel={formatBytes(currentRam)}
          peakVal={peakRam}
          avgVal={avgRam}
          formatValue={formatBytes}
          formatTooltip={(v) => `RAM: ${formatBytes(v)} / ${formatBytes(memoryLimitBytes)}`}
        />

        {/* 3. Player Count Chart (Active vs Total) */}
        <SingleChart
          title="Player Count (Active / Total)"
          icon={<Users className="w-4 h-4 text-amber-400" />}
          data={playersData}
          timestamps={timestamps}
          unit="players"
          color="amber"
          threshold={maxPlayers}
          thresholdLabel={`Capacity: ${maxPlayers}`}
          currentVal={currentPlayers}
          currentLabel={`${currentPlayers} / ${maxPlayers} Online`}
          peakVal={peakPlayers}
          avgVal={avgPlayers}
          formatValue={(v) => `${Math.round(v)}`}
          formatTooltip={(v) =>
            `Active: ${v} / ${maxPlayers} Capacity (${totalAllowlist} Allowed)`
          }
        />
      </div>

      {/* Quick Summary Pill Bar */}
      <div className="mt-4 pt-3 border-t border-obsidian-800/60 grid grid-cols-2 sm:grid-cols-4 gap-3 text-xs font-mono text-slate-400">
        <div className="bg-obsidian-900/60 border border-obsidian-800 px-3 py-2 rounded-lg flex items-center justify-between">
          <span className="text-slate-500">Core Limit:</span>
          <span className="text-slate-200 font-bold">{metrics?.cpu_limit || 2.0} Cores</span>
        </div>
        <div className="bg-obsidian-900/60 border border-obsidian-800 px-3 py-2 rounded-lg flex items-center justify-between">
          <span className="text-slate-500">RAM Limit:</span>
          <span className="text-slate-200 font-bold">{formatBytes(memoryLimitBytes)}</span>
        </div>
        <div className="bg-obsidian-900/60 border border-obsidian-800 px-3 py-2 rounded-lg flex items-center justify-between">
          <span className="text-slate-500">Max Capacity:</span>
          <span className="text-slate-200 font-bold">{maxPlayers} Players</span>
        </div>
        <div className="bg-obsidian-900/60 border border-obsidian-800 px-3 py-2 rounded-lg flex items-center justify-between">
          <span className="text-slate-500">Allowlist Count:</span>
          <span className="text-slate-200 font-bold">{totalAllowlist} Registered</span>
        </div>
      </div>
    </div>
  );
};
