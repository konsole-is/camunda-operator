"use client";

import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { REVISION_ANNOTATION } from "@/lib/labels";
import { refKey } from "@/lib/write/refs";
import type { Change, WriteResult } from "@/lib/write/types";

/** SetupObject is the part of a cluster-scoped object that useSetupEditor reads. */
export type SetupObject = {
  kind: string;
  metadata: {
    name?: string;
    resourceVersion?: string;
    annotations?: Record<string, string>;
  };
};

/** SetupReview is the change under review, a save or a delete. */
export type SetupReview = { change: Change; kind: "save" | "delete" };

/**
 * useSetupEditor holds the form state of the editor of one cluster-scoped
 * object of a Setup page. live is the object of the live list, undefined
 * for a new object and until the list shows it. base is the copy that the
 * draft started from, so a save of a stale draft fails with Stale. The form
 * is locked while a change is under review and after an ok write: after a
 * save until the list shows the saved copy, in Git mode the pushed revision;
 * after a create until the page moves to hrefOf of the new name, once the
 * list shows it. An ok delete moves the page to listHref. onReset runs on
 * each reset of the draft, and generation changes with it.
 */
export function useSetupEditor<T extends SetupObject, D>({
  live,
  isNew,
  listed,
  draftOf,
  listHref,
  hrefOf,
  onReset,
}: {
  live: T | undefined;
  isNew: boolean;
  /** listed are the names of the live list. */
  listed: readonly string[];
  draftOf: (object: T | undefined) => D;
  listHref: string;
  hrefOf: (name: string) => string;
  onReset?: () => void;
}): {
  base: T | undefined;
  draft: D;
  setDraft: (update: (draft: D) => D) => void;
  generation: number;
  reviewing: SetupReview | undefined;
  locked: boolean;
  review: (review: SetupReview) => void;
  cancel: () => void;
  discard: () => void;
  written: (result: WriteResult, name: string) => void;
} {
  const router = useRouter();
  const [base, setBase] = useState<T | undefined>(live);
  const [draft, setDraftState] = useState<D>(() => draftOf(live));
  const [generation, setGeneration] = useState(0);
  const [reviewing, setReviewing] = useState<SetupReview>();
  const [waiting, setWaiting] = useState<{ revision: string } | "save">();
  const [created, setCreated] = useState<string>();
  const [deleted, setDeleted] = useState(false);

  const reset = (to: T | undefined) => {
    setBase(to);
    setDraftState(draftOf(to));
    setGeneration((g) => g + 1);
    setReviewing(undefined);
    setWaiting(undefined);
    onReset?.();
  };
  // The first list can miss the object that the page found by name.
  if (!isNew && live && !base) reset(live);
  if (
    typeof waiting === "object" &&
    live?.metadata.annotations?.[REVISION_ANNOTATION] === waiting.revision
  )
    reset(live);
  if (
    waiting === "save" &&
    live &&
    live.metadata.resourceVersion !== base?.metadata.resourceVersion
  )
    reset(live);

  const createdShown = created !== undefined && listed.includes(created);
  useEffect(() => {
    if (createdShown) router.push(hrefOf(created));
  }, [createdShown, created, hrefOf, router]);
  useEffect(() => {
    if (deleted) router.push(listHref);
  }, [deleted, listHref, router]);

  return {
    base,
    draft,
    setDraft: (update) => setDraftState(update),
    generation,
    reviewing,
    locked:
      reviewing !== undefined ||
      waiting !== undefined ||
      created !== undefined ||
      deleted,
    review: setReviewing,
    cancel: () => setReviewing(undefined),
    discard: () => reset(live),
    written: (result, name) => {
      const kind = reviewing?.kind;
      setReviewing(undefined);
      if (!result.ok) return;
      if (kind === "delete") setDeleted(true);
      else if (isNew) setCreated(name);
      else {
        const revision =
          result.mode === "git"
            ? result.revisions[refKey({ kind: base?.kind ?? "", name })]
            : undefined;
        setWaiting(revision ? { revision } : "save");
      }
    },
  };
}
