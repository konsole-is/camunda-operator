import "server-only";
import { LAST_APPLIED_ANNOTATION } from "@/lib/labels";
import type { KubeObject } from "./types";

type JsonMap = Record<string, unknown>;

/**
 * lastAppliedOf returns the last applied file of kubectl in the annotation
 * LAST_APPLIED_ANNOTATION of live. It returns undefined when live is
 * undefined, has no such annotation, or the annotation is not a JSON object.
 */
export function lastAppliedOf(
  live: KubeObject | undefined,
): JsonMap | undefined {
  const text = annotationsOf(live)?.[LAST_APPLIED_ANNOTATION];
  if (typeof text !== "string") return undefined;
  try {
    const value: unknown = JSON.parse(text);
    return isMap(value) ? value : undefined;
  } catch {
    return undefined;
  }
}

/**
 * clientSideApplyPatch returns the JSON merge patch (RFC 7386) that kubectl
 * apply without --server-side sends for a custom resource. A list or a
 * value of body replaces the one of live whole. The patch never removes
 * metadata.annotations, and it keeps LAST_APPLIED_ANNOTATION at its live
 * value. It returns undefined when lastAppliedOf(live) is undefined.
 */
export function clientSideApplyPatch(
  live: KubeObject | undefined,
  body: KubeObject,
): JsonMap | undefined {
  const last = lastAppliedOf(live);
  if (last === undefined) return undefined;
  // kubectl's new file always carries the last applied annotation, so the
  // annotations map itself is never null in its patch.
  const next = {
    ...body,
    metadata: { ...body.metadata, annotations: { ...annotationsOf(body) } },
  };
  const patch = withRemovals(last, next, live as JsonMap);
  const metadata = patch.metadata as JsonMap;
  metadata.annotations = {
    ...(metadata.annotations as JsonMap),
    [LAST_APPLIED_ANNOTATION]: annotationsOf(live)![LAST_APPLIED_ANNOTATION],
  };
  return patch;
}

function withRemovals(
  last: JsonMap | undefined,
  next: JsonMap,
  live: JsonMap | undefined,
): JsonMap {
  const patch: JsonMap = {};
  for (const [key, value] of Object.entries(next)) {
    if (value === undefined) continue;
    if (value === null) {
      // kubectl sends no change for a null of body at a field that is null
      // on live. A null in a merge patch removes the field, so it would
      // remove a null that live holds.
      if (fieldOf(live, key) !== null) setOwn(patch, key, null);
    } else if (isMap(value)) {
      setOwn(
        patch,
        key,
        withRemovals(mapOf(last, key), value, mapOf(live, key)),
      );
    } else {
      setOwn(patch, key, value);
    }
  }
  for (const key of Object.keys(last ?? {})) {
    // JSON drops an undefined value, so the field is left out.
    if (fieldOf(next, key) === undefined) setOwn(patch, key, null);
  }
  return patch;
}

function fieldOf(map: JsonMap | undefined, key: string): unknown {
  return map !== undefined && Object.hasOwn(map, key) ? map[key] : undefined;
}

function mapOf(map: JsonMap | undefined, key: string): JsonMap | undefined {
  const value = fieldOf(map, key);
  return isMap(value) ? value : undefined;
}

// A plain assignment of the key __proto__ sets the prototype, not a field.
function setOwn(map: JsonMap, key: string, value: unknown) {
  Object.defineProperty(map, key, {
    value,
    enumerable: true,
    writable: true,
    configurable: true,
  });
}

function annotationsOf(object: KubeObject | undefined): JsonMap | undefined {
  return object?.metadata.annotations as JsonMap | undefined;
}

function isMap(value: unknown): value is JsonMap {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
