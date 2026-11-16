"use client";

import Link from "next/link";
import { type ReactNode, useState } from "react";
import type { ClaimActions } from "@/components/fleet/claim-edit";
import { type PendingPush, useGitPushes } from "@/components/fleet/git-pushes";
import {
  GitWaitingLine,
  type PushedObject,
} from "@/components/fleet/git-waiting-line";
import { InlineConfirm } from "@/components/fleet/inline-confirm";
import { Toast, useWriteToast } from "@/components/fleet/toast";
import { Button, buttonVariants } from "@/components/ui/button";
import type { Resource } from "@/lib/k8s/kinds";
import { REVISION_ANNOTATION } from "@/lib/labels";
import { suspendChange } from "@/lib/planes/suspend";
import { refKey } from "@/lib/write/refs";

/**
 * PlaneActions holds Suspend or Resume, and Delete, of a plane that a
 * claim made. Suspend asks in place and says that sign-in stops for every
 * cluster that the plane serves; served is that number. Resume writes at
 * once. Each write sets spec.suspend through apply, with the
 * resourceVersion of claim as its precondition. Delete links to deleteHref.
 * Both stay off while unread names a kind that the page could not read,
 * while a write runs, until the claim shows a newer version after a write,
 * and while a pushed change of the claim waits for GitOps; an open confirm
 * then closes. It needs a GitPushesProvider and a QueryClientProvider
 * above it.
 */
export function PlaneActions({
  claim,
  served,
  apply,
  deleteHref,
  unread = [],
  now,
}: {
  claim: Resource<"CamundaManagementClusterClaim">;
  served: number;
  apply: ClaimActions["apply"];
  deleteHref: string;
  unread?: readonly string[];
  now: string;
}): ReactNode {
  const { toast, show, dismiss } = useWriteToast();
  const { pushes } = useGitPushes();
  const { namespace, name = "", resourceVersion } = claim.metadata ?? {};
  const key = refKey({
    kind: "CamundaManagementClusterClaim",
    namespace,
    name,
  });
  const pushed = pushes.find(
    (p): p is PendingPush & PushedObject => p.key === key && !("deleted" in p),
  );
  const liveRevision = claim.metadata?.annotations?.[REVISION_ANNOTATION];
  const gitWaiting = pushed !== undefined && pushed.revision !== liveRevision;
  const [running, setRunning] = useState(false);
  // The version of the claim that a successful write read. The claim with
  // that version does not show the write yet.
  const [writtenAt, setWrittenAt] = useState<string>();
  const off =
    unread.length > 0 ||
    running ||
    gitWaiting ||
    (writtenAt !== undefined && writtenAt === resourceVersion);

  const write = async (suspend: boolean) => {
    if (off) return;
    setRunning(true);
    const result = await apply(suspendChange(claim, suspend)).catch(
      (error: unknown) => ({
        ok: false as const,
        errors: [{ code: "Unavailable" as const, message: String(error) }],
      }),
    );
    setRunning(false);
    if (result.ok) setWrittenAt(resourceVersion);
    show(
      result,
      `${suspend ? "Suspended" : "Resumed"} the management plane ${name}.`,
    );
  };

  const suspended = claim.spec.suspend === true;
  return (
    <div className="flex max-w-xl flex-col items-end gap-2">
      <div className="flex flex-wrap items-start justify-end gap-2">
        {suspended || off ? (
          <Button
            variant="outline"
            disabled={off}
            onClick={() => void write(false)}
          >
            {suspended ? "Resume" : "Suspend"}
          </Button>
        ) : (
          <InlineConfirm
            triggerLabel="Suspend"
            confirmLabel="Suspend the management plane"
            goes={[
              `Every workload of the plane stops. Sign-in to Console, Web Modeler, and Optimize stops for every cluster that this plane serves (${served}).`,
            ]}
            stays={[
              "The served clusters keep running, and the databases keep the data of the plane.",
            ]}
            onConfirm={() => void write(true)}
          />
        )}
        {off ? (
          <Button variant="outline" disabled>
            Delete
          </Button>
        ) : (
          <Link
            href={deleteHref}
            className={buttonVariants({ variant: "outline" })}
          >
            Delete
          </Link>
        )}
      </div>
      {unread.length > 0 && (
        <p className="text-right text-sm text-muted">
          Suspend and Delete are possible when the fleet manager can read{" "}
          {unread.join(", ")}.
        </p>
      )}
      <GitWaitingLine pushed={pushed} liveRevision={liveRevision} now={now} />
      {toast && <Toast result={toast} onDismiss={dismiss} />}
    </div>
  );
}
