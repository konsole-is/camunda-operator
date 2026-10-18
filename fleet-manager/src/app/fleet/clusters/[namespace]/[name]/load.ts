import "server-only";
import { ACCESS_SECTION } from "@/components/cluster/launch";
import type { ClusterPageData } from "@/components/cluster/cluster-view";
import type { KindName, Resource } from "@/lib/k8s/kinds";
import { get, list, type ListOptions } from "@/lib/k8s/read";
import { CLUSTER_LABEL, SECTION_LABEL } from "@/lib/labels";
import { ownerName } from "@/lib/labels/server";

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
    listOrEmpty("Pod", { namespace, labelSelector: podSelector }),
    listOrEmpty("LogicalBackupElasticsearch", inNamespace),
    listOrEmpty("LogicalBackupRDBMS", inNamespace),
    listOrEmpty("LogicalRestoreElasticsearch", inNamespace),
    listOrEmpty("LogicalRestoreRDBMS", inNamespace),
    listOrEmpty("PointInTimeRestore", inNamespace),
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

function listOrEmpty<K extends KindName>(
  kind: K,
  opts: ListOptions,
): Promise<Resource<K>[]> {
  return list(kind, opts).catch(() => []);
}
