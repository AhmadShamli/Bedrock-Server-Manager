export type DurationUnit = 'minutes' | 'hours' | 'days';

export function durationToMinutes(value: number, unit: DurationUnit): number {
  const safeVal = Number.isFinite(value) && value > 0 ? value : 1;
  switch (unit) {
    case 'days':
      return Math.max(1, Math.round(safeVal * 1440));
    case 'hours':
      return Math.max(1, Math.round(safeVal * 60));
    case 'minutes':
    default:
      return Math.max(1, Math.round(safeVal));
  }
}

export function durationToSeconds(value: number, unit: DurationUnit): number {
  return durationToMinutes(value, unit) * 60;
}

export function secondsToValueAndUnit(totalSeconds: number): { value: number; unit: DurationUnit } {
  const sec = Number.isFinite(totalSeconds) && totalSeconds > 0 ? Math.round(totalSeconds) : 7200;
  if (sec >= 86400 && sec % 86400 === 0) {
    return { value: sec / 86400, unit: 'days' };
  }
  if (sec >= 3600 && sec % 3600 === 0) {
    return { value: sec / 3600, unit: 'hours' };
  }
  return { value: Math.max(1, Math.round(sec / 60)), unit: 'minutes' };
}

export function formatDurationSeconds(totalSeconds?: number | null): string {
  if (!totalSeconds || totalSeconds <= 0) return '0 min';
  const totalMin = Math.max(1, Math.round(totalSeconds / 60));
  const days = Math.floor(totalMin / 1440);
  const hours = Math.floor((totalMin % 1440) / 60);
  const minutes = totalMin % 60;

  if (days > 0 && hours === 0 && minutes === 0) {
    return `${days} ${days === 1 ? 'day' : 'days'}`;
  }
  if (days === 0 && hours > 0 && minutes === 0) {
    return `${hours} ${hours === 1 ? 'hour' : 'hours'}`;
  }
  if (days === 0 && hours === 0) {
    return `${minutes} ${minutes === 1 ? 'min' : 'mins'}`;
  }

  const parts: string[] = [];
  if (days > 0) parts.push(`${days}d`);
  if (hours > 0) parts.push(`${hours}h`);
  if (minutes > 0) parts.push(`${minutes}m`);
  return parts.join(' ');
}

export function formatDurationMinutes(totalMinutes?: number | null): string {
  if (!totalMinutes || totalMinutes <= 0) return '0 min';
  return formatDurationSeconds(totalMinutes * 60);
}

export function getCalendarDayOffset(startInput: string | number | Date, endInput: string | number | Date): number {
  const start = new Date(startInput);
  const end = new Date(endInput);
  if (isNaN(start.getTime()) || isNaN(end.getTime())) return 0;
  const startMidnight = new Date(start.getFullYear(), start.getMonth(), start.getDate()).getTime();
  const endMidnight = new Date(end.getFullYear(), end.getMonth(), end.getDate()).getTime();
  return Math.round((endMidnight - startMidnight) / 86400000);
}

export function formatLeaseDateTime(
  iso: string,
  refIso?: string,
  nowMs: number = Date.now()
): {
  dateStr: string;
  timeStr: string;
  fullStr: string;
  dayOffsetFromRef: number;
  dayOffsetFromNow: number;
  isDifferentDay: boolean;
} {
  const dt = new Date(iso);
  if (isNaN(dt.getTime())) {
    return {
      dateStr: '—',
      timeStr: '—',
      fullStr: '—',
      dayOffsetFromRef: 0,
      dayOffsetFromNow: 0,
      isDifferentDay: false,
    };
  }

  const dateStr = dt.toLocaleDateString(undefined, {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
  });
  const timeStr = dt.toLocaleTimeString();
  const dayOffsetFromRef = refIso ? getCalendarDayOffset(refIso, dt) : 0;
  const dayOffsetFromNow = getCalendarDayOffset(nowMs, dt);
  const isDifferentDay = dayOffsetFromRef !== 0 || dayOffsetFromNow !== 0;

  return {
    dateStr,
    timeStr,
    fullStr: `${dateStr}, ${timeStr}`,
    dayOffsetFromRef,
    dayOffsetFromNow,
    isDifferentDay,
  };
}

export function formatLeaseTimeLeft(
  expiresAtIso: string,
  nowMs: number = Date.now()
): {
  text: string;
  shortText: string;
  totalSeconds: number;
  isExpired: boolean;
  isExpiringSoon: boolean;
} {
  const expMs = new Date(expiresAtIso).getTime();
  if (isNaN(expMs)) {
    return {
      text: 'Unknown',
      shortText: '—',
      totalSeconds: 0,
      isExpired: false,
      isExpiringSoon: false,
    };
  }

  const diffSec = Math.max(0, Math.floor((expMs - nowMs) / 1000));
  if (diffSec <= 0) {
    return {
      text: 'Expired',
      shortText: 'Expired',
      totalSeconds: 0,
      isExpired: true,
      isExpiringSoon: false,
    };
  }

  const days = Math.floor(diffSec / 86400);
  const hours = Math.floor((diffSec % 86400) / 3600);
  const minutes = Math.floor((diffSec % 3600) / 60);
  const seconds = diffSec % 60;

  let shortText: string;
  if (days > 0) {
    shortText = `${days}d ${hours}h ${minutes}m`;
  } else if (hours > 0) {
    shortText = `${hours}h ${minutes}m ${seconds}s`;
  } else if (minutes > 0) {
    shortText = `${minutes}m ${seconds}s`;
  } else {
    shortText = `${seconds}s`;
  }

  return {
    text: `${shortText} left`,
    shortText,
    totalSeconds: diffSec,
    isExpired: false,
    isExpiringSoon: diffSec < 300,
  };
}
