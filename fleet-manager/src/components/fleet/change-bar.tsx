"use client";

import { CircleAlert } from "lucide-react";
import type { ReactNode, Ref } from "react";
import { Button } from "@/components/ui/button";
import type { FieldError } from "@/lib/session";

/**
 * ChangeBar says how many changes wait for review and offers Discard and
 * Review. It renders nothing while count is 0. Review is disabled while
 * blocked is set or errors is not empty. It shows blocked and the errors
 * without a path; an error with a path shows next to its field. reviewRef
 * gets the Review button.
 */
export function ChangeBar({
  count,
  blocked,
  errors = [],
  reviewRef,
  onDiscard,
  onReview,
}: {
  count: number;
  blocked?: string;
  errors?: readonly FieldError[];
  reviewRef?: Ref<HTMLButtonElement>;
  onDiscard?: () => void;
  onReview?: () => void;
}): ReactNode {
  if (count === 0) return null;
  const pathless = errors.filter((e) => e.path === undefined);
  const notes = [
    ...(blocked ? [blocked] : []),
    ...(errors.length > 0
      ? [
          `Fix ${errors.length} ${errors.length === 1 ? "field" : "fields"} before review.`,
        ]
      : []),
    ...pathless.map((e) => e.message),
  ];
  return (
    <section
      aria-label="Unsaved changes"
      className="sticky bottom-4 z-30 flex flex-wrap items-center gap-x-4 gap-y-2 rounded-lg border border-border bg-panel px-4 py-3 shadow-lg"
    >
      <div className="flex min-w-0 flex-1 flex-col gap-1">
        <p className="flex items-center gap-2 text-sm font-medium text-text">
          <span aria-hidden className="size-2 shrink-0 rounded-full bg-info" />
          {count} unsaved {count === 1 ? "change" : "changes"}
        </p>
        {notes.length > 0 && (
          <ul className="flex flex-col gap-0.5 text-sm text-text-2">
            {notes.map((note, i) => (
              <li key={i} className="flex items-start gap-1.5">
                <CircleAlert
                  aria-hidden
                  className="mt-0.5 size-3.5 shrink-0 text-bad"
                />
                {note}
              </li>
            ))}
          </ul>
        )}
      </div>
      <div className="flex shrink-0 gap-2">
        <Button variant="ghost" onClick={() => onDiscard?.()}>
          Discard
        </Button>
        <Button
          ref={reviewRef}
          disabled={Boolean(blocked) || errors.length > 0}
          onClick={() => onReview?.()}
        >
          Review
        </Button>
      </div>
    </section>
  );
}
