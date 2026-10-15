import "server-only";
import {
  ListWatch,
  Watch,
  type KubernetesListObject,
  type KubernetesObject,
} from "@kubernetes/client-node";
import { getJSON, KubernetesRequestError, kubeConfig } from "./client";
import type { CacheChange } from "./deltas";
import {
  apiVersionOf,
  collectionPath,
  KindMap,
  type KindName,
  type Resource,
} from "./kinds";
import { stripSecret } from "./strip-secret";

/** SYNC_TIMEOUT_MS bounds the wait of a reader for the first list of a kind. */
export const SYNC_TIMEOUT_MS = 10_000;

/** CacheDeps are the API calls of the cache. Tests pass fakes. */
export interface CacheDeps {
  /** list returns all objects at a collection path. */
  list(path: string): Promise<KubernetesListObject<KubernetesObject>>;
  watch: Watch;
  /** syncTimeoutMs defaults to SYNC_TIMEOUT_MS. */
  syncTimeoutMs?: number;
  /** retryDelayMs is the first wait before a watch restarts after an error. It doubles up to maxRetryDelayMs. */
  retryDelayMs?: number;
  maxRetryDelayMs?: number;
  /** timeoutReconnectMs is the most a watch waits to start again after its request times out. */
  timeoutReconnectMs?: number;
}

export type CacheListener<K extends KindName> = (
  change: CacheChange,
  object: Resource<K>,
) => void;

/** Subscription is the objects of a kind at subscribe time and the way to stop the changes. */
export interface Subscription<K extends KindName> {
  snapshot: Resource<K>[];
  unsubscribe(): void;
}

/**
 * ResourceCache holds one informer for each kind that a reader asked for,
 * started by the first read of the kind. An informer lists the kind in every
 * namespace and keeps the list current from a watch. After 410 Gone it lists
 * again, and after another watch error it watches again after a delay. A
 * Secret enters the cache only as a SecretSummary.
 */
export class ResourceCache {
  private readonly informers = new Map<KindName, KindInformer>();
  private readonly deps: Required<CacheDeps>;

  constructor(deps: CacheDeps) {
    this.deps = {
      syncTimeoutMs: SYNC_TIMEOUT_MS,
      retryDelayMs: 1_000,
      maxRetryDelayMs: 30_000,
      timeoutReconnectMs: 1_000,
      ...deps,
    };
  }

  /**
   * objects waits for the first list of kind and returns the cached objects.
   * It rejects at once when the API server refuses the first list, with a
   * message that names the kind, and after syncTimeoutMs when the list does
   * not answer. A later call tries again. The objects are the cache itself:
   * a caller must not change them.
   */
  async objects<K extends KindName>(kind: K): Promise<readonly Resource<K>[]> {
    const informer = this.informer(kind);
    await informer.synced();
    return informer.list() as unknown as readonly Resource<K>[];
  }

  /**
   * subscribe waits as objects does, then calls listener for each change of
   * kind until unsubscribe. No change falls between the snapshot and the
   * first call of listener.
   */
  async subscribe<K extends KindName>(
    kind: K,
    listener: CacheListener<K>,
  ): Promise<Subscription<K>> {
    const informer = this.informer(kind);
    await informer.synced();
    return informer.subscribe(
      listener as CacheListener<KindName>,
    ) as unknown as Subscription<K>;
  }

  /** stop ends every watch of the cache. */
  async stop(): Promise<void> {
    await Promise.all([...this.informers.values()].map((i) => i.stop()));
    this.informers.clear();
  }

  private informer(kind: KindName): KindInformer {
    let informer = this.informers.get(kind);
    if (!informer) {
      informer = new KindInformer(kind, this.deps);
      this.informers.set(kind, informer);
    }
    return informer;
  }
}

class KindInformer {
  private readonly listWatch: ListWatch<KubernetesObject>;
  private starting?: Promise<void>;
  private hasSynced = false;
  private listed = false;
  private listError: unknown;
  private retryDelay: number;
  private retryTimer?: NodeJS.Timeout;
  private stopped = false;

  constructor(
    private readonly kind: KindName,
    private readonly deps: Required<CacheDeps>,
  ) {
    const info = KindMap[kind];
    const path = collectionPath(info);
    const shape = (object: KubernetesObject): KubernetesObject =>
      kind === "Secret" ? (stripSecret(object) as KubernetesObject) : object;
    this.retryDelay = deps.retryDelayMs;

    // The items of a list have no apiVersion and kind. Watch events have them.
    // A list that does not answer fails, so that the next read starts a new
    // one instead of waiting on it.
    const listFn = async () => {
      const list = await withTimeout(deps.list(path), deps.syncTimeoutMs, () =>
        this.syncTimeout(),
      );
      this.listed = true;
      this.retryDelay = deps.retryDelayMs;
      return {
        ...list,
        items: (list.items ?? []).map((o) =>
          shape({ ...o, apiVersion: apiVersionOf(info), kind }),
        ),
      };
    };
    const watch = {
      watch: (
        watchPath: string,
        params: Record<string, string | number | boolean | undefined>,
        callback: (
          phase: string,
          object: KubernetesObject,
          raw?: unknown,
        ) => void,
        done: (err: unknown) => void,
      ) =>
        deps.watch.watch(
          watchPath,
          params,
          (phase, object, raw) =>
            callback(
              phase,
              ["ADDED", "MODIFIED", "DELETED"].includes(phase)
                ? shape(object)
                : object,
              raw,
            ),
          done,
        ),
    } as unknown as Watch;

    // The client ends each watch after a request timeout of 30 seconds, also
    // when nothing changed. ListWatch then waits a delay that doubles up to 30
    // seconds, and a watch event resets it, so a quiet kind would be
    // unwatched half of the time. A request error other than 410 Gone goes to
    // onError instead, without this delay.
    const delayFn = (ms: number) =>
      new Promise<void>((resolve) =>
        setTimeout(resolve, Math.min(ms, deps.timeoutReconnectMs)),
      );
    this.listWatch = new ListWatch(
      path,
      watch,
      listFn,
      false,
      undefined,
      undefined,
      { delayFn },
    );
    this.listWatch.on("error", (err) => this.onError(err));
  }

