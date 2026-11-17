import "server-only";
import type { z } from "zod";
import type { KindName, Resource } from "@/lib/k8s/kinds";
import { listEach } from "@/lib/k8s/list-each";
import { type DeleteBlock, deleteRefusalText } from "@/lib/usage";
import type { ApiError, Change, ObjectRef } from "./types";

/** DeleteLists holds the list of each kind of kinds under its key. */
export type DeleteLists<M extends Readonly<Record<string, KindName>>> = {
  -readonly [K in keyof M]: Resource<M[K]>[];
};

/**
 * DeleteRule is the delete guard of the kinds of one page. needs gives the
 * kinds that the rule reads for the delete of ref. blockOf gets the lists of
 * kinds and the kinds that could not be read, and gives the block of each
 * delete, or undefined when the delete is possible.
 */
export type DeleteRule<M extends Readonly<Record<string, KindName>>> = {
  kinds: M;
  needs: (ref: ObjectRef) => readonly KindName[];
  blockOf: (
    lists: DeleteLists<M>,
    unread: readonly string[],
  ) => (ref: ObjectRef) => DeleteBlock | undefined;
};

/**
 * deleteRefusals returns a Forbidden error for each delete of change that
 * rule blocks. It returns none for a change that schema refuses, so that the
 * writer gives its Invalid errors. It reads from the cache of the server only
 * the kinds that rule needs for the deletes, and every other list is empty.
 * When rule throws, each delete is refused.
 */
export async function deleteRefusals<
  M extends Readonly<Record<string, KindName>>,
>(
  change: Change,
  schema: z.ZodType<Change>,
  rule: DeleteRule<M>,
): Promise<ApiError[]> {
  const parsed = schema.safeParse(change);
  const deletes = parsed.success ? (parsed.data.delete ?? []) : [];
  if (deletes.length === 0) return [];
  const needed = new Set(deletes.flatMap(rule.needs));
  const read = await listEach(
    Object.fromEntries(
      Object.entries(rule.kinds).filter(([, kind]) => needed.has(kind)),
    ),
  );
  const lists = Object.fromEntries(
    Object.keys(rule.kinds).map((key) => [key, read.lists[key] ?? []]),
  ) as DeleteLists<M>;
  const unread = read.readErrors.map((e) => e.kind);
  const refusal = (ref: ObjectRef, message: string): ApiError => ({
    code: "Forbidden",
    message,
    ref,
  });
  try {
    const blockOf = rule.blockOf(lists, unread);
    return deletes.flatMap((ref) => {
      const block = blockOf(ref);
      return block ? [refusal(ref, deleteRefusalText(block))] : [];
    });
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err);
    return deletes.map((ref) =>
      refusal(
        ref,
        `Could not check the users of ${ref.kind} ${ref.name}: ${message}`,
      ),
    );
  }
}
