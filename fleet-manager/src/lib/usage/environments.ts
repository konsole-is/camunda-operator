// This module runs in the browser and on the server.
import type { V1Namespace } from "@kubernetes/client-node";
import type { Environment } from "@/generated/crds/environment";
import { byCodeUnit, environmentOf } from "@/lib/policy";

/** EnvironmentCount is the number of namespaces and of clusters in one environment. */
export type EnvironmentCount = { namespaces: number; clusters: number };

/**
 * environmentCounts returns the count of each environment, zeros included.
 * A namespace counts for the Environment that its label names. A cluster
 * counts for the environment of its namespace, so a namespace that is not
 * in namespaces counts none of its clusters.
 */
export function environmentCounts(
  environments: readonly Environment[],
  namespaces: readonly V1Namespace[],
  clusters: readonly { metadata: { namespace?: string } }[],
): Record<string, EnvironmentCount> {
  const counts: Record<string, EnvironmentCount> = {};
  for (const environment of environments) {
    counts[environment.metadata.name ?? ""] = { namespaces: 0, clusters: 0 };
  }
  const environmentOfNamespace = new Map<string, string>();
  for (const namespace of namespaces) {
    const name = environmentOf(namespace, environments).environment?.metadata
      .name;
    if (name === undefined) continue;
    counts[name].namespaces++;
    environmentOfNamespace.set(namespace.metadata?.name ?? "", name);
  }
  for (const cluster of clusters) {
    const name = environmentOfNamespace.get(cluster.metadata.namespace ?? "");
    if (name !== undefined) counts[name].clusters++;
  }
  return counts;
}

/** EnvironmentListedKind is a kind whose objects an Environment lists as allowed. */
export type EnvironmentListedKind =
  | "CamundaClusterPreset"
  | "ElasticsearchClusterPreset"
  | "DatabaseServerPreset"
  | "CamundaRelease";

/** EnvironmentDefaultKind is a kind of which an Environment names one default. */
export type EnvironmentDefaultKind = "CamundaRelease" | "CamundaPlatformConfig";

/**
 * allowedIn returns the names of the environments whose spec lists name for
 * kind: spec.presets.camundaCluster, .elasticsearchCluster, .databaseServer,
 * or spec.releases.allowed. The names are sorted by code unit. An empty name
 * matches nothing.
 */
export function allowedIn(
  environments: readonly Environment[],
  kind: EnvironmentListedKind,
  name: string,
): string[] {
  if (name === "") return [];
  return namesWhere(environments, (spec) =>
    listedOf(spec, kind).includes(name),
  );
}

/**
 * defaultIn returns the names of the environments whose default for kind is
 * name: spec.releases.default or spec.platformConfigRef, sorted by code unit.
 * An empty name matches nothing.
 */
export function defaultIn(
  environments: readonly Environment[],
  kind: EnvironmentDefaultKind,
  name: string,
): string[] {
  if (name === "") return [];
  return namesWhere(
    environments,
    (spec) =>
      (kind === "CamundaRelease"
        ? spec.releases?.default
        : spec.platformConfigRef) === name,
  );
}

function listedOf(
  spec: Environment["spec"],
  kind: EnvironmentListedKind,
): readonly string[] {
  switch (kind) {
    case "CamundaClusterPreset":
      return spec.presets?.camundaCluster ?? [];
    case "ElasticsearchClusterPreset":
      return spec.presets?.elasticsearchCluster ?? [];
    case "DatabaseServerPreset":
      return spec.presets?.databaseServer ?? [];
    case "CamundaRelease":
      return spec.releases?.allowed ?? [];
  }
}

function namesWhere(
  environments: readonly Environment[],
  matches: (spec: Environment["spec"]) => boolean,
): string[] {
  return environments
    .filter((environment) => matches(environment.spec))
    .map((environment) => environment.metadata.name ?? "")
    .sort(byCodeUnit);
}