  synced(): Promise<void> {
    if (this.hasSynced) return Promise.resolve();
    this.starting ??= this.start();
    return withTimeout(this.starting, this.deps.syncTimeoutMs, () =>
      this.syncTimeout(),
    );
  }

  private syncTimeout(): SyncTimeoutError {
    const seconds = this.deps.syncTimeoutMs / 1000;
    return new SyncTimeoutError(
      `The fleet manager waited ${seconds} seconds for the first list of ${this.kind} from the API server and got no answer`,
    );
  }

  list(): readonly KubernetesObject[] {
    return this.listWatch.list();
  }

  subscribe(listener: CacheListener<KindName>): Subscription<KindName> {
    const on = (change: CacheChange) => (object: KubernetesObject) =>
      listener(change, object as Resource<KindName>);
    const handlers = {
      add: on("add"),
      update: on("update"),
      delete: on("delete"),
    };
    for (const [verb, handler] of Object.entries(handlers)) {
      this.listWatch.on(verb as CacheChange, handler);
    }
    return {
      snapshot: [...this.listWatch.list()] as Resource<KindName>[],
      unsubscribe: () => {
        for (const [verb, handler] of Object.entries(handlers)) {
          this.listWatch.off(verb as CacheChange, handler);
        }
      },
    };
  }

  async stop(): Promise<void> {
    this.stopped = true;
    clearTimeout(this.retryTimer);
    await this.listWatch.stop();
  }

  private async start(): Promise<void> {
    try {
      this.listed = false;
      this.listError = undefined;
      await this.listWatch.start();
      if (!this.listed) throw listFailure(this.kind, this.listError);
      this.hasSynced = true;
    } finally {
      this.starting = undefined;
    }
  }

  // ListWatch stops after an error that is not 410 Gone. Before the first
  // list the error goes to the waiting readers. After it the watch starts
  // again from the last resourceVersion.
  private onError(err: unknown): void {
    if (!this.listed) {
      this.listError = err;
      return;
    }
    if (this.stopped) return;
    clearTimeout(this.retryTimer);
    this.retryTimer = setTimeout(
      () => this.listWatch.start().catch((e: unknown) => this.onError(e)),
      this.retryDelay,
    );
    this.retryDelay = Math.min(this.retryDelay * 2, this.deps.maxRetryDelayMs);
  }
}

class SyncTimeoutError extends Error {}

function listFailure(kind: KindName, err: unknown): Error {
  if (err instanceof SyncTimeoutError) return err;
  const info = KindMap[kind];
  const where = `${apiVersionOf(info)}, resource ${info.plural}`;
  if (err instanceof KubernetesRequestError && err.statusCode === 404) {
    return new Error(
      `The API server does not serve ${kind} (${where}). Install the CRD of ${kind}, or check that the API server serves this version.`,
    );
  }
  const reason = err instanceof Error ? err.message : String(err);
  return new Error(
    `The fleet manager could not list ${kind} (${where}): ${reason}`,
    { cause: err },
  );
}

function withTimeout<T>(
  promise: Promise<T>,
  ms: number,
  error: () => Error,
): Promise<T> {
  let timer: NodeJS.Timeout | undefined;
  const timeout = new Promise<never>((_, reject) => {
    timer = setTimeout(() => reject(error()), ms);
  });
  return Promise.race([promise, timeout]).finally(() => clearTimeout(timer));
}

const CACHE_KEY: unique symbol = Symbol.for("fleet-manager.k8s.cache");

/**
 * defaultCache returns the cache of the server process, with the API server
 * of kubeConfig. Next.js can load this module once for each route, so the
 * cache lives on globalThis and every route shares its informers.
 */
export function defaultCache(): ResourceCache {
  const store = globalThis as typeof globalThis & {
    [CACHE_KEY]?: ResourceCache;
  };
  store[CACHE_KEY] ??= new ResourceCache({
    list: (path) => getJSON(kubeConfig(), path, SYNC_TIMEOUT_MS),
    watch: new Watch(kubeConfig()),
  });
  return store[CACHE_KEY];
}
