"use client";

import Link from "next/link";
import { type ReactNode, useEffect, useRef, useState } from "react";
import { CHANGES_OFF, type ClaimActions } from "@/components/fleet/claim-edit";
import { useGitPushes } from "@/components/fleet/git-pushes";
import { GitWaitingLine } from "@/components/fleet/git-waiting-line";
import { InlineConfirm } from "@/components/fleet/inline-confirm";
import {
  TakeOverChoice,
  takeOverOpen,
  withoutTakeOverConflicts,
} from "@/components/fleet/take-over-choice";
import { Toast, useWriteToast } from "@/components/fleet/toast";
import { Button, buttonVariants } from "@/components/ui/button";
import type { Resource } from "@/lib/k8s/kinds";
import { REVISION_ANNOTATION } from "@/lib/labels";
import { suspendChange } from "@/lib/planes/suspend";
import { withTakeOver } from "@/lib/write/ownership";
import { refKey } from "@/lib/write/refs";
import type {
  ApiError,
  Change,
  ObjectOwnership,
  Preview,
  WriteMode,
  WriteResult,
} from "@/lib/write/types";

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
 *
 * In direct mode, a write that fails with a Conflict previews the same
 * change. When the preview names fields of other tools, a confirm shows
 * them in TakeOverChoice, and its confirm writes the change with
 * withTakeOver once takeOverOpen is false. The choice starts off for each
 * write. Otherwise the refusal shows as before.
 */
export function PlaneActions({
  claim,
  served,
  ownKeycloak = false,
  apply,
  preview,
  deleteHref,
  writeMode,
  unread = [],
  now,
}: {
  claim: Resource<"CamundaManagementClusterClaim">;
  served: number;
  ownKeycloak?: boolean;
  apply: ClaimActions["apply"];
  preview: ClaimActions["preview"];
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

  // A take-over belongs to the version of the claim that its write read.
  const [taking, setTaking] = useState<TakeOver>();
  const takeOver = taking?.version === resourceVersion ? taking : undefined;
  const toggle = useRef<HTMLDivElement>(null);
  const refocus = useRef(false);
  // Cancel unmounts the confirm of the take-over, which held the focus.
  useEffect(() => {
    if (!refocus.current) return;
    refocus.current = false;
    toggle.current?.querySelector("button")?.focus();
  });

  const write = async (
    suspend: boolean,
    change = suspendChange(claim, suspend),
  ) => {
    if (suspendOff) return;
    setRunning(true);
    setTaking(undefined);
    const result = await apply(change).catch((error: unknown): WriteResult => ({
      ok: false,
      errors: [unavailable(error)],
    }));
    let asked: TakeOver | undefined;
    if (
      !result.ok &&
      writeMode === "direct" &&
      change.takeOver === undefined &&
      result.errors.some((e) => e.code === "Conflict")
    ) {
      const checked = await preview(change).catch(
        (error: unknown): Preview => ({
          objects: [],
          errors: [unavailable(error)],
        }),
      );
      const ownership = checked.ownership ?? [];
      if (ownership.length > 0)
        asked = {
          suspend,
          change,
          ownership,
          errors: withoutTakeOverConflicts(checked.errors, ownership),
          chosen: false,
          version: resourceVersion,
        };
    }
    setRunning(false);
    if (asked) {
      setTaking(asked);
      return;
    }
    if (result.ok) setWrittenAt(resourceVersion);
    show(
      result,
      `${suspend ? "Suspended" : "Resumed"} the management plane ${name}.`,
    );
  };

  const suspended = claim.spec.suspend === true;
  const dropped =
    taking !== undefined && taking.version !== resourceVersion
      ? taking.suspend
      : undefined;
  const goes = [
    `Every workload of the plane stops. Sign-in to Console, Web Modeler, and Optimize stops for every cluster that this plane serves (${served}).`,
    ...(ownKeycloak ? [KEYCLOAK_SUSPENDED] : []),
  ];
  const stays = [
    "The served clusters keep running, and the databases keep the data of the plane.",
  ];
  return (
    <div className="flex max-w-xl flex-col items-end gap-2">
      <div
        ref={toggle}
        className="flex flex-wrap items-start justify-end gap-2"
      >
        {takeOver && !suspendOff ? (
          <InlineConfirm
            key={`take-over-${takeOver.suspend}`}
            defaultOpen
            danger={takeOver.suspend}
            triggerLabel={takeOver.suspend ? "Suspend" : "Resume"}
            confirmLabel={`${takeOver.suspend ? "Suspend" : "Resume"} the management plane`}
            goes={takeOver.suspend ? goes : []}
            stays={takeOver.suspend ? stays : []}
            confirmDisabled={
              takeOver.errors.length > 0 ||
              takeOverOpen(takeOver.ownership, takeOver.chosen)
            }
            onCancel={() => {
              refocus.current = true;
              setTaking(undefined);
            }}
            onConfirm={() =>
              void write(
                takeOver.suspend,
                withTakeOver(takeOver.change, takeOver.ownership),
              )
            }
          >
            <TakeOverChoice
              ownership={takeOver.ownership}
              chosen={takeOver.chosen}
              onChoose={(chosen) => setTaking({ ...takeOver, chosen })}
            />
            {takeOver.errors.length > 0 && (
              <ul role="alert" className="flex flex-col gap-1 text-bad">
                {takeOver.errors.map((e, i) => (
                  <li key={i}>{e.message}</li>
                ))}
              </ul>
            )}
          </InlineConfirm>
        ) : suspended || suspendOff ? (
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
            goes={goes}
            stays={stays}
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
      {dropped !== undefined && (
        <p className="text-right text-sm text-muted">
          The plane changed.
          {dropped !== suspended &&
            ` Select ${dropped ? "Suspend" : "Resume"} again.`}
        </p>
      )}
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

// TakeOver is a write of spec.suspend that waits for the take-over of
// the fields of other tools in ownership. errors are the other errors of
// its preview, and version is the resourceVersion that it read.
type TakeOver = {
  suspend: boolean;
  change: Change;
  ownership: ObjectOwnership[];
  errors: ApiError[];
  chosen: boolean;
  version: string | undefined;
};

function unavailable(error: unknown): ApiError {
  return { code: "Unavailable", message: String(error) };
}

const KEYCLOAK_SUSPENDED =
  "Each cluster that uses the Keycloak of this plane as its OIDC issuer loses sign-in: its users and API clients cannot sign in or get tokens until the plane resumes.";
