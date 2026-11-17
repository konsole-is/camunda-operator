import "server-only";
import type { ClusterCreateData } from "@/components/create/cluster-create-flow";
import type { ReadError } from "@/lib/k8s/unread";
import type { KindName, Resource } from "@/lib/k8s/kinds";
import type { ListOptions } from "@/lib/k8s/read";
import { list } from "@/lib/k8s/read";
import { ACCESS_SECTION, SECTION_LABEL } from "@/lib/labels";

/**
 * loadCreateCluster reads the first snapshot of the create cluster page
 * from the cache of the server, in every namespace. A failed read of a kind
 * gives no objects of that kind and an entry in readErrors.
 */
export async function loadCreateCluster(): Promise<ClusterCreateData> {
  const readErrors: ReadError[] = [];
  // The steps and the review must tell a failed read of these kinds from an
  // empty list.
  const listOrRecord = <K extends KindName>(
    kind: K,
    opts: ListOptions = {},
  ): Promise<Resource<K>[]> =>
    list(kind, opts).catch((err: unknown) => {
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
    listOrRecord("Environment"),
    listOrRecord("Namespace"),
    listOrRecord("CamundaClusterClaim"),
    listOrRecord("CamundaCluster"),
    listOrRecord("CamundaClusterPreset"),
    listOrRecord("PluginSection"),
    listOrRecord("PluginRegistration"),
    listOrRecord("PluginRequest", {
      labelSelector: `${SECTION_LABEL}=${ACCESS_SECTION}`,
    }),
    listOrRecord("ElasticsearchClusterPreset"),
    listOrRecord("DatabaseServerPreset"),
    listOrRecord("SecondaryStorageConfig"),
    listOrRecord("ObjectStorageConfig"),
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
