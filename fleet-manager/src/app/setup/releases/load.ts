import "server-only";
import type { RolloutSnapshot } from "@/components/setup/releases/rollout-sources";
import {
  RELEASE_SOURCE_KINDS,
  ROLLOUT_SOURCE_KINDS,
} from "@/components/setup/releases/source-kinds";
import type { ReleasesSnapshot } from "@/components/setup/releases/sources";
import { loadSnapshot } from "@/lib/k8s/snapshot";

/**
 * loadReleases reads the first snapshot of the release pages, as
 * loadSnapshot does.
 */
export async function loadReleases(): Promise<ReleasesSnapshot> {
  // A Resource is plain JSON, so it differs from its model type only in its
  // timestamps, which the release pages do not read.
  return (await loadSnapshot(
    RELEASE_SOURCE_KINDS,
  )) as unknown as ReleasesSnapshot;
}

/**
 * loadRolloutLists reads the first snapshot of the lists that only the
 * Rollout tab checks, as loadReleases does.
 */
export async function loadRolloutLists(): Promise<RolloutSnapshot> {
  return (await loadSnapshot(
    ROLLOUT_SOURCE_KINDS,
  )) as unknown as RolloutSnapshot;
}
