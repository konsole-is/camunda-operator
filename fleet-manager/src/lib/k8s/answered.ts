"use client";
import { useState } from "react";
import { useWatchesFresh } from "./hooks";
import type { KindName } from "./kinds";
import type { ListOptions } from "./read";

/** AnsweredList is a list that useAnswered follows: a kind with no filter, or a kind with the options of its hook. */
export type AnsweredList =
  KindName | { kind: KindName; opts: ListOptions & { name?: string } };

/**
 * useAnswered gives the kinds of lists whose watch sent a snapshot at least
 * once. A kind stays in it after a reconnect, so a failed first read does
 * not come back then. Like useWatchesFresh, it opens no watch of its own.
 */
export function useAnswered(
  lists: readonly AnsweredList[],
): ReadonlySet<string> {
  const followed = lists.map((list) =>
    typeof list === "string" ? { kind: list, opts: {} } : list,
  );
  const fresh = useWatchesFresh(followed);
  const [answered, setAnswered] = useState<ReadonlySet<string>>(new Set());
  const added = followed
    .filter(({ kind }, i) => fresh[i] && !answered.has(kind))
    .map(({ kind }) => kind);
  if (added.length > 0) setAnswered(new Set([...answered, ...added]));
  return answered;
}
