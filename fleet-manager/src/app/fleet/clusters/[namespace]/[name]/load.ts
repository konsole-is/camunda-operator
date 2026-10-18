import "server-only";
import { ACCESS_SECTION } from "@/components/cluster/launch";
import type { ClusterPageData } from "@/components/cluster/cluster-view";
import { get, list } from "@/lib/k8s/read";
import { CLUSTER_LABEL, SECTION_LABEL } from "@/lib/labels";
import { ownerName } from "@/lib/labels/server";

/**
 * loadClusterPage reads the first snapshot of the cluster page from the
 * cache of the server. It returns undefined when the cluster does not exist.
 */
export async function loadClusterPage(
  namespace: string,
  name: string,
): Promise<ClusterPageData | undefined> {
  const cluster = await get("CamundaCluster", namespace, name);
  if (!cluster) return undefined;

  const inNamespace = { namespace };
  const podSelector = `${CLUSTER_LABEL}=${ownerName(name)}`;
  const [
    namespaceObject,
    environments,
    claims,
    presets,
    releases,
    storage,
    accessRequests,
    pods,
    backupsES,
    backupsRDBMS,
    restoresES,
    restoresRDBMS,
    restoresPITR,
  ] = await Promise.all([
    get("Namespace", undefined, namespace),
    list("Environment", {}),
    list("CamundaClusterClaim", inNamespace),
    list("CamundaClusterPreset", {}),
    list("CamundaRelease", {}),
    list("SecondaryStorageConfig", inNamespace),
    list("PluginRequest", {
      namespace,
      labelSelector: `${SECTION_LABEL}=${ACCESS_SECTION}`,
    }),
    list("Pod", { namespace, labelSelector: podSelector }),
    list("LogicalBackupElasticsearch", inNamespace),
    list("LogicalBackupRDBMS", inNamespace),
    list("LogicalRestoreElasticsearch", inNamespace),
    list("LogicalRestoreRDBMS", inNamespace),
    list("PointInTimeRestore", inNamespace),
  ]);
  return {
    cluster,
    namespace: namespaceObject,
    environments,
    claims,
    presets,
    releases,
    storage,
    accessRequests,
    pods,
    podSelector,
    backups: { elasticsearch: backupsES, rdbms: backupsRDBMS },
    restores: {
      elasticsearch: restoresES,
      rdbms: restoresRDBMS,
      pointInTime: restoresPITR,
    },
  };
}
