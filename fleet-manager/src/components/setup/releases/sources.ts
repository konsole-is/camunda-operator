"use client";

import type { V1Namespace } from "@kubernetes/client-node";
import type { CamundaCluster } from "@/generated/crds/camundacluster";
import type { CamundaClusterClaim } from "@/generated/crds/camundaclusterclaim";
import type { CamundaRelease } from "@/generated/crds/camundarelease";
import type { DatabaseServer } from "@/generated/crds/databaseserver";
import type { ElasticsearchCluster } from "@/generated/crds/elasticsearchcluster";
import type { Environment } from "@/generated/crds/environment";
import { useAnswered } from "@/lib/k8s/answered";
import { useWatchesFresh } from "@/lib/k8s/hooks";
import { useLive } from "@/lib/k8s/live";
import { currentReadErrors, type ReadError } from "@/lib/k8s/unread";
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
 * ReleasesSnapshot is the first snapshot of the release pages. readErrors holds
 * the error of each kind that the server could not read.
 */
export type ReleasesSnapshot = ReleasesData & { readErrors?: ReadError[] };

/**
 * useReleaseSources follows each list of initial live. errors holds the error
 * of each failed watch, and the error of initial.readErrors of each other kind
 * until the watch of that kind answers. The lists then keep their last items,
 * which can be out of date. stale holds the kind of each list whose watch is
 * not fresh (freshKey).
 */
export function useReleaseSources(
  initial: ReleasesSnapshot,
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
  const answered = useAnswered(kinds);
  const live = keys.flatMap((key) => {
    const error = results[key].error;
    return error ? [{ kind: RELEASE_SOURCE_KINDS[key], message: error }] : [];
  });
  const errors = currentReadErrors(live, initial.readErrors, answered);
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
