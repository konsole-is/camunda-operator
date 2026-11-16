import "server-only";
import type { ClusterCreateData } from "@/components/create/cluster-create-flow";
import type { ReadError } from "@/components/planes/planes-table";
import type { KindName, Resource } from "@/lib/k8s/kinds";
import { listOrEmpty } from "@/lib/k8s/list-or-empty";
import { list } from "@/lib/k8s/read";
import { ACCESS_SECTION, SECTION_LABEL } from "@/lib/labels";

/**
 * loadCreateCluster reads the first snapshot of the create cluster page
 * from the cache of the server, in every namespace. A failed read of a kind
 * gives no objects of that kind. A failed read of the releases, platform
 * configs, or ManagementAuthConfigs also gives an entry in readErrors.
 */
export async function loadCreateCluster(): Promise<ClusterCreateData> {
  const readErrors: ReadError[] = [];
  // Steps 3 and 4 say that a picked object of these kinds does not exist,
  // so they must tell a failed read from an empty list.
  const listOrRecord = <K extends KindName>(kind: K): Promise<Resource<K>[]> =>
    list(kind, {}).catch((err: unknown) => {
      readErrors.push({
        kind,
        message: err instanceof Error ? err.message : String(err),
      });
      return [];
    });
  const [
    environments,
    namespaces,
    claims,
    clusters,
    presets,
    sections,
    registrations,
    accessRequests,
    elasticsearchPresets,
    databaseServerPresets,
    secondaryStorageConfigs,
    objectStorageConfigs,
    releases,
    platformConfigs,
    managementAuthConfigs,
  ] = await Promise.all([
    listOrEmpty("Environment", {}),
    listOrEmpty("Namespace", {}),
    listOrEmpty("CamundaClusterClaim", {}),
    listOrEmpty("CamundaCluster", {}),
    listOrEmpty("CamundaClusterPreset", {}),
    listOrEmpty("PluginSection", {}),
    listOrEmpty("PluginRegistration", {}),
    listOrEmpty("PluginRequest", {
      labelSelector: `${SECTION_LABEL}=${ACCESS_SECTION}`,
    }),
    listOrEmpty("ElasticsearchClusterPreset", {}),
    listOrEmpty("DatabaseServerPreset", {}),
    listOrEmpty("SecondaryStorageConfig", {}),
    listOrEmpty("ObjectStorageConfig", {}),
    listOrRecord("CamundaRelease"),
    listOrRecord("CamundaPlatformConfig"),
    listOrRecord("ManagementAuthConfig"),
  ]);
  return {
    environments,
    namespaces,
    claims,
    clusters,
    presets,
    sections,
    registrations,
    accessRequests,
    elasticsearchPresets,
    databaseServerPresets,
    secondaryStorageConfigs,
    objectStorageConfigs,
    releases,
    platformConfigs,
    managementAuthConfigs,
    readErrors,
  };
}

export type { ClusterCreateData };
