"use client";

import { CircleAlert } from "lucide-react";
import Link from "next/link";
import { type ReactNode, useState } from "react";
import { ChangeReview } from "@/components/fleet/change-review";
import { DeletionChoiceField } from "@/components/fleet/deletion-choice";
import { PrecheckLine } from "@/components/fleet/precheck-line";
import type { PolicyAfterWrite } from "@/components/fleet/use-claim-delete";
import type { ReadError } from "@/components/planes/planes-table";
import { unreadDeleteText } from "@/components/setup/releases/rows";
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
import type { Resource } from "@/lib/k8s/kinds";
import type { DeletionPolicy } from "@/lib/policy";
import type {
  ApiError,
  Change,
  Preview,
  WriteMode,
  WriteResult,
} from "@/lib/write/types";
import type {
  DeleteClusterModel,
  DeleteState,
  DeleteTarget,
} from "./delete-cluster-model";
import { CHANGES_OFF } from "./edit/cluster-edit";
import { WaitingPanel } from "./edit/waiting-panel";

/**
 * KEEP_REFUSED_ON_DELETE says that a claim keeps its resources, but an
 * option cannot, so the delete removes them.
 */
export const KEEP_REFUSED_ON_DELETE =
  "The claim keeps its resources, but an option below cannot keep them. The delete removes them.";

/**
 * DeleteStage is what the delete page shows. choose holds the errors of a
 * delete refused after the policy write, and the policy of the claim after
 * a policy write that no delete followed. waiting: the delete waits for
 * the policy write. deleting holds the state of
 * each target, whether the delete commit waits for GitOps, the live claim,
 * and what to do when the operator refuses the delete because an option
 * cannot keep. deleted: the claim is gone. replaced: the claim was made
 * again under its name.
 */
export type DeleteStage =
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
      claim?: Resource<"CamundaClusterClaim">;
      keepRefused?: string;
    }
  | { kind: "deleted" }
  | { kind: "replaced" };

const PRUNE_NOTE =
  "Your GitOps tool must prune: the fleet manager removes the claim file, and GitOps deletes the claim.";

/**
 * DeleteClusterView is the delete page of a cluster. It never writes: the
 * confirm of its review calls apply with remove, and the review previews
 * remove once for each remove object. Without a mode it shows CHANGES_OFF
 * and what the delete takes, and no review. While model.unread has a kind
 * it shows what the delete takes, and no review, unless a write runs.
 * While keepRefused is set, a mode is set, and model.unread is empty, it
 * shows keepRefused in place of the review, unless a write runs.
 * policyStep says that the delete writes the deletion policy first.
 */
