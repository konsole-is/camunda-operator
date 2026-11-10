const BYTE_UNITS = ["B", "KiB", "MiB", "GiB", "TiB"];

/** formatCores writes CPU below one core in millicores, and one core or more in cores. */
export function formatCores(cores: number): string {
  return cores < 1 ? `${round(cores * 1000)}m` : `${round(cores)}`;
}

/** formatBytes writes bytes in the largest binary unit that keeps the number at 1 or more. */
export function formatBytes(bytes: number): string {
  let value = bytes;
  let unit = 0;
  while (value >= 1024 && unit < BYTE_UNITS.length - 1) {
    value /= 1024;
    unit++;
  }
  return `${round(value)} ${BYTE_UNITS[unit]}`;
}

const COMPACT = new Intl.NumberFormat("en", {
  notation: "compact",
  maximumSignificantDigits: 3,
});

/**
 * formatAxis writes a value of a chart axis short: bytes as formatBytes,
 * cores as formatCores, and any other unit as a compact number.
 */
export function formatAxis(value: number, unit?: string): string {
  if (unit === "bytes") return formatBytes(value);
  if (unit === "cores") return formatCores(value);
  return COMPACT.format(value);
}

/** formatClock writes a time in ms as "17:28", in the time zone of the viewer. */
export function formatClock(ms: number): string {
  const t = new Date(ms);
  return `${pad(t.getHours())}:${pad(t.getMinutes())}`;
}

/** formatDateClock writes a time in ms as "2026-10-18 17:28", in the time zone of the viewer. */
export function formatDateClock(ms: number): string {
  const t = new Date(ms);
  return `${t.getFullYear()}-${pad(t.getMonth() + 1)}-${pad(t.getDate())} ${formatClock(ms)}`;
}

function pad(n: number): string {
  return String(n).padStart(2, "0");
}

function round(value: number): string {
  return String(Math.round(value * 10) / 10);
}
