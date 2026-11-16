// This module runs in the browser and on the server.
import type { CamundaClusterClaimSpec } from "@/generated/crds/camundaclusterclaim";

/** ClusterAttachments is spec.attachments of a cluster claim. */
export type ClusterAttachments = NonNullable<
  CamundaClusterClaimSpec["attachments"]
>;

/** OPTIMIZE_NEEDS_ELASTICSEARCH says why Optimize cannot attach to a cluster without Elasticsearch. */
export const OPTIMIZE_NEEDS_ELASTICSEARCH =
  "Optimize needs Elasticsearch as secondary storage.";

/** NO_MANAGEMENT_AUTH_CONFIG says why Optimize cannot attach while no ManagementAuthConfig exists. */
export const NO_MANAGEMENT_AUTH_CONFIG =
  "No ManagementAuthConfig exists. Your admin creates one first.";

/** SCHEDULE_NEEDS_BACKUP_STORAGE says why a backup schedule cannot attach to a cluster without backup storage. */
export const SCHEDULE_NEEDS_BACKUP_STORAGE = "Needs a backup storage option.";

/** DEFAULT_SCHEDULE is the cron of a new backup schedule: each day at 02:00 UTC. */
export const DEFAULT_SCHEDULE = "0 2 * * *";

/**
 * optimizeBlocked returns why Optimize cannot attach, undefined when it
 * can. secondaryStorage is the type of the secondary storage, undefined
 * when it is not known, which counts as not Elasticsearch.
 * managementAuthConfigs are the names of the ManagementAuthConfigs.
 */
export function optimizeBlocked(
  secondaryStorage: string | undefined,
  managementAuthConfigs: readonly string[],
): string | undefined {
  if (secondaryStorage !== "elasticsearch") return OPTIMIZE_NEEDS_ELASTICSEARCH;
  if (managementAuthConfigs.length === 0) return NO_MANAGEMENT_AUTH_CONFIG;
  return undefined;
}

/** scheduleBlocked returns why a backup schedule cannot attach, undefined when the cluster has backup storage. */
export function scheduleBlocked(backupStorage: boolean): string | undefined {
  return backupStorage ? undefined : SCHEDULE_NEEDS_BACKUP_STORAGE;
}
