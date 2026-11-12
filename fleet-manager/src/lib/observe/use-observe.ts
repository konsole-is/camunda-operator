"use client";

import { useQuery } from "@tanstack/react-query";
import type { PodWarnings } from "@/lib/warnings";
import type {
  MetricsAnswer,
  MetricsRange,
  MetricsRef,
  PartitionHealth,
  PodUsage,
  Unavailable,
} from "./types";

const REFETCH_MS = 15_000;
const METRICS_REFETCH_MS = 30_000;

/**
 * usePodUsage polls /api/usage every 15 seconds for the usage of the pods in
 * namespace that selector selects. It is undefined until the first answer,
 * and "unavailable" when no metrics server answers, the route refuses, or
 * the fetch fails. It needs a QueryClientProvider above it.
 */
export function usePodUsage(
  namespace: string,
  selector: string,
): PodUsage[] | Unavailable | undefined {
  return usePolled(
    ["usage", namespace, selector],
    `/api/usage?${new URLSearchParams({ namespace, selector })}`,
    "usage",
  );
}

/**
 * usePartitions polls /api/partitions every 15 seconds for the partitions of
 * the cluster name in namespace. It is undefined until the first answer,
 * and "unavailable" when the management API does not answer, the route
 * refuses, or the fetch fails. It needs a QueryClientProvider above it.
 */
export function usePartitions(
  namespace: string,
  name: string,
): PartitionHealth[] | Unavailable | undefined {
  return usePolled(
    ["partitions", namespace, name],
    `/api/partitions?${new URLSearchParams({ namespace, name })}`,
    "partitions",
  );
}

/**
 * useWarnings polls /api/warnings every 15 seconds for the pod and metric
 * warnings of the fleet. It is undefined until the first answer, and
 * "unavailable" when the route refuses or the fetch fails. It needs a
 * QueryClientProvider above it.
 */
export function useWarnings(): PodWarnings | Unavailable | undefined {
  return usePolled(["warnings"], "/api/warnings", "warnings");
}

/**
 * useMetrics polls /api/metrics every 30 seconds for the charts of ref in
 * namespace over range. It is undefined until the first answer and while
 * ref is undefined, when it asks nothing. It is "failed" when the route
 * answers other than 200 or the fetch fails. It needs a
 * QueryClientProvider above it.
 */
export function useMetrics(
  namespace: string,
  ref: MetricsRef | undefined,
  range: MetricsRange,
): MetricsAnswer | "failed" | undefined {
  const params = new URLSearchParams({ namespace });
  if (ref && "request" in ref) {
    params.set("request", ref.request);
  } else if (ref) {
    params.set("cluster", ref.cluster);
    params.set("component", ref.component);
  }
  params.set("range", range);
  const url = `/api/metrics?${params}`;
  const { data } = useQuery({
    queryKey: ["metrics", url],
    queryFn: async (): Promise<MetricsAnswer | "failed"> => {
      try {
        const response = await fetch(url, { cache: "no-store" });
        if (response.status !== 200) return "failed";
        const body = (await response.json()) as { metrics?: MetricsAnswer };
        return body.metrics ?? "failed";
      } catch {
        return "failed";
      }
    },
    enabled: ref !== undefined,
    refetchInterval: METRICS_REFETCH_MS,
  });
  return data;
}

/**
 * useControllerPods polls /api/controller-pods every 15 seconds for the
 * namespace and the pod names of the controller of registration. It is
 * undefined until the first answer and while registration is undefined,
 * when it asks nothing. It is "unavailable" when the route refuses or the
 * fetch fails. It needs a QueryClientProvider above it.
 */
export function useControllerPods(
  registration: string | undefined,
): { namespace: string; pods: string[] } | Unavailable | undefined {
  return usePolled(
    ["controller-pods", registration ?? ""],
    `/api/controller-pods?${new URLSearchParams({ registration: registration ?? "" })}`,
    "controller",
    registration !== undefined,
  );
}

function usePolled<T>(
  queryKey: readonly string[],
  url: string,
  field: string,
  enabled = true,
): T | Unavailable | undefined {
  const { data } = useQuery({
    enabled,
    queryKey,
    queryFn: async (): Promise<T | Unavailable> => {
      try {
        const response = await fetch(url, { cache: "no-store" });
        if (response.status !== 200) return "unavailable";
        const body = (await response.json()) as Record<string, unknown>;
        return (body[field] as T | Unavailable | undefined) ?? "unavailable";
      } catch {
        return "unavailable";
      }
    },
    refetchInterval: REFETCH_MS,
  });
  return data;
}
