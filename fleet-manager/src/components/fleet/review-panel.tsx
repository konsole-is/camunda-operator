"use client";

import type { ReactNode } from "react";
import { useId } from "react";
import { TriangleAlert } from "lucide-react";
import { Button } from "@/components/ui/button";
import type { DiffLine } from "@/lib/diff";
import type { WriteMode } from "@/lib/write/types";
import { DiffView } from "./diff-view";

/**
 * ReviewPanel is the last screen before a write: the diff of the object, the
 * list of what the change does, an optional caution, and the confirm button.
 * details shows below the diff, and notices right above the confirm. In Git
 * mode the button reads gitConfirmLabel, "Push to Git" by default,
 * instead of confirmLabel. danger makes it a danger button. pending blocks
 * it while the write runs.
 */
export function ReviewPanel({
  lines,
  effects,
  caution,
  details,
  notices,
  mode,
  confirmLabel,
  gitConfirmLabel = "Push to Git",
  danger = false,
  pending = false,
  onConfirm,
  onCancel,
  diffLabel = "Changes to the claim",
}: {
  lines: DiffLine[];
  effects: ReactNode[];
  caution?: ReactNode;
  details?: ReactNode;
  notices?: ReactNode;
  mode: WriteMode;
  confirmLabel: string;
  gitConfirmLabel?: string;
  danger?: boolean;
  pending?: boolean;
  onConfirm?: () => void;
  onCancel?: () => void;
  diffLabel?: string;
}) {
  const effectsId = useId();
  return (
    <section aria-label="Review" className="flex min-w-0 flex-col gap-5">
      <DiffView label={diffLabel} lines={lines} />
      {details}
      {effects.length > 0 && (
        <div>
          <h3
            id={effectsId}
            className="mb-2 font-heading text-sm font-semibold"
          >
            What this change does
          </h3>
          <ul
            aria-labelledby={effectsId}
            className="flex list-disc flex-col gap-1 pl-5 text-sm text-text-2"
          >
            {effects.map((effect, i) => (
              <li key={i}>{effect}</li>
            ))}
          </ul>
        </div>
      )}
      {caution && (
        <div
          role="note"
          aria-label="Caution"
          className="flex gap-3 rounded-md border border-warn/40 bg-warn/10 p-3 text-sm text-text"
        >
          <TriangleAlert
            aria-hidden
            className="mt-0.5 size-4 shrink-0 text-warn"
          />
          <div>{caution}</div>
        </div>
      )}
      {notices}
      <div className="flex flex-wrap gap-2">
        {/* pending can end right after the panel shows, as when a preview
            returns. A fade of the opacity would show the label at half
            contrast for its duration. */}
        <Button
          variant={danger ? "destructive" : "default"}
          className="transition-colors"
          disabled={pending}
          onClick={() => onConfirm?.()}
        >
          {mode === "git" ? gitConfirmLabel : confirmLabel}
        </Button>
        {onCancel && (
          <Button variant="ghost" onClick={onCancel}>
            Back
          </Button>
        )}
      </div>
    </section>
  );
}