export function DeleteClusterView({
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
  model: DeleteClusterModel;
  mode: WriteMode | undefined;
  stage: DeleteStage;
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
  const clusterHref = `/fleet/clusters/${namespace}/${name}`;
  const effects = effectsOf(model, mode === "git" && policyStep);
  const clusterKept = model.kept.some((t) => t.kind === "CamundaCluster");
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
          href={clusterHref}
          className="relative w-fit text-sm text-muted hit-area hover:text-text"
        >
          Back to the cluster
        </Link>
        <h1 className="font-heading text-2xl font-bold tracking-[-0.02em] break-all">
          Delete the cluster {name}
        </h1>
      </div>
      {stage.kind === "deleted" ? (
        <div className="flex flex-col gap-2 text-sm">
          <p role="status" className="text-text">
            {clusterKept
              ? `The claim ${name} is deleted. The cluster ${name} stays: the claim did not make it.`
              : `The cluster ${name} is deleted.`}
          </p>
          <Link
            href="/fleet"
            className="relative w-fit text-accent hit-area hover:underline"
          >
            Back to the fleet
          </Link>
        </div>
      ) : stage.kind === "replaced" ? (
        <div className="flex flex-col gap-2 text-sm">
          <p role="status" className="text-text">
            The claim {name} was made again after this page opened. Open the
            delete page again to delete the new claim.
          </p>
          <Link
            href={clusterHref}
            className="relative w-fit text-accent hit-area hover:underline"
          >
            Back to the cluster
          </Link>
        </div>
      ) : stage.kind === "waiting" ? (
        <>
          <ReadErrors errors={errors} />
          <WaitingPanel
            text={
              mode === "git"
                ? "Waiting for GitOps to apply the deletion policy. The operator then reads it, and the fleet manager removes the claim file."
                : "Waiting for the operator to read the deletion policy. The fleet manager then deletes the claim."
            }
            onStopWaiting={onStopWaiting}
          />
        </>
      ) : stage.kind === "deleting" ? (
        <>
          <ReadErrors errors={errors} />
          <Progress name={name} stage={stage} />
        </>
      ) : (
        <>
          <DeletionChoiceField
            subject="cluster"
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
            <Alert texts={stage.errors.map((e) => e.message)} />
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
              caution={model.warning}
              confirm={{
                label: "Delete the cluster",
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
      <GroupTable key={group.id} group={group} known={model.fatesKnown} />
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
function GroupTable({
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

function Progress({
  name,
  stage,
}: {
  name: string;
  stage: Extract<DeleteStage, { kind: "deleting" }>;
}) {
  return (
    <section aria-label="Delete progress" className="flex flex-col gap-3">
      {stage.gitWaiting && (
        <div className="flex flex-col gap-1 text-sm">
          <p role="status" className="text-text">
            Waiting for GitOps to delete {name}
          </p>
          <p className="text-text-2">{PRUNE_NOTE}</p>
        </div>
      )}
      {stage.claim && <PrecheckLine claim={stage.claim} />}
      {stage.keepRefused && (
        <p className="text-sm text-warn">{stage.keepRefused}</p>
      )}
      <ol className="flex flex-col divide-y divide-divider rounded-md border border-border">
        {stage.progress.map(({ target, state }) => (
          <li
            key={`${target.kind}/${target.name}`}
            className="flex flex-wrap items-center justify-between gap-x-4 gap-y-1 px-3 py-2 text-sm"
          >
            <span className="flex min-w-0 flex-col">
              <span className="break-all text-text">{target.label}</span>
              {target.note && <span className="text-muted">{target.note}</span>}
            </span>
            <Badge
              variant={
                state === "Waiting" || state === "Not known"
                  ? "outline"
                  : "secondary"
              }
              className={state === "Deleted" ? "text-muted" : undefined}
            >
              {state}
            </Badge>
          </li>
        ))}
      </ol>
    </section>
  );
}

function policyNote(name: string, after: PolicyAfterWrite): string {
  const label = (policy: DeletionPolicy) =>
    policy === "Orphan" ? "Keep" : "Delete";
  const held = `The claim ${name} has the deletion policy ${label(after.held)}.`;
  return after.unapplied
    ? `The cluster is not deleted. The repository holds the commit that sets the deletion policy of ${name} to ${label(after.unapplied)}, and GitOps has not applied it yet. ${held}`
    : `The cluster is not deleted. ${held}`;
}

function ReadErrors({ errors }: { errors: readonly ReadError[] }) {
  return errors.map((e, i) => (
    <p key={i} role="alert" className="text-sm text-bad">
      Could not read {e.kind}: {e.message}
    </p>
  ));
}

function Alert({ texts }: { texts: string[] }) {
  return (
    <div
      role="alert"
      className="flex gap-3 rounded-md border border-bad/40 bg-bad/10 p-3 text-sm text-text"
    >
      <CircleAlert aria-hidden className="mt-0.5 size-4 shrink-0 text-bad" />
      <ul className="flex flex-col gap-1">
        {texts.map((text, i) => (
          <li key={i}>{text}</li>
        ))}
      </ul>
    </div>
  );
}
