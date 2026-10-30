"use client";

import { CircleAlert } from "lucide-react";
import { type ReactNode, useEffect, useRef, useState } from "react";
import { objectDiff, type DiffLine } from "@/lib/diff";
import { equalJSON } from "@/lib/policy";
import { itemStep, type Step } from "@/lib/fields";
import {
  changeOf,
  claimAfter,
  outcomeOf,
  valueAt,
  type ClaimObject,
  type FieldError,
  type Session,
  type WriteOutcome,
} from "@/lib/session";
import type {
  Change,
  Preview,
  WriteMode,
  WriteResult,
} from "@/lib/write/types";
import { DiffView } from "./diff-view";
import { ReviewPanel } from "./review-panel";

/**
 * ClaimReview is the review of the pending changes of a session: the diff
 * of the claim, the merged diff of the cluster when mergedLines is given,
 * the impacts, the caution, and the confirm. It previews the change of the session once
 * for each session and intent, and the confirm stays disabled until that
 * preview is clean. A confirm sends one write. An ok write goes to show.
 * Then it calls onWritten, and the parent closes the session. A refused
 * preview or write shows its errors and gives the errors of pending fields
 * to onRefused. It never changes the session.
 */
export function ClaimReview<C extends ClaimObject>({
  session,
  mode,
  intent,
  impacts,
  caution,
  mergedLines,
  preview,
  apply,
  show,
  onWritten,
  onRefused,
  onCancel,
}: {
  session: Session<C>;
  mode: WriteMode;
  /** intent is the commit subject in Git mode. */
  intent: string;
  impacts: string[];
  /** caution is a warning about the change, such as a backup that it makes fail. */
  caution?: ReactNode;
  mergedLines?: DiffLine[];
  preview: (change: Change) => Promise<Preview>;
  apply: (change: Change) => Promise<WriteResult>;
  show: (result: WriteResult, message: string) => void;
  onWritten?: () => void;
  onRefused?: (fields: FieldError[]) => void;
  onCancel?: () => void;
}) {
  const [previewed, setPreviewed] = useState<{
    session: Session<C>;
    intent: string;
    result: Preview;
  }>();
  const [refused, setRefused] = useState<{
    session: Session<C>;
    outcome: WriteOutcome;
  }>();
  const [writing, setWriting] = useState(false);
  // A second click arrives before React disables the button.
  const writingNow = useRef(false);
  const previewRef = useRef(preview);
  const onRefusedRef = useRef(onRefused);
  useEffect(() => {
    previewRef.current = preview;
    onRefusedRef.current = onRefused;
  });

  useEffect(() => {
    let current = true;
    previewRef
      .current(changeOf(session, intent))
      .catch((error: unknown): Preview => ({
        objects: [],
        errors: [{ code: "Unavailable", message: String(error) }],
      }))
      .then((result) => {
        if (!current) return;
        setPreviewed({ session, intent, result });
        const outcome = outcomeOf(result, session);
        if (!outcome.ok && outcome.fields.length > 0)
          onRefusedRef.current?.(outcome.fields);
      });
    return () => {
      current = false;
    };
  }, [session, intent]);

  const checked =
    previewed?.session === session && previewed.intent === intent
      ? previewed.result
      : undefined;
  const previewOutcome = checked && outcomeOf(checked, session);
  const writeOutcome =
    refused?.session === session ? refused.outcome : undefined;
  const blocked =
    !previewOutcome ||
    !previewOutcome.ok ||
    (writeOutcome !== undefined && !writeOutcome.ok && writeOutcome.stale);
  const alerts = [
    ...messagesOf(previewOutcome),
    ...messagesOf(writeOutcome),
    ...(checked ? keptResets(session, checked) : []),
  ];

  async function confirm() {
    if (writingNow.current || blocked) return;
    writingNow.current = true;
    setWriting(true);
    try {
      const result = await apply(changeOf(session, intent)).catch(
        (error: unknown): WriteResult => ({
          ok: false,
          errors: [{ code: "Unavailable", message: String(error) }],
        }),
      );
      if (result.ok) {
        show(result, `The claim ${session.claim.metadata.name} is saved.`);
        onWritten?.();
        return;
      }
      const outcome = outcomeOf(result, session);
      setRefused({ session, outcome });
      if (!outcome.ok && outcome.fields.length > 0) onRefused?.(outcome.fields);
    } finally {
      writingNow.current = false;
      setWriting(false);
    }
  }

  return (
    <ReviewPanel
      lines={objectDiff(session.claim, claimAfter(session))}
      details={
        mergedLines && (
          <DiffView label="Changes to the cluster" lines={mergedLines} />
        )
      }
      effects={impacts}
      caution={caution}
      notices={
        alerts.length > 0 && (
          <div
            role="alert"
            className="flex gap-3 rounded-md border border-bad/40 bg-bad/10 p-3 text-sm text-text"
          >
            <CircleAlert
              aria-hidden
              className="mt-0.5 size-4 shrink-0 text-bad"
            />
            <ul className="flex flex-col gap-1">
              {alerts.map((text, i) => (
                <li key={i}>{text}</li>
              ))}
            </ul>
          </div>
        )
      }
      mode={mode}
      confirmLabel="Save"
      pending={writing || blocked}
      onConfirm={confirm}
      onCancel={onCancel}
    />
  );
}

function messagesOf(outcome: WriteOutcome | undefined): string[] {
  if (!outcome || outcome.ok) return [];
  return [
    ...outcome.messages,
    ...outcome.fields.map((f) =>
      f.path ? `${f.path}: ${f.message}` : f.message,
    ),
  ];
}

// keptResets returns a note for each reset field that the claim of the
// preview still holds. Server-side apply keeps a field that another field
// manager owns, so such a reset changes nothing.
function keptResets(s: Session, preview: Preview): string[] {
  const { kind, metadata } = s.claim;
  const after = preview.objects.find(
    (o) =>
      o.ref.kind === kind &&
      o.ref.namespace === metadata.namespace &&
      o.ref.name === metadata.name,
  )?.after;
  if (!after) return [];
  const intended = claimAfter(s);
  return [...s.pending.values()]
    .filter((entry) => {
      if (!("reset" in entry)) return false;
      const steps = keyedSteps(s.claim, entry.steps);
      if (steps) return valueAt(after, steps) !== undefined;
      // A list item without keys has no name that stays after the reset,
      // so its list must read as the session wants it.
      const list = entry.steps.slice(0, keylessAt(s.claim, entry.steps));
      const named = keyedSteps(s.claim, list);
      return (
        named !== undefined &&
        !equalJSON(valueAt(after, named), valueAt(intended, named))
      );
    })
    .map(
      (entry) =>
        `Another field manager keeps ${entry.path}, so the reset does not remove it.`,
    );
}

// keyedSteps names each list item of steps by its keys in claim, because a
// reset can move the items after it. It returns undefined when an item has
// no keys.
function keyedSteps(
  claim: unknown,
  steps: readonly Step[],
): Step[] | undefined {
  const out: Step[] = [];
  let at = claim;
  for (const step of steps) {
    const next =
      "index" in step && Array.isArray(at)
        ? itemStep(at[step.index], step.index)
        : step;
    if ("index" in next) return undefined;
    out.push(next);
    at = valueAt(at, [step]);
  }
  return out;
}

// keylessAt returns the place of the first step of steps that names a list
// item without keys in claim.
function keylessAt(claim: unknown, steps: readonly Step[]): number {
  let k = 0;
  while (k < steps.length && keyedSteps(claim, steps.slice(0, k + 1))) k++;
  return k;
}
