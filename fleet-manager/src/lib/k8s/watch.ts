import "server-only";
import type { KubernetesObject } from "@kubernetes/client-node";
import type { ResourceCache } from "./cache";
import { deltaType, objectKey, type CacheChange } from "./deltas";
import { isKindName, KindMap, type KindName } from "./kinds";
import { parseSelector } from "./selector";

/** WatchRequest is one watch of an SSE connection. */
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
 * no kind, an unknown kind, a namespace for a cluster-scoped kind, a name of
 * a namespaced kind without a namespace, or a selector that is not valid.
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
    if (name && !namespace && KindMap[kind].scope === "Namespaced") {
      throw new WatchRequestError(
        `${kind} is namespaced, so a watch of one name needs a namespace`,
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

/** WatchEntry is one watch of a connection. id is the raw value of its parameter watch. */
export interface WatchEntry {
  id: string;
  request: WatchRequest;
}

/**
 * parseWatchRequests reads each value of the parameter watch as the query
 * string of one watch, with the rules of parseWatchRequest. It throws
 * WatchRequestError for no watch, for a value that appears twice, and with
 * the message of parseWatchRequest for a watch that is not valid.
 */
export function parseWatchRequests(params: URLSearchParams): WatchEntry[] {
  const ids = params.getAll("watch");
  if (ids.length === 0) {
    throw new WatchRequestError(
      "Name at least one watch with the parameter watch",
    );
  }
  const seen = new Set<string>();
  return ids.map((id) => {
    if (seen.has(id)) {
      throw new WatchRequestError(`The watch "${id}" appears more than once`);
    }
    seen.add(id);
    return { id, request: parseWatchRequest(new URLSearchParams(id)) };
  });
}

export interface WatchStreamOptions {
  heartbeatMs?: number;
  retryMs?: number;
  maxPendingBytes?: number;
}

/**
 * watchStream returns the SSE body of one connection that serves watches.
 * Every frame has the field watch, the id of the entry that it answers. The
 * watches start at once. For each kind of a watch the stream sends a frame
 * "snapshot" with the matching objects, then a frame "delta" for each change
 * that a subscriber of the filter sees (see deltaType). A kind that does not
 * sync gets a frame "failure" with the message, and the later kinds of that
 * watch do not start. The frame sets the retry time of EventSource to
 * retryMs. When a watch failed, the stream ends after every watch sent its
 * snapshots or its failure. A comment line goes out every heartbeatMs, so
 * idle proxies keep the connection. The stream also ends when signal aborts,
 * and at the next write when more than maxPendingBytes wait unread in its
 * queue.
 */
export function watchStream(
  cache: ResourceCache,
  watches: WatchEntry[],
  signal: AbortSignal,
  {
    heartbeatMs = 25_000,
    retryMs = 5_000,
    maxPendingBytes = 8 * 1024 * 1024,
  }: WatchStreamOptions = {},
): ReadableStream<Uint8Array> {
  const encoder = new TextEncoder();
  const unsubscribes: (() => void)[] = [];
  let heartbeat: NodeJS.Timeout | undefined;
  let closed = false;

  const close = () => {
    closed = true;
    clearInterval(heartbeat);
    unsubscribes.splice(0).forEach((unsubscribe) => unsubscribe());
  };

  return new ReadableStream<Uint8Array>(
    {
      async start(controller) {
        const end = () => {
          close();
          try {
            controller.close();
          } catch {
            // cancel already closed the stream.
          }
        };
        // The shared informer does not wait for its listener, so a write
        // cannot wait for a slow client. desiredSize is below 0 when more
        // than maxPendingBytes wait in the queue.
        const write = (text: string) => {
          if (closed) return;
          if ((controller.desiredSize ?? 0) < 0) {
            end();
            return;
          }
          controller.enqueue(encoder.encode(text));
        };
        const send = (event: string, data: unknown, fields = "") =>
          write(`${fields}event: ${event}\ndata: ${JSON.stringify(data)}\n\n`);
        signal.addEventListener("abort", end, { once: true });
        if (signal.aborted) {
          end();
          return;
        }

        // runWatch resolves false after it sent a failure frame.
        const runWatch = async ({ id, request }: WatchEntry) => {
          const matchesLabels = parseSelector(request.labelSelector ?? "");
          const inFilter = (o: KubernetesObject) =>
            (!request.namespace ||
              o.metadata?.namespace === request.namespace) &&
            (!request.name || o.metadata?.name === request.name) &&
            matchesLabels(o.metadata?.labels);
          for (const kind of request.kinds) {
            const seen = new Set<string>();
            // Changes that arrive before the snapshot is sent wait here, so
            // the snapshot always comes first.
            let queue: [CacheChange, KubernetesObject][] | undefined = [];
            const deliver = (change: CacheChange, object: KubernetesObject) => {
              const type = deltaType(
                seen,
                change,
                objectKey(object),
                inFilter(object),
              );
              if (type) send("delta", { watch: id, type, kind, object });
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
                return true;
              }
              unsubscribes.push(unsubscribe);
              const objects = (snapshot as KubernetesObject[]).filter(inFilter);
              objects.forEach((o) => seen.add(objectKey(o)));
              send("snapshot", { watch: id, kind, objects });
              const waiting = queue;
              queue = undefined;
              waiting.forEach(([change, object]) => deliver(change, object));
            } catch (error) {
              send(
                "failure",
                {
                  watch: id,
                  kind,
                  message:
                    error instanceof Error ? error.message : String(error),
                },
                `retry: ${retryMs}\n`,
              );
              return false;
            }
          }
          return true;
        };

        const results = await Promise.all(watches.map(runWatch));
        if (closed) return;
        if (results.includes(false)) {
          // EventSource connects again after the retry of the failure frame.
          end();
          return;
        }
        heartbeat = setInterval(() => write(": keep-alive\n\n"), heartbeatMs);
      },
      cancel() {
        close();
      },
    },
    new ByteLengthQueuingStrategy({ highWaterMark: maxPendingBytes }),
  );
}
