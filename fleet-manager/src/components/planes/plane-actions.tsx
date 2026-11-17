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
import { type ManagedObject, managerKind, ownerTree } from "@/lib/merge";
import { suspendChange } from "@/lib/planes/suspend";
import { withTakeOver } from "@/lib/write/ownership";
import { refKey } from "@/lib/write/refs";
import type {
  ApiError,
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
 * them in TakeOverChoice, and its confirm writes the change of the claim
 * that the page shows then, with withTakeOver of the fields that it showed,
 * once takeOverOpen is false. The choice starts off for each write. A
 * change of metadata.generation of the claim, or of the managers of
 * spec.suspend, closes that confirm and drops the choice. A line then says
 * that the plane changed, until a write starts or the Suspend confirm
 * opens. Otherwise the refusal shows as before.
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

  const generation = claim.metadata?.generation;
  const owners = suspendOwners(claim);
  const [takeOver, setTakeOver] = useState<TakeOver>();
  // The choice that a take-over dropped, until a write or a confirm starts.
  const [dropped, setDropped] = useState<boolean>();
  if (
    takeOver !== undefined &&
    (takeOver.generation !== generation || takeOver.owners !== owners)
  ) {
    setTakeOver(undefined);
    setDropped(takeOver.suspend);
  }
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
    setTakeOver(undefined);
    setDropped(undefined);
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
          ownership,
          errors: withoutTakeOverConflicts(checked.errors, ownership),
          chosen: false,
          generation,
          owners,
        };
    }
    setRunning(false);
    if (asked) {
      setTakeOver(asked);
      return;
    }
    if (result.ok) setWrittenAt(resourceVersion);
    show(
      result,
      `${suspend ? "Suspended" : "Resumed"} the management plane ${name}.`,
    );
  };

  const suspended = claim.spec.suspend === true;
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
              setTakeOver(undefined);
            }}
            onConfirm={() =>
              void write(
                takeOver.suspend,
                withTakeOver(
                  suspendChange(claim, takeOver.suspend),
                  takeOver.ownership,
                ),
              )
            }
          >
            <TakeOverChoice
              ownership={takeOver.ownership}
              chosen={takeOver.chosen}
              onChoose={(chosen) => setTakeOver({ ...takeOver, chosen })}
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
            onOpen={() => setDropped(undefined)}
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
// its preview. generation and owners are those of the claim that it read.
type TakeOver = {
  suspend: boolean;
  ownership: ObjectOwnership[];
  errors: ApiError[];
  chosen: boolean;
  generation: number | undefined;
  owners: string;
};

// suspendOwners names each managedFields entry of claim that owns
// spec.suspend, with the kind of its manager. A status write leaves it as
// it is. The server checks a take-over list by its paths only, so a new
// owner of the same path must close the confirm that named the old one.
function suspendOwners(claim: Resource<"CamundaManagementClusterClaim">) {
  const object = claim as unknown as ManagedObject;
  const tree = ownerTree(object, (entry) => !entry.subresource);
  const owners =
    tree.children.get("f:spec")?.children.get("f:suspend")?.owners ?? [];
  return owners
    .map((o) =>
      JSON.stringify([o.manager, o.operation, managerKind(o.manager, object)]),
    )
    .sort()
    .join("\n");
}

function unavailable(error: unknown): ApiError {
  return { code: "Unavailable", message: String(error) };
}

const KEYCLOAK_SUSPENDED =
  "Each cluster that uses the Keycloak of this plane as its OIDC issuer loses sign-in: its users and API clients cannot sign in or get tokens until the plane resumes.";
