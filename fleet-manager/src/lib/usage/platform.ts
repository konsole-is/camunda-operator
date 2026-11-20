// This module runs in the browser and on the server.
import type { CamundaCluster } from "@/generated/crds/camundacluster";
import type { CamundaClusterClaim } from "@/generated/crds/camundaclusterclaim";
import type { CamundaManagementCluster } from "@/generated/crds/camundamanagementcluster";
import type { CamundaManagementClusterClaim } from "@/generated/crds/camundamanagementclusterclaim";
import type { DatabaseServer } from "@/generated/crds/databaseserver";
import type { DatabaseServerPreset } from "@/generated/crds/databaseserverpreset";
import type { Environment } from "@/generated/crds/environment";
import { mergeDatabaseServer } from "@/lib/merge";
import { byCodeUnit } from "@/lib/policy";
import { defaultIn } from "./environments";

/** PlatformUsageSources are the lists of every kind that can use a CamundaPlatformConfig. */
export type PlatformUsageSources = {
  clusters: readonly CamundaCluster[];
  planes: readonly CamundaManagementCluster[];
  databaseServers: readonly DatabaseServer[];
  databasePresets: readonly DatabaseServerPreset[];
  clusterClaims: readonly CamundaClusterClaim[];
  planeClaims: readonly CamundaManagementClusterClaim[];
  environments: readonly Environment[];
};

/** PlatformUsers are the users of one CamundaPlatformConfig, each list sorted by namespace, then name. */
export type PlatformUsers = {
  clusters: CamundaCluster[];
  planes: CamundaManagementCluster[];
  /** preset names the DatabaseServerPreset that gave the reference, when the server spec does not set it. */
  databaseServers: { server: DatabaseServer; preset?: string }[];
  clusterClaims: CamundaClusterClaim[];
  planeClaims: CamundaManagementClusterClaim[];
  /** environments are the names of the Environments whose default is the config, sorted by code unit. */
  environments: string[];
};

/**
 * platformUsers returns the users of the CamundaPlatformConfig name. A
 * CamundaCluster and a CamundaManagementCluster use it when their
 * spec.platformConfigRef names it. A DatabaseServer uses it when its spec
 * merged over its DatabaseServerPreset names it. A claim uses it when its
 * spec.platformConfigRef names it and its own cluster or management plane,
 * of the same namespace and name, does not. An Environment uses it when its
 * spec.platformConfigRef names it. An empty name matches nothing.
 */
export function platformUsers(
  name: string,
  sources: PlatformUsageSources,
): PlatformUsers {
  if (name === "") {
    return {
      clusters: [],
      planes: [],
      databaseServers: [],
      clusterClaims: [],
      planeClaims: [],
      environments: [],
    };
  }
  const names = (o: { spec: { platformConfigRef?: string } }) =>
    o.spec.platformConfigRef === name;
  const clusters = sources.clusters.filter(names).sort(byRef);
  const planes = sources.planes.filter(names).sort(byRef);
  const unclaimed = <T extends Named>(claims: readonly T[], users: Named[]) => {
    const counted = new Set(users.map(refOf));
    return claims.filter((c) => names(c) && !counted.has(refOf(c))).sort(byRef);
  };
  return {
    clusters,
    planes,
    databaseServers: databaseServerUsers(name, sources).sort((a, b) =>
      byRef(a.server, b.server),
    ),
    clusterClaims: unclaimed(sources.clusterClaims, clusters),
    planeClaims: unclaimed(sources.planeClaims, planes),
    environments: defaultIn(
      sources.environments,
      "CamundaPlatformConfig",
      name,
    ),
  };
}

/** platformUserCount returns the number of users of every kind. */
export function platformUserCount(users: PlatformUsers): number {
  return (
    users.clusters.length +
    users.planes.length +
    users.databaseServers.length +
    users.clusterClaims.length +
    users.planeClaims.length +
    users.environments.length
  );
}

type Named = {
  metadata: { namespace?: string; name?: string };
  spec: { platformConfigRef?: string };
};

function databaseServerUsers(
  name: string,
  sources: PlatformUsageSources,
): { server: DatabaseServer; preset?: string }[] {
  return sources.databaseServers.flatMap((server) => {
    const preset = server.spec.presetRef
      ? sources.databasePresets.find(
          (p) => p.metadata.name === server.spec.presetRef,
        )
      : undefined;
    const merged = mergeDatabaseServer(preset?.spec, server.spec);
    if (merged.spec.platformConfigRef !== name) return [];
    return merged.source["spec.platformConfigRef"] === "preset" && preset
      ? [{ server, preset: preset.metadata.name ?? "" }]
      : [{ server }];
  });
}

function refOf(o: { metadata: { namespace?: string; name?: string } }): string {
  return `${o.metadata.namespace ?? ""}/${o.metadata.name ?? ""}`;
}

function byRef(
  a: { metadata: { namespace?: string; name?: string } },
  b: { metadata: { namespace?: string; name?: string } },
): number {
  return (
    byCodeUnit(a.metadata.namespace ?? "", b.metadata.namespace ?? "") ||
    byCodeUnit(a.metadata.name ?? "", b.metadata.name ?? "")
  );
}
