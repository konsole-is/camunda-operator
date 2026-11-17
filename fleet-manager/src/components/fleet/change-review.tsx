"use client";

import { CircleAlert } from "lucide-react";
import { type ReactNode, useEffect, useMemo, useRef, useState } from "react";
import { byCodeUnit, equalJSON, isObject } from "@/lib/policy";
import { withTakeOver } from "@/lib/write/ownership";
import type {
  ApiError,
  Change,
  KubeObject,
  Preview,
  WriteMode,
  WriteResult,
} from "@/lib/write/types";
import type { ReviewConfirm } from "./claim-review";
import { DiffView } from "./diff-view";
import { ReviewPanel } from "./review-panel";
import {
  TakeOverChoice,
  takeOverOpen,
  withoutTakeOverConflicts,
} from "./take-over-choice";

/**
 * ChangeReview is the review of one Change of a Setup page. It previews
 * change once for each change object, shows the diff of the first previewed
 * object under diffLabel and each other object as its own diff, the effects,
 * the caution, and the errors of the preview. The confirm stays disabled
 * until the preview is clean. The confirm also stays disabled when every
 * previewed object is unchanged.
 *
 * In direct mode, for a change without force and unless offerTakeOver is
 * false, it shows the fields of other tools of the preview in
 * TakeOverChoice, without their Conflict errors, and keeps the confirm
 * disabled while takeOverOpen is true. The choice starts off for each
 * change. When it is on, the review previews and shows the change of
 * withTakeOver, and the confirm writes that change. Otherwise it notes each
 * field under spec that the preview keeps although the change leaves it
 * out. A page whose write refuses takeOver and remove sets offerTakeOver
 * to false.
 *
 * A confirm sends one write, and Back is hidden while it runs. An ok write
 * goes to show with savedMessage, then onWritten. A refused write shows
 * its errors, and the confirm stays disabled unless the API server did not
 * answer. While hold is set, the
 * confirm stays disabled whatever the preview says; the effects say why.
 * onWriting gets true when the write starts and false when it settles.
 */
