"use client";

import type { V1Namespace } from "@kubernetes/client-node";
import type { ReadError } from "@/components/planes/planes-table";
import type { CamundaCluster } from "@/generated/crds/camundacluster";
import type { CamundaClusterClaim } from "@/generated/crds/camundaclusterclaim";
import type { CamundaClusterPreset } from "@/generated/crds/camundaclusterpreset";
import type { CamundaManagementCluster } from "@/generated/crds/camundamanagementcluster";
import type { CamundaManagementClusterClaim } from "@/generated/crds/camundamanagementclusterclaim";
import type { CamundaPlatformConfig } from "@/generated/crds/camundaplatformconfig";
import type { CamundaRelease } from "@/generated/crds/camundarelease";
import type { DatabaseServer } from "@/generated/crds/databaseserver";
import type { DatabaseServerPreset } from "@/generated/crds/databaseserverpreset";
import type { Environment } from "@/generated/crds/environment";
import { useResources, useWatchesFresh } from "@/lib/k8s/hooks";
import type { KindName, Resource } from "@/lib/k8s/kinds";
import { PLATFORM_SOURCE_KINDS } from "./source-kinds";

/** PlatformSources holds every list that the Platform pages read, in all namespaces. */
export type PlatformSources = {
  platformConfigs: CamundaPlatformConfig[];
  environments: Environment[];
  namespaces: V1Namespace[];
  clusters: CamundaCluster[];
  planes: CamundaManagementCluster[];
  databaseServers: DatabaseServer[];
  clusterPresets: CamundaClusterPreset[];
  databasePresets: DatabaseServerPreset[];
  releases: CamundaRelease[];
  clusterClaims: CamundaClusterClaim[];
  planeClaims: CamundaManagementClusterClaim[];
};

/**
 * usePlatformSources follows each list of initial live. errors holds the
 * error of each failed watch. The lists then keep their last items, which
 * can be out of date. stale holds the kind of each list whose watch is not
 * fresh (freshKey).
 */
export function usePlatformSources(
  initial: PlatformSources,
): PlatformSources & { errors: ReadError[]; stale: string[] } {
  const results = {
    platformConfigs: useLive("CamundaPlatformConfig", initial.platformConfigs),
    environments: useLive("Environment", initial.environments),
    namespaces: useLive("Namespace", initial.namespaces),
    clusters: useLive("CamundaCluster", initial.clusters),
    planes: useLive("CamundaManagementCluster", initial.planes),
    databaseServers: useLive("DatabaseServer", initial.databaseServers),
    clusterPresets: useLive("CamundaClusterPreset", initial.clusterPresets),
    databasePresets: useLive("DatabaseServerPreset", initial.databasePresets),
    releases: useLive("CamundaRelease", initial.releases),
    clusterClaims: useLive("CamundaClusterClaim", initial.clusterClaims),
    planeClaims: useLive("CamundaManagementClusterClaim", initial.planeClaims),
  };
  const keys = Object.keys(results) as (keyof PlatformSources)[];
  const kinds = keys.map((key) => PLATFORM_SOURCE_KINDS[key]);
  const fresh = useWatchesFresh(kinds.map((kind) => ({ kind, opts: {} })));
  const errors = keys.flatMap((key) => {
    const error = results[key].error;
    return error ? [{ kind: PLATFORM_SOURCE_KINDS[key], message: error }] : [];
  });
  return {
    platformConfigs: results.platformConfigs.items,
    environments: results.environments.items,
    namespaces: results.namespaces.items,
    clusters: results.clusters.items,
    planes: results.planes.items,
    databaseServers: results.databaseServers.items,
    clusterPresets: results.clusterPresets.items,
    databasePresets: results.databasePresets.items,
    releases: results.releases.items,
    clusterClaims: results.clusterClaims.items,
    planeClaims: results.planeClaims.items,
    errors,
    stale: kinds.filter((_, i) => !fresh[i]),
  };
}

// A Resource is plain JSON, so it differs from its model type only in its
// timestamps, which the Platform pages do not read.
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
