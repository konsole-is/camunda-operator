const UNITS: [suffix: string, seconds: number][] = [
  ["d", 86_400],
  ["h", 3_600],
  ["m", 60],
];

/**
 * age is the time from from to now in its largest whole unit: 45s, 12m, 3h,
 * or 5d. A missing or bad time is "-". A time after now is "0s", because a
 * clock on another node can run ahead.
 */
export function age(
  from: string | Date | undefined,
  now: string | Date,
): string {
  if (from === undefined) return "-";
  const start = new Date(from).getTime();
  const end = new Date(now).getTime();
  if (Number.isNaN(start) || Number.isNaN(end)) return "-";
  const seconds = Math.max(0, Math.floor((end - start) / 1000));
  for (const [suffix, size] of UNITS) {
    if (seconds >= size) return `${Math.floor(seconds / size)}${suffix}`;
  }
  return `${seconds}s`;
}
