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

/**
 * dateTime writes an RFC 3339 time as "2026-10-18 11:58 UTC", the same on
 * every machine. It writes "-" for an unset or bad time.
 */
export function dateTime(iso: string | undefined): string {
  const ms = iso ? Date.parse(iso) : NaN;
  if (Number.isNaN(ms)) return "-";
  const text = new Date(ms).toISOString();
  return `${text.slice(0, 10)} ${text.slice(11, 16)} UTC`;
}

/**
 * secondText writes a valid RFC 3339 time as "2026-11-10 09:30:15 UTC": a
 * point in time needs the seconds that dateTime leaves out.
 */
export function secondText(at: string): string {
  const text = new Date(at).toISOString();
  return `${text.slice(0, 10)} ${text.slice(11, 19)} UTC`;
}

/** formatClock writes a time in ms as "11:58", in UTC. */
export function formatClock(ms: number): string {
  return new Date(ms).toISOString().slice(11, 16);
}

function round(value: number): string {
  return String(Math.round(value * 10) / 10);
}
