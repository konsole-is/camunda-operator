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

/**
 * WatchEntry is one watch of a connection. id is the raw value of its
 * parameter watch. A watch that is not valid has error in place of request.
 */
export type WatchEntry =
  { id: string; request: WatchRequest } | { id: string; error: string };

/**
 * parseWatchRequests reads each value of the parameter watch as the query
 * string of one watch, with the rules of parseWatchRequest. A watch that is
 * not valid gets the message of parseWatchRequest as its error. It throws
 * WatchRequestError for no watch and for a value that appears twice.
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
    try {
      return { id, request: parseWatchRequest(new URLSearchParams(id)) };
    } catch (error) {
      if (!(error instanceof WatchRequestError)) throw error;
      return { id, error: error.message };
    }
  });
}

export interface WatchStreamOptions {
  heartbeatMs?: number;
  retryMs?: number;
  maxPendingBytes?: number;
}

/**
 * watchStream returns the SSE body of one connection. Every frame has the
 * field watch, the id of the entry that it answers. Each kind of a watch gets
 * a frame "snapshot" with the matching objects, then a frame "delta" for each
 * change that a subscriber of the filter sees (see deltaType). A watch that
 * is not valid gets a frame "failure" with its error. A kind that does not
 * sync gets a frame "failure" with the message and tries again retryMs later.
 * A failure does not end the stream. The stream ends when signal aborts, and
 * when a client that does not read falls more than maxPendingBytes behind.
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
  const drained: (() => void)[] = [];
  const stops = new Set<() => void>();
  // queuedBytes counts the changes of all watches that wait behind their
  // snapshot. They are not in the stream queue yet, so they count here.
  let queuedBytes = 0;
  let heartbeat: NodeJS.Timeout | undefined;
  let closed = false;

  const close = () => {
    closed = true;
    clearInterval(heartbeat);
    unsubscribes.splice(0).forEach((unsubscribe) => unsubscribe());
    drained.splice(0).forEach((wake) => wake());
    stops.forEach((stop) => stop());
  };
  const pause = (ms: number) =>
    new Promise<void>((wake) => {
      const stop = () => {
        clearTimeout(timer);
        stops.delete(stop);
        wake();
      };
      const timer = setTimeout(stop, ms);
      stops.add(stop);
    });

  return new ReadableStream<Uint8Array>(
    {
      // start does not wait for the watches: the stream calls pull only
      // after start resolved, and sendWhenRoom waits for pull.
      start(controller) {
        const end = () => {
          close();
          try {
            controller.close();
          } catch {
            // cancel already closed the stream.
          }
        };
        const frame = (event: string, data: unknown, fields = "") =>
          encoder.encode(
            `${fields}event: ${event}\ndata: ${JSON.stringify(data)}\n\n`,
          );
        // The shared informer does not wait for its listener, so a write
        // cannot wait for a slow client. desiredSize is below 0 when more
        // than maxPendingBytes wait in the queue.
        const write = (bytes: Uint8Array) => {
          if (closed) return;
          if ((controller.desiredSize ?? 0) < 0) {
            end();
            return;
          }
          controller.enqueue(bytes);
        };
        const send = (event: string, data: unknown, fields = "") =>
          write(frame(event, data, fields));
        // The snapshots and failures of all watches can be ready at once. A
        // frame that waits for room keeps them from passing maxPendingBytes
        // together.
        const sendWhenRoom = async (
          event: string,
          data: unknown,
          fields = "",
        ) => {
          const bytes = frame(event, data, fields);
          const room = Math.min(bytes.byteLength, maxPendingBytes);
          while (!closed && (controller.desiredSize ?? 0) < room) {
            await new Promise<void>((wake) => drained.push(wake));
          }
          write(bytes);
        };
        signal.addEventListener("abort", end, { once: true });
        if (signal.aborted) {
          end();
          return;
        }
        // A comment line every heartbeatMs keeps idle proxies from closing
        // the connection.
        heartbeat = setInterval(
          () => write(encoder.encode(": keep-alive\n\n")),
          heartbeatMs,
        );

        const runWatch = async (entry: WatchEntry) => {
          const { id } = entry;
          if ("error" in entry) {
            const kind = new URLSearchParams(id).get("kind") ?? "";
            await sendWhenRoom("failure", {
              watch: id,
              kind,
              message: entry.error,
            });
            return;
          }
          const { request } = entry;
          const matchesLabels = parseSelector(request.labelSelector ?? "");
          const inFilter = (o: KubernetesObject) =>
            (!request.namespace ||
              o.metadata?.namespace === request.namespace) &&
            (!request.name || o.metadata?.name === request.name) &&
            matchesLabels(o.metadata?.labels);
          // subscribeKind resolves false after it sent a failure frame.
          const subscribeKind = async (kind: KindName) => {
            const seen = new Set<string>();
            // Changes that arrive before the snapshot and its waiting changes
            // are sent wait here, so the order of the cache stays.
            let queue: [CacheChange, KubernetesObject, number][] | undefined =
              [];
            const delta = (change: CacheChange, object: KubernetesObject) => {
              const type = deltaType(
                seen,
                change,
                objectKey(object),
                inFilter(object),
              );
              return type && { watch: id, type, kind, object };
            };
            try {
              const { snapshot, unsubscribe } = await cache.subscribe(
                kind,
                (change, object) => {
                  if (!queue) {
                    const data = delta(change, object as KubernetesObject);
                    if (data) send("delta", data);
                    return;
                  }
                  const bytes = JSON.stringify(object).length;
                  queue.push([change, object as KubernetesObject, bytes]);
                  queuedBytes += bytes;
                  if (queuedBytes > maxPendingBytes) end();
                },
              );
              if (closed) {
                unsubscribe();
                return true;
              }
              unsubscribes.push(unsubscribe);
              const objects = (snapshot as KubernetesObject[]).filter(inFilter);
              objects.forEach((o) => seen.add(objectKey(o)));
              await sendWhenRoom("snapshot", { watch: id, kind, objects });
              while (queue.length > 0) {
                const [change, object, bytes] = queue.shift()!;
                queuedBytes -= bytes;
                const data = delta(change, object);
                if (data) await sendWhenRoom("delta", data);
              }
              queue = undefined;
            } catch (error) {
              await sendWhenRoom(
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
            return true;
          };
          // A kind that the API server does not serve stays in its own watch,
          // so the other watches of the page keep their data.
          for (const kind of request.kinds) {
            while (!(await subscribeKind(kind))) {
              await pause(retryMs);
              if (closed) return;
            }
          }
        };

        watches.forEach((entry) => void runWatch(entry));
      },
      pull() {
        drained.splice(0).forEach((wake) => wake());
      },
      cancel() {
        close();
      },
    },
    new ByteLengthQueuingStrategy({ highWaterMark: maxPendingBytes }),
  );
}
