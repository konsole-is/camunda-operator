"use client";

import { useState } from "react";
import { equalJSON } from "@/lib/policy";

/**
 * useEditorGeneration returns a key for editors that start from their
 * values and keep their own text. The key changes when the pending changes
 * of the page end without an edit through own, as after a discard or a
 * write, so that the editors show the values of the session again. It
 * also changes when value changes and no edit came through own since the
 * key last changed, as after a claim update. own wraps each edit of these
 * editors: an edit through it that takes back the last pending change keeps
 * them mounted.
 */
export function useEditorGeneration(
  pending: number,
  value?: unknown,
): {
  generation: number;
  own: <A extends unknown[]>(
    f: (...args: A) => unknown,
  ) => (...args: A) => void;
} {
  const [generation, setGeneration] = useState(0);
  const [edits, setEdits] = useState(0);
  const [seen, setSeen] = useState({
    pending: pending > 0,
    edits,
    value,
    touched: false,
  });
  const moved = !equalJSON(seen.value, value);
  if (seen.pending !== pending > 0 || seen.edits !== edits || moved) {
    const touched = seen.touched || seen.edits !== edits;
    const restart =
      (pending === 0 && seen.pending && seen.edits === edits) ||
      (moved && !touched);
    if (restart) setGeneration((g) => g + 1);
    setSeen({
      pending: pending > 0,
      edits,
      value,
      touched: touched && !restart,
    });
  }
  const own =
    <A extends unknown[]>(f: (...args: A) => unknown) =>
    (...args: A) => {
      setEdits((n) => n + 1);
      f(...args);
    };
  return { generation, own };
}
