import "server-only";
import type { KubernetesObject } from "@kubernetes/client-node";
import type { ResourceCache } from "./cache";
import { deltaType, objectKey, type CacheChange } from "./deltas";
import { isKindName, KindMap, type KindName } from "./kinds";
import { parseSelector } from "./selector";

/** WatchRequest is what one SSE connection watches. */
export interface WatchRequest {
  kinds: KindName[];
  namespace?: string;
  labelSelector?: string;
  /** name limits the watch to one object, for useResource. */
  name?: string;
}

/** WatchRequestError is a request that /api/watch refuses with 400. */
export class WatchRequestError extends Error {}

/**
 * parseWatchRequest reads the parameters kind (one or more), namespace,
 * labelSelector, and name. It throws WatchRequestError with the problem for
 * no kind, an unknown kind, a namespace for a cluster-scoped kind, or a
 * selector that is not valid.
 */
export function parseWatchRequest(params: URLSearchParams): WatchRequest {
  const kinds = params.getAll("kind");
  const namespace = params.get("namespace") || undefined;
  const labelSelector = params.get("labelSelector") || undefined;
  const name = params.get("name") || undefined;
  if (kinds.length === 0) {
    throw new WatchRequestError(
      "Name at least one kind with the parameter kind",
    );
  }
  for (const kind of kinds) {
    if (!isKindName(kind)) {
      throw new WatchRequestError(
        `The fleet manager does not read the kind "${kind}"`,
      );
    }
    if (namespace && KindMap[kind].scope === "Cluster") {
      throw new WatchRequestError(
        `${kind} is cluster-scoped, so a watch of it takes no namespace`,
      );
    }
  }
  try {
    parseSelector(labelSelector ?? "");
  } catch (error) {
    throw new WatchRequestError((error as Error).message);
  }
  return { kinds: kinds as KindName[], namespace, labelSelector, name };
}

/**
 * watchStream returns the SSE body of one connection. For each kind it sends
 * a frame "snapshot" with the matching objects, then a frame "delta" for each
 * change that a subscriber of the filter sees (see deltaType). A kind that
 * does not sync gets a frame "failure" with the message. A comment line goes
 * out every heartbeatMs, so idle proxies keep the connection. The stream ends
 * when signal aborts.
 */
export function watchStream(
  cache: ResourceCache,
  request: WatchRequest,
  signal: AbortSignal,
  heartbeatMs = 25_000,
): ReadableStream<Uint8Array> {
  const encoder = new TextEncoder();
  const matchesLabels = parseSelector(request.labelSelector ?? "");
  const inFilter = (o: KubernetesObject) =>
    (!request.namespace || o.metadata?.namespace === request.namespace) &&
    (!request.name || o.metadata?.name === request.name) &&
    matchesLabels(o.metadata?.labels);
  const unsubscribes: (() => void)[] = [];
  let heartbeat: NodeJS.Timeout | undefined;
  let closed = false;

  const close = () => {
    closed = true;
    clearInterval(heartbeat);
    unsubscribes.splice(0).forEach((unsubscribe) => unsubscribe());
  };

  return new ReadableStream<Uint8Array>({
    async start(controller) {
      const write = (text: string) => {
        if (!closed) controller.enqueue(encoder.encode(text));
      };
      const send = (event: string, data: unknown) =>
        write(`event: ${event}\ndata: ${JSON.stringify(data)}\n\n`);
      signal.addEventListener(
        "abort",
        () => {
          close();
          try {
            controller.close();
          } catch {
            // cancel already closed the stream.
          }
        },
        { once: true },
      );
      if (signal.aborted) return;

      for (const kind of request.kinds) {
        const seen = new Set<string>();
        // Changes that arrive before the snapshot is sent wait here, so the
        // snapshot always comes first.
        let queue: [CacheChange, KubernetesObject][] | undefined = [];
        const deliver = (change: CacheChange, object: KubernetesObject) => {
          const type = deltaType(
            seen,
            change,
            objectKey(object),
            inFilter(object),
          );
          if (type) send("delta", { type, kind, object });
        };
        try {
          const { snapshot, unsubscribe } = await cache.subscribe(
            kind,
            (change, object) => {
              if (queue) queue.push([change, object as KubernetesObject]);
              else deliver(change, object as KubernetesObject);
            },
          );
          if (closed) {
            unsubscribe();
            return;
          }
          unsubscribes.push(unsubscribe);
          const objects = (snapshot as KubernetesObject[]).filter(inFilter);
          objects.forEach((o) => seen.add(objectKey(o)));
          send("snapshot", { kind, objects });
          const waiting = queue;
          queue = undefined;
          waiting.forEach(([change, object]) => deliver(change, object));
        } catch (error) {
          send("failure", {
            kind,
            message: error instanceof Error ? error.message : String(error),
          });
        }
      }
      if (!closed) {
        heartbeat = setInterval(() => write(": keep-alive\n\n"), heartbeatMs);
      }
    },
    cancel() {
      close();
    },
  });
}
