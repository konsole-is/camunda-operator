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
 * LogStreamSource is a LogSource of useLogStreams. With request, the
 * stream keeps only the lines of that request, "namespace/name".
 */
export type LogStreamSource = LogSource & { request?: string };

/** PodLogError is the error of the log stream of one pod. */
export type PodLogError = { pod: string; error: string };

const LIMIT = 1000;

type Stream = { status: "open" | "ended" | "broke"; error?: string };
type State = { key: string; lines: LogLine[]; streams: Stream[] };
type Entry = { url: string; pod: string; request?: string };

/**
 * useLogStreams follows the log of each source through /api/logs and
 * returns the lines of the newest 1000 over all sources that filter keeps,
 * in the order they arrive. Each line has the pod of its source. errors
 * names each pod whose stream broke or that /api/logs refused, with the
 * text of the refusal. ended is true when each stream ended or broke, and
 * false without a source. A stream does not connect again. A changed list
 * of sources starts with no lines.
 */
export function useLogStreams(
  sources: readonly LogStreamSource[],
  filter: LogFilter = {},
): { lines: LogLine[]; errors: PodLogError[]; ended: boolean } {
  const { lines, streams, pods } = useStreams(sources, filter);
  const errors = streams.flatMap((s, i) =>
    s.error === undefined ? [] : [{ pod: pods[i], error: s.error }],
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
  const { lines, streams } = useStreams(source ? [source] : [], filter);
  const error = streams[0]?.error;
  return {
    lines,
    ended: streams[0]?.status === "ended",
    ...(error !== undefined && { error }),
  };
}

function useStreams(
  sources: readonly LogStreamSource[],
  filter: LogFilter,
): { lines: LogLine[]; streams: Stream[]; pods: string[] } {
  const entries: Entry[] = sources.map((s) => ({
    url: logURL(s),
    pod: s.pod,
    ...(s.request !== undefined && { request: s.request }),
  }));
  const key = JSON.stringify(entries);
  const [state, setState] = useState<State>(() => fresh(key, entries));
  // A new key, also the key of earlier sources, starts with no lines.
  if (state.key !== key) setState(fresh(key, entries));

  useEffect(() => {
    const list = JSON.parse(key) as Entry[];
    const update = (change: (s: State) => Partial<State>) =>
      setState((s) => (s.key === key ? { ...s, ...change(s) } : s));
    const mark = (index: number, stream: Stream) =>
      update((s) => ({
        streams: s.streams.map((old, i) => (i === index ? stream : old)),
      }));
    const stops = list.map(({ url, pod, request }, index) =>
      follow(url, {
        line(raw) {
          const line = { ...parseLogLine(raw), pod };
          if (request && filterLines([line], { request }).length === 0) return;
          update((s) => ({ lines: [...s.lines, line].slice(-LIMIT) }));
        },
        end: () => mark(index, { status: "ended" }),
        broke: (error) => mark(index, { status: "broke", error }),
      }),
    );
    return () => stops.forEach((stop) => stop());
  }, [key]);

  const { level, text, request } = filter;
  const lines = useMemo(
    () => filterLines(state.lines, { level, text, request }),
    [state.lines, level, text, request],
  );
  return { lines, streams: state.streams, pods: entries.map((e) => e.pod) };
}

function fresh(key: string, entries: readonly Entry[]): State {
  return {
    key,
    lines: [],
    streams: entries.map(() => ({ status: "open" })),
  };
}

// follow opens one stream and returns the function that closes it. broke
// can come twice for a stream that did not open: first with a fixed text,
// then with the text of the refusal of /api/logs.
function follow(
  url: string,
  on: { line(raw: string): void; end(): void; broke(error: string): void },
): () => void {
  const events = new EventSource(url);
  const abort = new AbortController();
  let opened = false;
  events.addEventListener("open", () => (opened = true));
  events.addEventListener("line", (event) =>
    on.line(JSON.parse((event as MessageEvent).data)),
  );
  events.addEventListener("end", () => {
    events.close();
    on.end();
  });
  events.addEventListener("failure", (event) => {
    events.close();
    on.broke(JSON.parse((event as MessageEvent).data).message);
  });
  // EventSource connects again by itself, and the route would then send
  // the lines that the page already has.
  events.addEventListener("error", () => {
    events.close();
    on.broke("The connection to the log stream broke");
    if (opened) return;
    void refusal(url, abort.signal).then((text) => text && on.broke(text));
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
