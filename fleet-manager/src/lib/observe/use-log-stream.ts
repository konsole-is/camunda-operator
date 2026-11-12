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

/**
 * LogStreamSource names the log of one container for useLogStreams. With
 * request, the view keeps only the lines of that request, "namespace/name".
 */
export type LogStreamSource = {
  namespace: string;
  pod: string;
  container?: string;
  request?: string;
};

/** LogStreamOptions apply to every log of useLogStreams. */
export type LogStreamOptions = { sinceSeconds?: number; tailLines?: number };

/** PodLogError is the error of the log of one pod. */
export type PodLogError = { pod: string; error: string };

const LIMIT = 1000;
const CONNECTION_BROKE = "The connection to the log stream broke";

type Stream = { status: "open" | "ended" | "broke"; error?: string };
type State = { key: string; lines: LogLine[]; streams: Stream[] };
type Entry = { pod: string; request?: string };
type Plan = { url?: string; merged: boolean; entries: Entry[] };

/**
 * useLogStreams follows the log of each source over one connection to
 * /api/logs/merged, so it takes one of the 6 connections that Chromium
 * opens to one origin without TLS. It takes at most MAX_LOG_STREAMS
 * sources. It returns the lines of the newest 1000 over all sources that
 * filter keeps, in the order they arrive, each with the pod of its source.
 * errors names each pod whose log broke or that the route refused, with
 * the text of the refusal. ended is true when each log ended or broke, and
 * false without a source. It does not connect again. A changed list of
 * sources starts with no lines.
 */
export function useLogStreams(
  sources: readonly LogStreamSource[],
  options: LogStreamOptions = {},
  filter: LogFilter = {},
): { lines: LogLine[]; errors: PodLogError[]; ended: boolean } {
  const { lines, streams } = useStreams(
    {
      ...(sources.length > 0 && { url: mergedURL(sources, options) }),
      merged: true,
      entries: sources.map((s) => ({
        pod: s.pod,
        ...(s.request !== undefined && { request: s.request }),
      })),
    },
    filter,
  );
  const errors = streams.flatMap((s, i) =>
    s.error === undefined ? [] : [{ pod: sources[i].pod, error: s.error }],
  );
  return {
    lines,
    errors,
    ended: streams.length > 0 && streams.every((s) => s.status !== "open"),
  };
}

/**
 * useLogStream follows the log of source through /api/logs and returns
 * the lines of the newest 1000 that filter keeps. ended is true when the
 * log ended. error is set when the stream broke or /api/logs refused it, with
 * the text of the refusal. The hook does not connect again. No source
 * opens no connection. A new source starts with no lines.
 */
export function useLogStream(
  source: LogSource | undefined,
  filter: LogFilter = {},
): { lines: LogLine[]; error?: string; ended: boolean } {
  const { lines, streams } = useStreams(
    {
      ...(source && { url: logURL(source) }),
      merged: false,
      entries: source ? [{ pod: source.pod }] : [],
    },
    filter,
  );
  const error = streams[0]?.error;
  return {
    lines,
    ended: streams[0]?.status === "ended",
    ...(error !== undefined && { error }),
  };
}

function useStreams(
  plan: Plan,
  filter: LogFilter,
): { lines: LogLine[]; streams: Stream[] } {
  const key = JSON.stringify(plan);
  const [state, setState] = useState<State>(() => fresh(key, plan));
  // A new key, also the key of earlier sources, starts with no lines.
  if (state.key !== key) setState(fresh(key, plan));

  useEffect(() => {
    const { url, merged, entries } = JSON.parse(key) as Plan;
    if (!url) return;
    const update = (change: (s: State) => Partial<State>) =>
      setState((s) => (s.key === key ? { ...s, ...change(s) } : s));
    return follow(url, merged, entries.length, {
      line(source, raw) {
        const { pod, request } = entries[source] ?? {};
        if (pod === undefined) return;
        const line = { ...parseLogLine(raw), pod };
        if (request && filterLines([line], { request }).length === 0) return;
        update((s) => ({ lines: [...s.lines, line].slice(-LIMIT) }));
      },
      end: (source) =>
        update((s) => ({
          streams: s.streams.map((old, i) =>
            i === source ? { status: "ended" } : old,
          ),
        })),
      broke: (source, error) =>
        update((s) => ({
          streams: s.streams.map((old, i) =>
            i === source ||
            (source === undefined &&
              (old.status === "open" || old.error === CONNECTION_BROKE))
              ? { status: "broke", error }
              : old,
          ),
        })),
    });
  }, [key]);

  // Until the reset above renders, state still holds the earlier sources.
  const current = state.key === key ? state : fresh(key, plan);
  const { level, text, request } = filter;
  const lines = useMemo(
    () => filterLines(current.lines, { level, text, request }),
    [current.lines, level, text, request],
  );
  return { lines, streams: current.streams };
}

function fresh(key: string, plan: Plan): State {
  return {
    key,
    lines: [],
    streams: plan.entries.map(() => ({ status: "open" })),
  };
}

// follow opens one connection for count logs and returns the function that
// closes it. An event of /api/logs/merged names its log by index, and an
// event of /api/logs is about log 0. broke without a log is about the
// connection, and can come twice for a connection that did not open: first
// with a fixed text, then with the text of the refusal.
function follow(
  url: string,
  merged: boolean,
  count: number,
  on: {
    line(source: number, raw: string): void;
    end(source: number): void;
    broke(source: number | undefined, error: string): void;
  },
): () => void {
  const events = new EventSource(url);
  const abort = new AbortController();
  const open = new Set(Array.from({ length: count }, (_, i) => i));
  let opened = false;
  const read = (event: Event) => JSON.parse((event as MessageEvent).data);
  const done = (source: number) => {
    open.delete(source);
    if (open.size === 0) events.close();
  };
  events.addEventListener("open", () => (opened = true));
  events.addEventListener("line", (event) => {
    const data = read(event);
    if (merged) on.line(data.source, data.line);
    else on.line(0, data);
  });
  events.addEventListener("end", (event) => {
    const source = merged ? read(event).source : 0;
    on.end(source);
    done(source);
  });
  events.addEventListener("failure", (event) => {
    const data = read(event);
    const source = merged ? data.source : 0;
    on.broke(source, data.message);
    done(source);
  });
  // EventSource connects again by itself, and the route would then send
  // the lines that the page already has.
  events.addEventListener("error", () => {
    events.close();
    if (open.size === 0) return;
    on.broke(undefined, CONNECTION_BROKE);
    if (opened) return;
    void refusal(url, abort.signal).then(
      (text) => text && on.broke(undefined, text),
    );
  });
  return () => {
    events.close();
    abort.abort();
  };
}

// EventSource does not give the text of an answer outside 200.
async function refusal(
  url: string,
  signal: AbortSignal,
): Promise<string | undefined> {
  try {
    const response = await fetch(url, { signal });
    if (response.ok) {
      await response.body?.cancel();
      return undefined;
    }
    return await response.text();
  } catch {
    return undefined;
  }
}

function mergedURL(
  sources: readonly LogStreamSource[],
  options: LogStreamOptions,
): string {
  const params = new URLSearchParams();
  for (const s of sources) {
    params.append(
      "log",
      [s.namespace, s.pod, ...(s.container ? [s.container] : [])].join("/"),
    );
  }
  for (const key of ["sinceSeconds", "tailLines"] as const) {
    if (options[key] !== undefined) params.set(key, String(options[key]));
  }
  return `/api/logs/merged?${params}`;
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
