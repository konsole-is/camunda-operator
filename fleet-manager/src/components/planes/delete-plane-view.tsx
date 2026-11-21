"use client";

import type { ReactNode } from "react";
import type { DeleteTarget } from "@/components/cluster/delete-cluster-model";
import { DeletionGroupTable } from "@/components/cluster/delete-cluster-view";
import {
  ClaimDeleteView,
  type DeleteStage,
} from "@/components/fleet/claim-delete-page";
import { notReadText } from "@/components/fleet/not-read";
import type { ReadError } from "@/lib/k8s/unread";
import type { DeletionPolicy } from "@/lib/policy";
import type {
  Change,
  ObjectOwnership,
  Preview,
  WriteMode,
  WriteResult,
} from "@/lib/write/types";
import type { DeletePlaneModel, ObjectRef } from "./delete-plane-model";

/** DATABASE_DATA says what the delete of a Database and of a DatabaseServer does to the data. */
export const DATABASE_DATA =
  "Deleting a Database keeps its data on its server. Deleting a DatabaseServer erases the server and its data.";

/**
 * DeletePlaneView is the delete page of a management plane:
 * ClaimDeleteView with what the delete of the plane takes. The other props
 * are those of ClaimDeleteView.
 */
export function DeletePlaneView({
  model,
  mode,
  policyStep,
  ...rest
}: {
  model: DeletePlaneModel;
  mode: WriteMode | undefined;
  stage: DeleteStage;
  errors: readonly ReadError[];
  remove: Change;
  policyStep: boolean;
  keepRefused?: string;
  policyOwned?: string;
  onPick(policy: DeletionPolicy): void;
  preview(change: Change, taken?: readonly ObjectOwnership[]): Promise<Preview>;
  apply(
    change: Change,
    taken?: readonly ObjectOwnership[],
  ): Promise<WriteResult>;
  show(result: WriteResult, message: string): void;
  onWritten(): void;
  onStopWaiting(): void;
}): ReactNode {
  const { name, namespace } = model;
  return (
    <ClaimDeleteView
      subject="management plane"
      name={name}
      back={{
        href: `/fleet/planes/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}`,
        label: "Back to the management plane",
      }}
      done={{ href: "/fleet/planes", label: "All management planes" }}
      deletedText={
        model.kept.length > 0
          ? `The claim ${name} is deleted. The management plane ${name} stays: the claim did not make it.`
          : `The management plane ${name} is deleted.`
      }
      choice={model.choice}
      keepRefusedOnDelete={model.keepRefused}
      unread={model.unread}
      effects={effectsOf(model, mode === "git" && policyStep)}
      caution={servedCaution(model)}
      mode={mode}
      policyStep={policyStep}
      {...rest}
    />
  );
}

// servedCaution warns that the delete changes the clusters that the plane
// serves. The finalizer of the CamundaManagementCluster withdraws the Console
// settings and the claim of the plane from each served cluster, which rolls
// its pods, and removes the web-modeler user of a basic-auth cluster. An OIDC
// cluster attaches to a plane in the keycloak mode only while it trusts the
// Keycloak of the plane, which the delete removes.
function servedCaution(model: DeletePlaneModel) {
  const served = model.served ?? [];
  const parts = [
    ...(served.length > 0
      ? [
          `This plane still serves ${served.length === 1 ? "1 cluster" : `${served.length} clusters`}. Sign-in to Console, Web Modeler, and Optimize stops for them.`,
        ]
      : []),
    ...(model.ownKeycloak
      ? [
          KEYCLOAK_DELETED,
          ...(model.choice.policy === "Delete" ? [REALM_ERASED] : []),
        ]
      : []),
  ];
  return parts.length > 0 ? parts.join(" ") : undefined;
}

const KEYCLOAK_DELETED =
  "Each cluster that uses the Keycloak of this plane as its OIDC issuer loses sign-in: its users and API clients can no longer sign in or get tokens.";

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
        <span>{notReadText(["CamundaManagementCluster"])}</span>
      ) : model.kept.length > 0 ? (
        <span>
          This delete does not touch the plane or the clusters that it serves.
        </span>
      ) : served.length === 0 ? (
        <span>
          {model.ownKeycloak
            ? `No cluster is attached now. ${KEYCLOAK_DELETED}`
            : "The plane serves no cluster now."}
        </span>
      ) : (
        <>
          <span className="text-warn">
            These clusters stay and keep running. The plane removes its Console
            settings and its claim from each, and each rolls its pods once. A
            cluster with basic authentication also loses its web-modeler user.
            {model.ownKeycloak && ` ${KEYCLOAK_DELETED}`}
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
