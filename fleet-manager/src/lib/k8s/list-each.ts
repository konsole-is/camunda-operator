import "server-only";
import type { KindName, Resource } from "./kinds";
import { list } from "./read";
import type { ReadError } from "./unread";

/**
 * listEach reads the list of each kind of kinds from the cache of the
 * server, in every namespace, under the key of the kind. A failed read
 * gives no objects of that kind and an entry in readErrors. readErrors has
 * the order of kinds.
 */
export async function listEach<T extends Record<string, KindName>>(
  kinds: T,
): Promise<{
  lists: { -readonly [K in keyof T]: Resource<T[K]>[] };
  readErrors: ReadError[];
}> {
  const reads = await Promise.all(
    Object.entries(kinds).map(async ([key, kind]) => {
      try {
        return { key, items: await list(kind) };
      } catch (err) {
        return {
          key,
          items: [],
          error: {
            kind,
            message: err instanceof Error ? err.message : String(err),
          },
        };
      }
    }),
  );
  return {
    lists: Object.fromEntries(
      reads.map((r) => [r.key, r.items]),
    ) as unknown as { -readonly [K in keyof T]: Resource<T[K]>[] },
    readErrors: reads.flatMap((r) => (r.error ? [r.error] : [])),
  };
}
