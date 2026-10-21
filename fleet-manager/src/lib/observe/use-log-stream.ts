"use client";

import { useEffect, useMemo, useState } from "react";
import { type LogFilter, filterLines, parseLogLine } from "./logs";
import type { LogLine } from "./types";

/** LogSource names the log of one container, as the parameters of /api/logs. */
export type LogSource = {
  namespace: string;
  pod: string;
  container?: string;
  sinceSeconds?: number;
  tailLines?: number;
};

const LIMIT = 1000;

type State = { url?: string; lines: LogLine[]; error?: string; ended: boolean };

/**
 * useLogStream follows the log of source through /api/logs and returns
 * the newest 1000 lines that filter keeps. ended is true when the log
 * ended. error is set when the stream broke; the hook does not connect
 * again. No source opens no connection. A new source starts with no lines.
 */
export function useLogStream(
  source: LogSource | undefined,
  filter: LogFilter = {},
): { lines: LogLine[]; error?: string; ended: boolean } {
  const url = source && logURL(source);
  const [state, setState] = useState<State>({ lines: [], ended: false });

  useEffect(() => {
    if (!url) return;
    const events = new EventSource(url);
    // Each update starts from empty state when an older source wrote it.
    const update = (change: (s: State) => Partial<State>) =>
      setState((s) => {
        const current = s.url === url ? s : { url, lines: [], ended: false };
        return { ...current, ...change(current) };
      });
    const stop = (change: Partial<State>) => {
      events.close();
      update(() => change);
    };
    events.addEventListener("line", (event) => {
      const line = parseLogLine(JSON.parse((event as MessageEvent).data));
      update((s) => ({ lines: [...s.lines, line].slice(-LIMIT) }));
    });
    events.addEventListener("end", () => stop({ ended: true }));
    events.addEventListener("failure", (event) =>
      stop({ error: JSON.parse((event as MessageEvent).data).message }),
    );
    // EventSource connects again by itself, and the route would then send
    // the lines that the page already has.
    events.addEventListener("error", () =>
      stop({ error: "The connection to the log stream broke" }),
    );
    return () => events.close();
  }, [url]);

  const current = state.url === url ? state : undefined;
  const lines = current?.lines;
  const { level, text, request } = filter;
  const filtered = useMemo(
    () => filterLines(lines ?? [], { level, text, request }),
    [lines, level, text, request],
  );
  return {
    lines: filtered,
    ended: current?.ended ?? false,
    ...(current?.error !== undefined && { error: current.error }),
  };
}

function logURL(source: LogSource): string {
  const params = new URLSearchParams();
  for (const key of [
    "namespace",
    "pod",
    "container",
    "sinceSeconds",
    "tailLines",
  ] as const) {
    if (source[key] !== undefined) params.set(key, String(source[key]));
  }
  return `/api/logs?${params}`;
}
