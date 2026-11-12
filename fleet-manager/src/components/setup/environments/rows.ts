import type { V1Namespace } from "@kubernetes/client-node";
import type { ConditionLike } from "@/components/fleet/status";
import type { FleetEnvironmentConditionType } from "@/generated/conditions";
import { ENVIRONMENT_LABEL } from "@/lib/labels";
import { byCodeUnit, environmentOf } from "@/lib/policy";
import type { EnvironmentSources } from "./sources";

const READY: FleetEnvironmentConditionType = "Ready";

/** EnvironmentRow is one environment of the list. */
export type EnvironmentRow = {
  name: string;
  displayName?: string;
  color?: string;
  namespaces: number;
  clusters: number;
  planes: number;
  ready?: ConditionLike;
};

/**
 * environmentRows returns one row for each environment by name. namespaces
 * is status.namespaces, or the count of labelled namespaces while the status
 * has none. clusters and planes count the claims of each kind in the
 * labelled namespaces.
 */
export function environmentRows(s: EnvironmentSources): EnvironmentRow[] {
  return s.environments
    .map((environment): EnvironmentRow => {
      const name = environment.metadata.name ?? "";
      const labelled = new Set(labelledNamespaces(name, s.namespaces));
      const inLabelled = (claim: { metadata: { namespace?: string } }) =>
        labelled.has(claim.metadata.namespace ?? "");
      const ready = environment.status?.conditions?.find(
        (c) => c.type === READY,
      );
      return {
        name,
        ...(environment.spec.displayName && {
          displayName: environment.spec.displayName,
        }),
        ...(environment.spec.color && { color: environment.spec.color }),
        namespaces: environment.status?.namespaces ?? labelled.size,
        clusters: s.clusterClaims.filter(inLabelled).length,
        planes: s.planeClaims.filter(inLabelled).length,
        ...(ready && { ready }),
      };
    })
    .sort((a, b) => byCodeUnit(a.name, b.name));
}

/**
 * UnassignedNamespace is a namespace that holds a claim or a cluster of
 * either kind and has no environment, with the reason of environmentOf.
 */
export type UnassignedNamespace = {
  name: string;
  reason: "NoEnvironment" | "UnknownEnvironment";
};

/** unassignedNamespaces returns them by name. It returns none while no Environment exists. */
export function unassignedNamespaces(
  s: EnvironmentSources,
): UnassignedNamespace[] {
  if (s.environments.length === 0) return [];
  const holding = new Set(
    [...s.clusterClaims, ...s.planeClaims, ...s.clusters, ...s.planes].map(
      (o) => o.metadata?.namespace ?? "",
    ),
  );
  return s.namespaces
    .flatMap((namespace): UnassignedNamespace[] => {
      const name = namespace.metadata?.name ?? "";
      if (!holding.has(name)) return [];
      const { reason } = environmentOf(namespace, s.environments);
      return reason ? [{ name, reason }] : [];
    })
    .sort((a, b) => byCodeUnit(a.name, b.name));
}

/** labelledNamespaces returns the names of the namespaces whose label names environment, sorted. */
export function labelledNamespaces(
  environment: string,
  namespaces: readonly V1Namespace[],
): string[] {
  if (environment === "") return [];
  return namespaces
    .filter((n) => n.metadata?.labels?.[ENVIRONMENT_LABEL] === environment)
    .map((n) => n.metadata?.name ?? "")
    .sort(byCodeUnit);
}

/** namespacesToAdd returns the namespaces without a deletionTimestamp whose environment label is absent or empty, sorted. */
export function namespacesToAdd(namespaces: readonly V1Namespace[]): string[] {
  return namespaces
    .filter(
      (n) =>
        !n.metadata?.deletionTimestamp &&
        !n.metadata?.labels?.[ENVIRONMENT_LABEL],
    )
    .map((n) => n.metadata?.name ?? "")
    .sort(byCodeUnit);
}
