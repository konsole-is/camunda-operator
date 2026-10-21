// This module runs in the browser and on the server, so it imports types only.
import type { LogLine } from "./types";

/**
 * LogFilter selects log lines. level compares without case, text is a
 * substring of the raw line without case, and request compares exactly. An
 * unset filter keeps every line.
 */
export type LogFilter = { level?: string; text?: string; request?: string };

const KEYS = {
  time: ["ts", "time", "timestamp", "@timestamp"],
  level: ["level", "severity", "log.level"],
  message: ["msg", "message"],
  logger: ["logger", "logger_name"],
  request: ["request"],
} as const;

/**
 * parseLogLine reads one log line. A JSON object gives time, level,
 * message, logger, and request from the keys of zap (ts, level, msg,
 * logger) and of Spring Boot (timestamp, severity, message, logger_name),
 * and its other keys as fields. A plugin controller writes the request as
 * "namespace/name" (contract 6.4). A key whose value is not a string stays
 * in fields. A JSON object without a message, and every other line, has the
 * raw line as its message.
 */
export function parseLogLine(raw: string): LogLine {
  const object = parseObject(raw);
  if (!object) return { raw, message: raw, fields: {} };
  const fields: Record<string, unknown> = { ...object };
  const line: LogLine = { raw, message: raw, fields };
  for (const [name, keys] of Object.entries(KEYS)) {
    const key = keys.find((k) => typeof object[k] === "string");
    if (key === undefined) continue;
    line[name as keyof typeof KEYS] = object[key] as string;
    delete fields[key];
  }
  return line;
}

/** filterLines returns the lines that match every set part of filter, in their order. */
export function filterLines(
  lines: readonly LogLine[],
  filter: LogFilter,
): LogLine[] {
  const level = filter.level?.toLowerCase();
  const text = filter.text?.toLowerCase();
  return lines.filter(
    (line) =>
      (!level || line.level?.toLowerCase() === level) &&
      (!text || line.raw.toLowerCase().includes(text)) &&
      (!filter.request || line.request === filter.request),
  );
}

function parseObject(raw: string): Record<string, unknown> | undefined {
  try {
    const value = JSON.parse(raw) as unknown;
    return typeof value === "object" && value !== null && !Array.isArray(value)
      ? (value as Record<string, unknown>)
      : undefined;
  } catch {
    return undefined;
  }
}
