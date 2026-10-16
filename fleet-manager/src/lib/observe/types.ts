/**
 * Unavailable is a value, not an error: the cluster has no metrics server,
 * or the app has no Prometheus. A page hides the column or the chart.
 */
export type Unavailable = "unavailable";

/** PodUsage is the live CPU and memory of one pod, with its requests and limits. */
export type PodUsage = {
  pod: string;
  cpu: { usedCores: number; requestCores?: number; limitCores?: number };
  memory: { usedBytes: number; requestBytes?: number; limitBytes?: number };
};

/** Series is one Prometheus range series. A point is a Unix time in milliseconds and a value. */
export type Series = { name: string; points: [time: number, value: number][] };

/**
 * LogLine is one log line. A JSON line gives time, level, message, logger,
 * request, and the rest of its fields. Another line has only raw and
 * message.
 */
export type LogLine = {
  raw: string;
  time?: string;
  level?: string;
  message: string;
  logger?: string;
  request?: string;
  fields: Record<string, unknown>;
};
