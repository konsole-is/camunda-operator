"use client";

import Link from "next/link";
import { type ReactNode, useState } from "react";
import type {
  DeleteState,
  DeleteTarget,
} from "@/components/cluster/delete-cluster-model";
import {
  DeleteAlert,
  DeleteProgress,
  DeletionGroupTable,
  KEEP_REFUSED_ON_DELETE,
  ReadErrors,
} from "@/components/cluster/delete-cluster-view";
import { CHANGES_OFF } from "@/components/cluster/edit/cluster-edit";
import { WaitingPanel } from "@/components/cluster/edit/waiting-panel";
import { ChangeReview } from "@/components/fleet/change-review";
import { DeletionChoiceField } from "@/components/fleet/deletion-choice";
import type { PolicyAfterWrite } from "@/components/fleet/use-claim-delete";
import { unreadDeleteText } from "@/components/setup/releases/rows";
import type { Resource } from "@/lib/k8s/kinds";
import type { DeletionPolicy } from "@/lib/policy";
import type {
  ApiError,
  Change,
  Preview,
  WriteMode,
  WriteResult,
} from "@/lib/write/types";
import type { DeletePlaneModel, ObjectRef } from "./delete-plane-model";
import type { ReadError } from "./planes-table";

/**
 * DeletePlaneStage is what the delete page of a plane shows. choose holds
 * the errors of a delete refused after the policy write, and the policy of
 * the claim after a policy write that no delete followed. waiting: the
 * policy write waits for GitOps, or for the claim controller to read it.
 * deleting holds the state of each target, whether the delete commit waits
 * for GitOps, the live claim, and what to do when the claim controller
 * refuses the delete because an option cannot keep. deleted: the claim is
 * gone. replaced: the claim was made again under its name.
 */
export type DeletePlaneStage =
  | {
      kind: "choose";
      errors?: ApiError[];
      afterPolicyWrite?: PolicyAfterWrite;
    }
  | { kind: "waiting" }
  | {
      kind: "deleting";
      progress: { target: DeleteTarget; state: DeleteState }[];
      gitWaiting: boolean;
      claim?: Resource<"CamundaManagementClusterClaim">;
      keepRefused?: string;
    }
  | { kind: "deleted" }
  | { kind: "replaced" };

/** DATABASE_DATA says what the delete of a Database and of a DatabaseServer does to the data. */
export const DATABASE_DATA =
  "Deleting a Database keeps its data on its server. Deleting a DatabaseServer erases the server and its data.";

/**
 * DeletePlaneView is the delete page of a management plane. It never
 * writes: the confirm of its review calls apply with remove, and the
 * review previews remove once for each remove object. Without a mode it
 * shows CHANGES_OFF and what the delete takes, and no review. While
 * model.unread has a kind it shows what the delete takes, with each fact
 * that depends on that kind as not known, and no review, unless a write
 * runs. While keepRefused is set it shows keepRefused in place of the
 * review, unless a write runs. policyStep says that the delete writes the
 * deletion policy first.
 */
