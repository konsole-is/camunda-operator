"use client";

import type { Change, Preview, WriteMode } from "@/lib/write/types";
import { ChangeReview } from "./change-review";
import { Toast, useWriteToast } from "./toast";

/**
 * ChangeReviewStory shows ChangeReview in the gallery. A story passes no
 * event handler, so this wrapper answers the preview with preview and each
 * write with an ok result in mode.
 */
export function ChangeReviewStory({
  change,
  preview,
  mode,
  effects,
}: {
  change: Change;
  preview: Preview;
  mode: WriteMode;
  effects: string[];
}) {
  const { toast, show, dismiss } = useWriteToast();
  return (
    <div className="flex flex-col gap-4">
      <ChangeReview
        change={change}
        mode={mode}
        diffLabel="Changes to the environment"
        effects={effects}
        confirmLabel="Save"
        savedMessage={`The environment ${change.apply[0]?.metadata.name} is saved.`}
        preview={async () => preview}
        apply={async () => ({ ok: true, mode: "direct" })}
        show={show}
      />
      {toast && <Toast result={toast} onDismiss={dismiss} />}
    </div>
  );
}
