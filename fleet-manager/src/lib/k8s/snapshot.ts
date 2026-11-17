import "server-only";
import type { KindName, Resource } from "./kinds";
import { list } from "./read";
import type { ReadError } from "./unread";

/** Snapshot holds the list of each kind of kinds under its key, and readErrors. */
export type Snapshot<M extends Readonly<Record<string, KindName>>> = {
  -readonly [K in keyof M]: Resource<M[K]>[];
} & { readErrors: ReadError[] };

/**
 * loadSnapshot reads the first snapshot of a page from the cache of the
 * server: the list of each kind of kinds, in every namespace. A failed
 * read gives no objects of that kind and an entry in readErrors, in the
 * order of kinds.
 */
export async function loadSnapshot<
  M extends Readonly<Record<string, KindName>>,
>(kinds: M): Promise<Snapshot<M>> {
  const reads = await Promise.all(
    Object.entries(kinds).map(async ([key, kind]) => {
      try {
        return { key, items: await list(kind) };
      } catch (err) {
        const message = err instanceof Error ? err.message : String(err);
        return { key, items: [], error: { kind, message } };
      }
    }),
  );
  return {
    ...Object.fromEntries(reads.map((r) => [r.key, r.items])),
    readErrors: reads.flatMap((r) => (r.error ? [r.error] : [])),
  } as Snapshot<M>;
}
