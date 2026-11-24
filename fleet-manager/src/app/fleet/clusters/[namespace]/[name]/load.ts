import "server-only";
import { claimNameOf } from "@/components/cluster/claim";
import type { ClusterPageData } from "@/components/cluster/cluster-view";
import { clusterOptimize } from "@/components/cluster/optimize";
import { clusterFieldsInputOf } from "@/components/cluster/settings";
import type { Environment } from "@/generated/crds/environment";
import { claimClusterOf } from "@/lib/clusters/draft";
import { fieldDocsOf } from "@/lib/fielddocs";
import { clusterFields } from "@/lib/fields";
import type { KindName } from "@/lib/k8s/kinds";
import { get, list, type ListOptions } from "@/lib/k8s/read";
import { CLAIM_LABEL, CLUSTER_LABEL, ENVIRONMENT_LABEL } from "@/lib/labels";
import { ownerName } from "@/lib/labels/owner-name";
import { mergeCluster } from "@/lib/merge";

/**
 * loadClusterPage reads the first snapshot of the cluster page from the
 * cache of the server. It returns undefined only when it read the
 * CamundaCluster and the claims of the namespace, and neither the cluster
 * nor a claim of that name exists. A failed read of the cluster gives no
 * cluster and an entry in readErrors. A failed read of another kind gives
 * no objects of that kind and an entry in readErrors.
 */
export async function loadClusterPage(
  namespace: string,
  name: string,
): Promise<ClusterPageData | undefined> {
  const readErrors: { kind: string; message: string }[] = [];
  const failed = (kind: KindName) => (err: unknown) => {
    readErrors.push({
      kind,
      message: err instanceof Error ? err.message : String(err),
    });
  };
  const cluster = await get("CamundaCluster", namespace, name).catch(
    (err: unknown) => {
      failed("CamundaCluster")(err);
      return undefined;
    },
  );
  const listRecorded = <K extends KindName>(kind: K, opts: ListOptions) =>
    list(kind, opts).catch((err: unknown) => {
      failed(kind)(err);
      return [];
    });

  const inNamespace = { namespace };
  const podSelector = `${CLUSTER_LABEL}=${ownerName(name)}`;
  const claimName = claimNameOf(cluster, name);
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
  const claim = claims.find((c) => c.metadata?.name === claimName);
  const unread = (kind: KindName) => readErrors.some((e) => e.kind === kind);
  if (
    !cluster &&
    !claim &&
    !unread("CamundaCluster") &&
    !unread("CamundaClusterClaim")
  ) {
    return undefined;
  }

  const environmentName =
    namespaceObject?.metadata?.labels?.[ENVIRONMENT_LABEL];
  const subject =
    cluster ??
    (claim && !unread("CamundaCluster")
      ? claimClusterOf(
          { name, namespace, spec: claim.spec },
          environments.find((e) => e.metadata?.name === environmentName) as
            Environment | undefined,
        )
      : undefined);
  const fields = subject
    ? clusterFields(
        clusterFieldsInputOf({
          cluster: subject,
          merged: mergeCluster(
            presets.find((p) => p.metadata?.name === subject.spec?.presetRef)
              ?.spec,
            releases.find((r) => r.metadata?.name === subject.spec?.releaseRef)
              ?.spec,
            subject.spec,
          ),
          claim,
          sections,
          registrations,
          optimize: clusterOptimize(name, optimizes),
          schedules,
        }),
      )
    : [];
  return {
    ref: { namespace, name },
    ...(cluster && { cluster }),
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
