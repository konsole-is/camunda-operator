// This module is isomorphic: the hooks import it in the browser.

/** CacheChange is what happened to one object in the cache. */
export type CacheChange = "add" | "update" | "delete";

export type WatchEventType = "ADDED" | "MODIFIED" | "DELETED";

export type NamedObject = { metadata?: { namespace?: string; name?: string } };

/** WatchDelta is the data of an SSE frame "delta". */
export interface WatchDelta<T = NamedObject> {
  type: WatchEventType;
  kind: string;
  object: T;
}

/** WatchSnapshot is the data of an SSE frame "snapshot": all matching objects at connect time. */
export interface WatchSnapshot<T = NamedObject> {
  kind: string;
  objects: T[];
}

/** WatchFailure is the data of an SSE frame "failure": a kind that the server could not read. */
export interface WatchFailure {
  kind: string;
  message: string;
}

/** objectKey is "<namespace>/<name>", with an empty namespace for a cluster-scoped object. */
export function objectKey(object: NamedObject): string {
  return `${object.metadata?.namespace ?? ""}/${object.metadata?.name ?? ""}`;
}

/**
 * deltaType returns the event that one subscriber gets for a change of the
 * cache, or undefined when the subscriber does not see the change. seen
 * holds the keys of the objects that the subscriber has, and deltaType
 * updates it. A change that takes an object out of the filter of the
 * subscriber is DELETED. A change that brings one in is ADDED.
 */
export function deltaType(
  seen: Set<string>,
  change: CacheChange,
  key: string,
  matches: boolean,
): WatchEventType | undefined {
  const had = seen.has(key);
  if (change === "delete" || !matches) {
    if (!had) return undefined;
    seen.delete(key);
    return "DELETED";
  }
  seen.add(key);
  return had ? "MODIFIED" : "ADDED";
}

/**
 * applyDelta returns a new list with delta applied. ADDED and MODIFIED
 * replace the object with the same key in place, or append it. DELETED
 * removes it. The given list does not change.
 */
export function applyDelta<T extends NamedObject>(
  list: readonly T[],
  delta: WatchDelta<T>,
): T[] {
  const key = objectKey(delta.object);
  const index = list.findIndex((o) => objectKey(o) === key);
  if (delta.type === "DELETED") return list.filter((_, i) => i !== index);
  if (index < 0) return [...list, delta.object];
  return list.map((o, i) => (i === index ? delta.object : o));
}
