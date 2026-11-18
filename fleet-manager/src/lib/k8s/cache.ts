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

/**
 * WATCH_RECOVERY_MS is the time that a kind has, after its watch failed, to
 * list again and start a new watch. After that time the cache has no current
 * list of the kind until it does.
 */
// After an error the cache lists again after 1, 2, and 4 seconds, so it gets
// over a dropped connection or a short restart of the API server within this
// time. A longer time lets a delete guard pass on a list that misses the
// changes of that time. A connection that goes silent counts as failed only
// when the client ends the watch for no data, after 30 seconds.
export const WATCH_RECOVERY_MS = 10_000;

/**
 * WATCH_TIMEOUT_SECONDS is the time after which the cache asks the API
 * server to end a watch. The server ends it with no error, and the cache
 * watches again from where it was, with no new list.
 */
// The client ends a watch that sent no data for 30 seconds, and the cache
// counts that as a failure and lists again. This value stays below it, so a
// watch of a kind that does not change ends cleanly first.
export const WATCH_TIMEOUT_SECONDS = 25;

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
  /** timeoutReconnectMs is the most a watch waits to start again after the API server ends it. */
  timeoutReconnectMs?: number;
  /** watchRecoveryMs defaults to WATCH_RECOVERY_MS. */
  watchRecoveryMs?: number;
  /** watchTimeoutSeconds defaults to WATCH_TIMEOUT_SECONDS. */
  watchTimeoutSeconds?: number;
}

export type CacheListener<K extends KindName> = (
  change: CacheChange,
  object: Resource<K>,
) => void;

/**
 * CacheLost gets the error of a kind that has no current list any more. The
 * subscription has then ended.
 */
export type CacheLost = (error: Error) => void;

/** Subscription is the objects of a kind at subscribe time and the way to stop the changes. */
export interface Subscription<K extends KindName> {
  snapshot: Resource<K>[];
  unsubscribe(): void;
}

