"use client";

import { useEffect, useRef, useState } from "react";
import type { Change, Preview, WriteResult } from "@/lib/write/types";

/**
 * useCreateReview holds the dry run and the confirm of the review step of a
 * create flow. It previews change once for each change, and a rejected
 * preview counts as an Unavailable refusal. refusals are the messages of
 * the preview of the current change. pending is true while that preview
 * is missing or has refusals, while a write runs, and while blocked.
 * confirm does nothing while pending. Else it sends change to create once,
 * also on a second call before the first one ends, and gives the result to
 * onWritten, also when the write is refused or create throws.
 */
export function useCreateReview(input: {
  change: Change;
  preview: (change: Change) => Promise<Preview>;
  create: (change: Change) => Promise<WriteResult>;
  onWritten: (result: WriteResult) => void;
  blocked?: boolean;
}): { refusals: string[]; pending: boolean; confirm: () => Promise<void> } {
  const { change, create, onWritten, blocked = false } = input;
  const key = JSON.stringify(change);
  const [previewed, setPreviewed] = useState<{
    key: string;
    result: Preview;
  }>();
  const [writing, setWriting] = useState(false);
  // A second click arrives before React disables the button.
  const writingNow = useRef(false);
  const previewRef = useRef(input.preview);
  useEffect(() => {
    previewRef.current = input.preview;
  });

  useEffect(() => {
    let current = true;
    previewRef
      .current(JSON.parse(key) as Change)
      .catch((error: unknown): Preview => ({
        objects: [],
        errors: [{ code: "Unavailable", message: String(error) }],
      }))
      .then((result) => {
        if (current) setPreviewed({ key, result });
      });
    return () => {
      current = false;
    };
  }, [key]);

  const checked = previewed?.key === key ? previewed.result : undefined;
  const refusals = checked?.errors.map((e) => e.message) ?? [];
  const pending = writing || blocked || !checked || refusals.length > 0;

  async function confirm() {
    if (writingNow.current || pending) return;
    writingNow.current = true;
    setWriting(true);
    try {
      const result = await create(change).catch(
        (error: unknown): WriteResult => ({
          ok: false,
          errors: [{ code: "Unavailable", message: String(error) }],
        }),
      );
      onWritten(result);
    } finally {
      writingNow.current = false;
      setWriting(false);
    }
  }

  return { refusals, pending, confirm };
}
