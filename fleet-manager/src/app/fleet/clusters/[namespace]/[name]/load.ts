import "server-only";
import type { ClusterPageData } from "@/components/cluster/cluster-view";
import { clusterOptimize } from "@/components/cluster/optimize";
import { clusterFieldsInputOf } from "@/components/cluster/settings";
import { fieldDocsOf } from "@/lib/fielddocs";
import { clusterFields } from "@/lib/fields";
import type { KindName } from "@/lib/k8s/kinds";
import { get, list, type ListOptions } from "@/lib/k8s/read";
import { CLAIM_LABEL, CLUSTER_LABEL } from "@/lib/labels";
import { ownerName } from "@/lib/labels/owner-name";
import { mergeCluster } from "@/lib/merge";

/**
 * loadClusterPage reads the first snapshot of the cluster page from the
 * cache of the server. It returns undefined when the cluster does not exist.
 * A failed read of another kind gives no objects of that kind and an entry
 * in readErrors.
 */
export async function loadClusterPage(
  namespace: string,
  name: string,
): Promise<ClusterPageData | undefined> {
  const cluster = await get("CamundaCluster", namespace, name);
  if (!cluster) return undefined;

  const readErrors: { kind: string; message: string }[] = [];
  const failed = (kind: KindName) => (err: unknown) => {
    readErrors.push({
      kind,
      message: err instanceof Error ? err.message : String(err),
    });
  };
  const listRecorded = <K extends KindName>(kind: K, opts: ListOptions) =>
    list(kind, opts).catch((err: unknown) => {
      failed(kind)(err);
      return [];
    });

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
    grants,
    elasticsearchClusters,
  ] = await Promise.all([
    get("Namespace", undefined, namespace).catch((err: unknown) => {
      failed("Namespace")(err);
      return undefined;
    }),
    listRecorded("Environment", {}),
    listRecorded("CamundaClusterClaim", inNamespace),
    listRecorded("CamundaClusterPreset", {}),
    listRecorded("CamundaRelease", {}),
    listRecorded("SecondaryStorageConfig", inNamespace),
    listRecorded("PluginRequest", {
      namespace,
      labelSelector: `${CLAIM_LABEL}=${claimName ?? ""}`,
    }),
    listRecorded("Pod", { namespace, labelSelector: podSelector }),
    listRecorded("LogicalBackupElasticsearch", inNamespace),
    listRecorded("LogicalBackupRDBMS", inNamespace),
    listRecorded("LogicalRestoreElasticsearch", inNamespace),
    listRecorded("LogicalRestoreRDBMS", inNamespace),
    listRecorded("PointInTimeRestore", inNamespace),
    listRecorded("ObjectStorageConfig", inNamespace),
    listRecorded("DatabaseConfig", inNamespace),
    listRecorded("DatabaseServerConfig", inNamespace),
    listRecorded("DatabaseServer", inNamespace),
    listRecorded("DatabaseServerPreset", {}),
    listRecorded("BackupSchedule", inNamespace),
    listRecorded("CamundaOptimize", inNamespace),
    listRecorded("PluginSection", {}),
    listRecorded("PluginRegistration", {}),
    listRecorded("CamundaPlatformConfig", {}),
    listRecorded("ManagementAuthConfig", {}),
    listRecorded("Grant", inNamespace),
    listRecorded("ElasticsearchCluster", inNamespace),
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
    ref: { namespace, name },
    cluster,
    namespace: namespaceObject,
    environments,
    claims,
    presets,
    releases,
    storage,
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
    grants,
    elasticsearchClusters,
    fieldDocs: fieldDocsOf(fields),
    readErrors,
  };
}
