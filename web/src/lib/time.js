const RELATIVE_TIME_UNITS = [
  { name: 'year', milliseconds: 365 * 24 * 60 * 60 * 1000 },
  { name: 'month', milliseconds: 30 * 24 * 60 * 60 * 1000 },
  { name: 'day', milliseconds: 24 * 60 * 60 * 1000 },
  { name: 'h', milliseconds: 60 * 60 * 1000 },
  { name: 'min', milliseconds: 60 * 1000 },
  { name: 's', milliseconds: 1000 }
];

export function formatRelativeTime(value, { now = Date.now(), empty = 'Not recorded' } = {}) {
  if (!value) return empty;

  const timestamp = value instanceof Date ? value.getTime() : new Date(value).getTime();
  if (!Number.isFinite(timestamp)) return empty;

  const elapsed = now - timestamp;
  const absolute = Math.abs(elapsed);
  const unit = RELATIVE_TIME_UNITS.find(({ milliseconds }) => absolute >= milliseconds)
    || RELATIVE_TIME_UNITS[RELATIVE_TIME_UNITS.length - 1];
  const amount = Math.max(0, Math.floor(absolute / unit.milliseconds));
  const label = ['year', 'month', 'day'].includes(unit.name)
    ? `${amount} ${unit.name}${amount === 1 ? '' : 's'}`
    : `${amount}${unit.name}`;

  return elapsed >= 0 ? `${label} ago` : `in ${label}`;
}
