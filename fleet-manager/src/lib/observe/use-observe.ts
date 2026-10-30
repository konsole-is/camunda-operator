"use client";

import { useQuery } from "@tanstack/react-query";
import type { PartitionHealth, PodUsage, Unavailable } from "./types";

const REFETCH_MS = 15_000;

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

function usePolled<T>(
  queryKey: readonly string[],
  url: string,
  field: string,
): T | Unavailable | undefined {
  const { data } = useQuery({
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
