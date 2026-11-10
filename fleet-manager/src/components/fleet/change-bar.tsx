"use client";

import { Button } from "@/components/ui/button";
import type { FieldError } from "@/lib/session";

/**
 * ChangeBar shows the count of pending changes with Discard and Review.
 * blocked is the reason that Review is disabled, such as a form error.
 * Review is disabled too while errors is not empty. The bar shows blocked
 * and each error without a path; an error with a path shows at its field.
 * It renders nothing while count is 0.
 */
export function ChangeBar({
  count,
  blocked,
  errors = [],
  onDiscard,
  onReview,
}: {
  count: number;
  blocked?: string;
  errors?: readonly FieldError[];
  onDiscard?: () => void;
  onReview?: () => void;
}) {
  if (count === 0) return null;
  const messages = [
    ...(blocked ? [blocked] : []),
    ...errors.filter((e) => !e.path).map((e) => e.message),
  ];
  return (
    <div
      role="region"
      aria-label="Pending changes"
      className="sticky bottom-4 z-10 flex flex-wrap items-center gap-x-4 gap-y-2 rounded-md border border-border bg-panel px-4 py-3 shadow-lg"
    >
      <p className="font-medium text-text">
        {count === 1 ? "1 change" : `${count} changes`}
      </p>
      {messages.length > 0 && (
        <ul className="flex min-w-0 flex-1 flex-col gap-0.5 text-sm text-bad">
          {messages.map((message, i) => (
            <li key={i}>{message}</li>
          ))}
        </ul>
      )}
      <div className="ml-auto flex gap-2">
        <Button variant="ghost" onClick={() => onDiscard?.()}>
          Discard
        </Button>
        <Button
          disabled={blocked !== undefined || errors.length > 0}
          onClick={() => onReview?.()}
        >
          Review
        </Button>
      </div>
    </div>
  );
}
