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

function round(value: number): string {
  return String(Math.round(value * 10) / 10);
}
