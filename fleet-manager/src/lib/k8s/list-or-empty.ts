import "server-only";
import type { KindName, Resource } from "./kinds";
import { list, type ListOptions } from "./read";

/**
 * listOrEmpty is list for the first snapshot of a page: a failed read gives
 * no objects of that kind, and the watch of the kind then shows its error.
 */
export function listOrEmpty<K extends KindName>(
  kind: K,
  opts: ListOptions = {},
): Promise<Resource<K>[]> {
  return list(kind, opts).catch(() => []);
}
