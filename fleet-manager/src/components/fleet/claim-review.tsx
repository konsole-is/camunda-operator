"use client";

import { CircleAlert } from "lucide-react";
import { type ReactNode, useEffect, useMemo, useRef, useState } from "react";
import { objectDiff, type DiffLine } from "@/lib/diff";
import { equalJSON } from "@/lib/policy";
import { itemStep, type Step } from "@/lib/fields";
import {
  changeOf,
  claimAfter,
  outcomeOf,
  savedSession,
  valueAt,
  type ClaimObject,
  type FieldError,
  type Session,
  type WriteOutcome,
} from "@/lib/session";
import { withTakeOver } from "@/lib/write/ownership";
import { refKey } from "@/lib/write/refs";
import type {
  Change,
  ObjectOwnership,
  Preview,
  WriteMode,
  WriteResult,
} from "@/lib/write/types";
import { DiffView } from "./diff-view";
import { ReviewPanel } from "./review-panel";
import {
  TakeOverChoice,
  takeOverOpen,
  withoutTakeOverConflicts,
} from "./take-over-choice";

/**
 * ReviewConfirm changes the confirm of ClaimReview and ChangeReview. label is the button in
 * direct mode, "Save" by default, and gitLabel in Git mode, "Push to Git" by
 * default. typed is a word that the user types before the confirm enables.
 */
export type ReviewConfirm = {
  label?: string;
  gitLabel?: string;
  danger?: boolean;
  typed?: string;
};

/** ClaimReviewInput is what a page adds to the review of a claim change. */
export type ClaimReviewInput = {
  impacts: string[];
  caution?: ReactNode;
  confirm?: ReviewConfirm;
  mergedLines?: DiffLine[];
};

/**
 * ClaimReview is the review of the pending changes of a session: the diff
 * of the claim, the merged diff of the cluster when mergedLines is given,
 * the impacts, the caution, and the confirm. It previews the change of the session once
 * for each session and intent, and the confirm stays disabled until that
 * preview is clean. A confirm sends one write. An ok write goes to show.
 * Then it calls onWritten, and the parent closes the session. A refused
 * preview or write shows its errors and gives the errors of pending fields
 * to onRefused. It never changes the session.
 *
 * In direct mode it shows the fields of other tools of the preview in
 * TakeOverChoice, without their Conflict errors, and keeps the confirm
 * disabled while takeOverOpen is true. The choice starts off for each
 * session and intent, so a rebase resets it. When it is on, the review
 * previews the change of withTakeOver, and the confirm writes that change.
 * The diff of the claim then shows the claim that the save leaves: a
 * cleared field of another tool stays in it until the take-over.
 * In Git mode it notes each reset field that the preview keeps.
 *
 * input, when given, gives the impacts, the caution, the confirm, and
 * mergedLines in place of those props, from saved: a session whose change
 * gives the claim that the save leaves, as the diff of the claim shows it.
 */
