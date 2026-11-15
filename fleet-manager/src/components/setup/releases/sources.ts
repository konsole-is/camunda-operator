"use client";

import type { V1Namespace } from "@kubernetes/client-node";
import type { ReadError } from "@/components/planes/planes-table";
import type { CamundaCluster } from "@/generated/crds/camundacluster";
import type { CamundaClusterClaim } from "@/generated/crds/camundaclusterclaim";
import type { CamundaRelease } from "@/generated/crds/camundarelease";
import type { DatabaseServer } from "@/generated/crds/databaseserver";
import type { ElasticsearchCluster } from "@/generated/crds/elasticsearchcluster";
import type { Environment } from "@/generated/crds/environment";
import { useResources, useWatchesFresh } from "@/lib/k8s/hooks";
import type { KindName, Resource } from "@/lib/k8s/kinds";
import { RELEASE_SOURCE_KINDS } from "./source-kinds";

/** ReleasesData holds every list that the release pages read, in all namespaces. */
export type ReleasesData = {
  releases: CamundaRelease[];
  environments: Environment[];
  namespaces: V1Namespace[];
  clusters: CamundaCluster[];
  elasticsearchClusters: ElasticsearchCluster[];
  databaseServers: DatabaseServer[];
  claims: CamundaClusterClaim[];
};

/**
 * useReleaseSources follows each list of initial live. errors holds the
 * error of each failed watch. The lists then keep their last items, which
 * can be out of date. stale holds the kind of each list whose watch has no
 * snapshot on its current connection.
 */
export function useReleaseSources(
  initial: ReleasesData,
): ReleasesData & { errors: ReadError[]; stale: string[] } {
  const results = {
    releases: useLive("CamundaRelease", initial.releases),
    environments: useLive("Environment", initial.environments),
    namespaces: useLive("Namespace", initial.namespaces),
    clusters: useLive("CamundaCluster", initial.clusters),
    elasticsearchClusters: useLive(
      "ElasticsearchCluster",
      initial.elasticsearchClusters,
    ),
    databaseServers: useLive("DatabaseServer", initial.databaseServers),
    claims: useLive("CamundaClusterClaim", initial.claims),
  };
  const keys = Object.keys(results) as (keyof ReleasesData)[];
  const kinds = keys.map((key) => RELEASE_SOURCE_KINDS[key]);
  const fresh = useWatchesFresh(kinds.map((kind) => ({ kind, opts: {} })));
  const errors = keys.flatMap((key) => {
    const error = results[key].error;
    return error ? [{ kind: RELEASE_SOURCE_KINDS[key], message: error }] : [];
  });
  return {
    releases: results.releases.items,
    environments: results.environments.items,
    namespaces: results.namespaces.items,
    clusters: results.clusters.items,
    elasticsearchClusters: results.elasticsearchClusters.items,
    databaseServers: results.databaseServers.items,
    claims: results.claims.items,
    errors,
    stale: kinds.filter((_, i) => !fresh[i]),
  };
}

// A Resource is plain JSON, so it differs from its model type only in its
// timestamps, which the release pages do not read.
function useLive<K extends KindName, T>(
  kind: K,
  initial: T[],
): { items: T[]; error?: string } {
  const { items, error } = useResources(
    kind,
    {},
    initial as unknown as Resource<K>[],
  );
  return { items: items as unknown as T[], ...(error && { error }) };
}
