"use client";

import { useMemo } from "react";
import type { DiffLine } from "@/lib/diff";
import type { Step } from "@/lib/fields";
import { CLAIM_LABEL } from "@/lib/labels";
import {
  openSession,
  resetValue,
  setValue,
  type ClaimObject,
  type Session,
} from "@/lib/session";
import type { Preview, WriteMode, WriteResult } from "@/lib/write/types";
import { ClaimReview } from "./claim-review";

/**
 * ClaimReviewStory shows ClaimReview in the gallery. A story passes no
 * event handler, so this wrapper opens the session over claim with
 * changes, where an undefined value is a reset, and answers the preview
 * and the write with the scripted preview and result.
 */
export function ClaimReviewStory({
  claim,
  changes,
  mode,
  impacts = [],
  mergedLines,
  preview = { objects: [], errors: [] },
  result = { ok: true, mode: "direct" },
}: {
  claim: ClaimObject;
  changes: { steps: Step[]; value?: unknown }[];
  mode: WriteMode;
  impacts?: string[];
  mergedLines?: DiffLine[];
  preview?: Preview;
  result?: WriteResult;
}) {
  // ClaimReview previews again for each new session.
  const session = useMemo(() => {
    let s: Session = openSession({
      claim,
      resource: {
        metadata: { labels: { [CLAIM_LABEL]: claim.metadata.name! } },
      },
      locks: () => undefined,
    })!;
    for (const { steps, value } of changes) {
      const next =
        value === undefined ? resetValue(s, steps) : setValue(s, steps, value);
      if ("session" in next) s = next.session;
    }
    return s;
  }, [claim, changes]);
  return (
    <ClaimReview
      session={session}
      mode={mode}
      intent={`Change the claim ${claim.metadata.name}`}
      impacts={impacts}
      mergedLines={mergedLines}
      preview={async () => preview}
      apply={async () => result}
    />
  );
}
