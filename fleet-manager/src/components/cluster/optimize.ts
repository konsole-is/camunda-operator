import type { Resource } from "@/lib/k8s/kinds";

/**
 * clusterOptimize returns the CamundaOptimize whose clusterRef names the
 * cluster, or undefined. When more than one names it, it returns the one
 * that holds the cluster, as the operator picks it: the oldest, then the
 * first by name.
 */
export function clusterOptimize(
  clusterName: string,
  items: readonly Resource<"CamundaOptimize">[],
): Resource<"CamundaOptimize"> | undefined {
  return items
    .filter((o) => o.spec.clusterRef.name === clusterName)
    .sort(
      (a, b) =>
        String(a.metadata?.creationTimestamp ?? "").localeCompare(
          String(b.metadata?.creationTimestamp ?? ""),
        ) || (a.metadata?.name ?? "").localeCompare(b.metadata?.name ?? ""),
    )[0];
}
