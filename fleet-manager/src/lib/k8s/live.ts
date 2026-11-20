"use client";
import { useResources } from "./hooks";
import type { KindName, Resource } from "./kinds";

/**
 * useLive is useResources of kind in every namespace, for a page that
 * holds the objects as their model type T. A Resource is plain JSON, so it
 * differs from T only in its timestamps, which are strings, not Dates: such
 * a page may only check whether one is set.
 */
export function useLive<K extends KindName, T>(
  kind: K,
  initial: readonly T[],
): { items: T[]; error?: string } {
  const { items, error } = useResources(
    kind,
    {},
    initial as unknown as Resource<K>[],
  );
  return { items: items as unknown as T[], ...(error && { error }) };
}
