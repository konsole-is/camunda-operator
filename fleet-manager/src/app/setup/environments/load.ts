import "server-only";
import { SOURCE_KINDS } from "@/components/setup/environments/source-kinds";
import type { EnvironmentSources } from "@/components/setup/environments/sources";
import { listOrEmpty } from "@/lib/k8s/list-or-empty";

/**
 * loadEnvironments reads the first snapshot of the Environments page from
 * the cache of the server, in every namespace. A failed read of a kind
 * gives no objects of that kind.
 */
export async function loadEnvironments(): Promise<EnvironmentSources> {
  const lists = await Promise.all(
    Object.entries(SOURCE_KINDS).map(
      async ([key, kind]) => [key, await listOrEmpty(kind)] as const,
    ),
  );
  // A Resource is plain JSON, so it differs from its model type only in its
  // timestamps, which the Environments page does not read.
  return Object.fromEntries(lists) as unknown as EnvironmentSources;
}
