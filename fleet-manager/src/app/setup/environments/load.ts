import "server-only";
import type { EnvironmentSources } from "@/components/setup/environments/sources";
import type { KindName, Resource } from "@/lib/k8s/kinds";
import { list } from "@/lib/k8s/read";

/**
 * loadEnvironments reads the first snapshot of the Environments page from
 * the cache of the server, in every namespace. A failed read of a kind
 * gives no objects of that kind.
 */
export async function loadEnvironments(): Promise<EnvironmentSources> {
  const [
    environments,
    namespaces,
    clusterClaims,
    planeClaims,
    clusters,
    planes,
    sections,
    registrations,
    requests,
    clusterPresets,
    elasticsearchPresets,
    databasePresets,
    releases,
    platformConfigs,
  ] = await Promise.all([
    listOrEmpty("Environment"),
    listOrEmpty("Namespace"),
    listOrEmpty("CamundaClusterClaim"),
    listOrEmpty("CamundaManagementClusterClaim"),
    listOrEmpty("CamundaCluster"),
    listOrEmpty("CamundaManagementCluster"),
    listOrEmpty("PluginSection"),
    listOrEmpty("PluginRegistration"),
    listOrEmpty("PluginRequest"),
    listOrEmpty("CamundaClusterPreset"),
    listOrEmpty("ElasticsearchClusterPreset"),
    listOrEmpty("DatabaseServerPreset"),
    listOrEmpty("CamundaRelease"),
    listOrEmpty("CamundaPlatformConfig"),
  ]);
  // A Resource is plain JSON, so it differs from its model type only in its
  // timestamps, which the Environments page does not read.
  const sources = {
    environments,
    namespaces,
    clusterClaims,
    planeClaims,
    clusters,
    planes,
    sections,
    registrations,
    requests,
    clusterPresets,
    elasticsearchPresets,
    databasePresets,
    releases,
    platformConfigs,
  };
  return sources as unknown as EnvironmentSources;
}

function listOrEmpty<K extends KindName>(kind: K): Promise<Resource<K>[]> {
  return list(kind).catch(() => []);
}
