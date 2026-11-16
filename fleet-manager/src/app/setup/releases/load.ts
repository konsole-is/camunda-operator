import "server-only";
import type { RolloutLists } from "@/components/setup/releases/rollout-sources";
import {
  RELEASE_SOURCE_KINDS,
  ROLLOUT_SOURCE_KINDS,
} from "@/components/setup/releases/source-kinds";
import type { ReleasesData } from "@/components/setup/releases/sources";
import { listOrEmpty } from "@/lib/k8s/list-or-empty";

/**
 * loadReleases reads the first snapshot of the release pages from the cache
 * of the server, in every namespace. A failed read of a kind gives no
 * objects of that kind.
 */
export async function loadReleases(): Promise<ReleasesData> {
  const lists = await Promise.all(
    Object.entries(RELEASE_SOURCE_KINDS).map(
      async ([key, kind]) => [key, await listOrEmpty(kind)] as const,
    ),
  );
  // A Resource is plain JSON, so it differs from its model type only in its
  // timestamps, which the release pages do not read.
  return Object.fromEntries(lists) as unknown as ReleasesData;
}

/**
 * loadRolloutLists reads the first snapshot of the lists that only the
 * Rollout tab checks, as loadReleases does.
 */
export async function loadRolloutLists(): Promise<RolloutLists> {
  const lists = await Promise.all(
    Object.entries(ROLLOUT_SOURCE_KINDS).map(
      async ([key, kind]) => [key, await listOrEmpty(kind)] as const,
    ),
  );
  return Object.fromEntries(lists) as unknown as RolloutLists;
}
