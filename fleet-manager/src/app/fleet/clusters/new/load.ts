import "server-only";
import type { ClusterCreateData } from "@/components/create/cluster-create-flow";
import { listOrEmpty } from "@/lib/k8s/list-or-empty";
import { ACCESS_SECTION, SECTION_LABEL } from "@/lib/labels";

/**
 * loadCreateCluster reads the first snapshot of the create cluster page
 * from the cache of the server, in every namespace. A failed read of a kind
 * gives no objects of that kind.
 */
export async function loadCreateCluster(): Promise<ClusterCreateData> {
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
  };
}

export type { ClusterCreateData };
