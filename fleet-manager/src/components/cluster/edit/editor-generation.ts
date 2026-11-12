"use client";

import { useState } from "react";

/**
 * useEditorGeneration returns a key for editors that start from their
 * values and keep their own text. The key changes when the pending changes
 * of the page end without an edit through own, as after a discard or a
 * write, so that the editors show the values of the session again. own
 * wraps a set or a reset of these editors: an edit through it that takes
 * back the last pending change keeps them mounted.
 */
export function useEditorGeneration(pending: number): {
  generation: number;
  own: <A extends unknown[]>(
    f: (...args: A) => unknown,
  ) => (...args: A) => void;
} {
  const [generation, setGeneration] = useState(0);
  const [edits, setEdits] = useState(0);
  const [seen, setSeen] = useState({ pending: pending > 0, edits });
  if (seen.pending !== pending > 0 || seen.edits !== edits) {
    if (pending === 0 && seen.pending && seen.edits === edits)
      setGeneration((g) => g + 1);
    setSeen({ pending: pending > 0, edits });
  }
  const own =
    <A extends unknown[]>(f: (...args: A) => unknown) =>
    (...args: A) => {
      setEdits((n) => n + 1);
      f(...args);
    };
  return { generation, own };
}