/**
 * ResourceCache holds one informer for each kind that a reader asked for,
 * started by the first read of the kind. An informer lists the kind in every
 * namespace and keeps the list current from a watch. After a watch failed,
 * it lists again before it watches again, after a delay when the failure was
 * an error. A kind that does not list and watch again within watchRecoveryMs
 * is not current: its reads fail until it does. A Secret enters the cache
 * only as a SecretSummary.
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
      watchRecoveryMs: WATCH_RECOVERY_MS,
      watchTimeoutSeconds: WATCH_TIMEOUT_SECONDS,
      ...deps,
    };
  }

  /**
   * objects waits for the first list of kind and returns the cached objects.
   * It rejects at once when the API server refuses the first list, with a
   * message that names the kind, and after syncTimeoutMs when the list does
   * not answer. A later call tries again. It also rejects, with a message
   * that names the kind, while the kind is not current. The objects are the
   * cache itself: a caller must not change them.
   */
  async objects<K extends KindName>(kind: K): Promise<readonly Resource<K>[]> {
    const informer = this.informer(kind);
    await informer.synced();
    informer.checkCurrent();
    return informer.list() as unknown as readonly Resource<K>[];
  }

  /**
   * subscribe waits and rejects as objects does, then calls listener for
   * each change of kind until unsubscribe. No change falls between the
   * snapshot and the first call of listener. When the kind stops being
   * current, the subscription ends and lost gets the error, once.
   */
  async subscribe<K extends KindName>(
    kind: K,
    listener: CacheListener<K>,
    lost?: CacheLost,
  ): Promise<Subscription<K>> {
    const informer = this.informer(kind);
    await informer.synced();
    informer.checkCurrent();
    return informer.subscribe(
      listener as CacheListener<KindName>,
      lost,
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
  private recoveryTimer?: NodeJS.Timeout;
  private watchError: unknown;
  private current = true;
  // failing is set from a failure of the watch until a new list and a new
  // watch. listedAgain is set once that new list answered.
  private failing = false;
  private listedAgain = false;
  private readonly lostHandlers = new Set<CacheLost>();

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
      if (this.failing) this.listedAgain = true;
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
      ) => {
        // ListWatch does not check that it stopped after the first list or
        // after the reconnect delay, so a stopped informer opens no watch.
        if (this.stopped) return Promise.resolve(new AbortController());
        // A watch that the API server accepts after a failure does not show
        // what changed in the meantime. 410 Gone makes ListWatch list again
        // first. It must come after this request is in place, because
        // ListWatch keeps the request that it got last.
        if (this.failing && !this.listedAgain) {
          setTimeout(() => {
            void (done(GONE) as unknown as Promise<void>).catch((e: unknown) =>
              this.onError(e),
            );
          });
          return Promise.resolve(new AbortController());
        }
        // The client ends a watch that the API server refuses before the
        // request resolves, and a watch that it accepted only after.
        let ended = false;
        return deps.watch
          .watch(
            watchPath,
            {
              ...params,
              timeoutSeconds: deps.watchTimeoutSeconds,
              allowWatchBookmarks: true,
            },
            (phase, object, raw) => {
              // The API server ends a watch after an ERROR event.
              if (phase === "ERROR") this.watchFailed(object);
              callback(
                phase,
                ["ADDED", "MODIFIED", "DELETED"].includes(phase)
                  ? shape(object)
                  : object,
                raw,
              );
            },
            // done of ListWatch is async and the client does not await it, so
            // a watch that cannot start again would reject with no handler.
            (err) => {
              // ListWatch lists again after 410 Gone and watches again at
              // once after a TimeoutError, and gives no error for either.
              if (err) this.watchFailed(err);
              ended = true;
              void (done(err) as unknown as Promise<void>).catch((e: unknown) =>
                this.onError(e),
              );
            },
          )
          .then((request) => {
            if (this.stopped) request.abort();
            else if (!ended && this.listedAgain) this.recovered();
            return request;
          });
      },
    } as unknown as Watch;

    // The API server ends each watch after its own timeout, with no error.
    // ListWatch then waits a delay that doubles up to 30 seconds, and only a
    // watch event resets it, so a quiet kind would be unwatched for up to 30
    // seconds after each end. A request error other than 410 Gone goes to
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

  subscribe(
    listener: CacheListener<KindName>,
    lost?: CacheLost,
  ): Subscription<KindName> {
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
    const unsubscribe = () => {
      this.lostHandlers.delete(end);
      for (const [verb, handler] of Object.entries(handlers)) {
        this.listWatch.off(verb as CacheChange, handler);
      }
    };
    const end: CacheLost = (error) => {
      unsubscribe();
      lost?.(error);
    };
    this.lostHandlers.add(end);
    return {
      snapshot: [...this.listWatch.list()] as Resource<KindName>[],
      unsubscribe,
    };
  }

  /** checkCurrent throws the error of the kind while it is not current. */
  checkCurrent(): void {
    if (!this.current) throw this.notCurrentError();
  }

  async stop(): Promise<void> {
    this.stopped = true;
    clearTimeout(this.retryTimer);
    clearTimeout(this.recoveryTimer);
    await this.listWatch.stop();
  }

  private watchFailed(err: unknown): void {
    this.watchError = err;
    this.failing = true;
    this.listedAgain = false;
    if (this.recoveryTimer || !this.current || this.stopped) return;
    this.recoveryTimer = setTimeout(() => {
      this.recoveryTimer = undefined;
      this.current = false;
      const error = this.notCurrentError();
      [...this.lostHandlers].forEach((end) => end(error));
    }, this.deps.watchRecoveryMs);
  }

  private recovered(): void {
    clearTimeout(this.recoveryTimer);
    this.recoveryTimer = undefined;
    this.failing = false;
    this.listedAgain = false;
    this.current = true;
    this.retryDelay = this.deps.retryDelayMs;
  }

  private notCurrentError(): Error {
    const seconds = this.deps.watchRecoveryMs / 1000;
    const reason =
      (this.watchError as { message?: string } | undefined)?.message ??
      String(this.watchError);
    return new Error(
      `The fleet manager has no current list of ${this.kind}: its watch failed and did not start again within ${seconds} seconds: ${reason}`,
      { cause: this.watchError },
    );
  }

  private async start(): Promise<void> {
    // listed stays true from an earlier start whose watch did not start:
    // ListWatch keeps that list and watches without a new one.
    try {
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
    this.watchFailed(err);
    clearTimeout(this.retryTimer);
    this.retryTimer = setTimeout(
      () => this.listWatch.start().catch((e: unknown) => this.onError(e)),
      this.retryDelay,
    );
    this.retryDelay = Math.min(this.retryDelay * 2, this.deps.maxRetryDelayMs);
  }
}

class SyncTimeoutError extends Error {}

const GONE = { statusCode: 410 };

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
