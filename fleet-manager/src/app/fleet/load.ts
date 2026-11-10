import "server-only";
import type {
  FleetPageData,
  ReadError,
} from "@/components/fleet-view/fleet-view";
import type { KindName, Resource } from "@/lib/k8s/kinds";
import { list } from "@/lib/k8s/read";

/**
 * loadFleetPage reads the first snapshot of the fleet page from the cache of
 * the server, across all namespaces. A failed read of a kind gives no
 * objects of that kind and an entry in readErrors.
 */
export async function loadFleetPage(): Promise<FleetPageData> {
  const readErrors: ReadError[] = [];
  const listOrEmpty = <K extends KindName>(kind: K): Promise<Resource<K>[]> =>
    list(kind).catch((err: unknown) => {
      readErrors.push({
        kind,
        message: err instanceof Error ? err.message : String(err),
      });
      return [];
    });
  const [
    clusters,
    claims,
    environments,
    namespaces,
    presets,
    releases,
    sections,
    registrations,
    requests,
    storage,
    optimizes,
    backupsES,
    backupsRDBMS,
    managementClusters,
    managementClaims,
  ] = await Promise.all([
    listOrEmpty("CamundaCluster"),
    listOrEmpty("CamundaClusterClaim"),
    listOrEmpty("Environment"),
    listOrEmpty("Namespace"),
    listOrEmpty("CamundaClusterPreset"),
    listOrEmpty("CamundaRelease"),
    listOrEmpty("PluginSection"),
    listOrEmpty("PluginRegistration"),
    listOrEmpty("PluginRequest"),
    listOrEmpty("SecondaryStorageConfig"),
    listOrEmpty("CamundaOptimize"),
    listOrEmpty("LogicalBackupElasticsearch"),
    listOrEmpty("LogicalBackupRDBMS"),
    listOrEmpty("CamundaManagementCluster"),
    listOrEmpty("CamundaManagementClusterClaim"),
  ]);
  return {
    clusters,
    claims,
    environments,
    namespaces,
    presets,
    releases,
    sections,
    registrations,
    requests,
    storage,
    optimizes,
    backups: [...backupsES, ...backupsRDBMS],
    planes: {
      clusters: managementClusters,
      claims: managementClaims,
      namespaces,
      environments,
    },
    readErrors,
  };
}
