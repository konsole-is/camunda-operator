"use client";
import { useState } from "react";
import { useWatchesFresh } from "./hooks";
import type { KindName } from "./kinds";

/**
 * useAnswered gives the kinds whose watch sent a snapshot at least once. A
 * kind stays in it after a reconnect, so a failed first read does not come
 * back then. Like useWatchesFresh, it opens no watch of its own.
 */
export function useAnswered(kinds: readonly KindName[]): ReadonlySet<string> {
  const fresh = useWatchesFresh(kinds.map((kind) => ({ kind, opts: {} })));
  const [answered, setAnswered] = useState<ReadonlySet<string>>(new Set());
  const added = kinds.filter((kind, i) => fresh[i] && !answered.has(kind));
  if (added.length > 0) setAnswered(new Set([...answered, ...added]));
  return answered;
}
