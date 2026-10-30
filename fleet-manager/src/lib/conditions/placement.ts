// This module runs in the browser and on the server, so it imports only types.
import type { CoreComponent } from "@/lib/observe/types";

/**
 * Placement is where a condition shows on a page: the page header, a
 * component of the cluster, the claim group, the Optimize component, or the
 * Conditions section (resource).
 */
export type Placement =
  "header" | "resource" | "claim" | "optimize" | CoreComponent;

/** PlacementMap places each condition type of one kind. */
export type PlacementMap<T extends string> = Readonly<Record<T, Placement>>;

/**
 * placementOf returns the place of a condition type. A type that the map does
 * not know, such as one of a newer operator, goes to resource.
 */
export function placementOf(
  map: PlacementMap<string>,
  type: string,
): Placement {
  return Object.hasOwn(map, type) ? map[type] : "resource";
}

/** conditionsAt returns the conditions at one place, in their order. */
export function conditionsAt<C extends { type: string }>(
  map: PlacementMap<string>,
  conditions: readonly C[] | undefined,
  at: Placement,
): C[] {
  return (conditions ?? []).filter((c) => placementOf(map, c.type) === at);
}