export function ChangeReview({
  change,
  mode,
  diffLabel,
  effects,
  caution,
  confirm: confirmProps,
  hold = false,
  offerTakeOver = true,
  savedMessage,
  preview,
  apply,
  show,
  onWritten,
  onCancel,
  onWriting,
}: {
  change: Change;
  mode: WriteMode;
  diffLabel: string;
  /** effects are the lines of the effects, or a function of the preview that gives them once the preview is in. */
  effects: ReactNode[] | ((preview: Preview) => ReactNode[]);
  caution?: ReactNode;
  confirm: ReviewConfirm;
  hold?: boolean;
  offerTakeOver?: boolean;
  savedMessage: string;
  preview: (change: Change) => Promise<Preview>;
  apply: (change: Change) => Promise<WriteResult>;
  show: (result: WriteResult, message: string) => void;
  onWritten?: () => void;
  onCancel?: () => void;
  onWriting?: (writing: boolean) => void;
}): ReactNode {
  const [previewed, setPreviewed] = useState<{
    change: Change;
    result: Preview;
  }>();
  const [takenPreview, setTakenPreview] = useState<{
    change: Change;
    result: Preview;
  }>();
  const [choice, setChoice] = useState<{ change: Change; chosen: boolean }>();
  const [refused, setRefused] = useState<{
    change: Change;
    errors: ApiError[];
  }>();
  const [writing, setWriting] = useState(false);
  // A second click arrives before React disables the button.
  const writingNow = useRef(false);
  const previewRef = useRef(preview);
  useEffect(() => {
    previewRef.current = preview;
  });

  const given = previewed?.change === change ? previewed.result : undefined;
  const choosing = mode === "direct" && !change.force && offerTakeOver;
  const ownership = useMemo(
    () => (choosing ? (given?.ownership ?? []) : []),
    [choosing, given],
  );
  const chosen = choice?.change === change && choice.chosen;
  const effective = useMemo(
    () => (chosen ? withTakeOver(change, ownership) : change),
    [chosen, change, ownership],
  );

  useEffect(() => previewOnce(previewRef, change, setPreviewed), [change]);
  useEffect(() => {
    if (effective === change) return;
    return previewOnce(previewRef, effective, setTakenPreview);
  }, [effective, change]);

  const checked =
    effective === change
      ? given
      : takenPreview?.change === effective
        ? takenPreview.result
        : undefined;
  const previewErrors = withoutTakeOverConflicts(
    checked?.errors ?? [],
    ownership,
  );
  const writeErrors = refused?.change === effective ? refused.errors : [];
  const changesNothing =
    checked !== undefined &&
    previewErrors.length === 0 &&
    checked.objects.length > 0 &&
    checked.objects.every((o) => o.diff.every((line) => line.op === "same"));
  const blocked =
    hold ||
    !checked ||
    previewErrors.length > 0 ||
    changesNothing ||
    takeOverOpen(ownership, chosen) ||
    writeErrors.some((e) => e.code !== "Unavailable");
  const alerts = [...previewErrors, ...writeErrors].map(textOf);
  const kept = choosing
    ? []
    : (checked?.objects ?? []).flatMap((object) => {
        const sent = change.apply.find(
          (o) =>
            o.kind === object.ref.kind &&
            o.metadata.name === object.ref.name &&
            o.metadata.namespace === object.ref.namespace,
        );
        return sent ? keptPaths(sent, object.after) : [];
      });
  const [first, ...others] = checked?.objects ?? [];

  async function confirm() {
    if (writingNow.current || blocked) return;
    writingNow.current = true;
    setWriting(true);
    onWriting?.(true);
    try {
      const result = await apply(effective).catch(
        (error: unknown): WriteResult => ({
          ok: false,
          errors: [{ code: "Unavailable", message: String(error) }],
        }),
      );
      if (result.ok) {
        show(result, savedMessage);
        onWritten?.();
        return;
      }
      setRefused({ change: effective, errors: result.errors });
    } finally {
      writingNow.current = false;
      setWriting(false);
      onWriting?.(false);
    }
  }

  return (
    <ReviewPanel
      lines={first?.diff ?? []}
      diffLabel={diffLabel}
      details={others.map((object) => (
        <DiffView
          key={`${object.ref.kind}/${object.ref.namespace ?? ""}/${object.ref.name}`}
          label={`Changes to ${object.ref.kind} ${object.ref.name}`}
          lines={object.diff}
        />
      ))}
      effects={
        typeof effects === "function"
          ? checked
            ? effects(checked)
            : []
          : effects
      }
      caution={caution}
      notices={
        <>
          {!checked && (
            <p role="status" className="text-sm text-muted">
              Checking the change with the API server.
            </p>
          )}
          {changesNothing && (
            <p className="text-sm text-muted">The change writes nothing.</p>
          )}
          {ownership.length > 0 && (
            <TakeOverChoice
              ownership={ownership}
              chosen={chosen}
              onChoose={(next) => setChoice({ change, chosen: next })}
            />
          )}
          {kept.length > 0 && (
            <div role="note" aria-label="Kept fields">
              <ul className="flex flex-col gap-1 text-sm text-text-2">
                {kept.map((path) => (
                  <li key={path}>
                    {path} stays, because another field manager set it.
                  </li>
                ))}
              </ul>
            </div>
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
      confirmLabel={confirmProps.label ?? "Save"}
      gitConfirmLabel={confirmProps.gitLabel}
      danger={confirmProps.danger}
      typed={confirmProps.typed}
      pending={writing || blocked}
      onConfirm={confirm}
      onCancel={writing ? undefined : onCancel}
    />
  );
}

/**
 * keptPaths returns the path of each field under spec that after holds and
 * applied does not, in code-unit order, as "spec.sections.backup-storage",
 * and the path of each list in which after keeps a member that applied
 * leaves out.
 * Another field manager owns such a field, so the save keeps it. A field
 * whose parent applied also leaves out is named once, by the parent.
 */
export function keptPaths(
  applied: KubeObject,
  after: KubeObject | undefined,
): string[] {
  if (!after) return [];
  return missingIn(applied.spec, after.spec, "spec").sort(byCodeUnit);
}

function missingIn(sent: unknown, kept: unknown, path: string): string[] {
  if (Array.isArray(kept)) {
    const members = Array.isArray(sent) ? sent : [];
    return kept.some((item) => !members.some((m) => equalJSON(m, item)))
      ? [path]
      : [];
  }
  if (!isObject(kept)) return [];
  const object = isObject(sent) ? sent : {};
  return Object.entries(kept).flatMap(([key, value]) =>
    Object.hasOwn(object, key)
      ? missingIn(object[key], value, `${path}.${key}`)
      : [`${path}.${key}`],
  );
}

// previewOnce previews change and hands the result to set, unless the
// returned cleanup ran first.
function previewOnce(
  previewRef: { current: (change: Change) => Promise<Preview> },
  change: Change,
  set: (previewed: { change: Change; result: Preview }) => void,
): () => void {
  let current = true;
  previewRef
    .current(change)
    .catch((error: unknown): Preview => ({
      objects: [],
      errors: [{ code: "Unavailable", message: String(error) }],
    }))
    .then((result) => {
      if (current) set({ change, result });
    });
  return () => {
    current = false;
  };
}

function textOf(error: ApiError): string {
  return error.path ? `${error.path}: ${error.message}` : error.message;
}
