"use client";

import { CircleCheck, CircleAlert, GitCommitHorizontal, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import type { WriteResult } from "@/lib/write/types";

/**
 * ToastResult is the result of a write as the user sees it: saved to the
 * API, pushed to Git and waiting for GitOps, applied by GitOps, or an error.
 */
export type ToastResult =
  | { kind: "saved"; message: string }
  | { kind: "waiting"; commit: string }
  | { kind: "applied"; message: string; commit?: string }
  | { kind: "error"; message: string };

/**
 * toastResultOf returns the toast of a write. message is the text of a
 * direct save. A refusal shows the messages of all its errors.
 */
export function toastResultOf(
  result: WriteResult,
  message: string,
): ToastResult {
  if (!result.ok) {
    return {
      kind: "error",
      message: result.errors.map((e) => e.message).join(" "),
    };
  }
  if (result.mode === "git") {
    return { kind: "waiting", commit: result.commit };
  }
  return { kind: "saved", message };
}

/**
 * Toast shows the result of a write. An error is an alert. Every other
 * result is a status that screen readers read when it changes. The waiting
 * state says "Waiting for GitOps to apply" with the first seven characters
 * of the commit, and names no tool.
 */
export function Toast({
  result,
  onDismiss,
}: {
  result: ToastResult;
  onDismiss?: () => void;
}) {
  const error = result.kind === "error";
  const Icon = error
    ? CircleAlert
    : result.kind === "waiting"
      ? GitCommitHorizontal
      : CircleCheck;
  return (
    <div
      role={error ? "alert" : "status"}
      className={cn(
        "flex w-full max-w-sm items-start gap-3 rounded-md border bg-panel p-3 text-sm shadow-lg",
        error
          ? "border-bad/50"
          : result.kind === "waiting"
            ? "border-info/50"
            : "border-ok/50",
      )}
    >
      <Icon
        aria-hidden
        className={cn(
          "mt-0.5 size-4 shrink-0",
          error
            ? "text-bad"
            : result.kind === "waiting"
              ? "text-info"
              : "text-ok",
        )}
      />
      <div className="min-w-0 flex-1">
        {result.kind === "waiting" ? (
          <>
            <p className="text-text">Waiting for GitOps to apply</p>
            <p className="text-muted">Commit {result.commit.slice(0, 7)}</p>
          </>
        ) : (
          <p className="text-text">{result.message}</p>
        )}
      </div>
      {onDismiss && (
        <Button
          variant="ghost"
          size="icon-xs"
          aria-label="Dismiss"
          onClick={onDismiss}
        >
          <X />
        </Button>
      )}
    </div>
  );
}
