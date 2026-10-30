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
 * apply without --server-side sends for a custom resource: body, plus null
 * at each field of the last applied file of live that body leaves out. A
 * list or a value of body replaces the one of live whole. The patch never
 * removes metadata.annotations, and it keeps LAST_APPLIED_ANNOTATION at its
 * live value. It returns undefined when lastAppliedOf(live) is undefined.
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
  const patch = withRemovals(last, next);
  const metadata = patch.metadata as JsonMap;
  metadata.annotations = {
    ...(metadata.annotations as JsonMap),
    [LAST_APPLIED_ANNOTATION]: annotationsOf(live)![LAST_APPLIED_ANNOTATION],
  };
  return patch;
}

function withRemovals(last: JsonMap, next: JsonMap): JsonMap {
  const patch: JsonMap = { ...next };
  for (const [key, old] of Object.entries(last)) {
    const value = next[key];
    if (!Object.hasOwn(next, key)) patch[key] = null;
    else if (isMap(old) && isMap(value)) patch[key] = withRemovals(old, value);
  }
  return patch;
}

function annotationsOf(object: KubeObject | undefined): JsonMap | undefined {
  return object?.metadata.annotations as JsonMap | undefined;
}

function isMap(value: unknown): value is JsonMap {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
