// This module runs in the browser and on the server.
import type { CamundaCluster } from "@/generated/crds/camundacluster";
import { byCodeUnit } from "@/lib/policy";

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
