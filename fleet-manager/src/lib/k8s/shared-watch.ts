"use client";
// The one connection to /api/watch of a QueryClient. It imports only types
// from the server modules of src/lib/k8s, and the isomorphic deltas module.
import type { QueryClient } from "@tanstack/react-query";
import {
  applyDelta,
  type NamedObject,
  type WatchDelta,
  type WatchFailure,
  type WatchSnapshot,
} from "./deltas";
import type { KindName } from "./kinds";

/** WatchSpec is one watch: a kind and the filter of a hook. */
export interface WatchSpec {
  kind: KindName;
  namespace?: string;
  labelSelector?: string;
  name?: string;
}

/** watchKey is the query key of the list of a watch. */
export function watchKey(spec: WatchSpec) {
  return [
    "k8s",
    spec.kind,
    spec.namespace ?? "",
    spec.labelSelector ?? "",
    spec.name ?? "",
  ] as const;
}

/**
 * failureKey is the query key that holds the message of the last failure of
 * a watch, or null when its last frame was a snapshot.
 */
export function failureKey(spec: WatchSpec) {
  return [
    "k8s-failure",
    spec.kind,
    spec.namespace ?? "",
    spec.labelSelector ?? "",
    spec.name ?? "",
  ] as const;
}

/** watchId is the value of the parameter watch of /api/watch for spec. */
export function watchId(spec: WatchSpec): string {
  const params = new URLSearchParams({ kind: spec.kind });
  if (spec.namespace) params.set("namespace", spec.namespace);
  if (spec.labelSelector) params.set("labelSelector", spec.labelSelector);
  if (spec.name) params.set("name", spec.name);
  return params.toString();
}

/**
 * REOPEN_DELAY_MS is the wait before a new connection after the browser gave
 * up on the old one. It is the retry time that /api/watch sends.
 */
export const REOPEN_DELAY_MS = 5_000;

/**
 * SharedWatch keeps one EventSource to /api/watch for all watches of its
 * QueryClient.
 */
export interface SharedWatch {
  /**
   * subscribe adds one subscriber to the watch of spec. Subscribers of an
   * equal spec share one watch, which is the only writer of its entries at
   * watchKey and failureKey. It writes nothing before the first snapshot, so
   * the initial data of a query stays. The function it returns removes the
   * subscriber once, also when it is called again. A change of the set of
   * watches opens a new connection in a microtask, so the subscribes of one
   * render share one connection. A watch without subscribers leaves its list
   * in the cache and sets its failure to null.
   */
  subscribe(spec: WatchSpec): () => void;
}

const watches = new WeakMap<QueryClient, SharedWatch>();

/** sharedWatch returns the one SharedWatch of client. */
export function sharedWatch(client: QueryClient): SharedWatch {
  let watch = watches.get(client);
  if (!watch) {
    watch = new Connection(client);
    watches.set(client, watch);
  }
  return watch;
}

class Connection implements SharedWatch {
  private readonly watches = new Map<
    string,
    { spec: WatchSpec; count: number }
  >();
  private source?: EventSource;
  private openIds: string[] = [];
  private reopen?: ReturnType<typeof setTimeout>;
  private scheduled = false;

  constructor(private readonly client: QueryClient) {}

  subscribe(spec: WatchSpec): () => void {
    const id = watchId(spec);
    const watch = this.watches.get(id) ?? { spec, count: 0 };
    watch.count++;
    this.watches.set(id, watch);
    this.schedule();
    let left = false;
    return () => {
      if (left) return;
      left = true;
      watch.count--;
      this.schedule();
    };
  }

  // A hook that unmounts and mounts again in one tick, as React Strict Mode
  // does, keeps its connection and its failure, because the microtask runs
  // after both.
  private schedule() {
    if (this.scheduled) return;
    this.scheduled = true;
    queueMicrotask(() => {
      this.scheduled = false;
      for (const [id, watch] of this.watches) {
        if (watch.count > 0) continue;
        this.watches.delete(id);
        this.client.setQueryData(failureKey(watch.spec), null);
      }
      const ids = [...this.watches.keys()].sort();
      if (ids.join("\n") !== this.openIds.join("\n")) this.open(ids);
    });
  }

  private open(ids: string[]) {
    this.source?.close();
    clearTimeout(this.reopen);
    this.openIds = ids;
    this.source = undefined;
    if (ids.length === 0) return;
    const params = new URLSearchParams(ids.map((id) => ["watch", id]));
    const source = new EventSource(`/api/watch?${params}`);
    this.source = source;
    // A closed source can still deliver frames that were in flight. Only the
    // open source writes, so a late frame cannot bring back an old state.
    const on = <T extends { watch: string }>(
      event: string,
      apply: (spec: WatchSpec, frame: T) => void,
    ) =>
      source.addEventListener(event, (message) => {
        if (this.source !== source) return;
        const frame = JSON.parse((message as MessageEvent<string>).data) as T;
        const watch = this.watches.get(frame.watch);
        if (watch) apply(watch.spec, frame);
      });
    on<WatchSnapshot>("snapshot", (spec, frame) => {
      this.client.setQueryData(watchKey(spec), frame.objects);
      this.client.setQueryData(failureKey(spec), null);
    });
    on<WatchDelta>("delta", (spec, frame) => {
      this.client.setQueryData<NamedObject[]>(watchKey(spec), (list = []) =>
        applyDelta(list, frame),
      );
    });
    on<WatchFailure>("failure", (spec, frame) => {
      this.client.setQueryData(failureKey(spec), frame.message);
    });
    // EventSource does not connect again after an HTTP error status or a
    // body that is not text/event-stream. It then has readyState CLOSED.
    source.addEventListener("error", () => {
      if (this.source !== source || source.readyState !== EventSource.CLOSED) {
        return;
      }
      this.reopen = setTimeout(() => this.open(this.openIds), REOPEN_DELAY_MS);
    });
  }
}
