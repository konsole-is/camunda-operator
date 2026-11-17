/** ReadError is the error of the read or the watch of one kind. */
export type ReadError = { kind: string; message: string };

/**
 * SourceReads says which lists of a page are not read: errors holds the
 * error of each failed read, and stale the kind of each list whose watch is
 * not fresh (freshKey).
 */
export type SourceReads = {
  errors?: readonly ReadError[];
  stale?: readonly string[];
};

/** unreadKinds returns each kind of reads that failed or is stale, once. */
export function unreadKinds(reads: SourceReads): string[] {
  const kinds = [
    ...(reads.errors ?? []).map((e) => e.kind),
    ...(reads.stale ?? []),
  ];
  return [...new Set(kinds)];
}

/**
 * currentReadErrors returns the errors that a page shows: each error of
 * live, then each error of the read of the server whose kind has no error in
 * live and is not in answered (useAnswered). A failed server read so stays
 * an error until the watch of its kind sends a list.
 */
export function currentReadErrors(
  live: readonly ReadError[],
  server: readonly ReadError[] | undefined,
  answered: ReadonlySet<string>,
): ReadError[] {
  const first = (server ?? []).filter(
    (e) => !answered.has(e.kind) && !live.some((l) => l.kind === e.kind),
  );
  return [...live, ...first];
}

/** unreadDeleteText says why a delete is off while the lists of kinds are not read. */
export function unreadDeleteText(kinds: readonly string[]): string {
  return `Delete is possible when the fleet manager can read ${kinds.join(", ")}.`;
}

/** unreadChangeText says why a change is off while the lists of kinds are not read. */
export function unreadChangeText(kinds: readonly string[]): string {
  return `A change is possible when the fleet manager can read ${kinds.join(", ")}.`;
}

/** unknownUsersText says that the users of an object are not known while the lists of kinds are not read. */
export function unknownUsersText(kinds: readonly string[]): string {
  return `The users are not known while the fleet manager cannot read ${kinds.join(", ")}.`;
}
