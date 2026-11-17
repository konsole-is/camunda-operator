import { refKey } from "./refs";
import type { Change, RefKey } from "./types";

/** DeletedObject is an object that a page deletes, as the page read it. */
export type DeletedObject = {
  apiVersion: string;
  kind: string;
  metadata: {
    name?: string;
    namespace?: string;
    resourceVersion?: string;
    uid?: string;
  };
};

/**
 * deleteChangeOf deletes objects in one change with intent. Each delete
 * carries the resourceVersion and the UID that the page read, so it fails
 * with Stale when the object changed or was created again. An object
 * without a resourceVersion or a UID has no precondition or UID, and a
 * change with none leaves out preconditions or uids.
 */
export function deleteChangeOf(
  objects: readonly DeletedObject[],
  intent: string,
): Change {
  const preconditions: Record<RefKey, string> = {};
  const uids: Record<RefKey, string> = {};
  for (const { kind, metadata } of objects) {
    const key = refKey({
      kind,
      namespace: metadata.namespace,
      name: metadata.name ?? "",
    });
    if (metadata.resourceVersion !== undefined)
      preconditions[key] = metadata.resourceVersion;
    if (metadata.uid) uids[key] = metadata.uid;
  }
  return {
    intent,
    apply: [],
    delete: objects.map(({ apiVersion, kind, metadata }) => ({
      apiVersion,
      kind,
      ...(metadata.namespace !== undefined && {
        namespace: metadata.namespace,
      }),
      name: metadata.name ?? "",
    })),
    ...(Object.keys(preconditions).length > 0 && { preconditions }),
    ...(Object.keys(uids).length > 0 && { uids }),
  };
}
