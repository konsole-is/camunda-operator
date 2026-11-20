import { LAST_APPLIED_ANNOTATION } from "@/lib/labels";
import { revisionManager, type ManagedObject } from "./managers";

/** LiveObject is the part of a live object that the GitOps readers read. */
export type LiveObject = {
  apiVersion: string;
  metadata: { [key: string]: unknown };
  [key: string]: unknown;
};

/**
 * lastAppliedOf returns the last applied file of kubectl in the annotation
 * LAST_APPLIED_ANNOTATION of live. It returns undefined when live is
 * undefined, has no such annotation, or the annotation is not a JSON object.
 */
export function lastAppliedOf(
  live: LiveObject | undefined,
): Record<string, unknown> | undefined {
  const annotations = live?.metadata.annotations as
    Record<string, unknown> | undefined;
  const text = annotations?.[LAST_APPLIED_ANNOTATION];
  if (typeof text !== "string") return undefined;
  try {
    const value: unknown = JSON.parse(text);
    return typeof value === "object" && value !== null && !Array.isArray(value)
      ? (value as Record<string, unknown>)
      : undefined;
  } catch {
    return undefined;
  }
}

/**
 * gitOpsManager returns the field manager of the GitOps tool on live, as
 * revisionManager finds it, or undefined when live is undefined.
 */
export function gitOpsManager(
  live: LiveObject | undefined,
): string | undefined {
  return live === undefined
    ? undefined
    : revisionManager(live as ManagedObject);
}

/**
 * appliedClientSide reports whether the GitOps tool applied live with
 * client-side apply in a way that a JSON merge patch repeats: live is a
 * custom resource and lastAppliedOf(live) is not undefined. A kind of a
 * group without a dot, such as the core group or apps, or of a *.k8s.io
 * group counts as built in, because kubectl sends a built-in kind a
 * strategic merge patch, which merges some lists by key.
 */
export function appliedClientSide(live: LiveObject | undefined): boolean {
  if (live === undefined) return false;
  const slash = live.apiVersion.indexOf("/");
  const group = slash === -1 ? "" : live.apiVersion.slice(0, slash);
  if (!group.includes(".") || group.endsWith(".k8s.io")) return false;
  return lastAppliedOf(live) !== undefined;
}