export function DeletePlaneView({
  model,
  mode,
  stage,
  errors,
  remove,
  policyStep,
  keepRefused,
  onPick,
  preview,
  apply,
  show,
  onWritten,
  onStopWaiting,
}: {
  model: DeletePlaneModel;
  mode: WriteMode | undefined;
  stage: DeletePlaneStage;
  errors: readonly ReadError[];
  remove: Change;
  policyStep: boolean;
  keepRefused?: string;
  onPick(policy: DeletionPolicy): void;
  preview(change: Change): Promise<Preview>;
  apply(change: Change): Promise<WriteResult>;
  show(result: WriteResult, message: string): void;
  onWritten(): void;
  onStopWaiting(): void;
}): ReactNode {
  const { name, namespace } = model;
  const planeHref = `/fleet/planes/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}`;
  const effects = effectsOf(model, mode === "git" && policyStep);
  const [writing, setWriting] = useState(false);
  const takes = (
    <section aria-label="What the delete takes">
      <ul className="flex list-disc flex-col gap-1 pl-5 text-sm text-text-2">
        {effects.map((effect, i) => (
          <li key={i}>{effect}</li>
        ))}
      </ul>
    </section>
  );
  return (
    <div className="flex max-w-3xl min-w-0 flex-col gap-6">
      <div className="flex flex-col gap-2">
        <Link
          href={planeHref}
          className="relative w-fit text-sm text-muted hit-area hover:text-text"
        >
          Back to the management plane
        </Link>
        <h1 className="font-heading text-2xl font-bold tracking-[-0.02em] break-all">
          Delete the management plane {name}
        </h1>
      </div>
      {stage.kind === "deleted" ? (
        <div className="flex flex-col gap-2 text-sm">
          <p role="status" className="text-text">
            {model.kept.length > 0
              ? `The claim ${name} is deleted. The management plane ${name} stays: the claim did not make it.`
              : `The management plane ${name} is deleted.`}
          </p>
          <Link
            href="/fleet/planes"
            className="relative w-fit text-accent hit-area hover:underline"
          >
            All management planes
          </Link>
        </div>
      ) : stage.kind === "replaced" ? (
        <div className="flex flex-col gap-2 text-sm">
          <p role="status" className="text-text">
            The claim {name} was made again after this page opened. Open the
            delete page again to delete the new claim.
          </p>
          <Link
            href={planeHref}
            className="relative w-fit text-accent hit-area hover:underline"
          >
            Back to the management plane
          </Link>
        </div>
      ) : stage.kind === "waiting" ? (
        <>
          <ReadErrors errors={errors} />
          <WaitingPanel
            text={
              mode === "git"
                ? "Waiting for GitOps to apply the deletion policy. The fleet manager then removes the claim file."
                : "Waiting for the operator to read the deletion policy. The fleet manager then deletes the claim."
            }
            onStopWaiting={onStopWaiting}
          />
        </>
      ) : stage.kind === "deleting" ? (
        <>
          <ReadErrors errors={errors} />
          <DeleteProgress name={name} stage={stage} />
        </>
      ) : (
        <>
          <DeletionChoiceField
            subject="management plane"
            choice={model.choice}
            onChange={onPick}
            disabled={writing}
          />
          {model.keepRefused && model.choice.policy !== "Orphan" && (
            <p className="-mt-3 text-sm text-warn">{KEEP_REFUSED_ON_DELETE}</p>
          )}
          {mode === "git" && (
            <p className="text-sm text-text-2">{PRUNE_NOTE}</p>
          )}
          <ReadErrors errors={errors} />
          {stage.errors && stage.errors.length > 0 && (
            <DeleteAlert texts={stage.errors.map((e) => e.message)} />
          )}
          {stage.afterPolicyWrite && (
            <p role="status" className="text-sm text-warn">
              {policyNote(name, stage.afterPolicyWrite)}
            </p>
          )}
          {mode === undefined ? (
            <>
              <p role="alert" className="text-sm text-bad">
                {CHANGES_OFF}
              </p>
              {takes}
            </>
          ) : model.unread.length > 0 && !writing ? (
            <>
              <p className="text-sm text-muted">
                {unreadDeleteText(model.unread)}
              </p>
              {takes}
            </>
          ) : keepRefused && !writing ? (
            <>
              <p className="text-sm text-warn">{keepRefused}</p>
              {takes}
            </>
          ) : (
            <ChangeReview
              change={remove}
              mode={mode}
              diffLabel="The claim that the delete removes"
              effects={effects}
              caution={servedCaution(model)}
              confirm={{
                label: "Delete the management plane",
                gitLabel: "Push the delete to Git",
                danger: true,
                typed: name,
              }}
              savedMessage={
                mode === "direct"
                  ? policyStep
                    ? `Set the deletion policy of ${name}. The delete follows when the operator reads it.`
                    : `The claim ${name} is being deleted.`
                  : policyStep
                    ? `Pushed the deletion policy of ${name}. The delete follows when GitOps applies it.`
                    : `Pushed the delete of ${name}.`
              }
              preview={preview}
              apply={apply}
              show={show}
              onWritten={onWritten}
              onWriting={setWriting}
            />
          )}
        </>
      )}
    </div>
  );
}

const PRUNE_NOTE =
  "Your GitOps tool must prune: the fleet manager removes the claim file, and GitOps deletes the claim.";

// servedCaution warns that the delete changes the clusters that the plane
// serves. The finalizer of the CamundaManagementCluster withdraws the Console
// settings and the claim of the plane from each served cluster, which rolls
// its pods, and removes the web-modeler user of a basic-auth cluster. An OIDC
// cluster attaches to a plane in the keycloak mode only while it trusts the
// Keycloak of the plane, which the delete removes.
function servedCaution(model: DeletePlaneModel) {
  const { served } = model;
  if (!served || served.length === 0) return undefined;
  return [
    `This plane still serves ${served.length === 1 ? "1 cluster" : `${served.length} clusters`}. Sign-in to Console, Web Modeler, and Optimize stops for them.`,
    ...(model.ownKeycloak
      ? [
          "Those that use OIDC sign in at the Keycloak of this plane, so their users and API clients can no longer sign in or get tokens.",
          ...(model.choice.policy === "Delete" ? [REALM_ERASED] : []),
        ]
      : []),
  ].join(" ");
}

