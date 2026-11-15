// This module runs in the browser and on the server.
import type { CamundaCluster } from "@/generated/crds/camundacluster";
import {
  BUILTIN_OPTIONS,
  byCodeUnit,
  decodeObject,
  nameOf,
  own,
} from "@/lib/policy";

/**
 * presetUsage returns, for each preset that a cluster uses, the
 * <namespace>/<name> of each cluster whose spec.presetRef names it, sorted.
 * A preset that no cluster uses has no entry. Usage comes from the presetRef
 * of each cluster (contract 6.5, D16).
 */
export function presetUsage(
  clusters: readonly CamundaCluster[],
): Map<string, string[]> {
  const usage = new Map<string, string[]>();
  for (const cluster of clusters) {
    const preset = cluster.spec.presetRef;
    if (!preset) continue;
    const ref = `${cluster.metadata.namespace ?? ""}/${cluster.metadata.name ?? ""}`;
    const refs = usage.get(preset);
    if (refs) refs.push(ref);
    else usage.set(preset, [ref]);
  }
  for (const refs of usage.values()) refs.sort(byCodeUnit);
  return usage;
}

/** PresetKind is a kind of preset. */
export type PresetKind =
  | "CamundaClusterPreset"
  | "ElasticsearchClusterPreset"
  | "DatabaseServerPreset";

/** PresetUserKind is a kind of object that can name a preset, in the order that presetUsers sorts by. */
export type PresetUserKind =
  | "CamundaCluster"
  | "ElasticsearchCluster"
  | "DatabaseServer"
  | "CamundaClusterClaim"
  | "CamundaManagementClusterClaim"
  | "PluginRequest";

/** PresetUser is one object that names a preset. */
export type PresetUser = {
  kind: PresetUserKind;
  namespace: string;
  name: string;
};

type Named = { metadata: { namespace?: string; name?: string } };
type Selecting = Named & {
  spec: { plugins?: { registration: string; parameters?: unknown }[] };
};
type Referring = Named & { spec: { presetRef?: string } };

/** PresetUsageSources holds the lists that presetUsers reads, in all namespaces. */
export type PresetUsageSources = {
  clusters: readonly Referring[];
  elasticsearchClusters: readonly Referring[];
  databaseServers: readonly Referring[];
  clusterClaims: readonly (Selecting & Referring)[];
  planeClaims: readonly Selecting[];
  requests: readonly (Named & {
    spec: { registration: string; parameters?: unknown };
  })[];
};

const USER_ORDER: readonly PresetUserKind[] = [
  "CamundaCluster",
  "ElasticsearchCluster",
  "DatabaseServer",
  "CamundaClusterClaim",
  "CamundaManagementClusterClaim",
  "PluginRequest",
];

/**
 * presetResources returns the resources of the kind that kind is the preset
 * of (CamundaCluster, ElasticsearchCluster, or DatabaseServer) whose
 * spec.presetRef is name. They are sorted by namespace, then name, by code
 * unit. An empty name matches nothing.
 */
export function presetResources(
  kind: PresetKind,
  name: string,
  sources: PresetUsageSources,
): PresetUser[] {
  if (name === "") return [];
  const [userKind, list] = resourcesOf(kind, sources);
  return sortUsers(
    list
      .filter((r) => r.spec.presetRef === name)
      .map((r) => userOf(userKind, r)),
  );
}

/**
 * presetUsers returns every object that names the preset name of kind: the
 * resources of presetResources; for a cluster preset, the cluster claims
 * whose spec.presetRef is name; each claim of either kind with a selection,
 * and each request, of a built-in registration whose preset parameter lists
 * kind, when that parameter is name. A claim names a preset this way before
 * its resource exists. Each object is in the list once, sorted by kind in
 * the order of PresetUserKind, then namespace, then name, by code unit. An
 * empty name matches nothing.
 */
export function presetUsers(
  kind: PresetKind,
  name: string,
  sources: PresetUsageSources,
): PresetUser[] {
  if (name === "") return [];
  const users = presetResources(kind, name, sources);
  const names = (registration: string, parameters: unknown) =>
    own(BUILTIN_OPTIONS, registration)?.lists.preset === kind &&
    nameOf(decodeObject(parameters)?.preset) === name;
  const selects = (claim: Selecting) =>
    (claim.spec.plugins ?? []).some((s) => names(s.registration, s.parameters));
  for (const claim of sources.clusterClaims) {
    const byRef =
      kind === "CamundaClusterPreset" && claim.spec.presetRef === name;
    if (byRef || selects(claim))
      users.push(userOf("CamundaClusterClaim", claim));
  }
  for (const claim of sources.planeClaims) {
    if (selects(claim))
      users.push(userOf("CamundaManagementClusterClaim", claim));
  }
  for (const request of sources.requests) {
    if (names(request.spec.registration, request.spec.parameters))
      users.push(userOf("PluginRequest", request));
  }
  return sortUsers(users);
}

function resourcesOf(
  kind: PresetKind,
  sources: PresetUsageSources,
): [PresetUserKind, readonly Referring[]] {
  switch (kind) {
    case "CamundaClusterPreset":
      return ["CamundaCluster", sources.clusters];
    case "ElasticsearchClusterPreset":
      return ["ElasticsearchCluster", sources.elasticsearchClusters];
    case "DatabaseServerPreset":
      return ["DatabaseServer", sources.databaseServers];
  }
}

function userOf(kind: PresetUserKind, object: Named): PresetUser {
  return {
    kind,
    namespace: object.metadata.namespace ?? "",
    name: object.metadata.name ?? "",
  };
}

function sortUsers(users: PresetUser[]): PresetUser[] {
  const seen = new Set<string>();
  return users
    .filter((u) => {
      const key = `${u.kind}/${u.namespace}/${u.name}`;
      if (seen.has(key)) return false;
      seen.add(key);
      return true;
    })
    .sort(
      (a, b) =>
        USER_ORDER.indexOf(a.kind) - USER_ORDER.indexOf(b.kind) ||
        byCodeUnit(a.namespace, b.namespace) ||
        byCodeUnit(a.name, b.name),
    );
}
