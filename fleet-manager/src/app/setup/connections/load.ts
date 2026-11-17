import "server-only";
import type { ReadError } from "@/lib/k8s/unread";
import { CONNECTION_SOURCE_KINDS } from "@/components/setup/connections/source-kinds";
import type { ConnectionPageData } from "@/components/setup/connections/sources";
import { list } from "@/lib/k8s/read";

/**
 * loadConnections reads the first snapshot of the Connections pages from
 * the cache of the server, in every namespace. A failed read of a kind
 * gives no objects of that kind and an entry in readErrors.
 */
export async function loadConnections(): Promise<ConnectionPageData> {
  const readErrors: ReadError[] = [];
  const lists = await Promise.all(
    Object.entries(CONNECTION_SOURCE_KINDS).map(
      async ([key, kind]) =>
        [
          key,
          await list(kind).catch((err: unknown) => {
            readErrors.push({
              kind,
              message: err instanceof Error ? err.message : String(err),
            });
            return [];
          }),
        ] as const,
    ),
  );
  // A Resource is plain JSON, so it differs from its model type only in its
  // timestamps, which the Connections pages do not read.
  return {
    ...(Object.fromEntries(lists) as unknown as ConnectionPageData),
    readErrors,
  };
}
