"use client";

import { CircleAlert } from "lucide-react";
import { type ReactNode, useEffect, useRef, useState } from "react";
import { byCodeUnit, equalJSON, isObject } from "@/lib/policy";
import type {
  ApiError,
  Change,
  KubeObject,
  Preview,
  WriteMode,
  WriteResult,
} from "@/lib/write/types";
import { DiffView } from "./diff-view";
import { ReviewPanel } from "./review-panel";

/**
 * ChangeReview is the review of one Change of a Setup page. It previews
 * change once for each change object, shows the diff of the first previewed
 * object under diffLabel and each other object as its own diff, the effects,
 * the caution, the errors of the preview, and a note for each field that the
 * preview keeps although the change leaves it out. The confirm stays disabled
 * until the preview is clean. A confirm sends one write. An ok write goes to
 * show with savedMessage, then onWritten. A refused write shows its errors,
 * and the confirm stays disabled unless the API server did not answer.
 */
export function ChangeReview({
  change,
  mode,
  diffLabel,
  effects,
  caution,
  confirmLabel,
  gitConfirmLabel,
  danger,
  savedMessage,
  preview,
  apply,
  show,
  onWritten,
  onCancel,
}: {
  change: Change;
  mode: WriteMode;
  diffLabel: string;
  effects: ReactNode[];
  caution?: ReactNode;
  confirmLabel: string;
  gitConfirmLabel?: string;
  danger?: boolean;
  savedMessage: string;
  preview: (change: Change) => Promise<Preview>;
  apply: (change: Change) => Promise<WriteResult>;
  show: (result: WriteResult, message: string) => void;
  onWritten?: () => void;
  onCancel?: () => void;
}): ReactNode {
  const [previewed, setPreviewed] = useState<{
    change: Change;
    result: Preview;
  }>();
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

  useEffect(() => {
    let current = true;
    previewRef
      .current(change)
      .catch((error: unknown): Preview => ({
        objects: [],
        errors: [{ code: "Unavailable", message: String(error) }],
      }))
      .then((result) => {
        if (current) setPreviewed({ change, result });
      });
    return () => {
      current = false;
    };
  }, [change]);

  const checked = previewed?.change === change ? previewed.result : undefined;
  const writeErrors = refused?.change === change ? refused.errors : [];
  const blocked =
    !checked ||
    checked.errors.length > 0 ||
    writeErrors.some((e) => e.code !== "Unavailable");
  const alerts = [...(checked?.errors ?? []), ...writeErrors].map(textOf);
  const kept = (checked?.objects ?? []).flatMap((object) => {
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
    try {
      const result = await apply(change).catch(
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
      setRefused({ change, errors: result.errors });
    } finally {
      writingNow.current = false;
      setWriting(false);
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
      effects={effects}
      caution={caution}
      notices={
        <>
          {!checked && (
            <p role="status" className="text-sm text-muted">
              Checking the change with the API server.
            </p>
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
      confirmLabel={confirmLabel}
      gitConfirmLabel={gitConfirmLabel}
      danger={danger}
      pending={writing || blocked}
      onConfirm={confirm}
      onCancel={onCancel}
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

function textOf(error: ApiError): string {
  return error.path ? `${error.path}: ${error.message}` : error.message;
}
