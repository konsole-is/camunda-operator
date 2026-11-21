"use client";

import type { ReactNode } from "react";
import {
  ClaimDeleteView,
  type DeleteStage,
} from "@/components/fleet/claim-delete-page";
import { Badge } from "@/components/ui/badge";
import {
  Table,
  TableBody,
  TableCaption,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import type { DeletionGroup } from "@/lib/claims/deletion";
import type { ReadError } from "@/lib/k8s/unread";
import type { DeletionPolicy } from "@/lib/policy";
import type {
  Change,
  ObjectOwnership,
  Preview,
  WriteMode,
  WriteResult,
} from "@/lib/write/types";
import type { DeleteClusterModel, DeleteTarget } from "./delete-cluster-model";

/**
 * DeleteClusterView is the delete page of a cluster: ClaimDeleteView with
 * what the delete of the cluster takes. The other props are those of
 * ClaimDeleteView.
 */
export function DeleteClusterView({
  model,
  mode,
  policyStep,
  ...rest
}: {
  model: DeleteClusterModel;
  mode: WriteMode | undefined;
  stage: DeleteStage;
  errors: readonly ReadError[];
  remove: Change;
  policyStep: boolean;
  keepRefused?: string;
  policyOwned?: string;
  onPick(policy: DeletionPolicy): void;
  preview(change: Change): Promise<Preview>;
  apply(
    change: Change,
    taken?: readonly ObjectOwnership[],
  ): Promise<WriteResult>;
  show(result: WriteResult, message: string): void;
  onWritten(): void;
  onStopWaiting(): void;
}): ReactNode {
  const { name, namespace } = model;
  const clusterKept = model.kept.some((t) => t.kind === "CamundaCluster");
  return (
    <ClaimDeleteView
      subject="cluster"
      name={name}
      back={{
        href: `/fleet/clusters/${namespace}/${name}`,
        label: "Back to the cluster",
      }}
      done={{ href: "/fleet", label: "Back to the fleet" }}
      deletedText={
        clusterKept
          ? `The claim ${name} is deleted. The cluster ${name} stays: the claim did not make it.`
          : `The cluster ${name} is deleted.`
      }
      choice={model.choice}
      keepRefusedOnDelete={model.keepRefused}
      unread={model.unread}
      effects={effectsOf(model, mode === "git" && policyStep)}
      caution={model.warning}
      mode={mode}
      policyStep={policyStep}
      {...rest}
    />
  );
}

// The page decides from the lists of these kinds whether the claim controls
// an object, so an object of such a kind that it cannot read is not known.
const ATTACHMENT_KINDS: readonly string[] = [
  "CamundaCluster",
  "CamundaOptimize",
  "BackupSchedule",
];

function effectsOf(
  model: DeleteClusterModel,
  twoCommits: boolean,
): ReactNode[] {
  const unknownKinds = model.unread.filter((k) => ATTACHMENT_KINDS.includes(k));
  const known = (t: DeleteTarget) => !unknownKinds.includes(t.kind);
  const always = model.targets.filter(
    (t) => t.kind !== "PluginRequest" && known(t),
  );
  const kept = model.kept.filter(known);
  const unknown = [...model.targets, ...model.kept].filter((t) => !known(t));
  const rows = (targets: readonly DeleteTarget[], note?: string) =>
    targets.map((t) => (
      <li key={`${t.kind}/${t.name}`}>
        {t.label}
        {(note ?? t.note) && (
          <span className="text-muted">: {note ?? t.note}</span>
        )}
      </li>
    ));
  return [
    <div key="always" className="flex flex-col gap-1">
      <span className="font-medium text-text">Always deleted</span>
      <ul className="flex flex-col gap-0.5">{rows(always)}</ul>
    </div>,
    ...(kept.length > 0
      ? [
          <div key="kept" className="flex flex-col gap-1">
            <span className="font-medium text-text">Kept</span>
            <ul className="flex flex-col gap-0.5">{rows(kept)}</ul>
          </div>,
        ]
      : []),
    ...(unknownKinds.length > 0
      ? [
          <div key="unknown" className="flex flex-col gap-1">
            <span className="font-medium text-text">Not known</span>
            <ul className="flex flex-col gap-0.5">
              {rows(unknown, "not known")}
              {unknownKinds
                .filter((kind) => !unknown.some((t) => t.kind === kind))
                .map((kind) => (
                  <li key={kind}>
                    {kind}
                    <span className="text-muted">: not known</span>
                  </li>
                ))}
            </ul>
          </div>,
        ]
      : []),
    ...model.groups.map((group) => (
      <DeletionGroupTable
        key={group.id}
        group={group}
        known={model.fatesKnown}
      />
    )),
    ...(model.backups > 0
      ? [
          `The ${model.backups} backups of ${model.name} stay, and their artifacts stay in the bucket.`,
        ]
      : []),
    ...(twoCommits
      ? [
          "Pushes two commits: the deletion policy first, then the delete once GitOps applied it.",
        ]
      : []),
  ];
}

// The fate is the one emphasis of the page: Deleted in the bad tone, so
// that a reader sees at a glance what goes.

/**
 * DeletionGroupTable shows the resources of one option of a claim with the
 * fate of each. While known is false, each fate reads Not known.
 */
export function DeletionGroupTable({
  group,
  known,
}: {
  group: DeletionGroup;
  known: boolean;
}) {
  const many = group.id.includes("/");
  const caption = many ? `${group.label} (${group.registration})` : group.label;
  return (
    <div className="flex min-w-0 flex-col gap-1">
      {group.rows.length === 0 ? (
        <>
          <span className="font-medium text-text">{caption}</span>
          <span>
            {known
              ? "This option made no resources."
              : "What this option made is not known."}
          </span>
        </>
      ) : (
        <Table label={`Resources of ${caption}`}>
          <TableCaption className="mt-0 mb-1 caption-top text-left font-medium text-text">
            {caption}
          </TableCaption>
          <TableHeader>
            <TableRow>
              <TableHead>Kind</TableHead>
              <TableHead>Name</TableHead>
              <TableHead>ID</TableHead>
              <TableHead>What happens</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {group.rows.map((row) => (
              <TableRow key={`${row.kind}/${row.name}`}>
                <TableCell>{row.kind}</TableCell>
                <TableCell className="break-all whitespace-normal">
                  {row.name}
                </TableCell>
                <TableCell className="break-all whitespace-normal text-muted">
                  {row.id ?? "-"}
                </TableCell>
                <TableCell>
                  {known ? (
                    <Badge
                      variant={
                        row.fate === "Deleted" ? "destructive" : "outline"
                      }
                      className={row.fate === "Kept" ? "text-muted" : undefined}
                    >
                      {row.fate}
                    </Badge>
                  ) : (
                    <Badge variant="outline">Not known</Badge>
                  )}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
      {group.alwaysDeletes && (
        <span className="text-warn">
          This option always deletes what it made.
        </span>
      )}
    </div>
  );
}