export function ClaimReview<C extends ClaimObject>({
  session,
  mode,
  intent,
  impacts,
  caution,
  confirm: confirmProps,
  mergedLines,
  input,
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
  impacts?: string[];
  /** caution is a warning about the change, such as a backup that it makes fail. */
  caution?: ReactNode;
  confirm?: ReviewConfirm;
  mergedLines?: DiffLine[];
  input?: (saved: Session<C>) => ClaimReviewInput;
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
  const [takenPreview, setTakenPreview] = useState<{
    change: Change;
    result: Preview;
  }>();
  const [choice, setChoice] = useState<{
    session: Session<C>;
    intent: string;
    chosen: boolean;
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

  const base = useMemo(() => changeOf(session, intent), [session, intent]);
  const given =
    previewed?.session === session && previewed.intent === intent
      ? previewed.result
      : undefined;
  const ownership = useMemo(
    () => (mode === "direct" ? (given?.ownership ?? []) : []),
    [mode, given],
  );
  const chosen =
    choice?.session === session && choice.intent === intent && choice.chosen;
  const effective = useMemo(
    () => (chosen ? withTakeOver(base, ownership) : base),
    [chosen, base, ownership],
  );
  // The diff shows the claim that the save leaves: a cleared field of
  // another tool stays until the take-over.
  const saved = useMemo(
    () => savedSession(session, ownershipOf(ownership, session), chosen),
    [session, ownership, chosen],
  );
  const shown: ClaimReviewInput = input
    ? input(saved)
    : { impacts: impacts ?? [], caution, confirm: confirmProps, mergedLines };

  useEffect(() => {
    let current = true;
    previewOf(previewRef.current, base).then((result) => {
      if (!current) return;
      setPreviewed({ session, intent, result });
      const shown = mode === "direct" ? (result.ownership ?? []) : [];
      const outcome = outcomeOf(withoutConflicts(result, shown), session);
      if (!outcome.ok && outcome.fields.length > 0)
        onRefusedRef.current?.(outcome.fields);
    });
    return () => {
      current = false;
    };
  }, [base, session, intent, mode]);

  useEffect(() => {
    if (effective === base) return;
    let current = true;
    previewOf(previewRef.current, effective).then((result) => {
      if (!current) return;
      setTakenPreview({ change: effective, result });
      const outcome = outcomeOf(result, session);
      if (!outcome.ok && outcome.fields.length > 0)
        onRefusedRef.current?.(outcome.fields);
    });
    return () => {
      current = false;
    };
  }, [effective, base, session]);

  const checked =
    effective === base
      ? given
      : takenPreview?.change === effective
        ? takenPreview.result
        : undefined;
  const previewOutcome =
    checked &&
    outcomeOf(
      effective === base ? withoutConflicts(checked, ownership) : checked,
      session,
    );
  const writeOutcome =
    refused?.session === session ? refused.outcome : undefined;
  const blocked =
    !previewOutcome ||
    !previewOutcome.ok ||
    takeOverOpen(ownership, chosen) ||
    (writeOutcome !== undefined && !writeOutcome.ok && writeOutcome.stale);
  const alerts = [
    ...messagesOf(previewOutcome),
    ...messagesOf(writeOutcome),
    ...(checked && mode === "git" ? keptResets(session, checked) : []),
  ];

  async function confirm() {
    if (writingNow.current || blocked) return;
    writingNow.current = true;
    setWriting(true);
    try {
      const result = await apply(effective).catch(
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
      lines={objectDiff(session.claim, claimAfter(saved))}
      details={
        shown.mergedLines && (
          <DiffView label="Changes to the cluster" lines={shown.mergedLines} />
        )
      }
      effects={shown.impacts}
      caution={shown.caution}
      notices={
        <>
          {ownership.length > 0 && (
            <TakeOverChoice
              ownership={ownership}
              chosen={chosen}
              onChoose={(next) => setChoice({ session, intent, chosen: next })}
            />
          )}
          {alerts.length > 0 && (
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
          )}
        </>
      }
      mode={mode}
      confirmLabel={shown.confirm?.label ?? "Save"}
      gitConfirmLabel={shown.confirm?.gitLabel}
      danger={shown.confirm?.danger}
      typed={shown.confirm?.typed}
      pending={writing || blocked}
      onConfirm={confirm}
      onCancel={onCancel}
    />
  );
}

function previewOf(
  preview: (change: Change) => Promise<Preview>,
  change: Change,
): Promise<Preview> {
  return preview(change).catch((error: unknown): Preview => ({
    objects: [],
    errors: [{ code: "Unavailable", message: String(error) }],
  }));
}

function ownershipOf(
  ownership: readonly ObjectOwnership[],
  s: Session,
): ObjectOwnership | undefined {
  const { kind, metadata } = s.claim;
  const key = refKey({
    kind,
    namespace: metadata.namespace,
    name: metadata.name ?? "",
  });
  return ownership.find((o) => refKey(o.ref) === key);
}

// TakeOverChoice names the fields of these Conflict errors, so neither an
// alert nor a pending field repeats them.
function withoutConflicts(
  preview: Preview,
  ownership: readonly ObjectOwnership[],
): Preview {
  return {
    ...preview,
    errors: withoutTakeOverConflicts(preview.errors, ownership),
  };
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
