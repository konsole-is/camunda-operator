import "server-only";
import { CONNECTION_SOURCE_KINDS } from "@/components/setup/connections/source-kinds";
import type { ConnectionSources } from "@/components/setup/connections/sources";
import { listOrEmpty } from "@/lib/k8s/list-or-empty";

/**
 * loadConnections reads the first snapshot of the Connections pages from
 * the cache of the server, in every namespace. A failed read of a kind
 * gives no objects of that kind.
 */
export async function loadConnections(): Promise<ConnectionSources> {
  const lists = await Promise.all(
    Object.entries(CONNECTION_SOURCE_KINDS).map(
      async ([key, kind]) => [key, await listOrEmpty(kind)] as const,
    ),
  );
  // A Resource is plain JSON, so it differs from its model type only in its
  // timestamps, which the Connections pages do not read.
  return Object.fromEntries(lists) as unknown as ConnectionSources;
}
