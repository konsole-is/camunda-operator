import "server-only";
import type { ClusterCreateData } from "@/components/create/cluster-create-flow";
import type { KindName, Resource } from "@/lib/k8s/kinds";
import { list, type ListOptions } from "@/lib/k8s/read";
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
  };
}

function listOrEmpty<K extends KindName>(
  kind: K,
  opts: ListOptions,
): Promise<Resource<K>[]> {
  return list(kind, opts).catch(() => []);
}

export type { ClusterCreateData };
