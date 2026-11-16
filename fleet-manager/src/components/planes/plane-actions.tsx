"use client";

import Link from "next/link";
import { type ReactNode, useState } from "react";
import { CHANGES_OFF, type ClaimActions } from "@/components/fleet/claim-edit";
import { useGitPushes } from "@/components/fleet/git-pushes";
import { GitWaitingLine } from "@/components/fleet/git-waiting-line";
import { InlineConfirm } from "@/components/fleet/inline-confirm";
import { Toast, useWriteToast } from "@/components/fleet/toast";
import { Button, buttonVariants } from "@/components/ui/button";
import type { Resource } from "@/lib/k8s/kinds";
import { REVISION_ANNOTATION } from "@/lib/labels";
import { suspendChange } from "@/lib/planes/suspend";
import type { WriteMode } from "@/lib/write/types";
import { refKey } from "@/lib/write/refs";

/**
 * PlaneActions holds Suspend or Resume, and Delete, of a plane that a
 * claim made. Suspend asks in place and says that sign-in stops for every
 * cluster that the plane serves; served is that number. ownKeycloak says
 * that the plane runs the Keycloak that its OIDC clusters sign in at.
 * Resume writes at once. Each write sets spec.suspend through apply, with
 * the resourceVersion of claim as its precondition. Delete links to
 * deleteHref. Both stay off while unread names a kind that the page could
 * not read, while a write runs, until the claim shows a newer version after
 * a write succeeds, and while a pushed change or delete of the claim waits
 * for GitOps; an open confirm then closes. Suspend and Resume are off while
 * the claim is being deleted, and while writeMode is undefined. It needs a
 * GitPushesProvider and a QueryClientProvider above it.
 */
export function PlaneActions({
  claim,
  served,
  ownKeycloak = false,
  apply,
  deleteHref,
  writeMode,
  unread = [],
  now,
}: {
  claim: Resource<"CamundaManagementClusterClaim">;
  served: number;
  ownKeycloak?: boolean;
  apply: ClaimActions["apply"];
  deleteHref: string;
  writeMode: WriteMode | undefined;
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
  const pushed = pushes.find((p) => p.key === key);
  const liveRevision = claim.metadata?.annotations?.[REVISION_ANNOTATION];
  const gitWaiting =
    pushed !== undefined &&
    ("deleted" in pushed || pushed.revision !== liveRevision);
  const [running, setRunning] = useState(false);
  // The version of the claim that a successful write read. The claim with
  // that version does not show the write yet.
  const [writtenAt, setWrittenAt] = useState<string>();
  const off =
    unread.length > 0 ||
    running ||
    gitWaiting ||
    (writtenAt !== undefined && writtenAt === resourceVersion);

  // The claim controller applies no plane while the claim is being deleted.
  const deleting = claim.metadata?.deletionTimestamp != null;
  const suspendOff = off || deleting || writeMode === undefined;

  const write = async (suspend: boolean) => {
    if (suspendOff) return;
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
        {suspended || suspendOff ? (
          <Button
            variant="outline"
            disabled={suspendOff}
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
              ...(ownKeycloak && served > 0 ? [KEYCLOAK_SUSPENDED] : []),
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
      {writeMode === undefined && (
        <p className="text-right text-sm text-muted">{CHANGES_OFF}</p>
      )}
      {deleting && (
        <p className="text-right text-sm text-muted">
          Suspend and Resume are off while the claim is being deleted.
        </p>
      )}
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

const KEYCLOAK_SUSPENDED =
  "The served clusters that use OIDC sign in at the Keycloak of this plane. Their users and API clients cannot sign in or get tokens until the plane resumes.";
