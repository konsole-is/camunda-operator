import "server-only";
import type { ClusterPageData } from "@/components/cluster/cluster-view";
import { clusterOptimize } from "@/components/cluster/optimize";
import { clusterFieldsInputOf } from "@/components/cluster/settings";
import { fieldDocsOf } from "@/lib/fielddocs";
import { clusterFields } from "@/lib/fields";
import { listOrEmpty } from "@/lib/k8s/list-or-empty";
import { get } from "@/lib/k8s/read";
import {
  ACCESS_SECTION,
  CLAIM_LABEL,
  CLUSTER_LABEL,
  SECTION_LABEL,
} from "@/lib/labels";
import { ownerName } from "@/lib/labels/server";
import { mergeCluster } from "@/lib/merge";

/**
 * loadClusterPage reads the first snapshot of the cluster page from the
 * cache of the server. It returns undefined when the cluster does not exist.
 * A failed read of another kind gives no objects of that kind.
 */
export async function loadClusterPage(
  namespace: string,
  name: string,
): Promise<ClusterPageData | undefined> {
  const cluster = await get("CamundaCluster", namespace, name);
  if (!cluster) return undefined;

  const inNamespace = { namespace };
  const podSelector = `${CLUSTER_LABEL}=${ownerName(name)}`;
  const claimName = cluster.metadata?.labels?.[CLAIM_LABEL];
  const [
    namespaceObject,
    environments,
    claims,
    presets,
    releases,
    storage,
    accessRequests,
    requests,
    pods,
    backupsES,
    backupsRDBMS,
    restoresES,
    restoresRDBMS,
    restoresPITR,
    objectStorage,
    databaseConfigs,
    databaseServerConfigs,
    databaseServers,
    databaseServerPresets,
    schedules,
    optimizes,
    sections,
    registrations,
    platformConfigs,
    managementAuthConfigs,
  ] = await Promise.all([
    get("Namespace", undefined, namespace).catch(() => undefined),
    listOrEmpty("Environment", {}),
    listOrEmpty("CamundaClusterClaim", inNamespace),
    listOrEmpty("CamundaClusterPreset", {}),
    listOrEmpty("CamundaRelease", {}),
    listOrEmpty("SecondaryStorageConfig", inNamespace),
    listOrEmpty("PluginRequest", {
      namespace,
      labelSelector: `${SECTION_LABEL}=${ACCESS_SECTION}`,
    }),
    listOrEmpty("PluginRequest", {
      namespace,
      labelSelector: `${CLAIM_LABEL}=${claimName ?? ""}`,
    }),
    listOrEmpty("Pod", { namespace, labelSelector: podSelector }),
    listOrEmpty("LogicalBackupElasticsearch", inNamespace),
    listOrEmpty("LogicalBackupRDBMS", inNamespace),
    listOrEmpty("LogicalRestoreElasticsearch", inNamespace),
    listOrEmpty("LogicalRestoreRDBMS", inNamespace),
    listOrEmpty("PointInTimeRestore", inNamespace),
    listOrEmpty("ObjectStorageConfig", inNamespace),
    listOrEmpty("DatabaseConfig", inNamespace),
    listOrEmpty("DatabaseServerConfig", inNamespace),
    listOrEmpty("DatabaseServer", inNamespace),
    listOrEmpty("DatabaseServerPreset", {}),
    listOrEmpty("BackupSchedule", inNamespace),
    listOrEmpty("CamundaOptimize", inNamespace),
    listOrEmpty("PluginSection", {}),
    listOrEmpty("PluginRegistration", {}),
    listOrEmpty("CamundaPlatformConfig", {}),
    listOrEmpty("ManagementAuthConfig", {}),
  ]);
  const fields = clusterFields(
    clusterFieldsInputOf({
      cluster,
      merged: mergeCluster(
        presets.find((p) => p.metadata?.name === cluster.spec?.presetRef)?.spec,
        releases.find((r) => r.metadata?.name === cluster.spec?.releaseRef)
          ?.spec,
        cluster.spec,
      ),
      claim: claims.find((c) => c.metadata?.name === claimName),
      sections,
      registrations,
      optimize: clusterOptimize(name, optimizes),
      schedules,
    }),
  );
  return {
    cluster,
    namespace: namespaceObject,
    environments,
    claims,
    presets,
    releases,
    storage,
    accessRequests,
    requests,
    pods,
    podSelector,
    backups: { elasticsearch: backupsES, rdbms: backupsRDBMS },
    restores: {
      elasticsearch: restoresES,
      rdbms: restoresRDBMS,
      pointInTime: restoresPITR,
    },
    backupStorage: {
      objectStorage,
      databaseConfigs,
      databaseServerConfigs,
      databaseServers,
      databaseServerPresets,
      schedules,
    },
    optimizes,
    sections,
    registrations,
    platformConfigs,
    managementAuthConfigs,
    fieldDocs: fieldDocsOf(fields),
  };
}
