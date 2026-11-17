import "server-only";
import { SOURCE_KINDS } from "@/components/setup/environments/source-kinds";
import type { EnvironmentSnapshot } from "@/components/setup/environments/sources";
import { loadSnapshot } from "@/lib/k8s/snapshot";

/**
 * loadEnvironments reads the first snapshot of the Environments pages, as
 * loadSnapshot does.
 */
export async function loadEnvironments(): Promise<EnvironmentSnapshot> {
  // A Resource is plain JSON, so it differs from its model type only in its
  // timestamps, which the Environments pages do not read.
  return (await loadSnapshot(SOURCE_KINDS)) as unknown as EnvironmentSnapshot;
}
