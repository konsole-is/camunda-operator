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
 * apply without --server-side sends for a custom resource, with body as the
 * new file. The patch leaves LAST_APPLIED_ANNOTATION at its live value. It
 * keeps the metadata.resourceVersion of body. It returns undefined when
 * lastAppliedOf(live) is undefined.
 */
export function clientSideApplyPatch(
  live: KubeObject | undefined,
  body: KubeObject,
): JsonMap | undefined {
  const last = lastAppliedOf(live);
  if (last === undefined) return undefined;
  const { resourceVersion, ...metadata } = body.metadata;
  // The JSON form drops undefined values, as the file that kubectl reads has
  // none. kubectl's new file always carries the last applied annotation, so
  // the annotations map is never null in its patch.
  const next = JSON.parse(
    JSON.stringify({
      ...body,
      metadata: {
        ...metadata,
        annotations: {
          ...annotationsOf(body),
          [LAST_APPLIED_ANNOTATION]:
            annotationsOf(live)![LAST_APPLIED_ANNOTATION],
        },
      },
    }),
  ) as JsonMap;
  // The steps of CreateThreeWayJSONMergePatch of
  // k8s.io/apimachinery/pkg/util/jsonmergepatch, which kubectl calls,
  // without its conflict and precondition checks.
  const changes = keepOrDeleteNull(diff(live as JsonMap, next), false);
  const removals = keepOrDeleteNull(diff(last, next), true);
  const patch = merged(removals, changes);
  if (resourceVersion !== undefined) {
    const meta = isMap(patch.metadata) ? patch.metadata : {};
    setOwn(patch, "metadata", { ...meta, resourceVersion });
  }
  return patch;
}

// diff is CreateMergePatch of gopkg.in/evanphx/json-patch.v4.
function diff(from: JsonMap, to: JsonMap): JsonMap {
  const patch: JsonMap = {};
  for (const [key, value] of Object.entries(to)) {
    if (!Object.hasOwn(from, key)) {
      setOwn(patch, key, value);
    } else if (isMap(from[key]) && isMap(value)) {
      const inner = diff(from[key], value);
      if (Object.keys(inner).length > 0) setOwn(patch, key, inner);
    } else if (!equal(from[key], value)) {
      setOwn(patch, key, value);
    }
  }
  for (const key of Object.keys(from)) {
    if (!Object.hasOwn(to, key)) setOwn(patch, key, null);
  }
  return patch;
}

// keepOrDeleteNull is keepOrDeleteNullInObj of jsonmergepatch. An empty
// map of the patch counts as a value there.
function keepOrDeleteNull(patch: JsonMap, keepNull: boolean): JsonMap {
  const kept: JsonMap = {};
  for (const [key, value] of Object.entries(patch)) {
    if (value === null) {
      if (keepNull) setOwn(kept, key, null);
    } else if (isMap(value) && Object.keys(value).length > 0) {
      const inner = keepOrDeleteNull(value, keepNull);
      if (Object.keys(inner).length > 0) setOwn(kept, key, inner);
    } else if (!keepNull) {
      setOwn(kept, key, value);
    }
  }
  return kept;
}

// merged is MergePatch of gopkg.in/evanphx/json-patch.v4.
function merged(doc: JsonMap, patch: JsonMap): JsonMap {
  const result: JsonMap = { ...doc };
  for (const [key, value] of Object.entries(patch)) {
    const old = Object.hasOwn(result, key) ? result[key] : undefined;
    if (value === null) delete result[key];
    else if (isMap(old)) {
      setOwn(result, key, isMap(value) ? merged(old, value) : value);
    } else setOwn(result, key, pruned(value));
  }
  return result;
}

function pruned(value: unknown): unknown {
  if (Array.isArray(value)) return value.map(pruned);
  if (!isMap(value)) return value;
  const result: JsonMap = {};
  for (const [key, inner] of Object.entries(value)) {
    if (inner !== null) setOwn(result, key, pruned(inner));
  }
  return result;
}

function equal(a: unknown, b: unknown): boolean {
  if (Array.isArray(a) && Array.isArray(b)) {
    return a.length === b.length && a.every((item, i) => equal(item, b[i]));
  }
  if (isMap(a) && isMap(b)) {
    const keys = Object.keys(a);
    return (
      keys.length === Object.keys(b).length &&
      keys.every((key) => Object.hasOwn(b, key) && equal(a[key], b[key]))
    );
  }
  return a === b;
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