const REALM_ERASED =
  "When a DatabaseServer of an option holds the Keycloak database, the delete erases the users and clients of the realm.";

function refText({ namespace, name }: ObjectRef) {
  return `${namespace}/${name}`;
}

function effectsOf(model: DeletePlaneModel, twoCommits: boolean): ReactNode[] {
  const rows = (targets: readonly DeleteTarget[]) =>
    targets.map((t) => (
      <li key={`${t.kind}/${t.name}`}>
        {t.label}
        {t.note && <span className="text-muted">: {t.note}</span>}
      </li>
    ));
  const planeUnread = model.unread.includes("CamundaManagementCluster");
  const known = (t: DeleteTarget) =>
    !planeUnread || t.kind !== "CamundaManagementCluster";
  const always = model.targets.filter(
    (t) => t.kind !== "PluginRequest" && known(t),
  );
  const kept = model.kept.filter(known);
  const served = model.served;
  return [
    <div key="always" className="flex flex-col gap-1">
      <span className="font-medium text-text">Always deleted</span>
      <ul className="flex flex-col gap-0.5">
        {rows(always)}
        {always.some((t) => t.kind === "CamundaManagementCluster") && (
          <li className="text-muted">
            The plane takes its workloads, Services, and generated Secrets with
            it.
          </li>
        )}
      </ul>
    </div>,
    ...(kept.length > 0
      ? [
          <div key="kept" className="flex flex-col gap-1">
            <span className="font-medium text-text">Kept</span>
            <ul className="flex flex-col gap-0.5">{rows(kept)}</ul>
          </div>,
        ]
      : []),
    ...(planeUnread
      ? [
          <div key="unknown" className="flex flex-col gap-1">
            <span className="font-medium text-text">Not known</span>
            <ul className="flex flex-col gap-0.5">
              <li>
                CamundaManagementCluster {model.name}
                <span className="text-muted">: not known</span>
              </li>
            </ul>
          </div>,
        ]
      : []),
    <div key="served" className="flex flex-col gap-1">
      <span className="font-medium text-text">Clusters served</span>
      {planeUnread || served === undefined ? (
        <span>
          Not known: the page could not read CamundaManagementCluster.
        </span>
      ) : model.kept.length > 0 ? (
        <span>
          This delete does not touch the plane or the clusters that it serves.
        </span>
      ) : served.length === 0 ? (
        <span>The plane serves no cluster now.</span>
      ) : (
        <>
          <span className="text-warn">
            These clusters stay and keep running. The plane removes its Console
            settings and its claim from each, and each rolls its pods once. A
            cluster with basic authentication also loses its web-modeler user.
            {model.ownKeycloak &&
              " A cluster that uses OIDC signs in at the Keycloak of this plane, so its users and API clients can no longer sign in or get tokens."}
          </span>
          <ul className="flex flex-col gap-0.5">
            {served.map((c) => (
              <li key={refText(c)} className="break-all">
                {refText(c)}
              </li>
            ))}
          </ul>
        </>
      )}
    </div>,
    ...(planeUnread || model.contract === undefined
      ? []
      : [
          <div key="contract" className="flex flex-col gap-1">
            <span className="font-medium text-text">Contract</span>
            <span>
              The ManagementAuthConfig {model.contract} goes.{" "}
              {model.readers === undefined
                ? "Which CamundaOptimize reads it is not known: the page could not read CamundaOptimize."
                : model.readers.length === 0
                  ? "No CamundaOptimize reads it."
                  : `Each CamundaOptimize that reads it then reports InvalidReference: ${model.readers.map(refText).join(", ")}.`}
            </span>
          </div>,
        ]),
    ...model.groups.map((group) => (
      <DeletionGroupTable
        key={group.id}
        group={group}
        known={model.fatesKnown}
      />
    )),
    ...(model.choice.policy === "Delete" ? [DATABASE_DATA] : []),
    "The Secrets that you referenced stay.",
    ...(twoCommits
      ? [
          "Pushes two commits: the deletion policy first, then the delete once GitOps applied it.",
        ]
      : []),
  ];
}

function policyNote(name: string, after: PolicyAfterWrite): string {
  const label = (policy: DeletionPolicy) =>
    policy === "Orphan" ? "Keep" : "Delete";
  const held = `The claim ${name} has the deletion policy ${label(after.held)}.`;
  return after.unapplied
    ? `The management plane is not deleted. The repository holds the commit that sets the deletion policy of ${name} to ${label(after.unapplied)}, and GitOps has not applied it yet. ${held}`
    : `The management plane is not deleted. ${held}`;
}
