// This module runs in the browser and on the server.
import type { V1Namespace } from "@kubernetes/client-node";
import type { CamundaCluster } from "@/generated/crds/camundacluster";
import type { Environment } from "@/generated/crds/environment";
import { byCodeUnit, environmentOf } from "@/lib/policy";

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
    usage.set(preset, [...(usage.get(preset) ?? []), ref]);
  }
  for (const refs of usage.values()) refs.sort(byCodeUnit);
  return usage;
}

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
