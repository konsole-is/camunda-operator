"use client";

import { GitCommitHorizontal } from "lucide-react";
import { Button } from "@/components/ui/button";

/** WAITING_FOR_GITOPS is the wait of a push that this page made. */
export const WAITING_FOR_GITOPS =
  "Waiting for GitOps to apply your last change.";

/** WAITING_ON_LIVE is the wait of a push that an earlier visit of the page made. */
export const WAITING_ON_LIVE = `${WAITING_FOR_GITOPS} The page shows the live claim until GitOps applies it.`;

/**
 * WaitingPanel says that Review waits for GitOps, with text, and offers
 * Stop waiting.
 */
export function WaitingPanel({
  text,
  onStopWaiting,
}: {
  text: string;
  onStopWaiting: () => void;
}) {
  return (
    <div className="flex w-full flex-wrap items-center gap-x-4 gap-y-2 rounded-lg border border-info/40 bg-panel px-4 py-2 text-sm text-text-2 shadow-lg">
      <GitCommitHorizontal aria-hidden className="size-4 shrink-0 text-info" />
      <p className="min-w-0 flex-1">{text}</p>
      <Button variant="outline" size="sm" onClick={onStopWaiting}>
        Stop waiting
      </Button>
    </div>
  );
}
