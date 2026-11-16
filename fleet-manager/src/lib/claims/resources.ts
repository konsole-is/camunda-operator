// This module runs in the browser and on the server.
import type { PluginRequestStatus } from "@/generated/crds/pluginrequest";
import type { DeletionPolicy } from "@/lib/policy";

/** ResourceRow is one resource that a plugin made for a request, as status.resources of the request lists it. */
export type ResourceRow = NonNullable<PluginRequestStatus["resources"]>[number];

/**
 * resourceFate returns what happens to row when its request is deleted
 * under policy. It is the one rule for the fate of a resource row: Delete
 * deletes every row, and Orphan keeps a row unless its plugin says that it
 * deletes it also then. A registration that does not support Orphan does
 * not change the fate of a row, because the operator then refuses Orphan
 * for the whole deletion.
 */
export function resourceFate(
  row: ResourceRow,
  policy: DeletionPolicy,
): "Kept" | "Deleted" {
  return policy === "Delete" ? "Deleted" : row.onOrphan;
}
