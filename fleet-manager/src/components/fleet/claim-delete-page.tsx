"use client";

import { CircleAlert } from "lucide-react";
import Link from "next/link";
import { type ReactNode, useState } from "react";
import {
  deleteProgress,
  type DeleteState,
  type DeleteTarget,
  mergeTargets,
} from "@/components/cluster/delete-cluster-model";
import { WaitingPanel } from "@/components/cluster/edit/waiting-panel";
import { ChangeReview } from "@/components/fleet/change-review";
import { CHANGES_OFF } from "@/components/fleet/claim-edit";
import { DeletionChoiceField } from "@/components/fleet/deletion-choice";
import { PrecheckLine } from "@/components/fleet/precheck-line";
import type {
  ClaimDeleteStage,
  PolicyAfterWrite,
} from "@/components/fleet/use-claim-delete";
import { Badge } from "@/components/ui/badge";
import { useWatchesFresh } from "@/lib/k8s/hooks";
import type { KindName } from "@/lib/k8s/kinds";
import type { ListOptions } from "@/lib/k8s/read";
import {
  type ReadError,
  unreadDeleteText,
  unreadKinds,
} from "@/lib/k8s/unread";
import type { DeletionChoice, DeletionPolicy } from "@/lib/policy";
import type {
  ApiError,
  Change,
  ObjectOwnership,
  Preview,
  WriteMode,
  WriteResult,
} from "@/lib/write/types";

/**
 * KEEP_REFUSED_ON_DELETE says that a claim keeps its resources, but an
 * option cannot, so the delete removes them.
 */
export const KEEP_REFUSED_ON_DELETE =
  "The claim keeps its resources, but an option below cannot keep them. The delete removes them.";

const PRUNE_NOTE =
  "Your GitOps tool must prune: the fleet manager removes the claim file, and GitOps deletes the claim.";

type DeleteClaim = Parameters<typeof PrecheckLine>[0]["claim"];

/**
 * DeleteStage is what a delete page of a claim shows. choose holds the
 * errors of a delete refused after the policy write, and the policy of the
 * claim after a policy write that no delete followed. waiting: the delete
 * waits for the policy write, and blocked says why the options do not get
 * it. deleting holds the state of each target, whether the delete commit
 * waits for GitOps, the live claim, and what to do when the operator
 * refuses the delete because an option cannot keep. deleted: the claim is
 * gone. replaced: the claim was made again under its name.
 */
export type DeleteStage =
  | {
      kind: "choose";
      errors?: ApiError[];
      afterPolicyWrite?: PolicyAfterWrite;
    }
  | { kind: "waiting"; blocked?: string }
  | {
      kind: "deleting";
      progress: { target: DeleteTarget; state: DeleteState }[];
      gitWaiting: boolean;
      claim?: DeleteClaim;
      keepRefused?: string;
    }
  | { kind: "deleted" }
  | { kind: "replaced" };

/** DeleteWatch is one watch of a delete page, with the result of its hook. */
export type DeleteWatch = {
  kind: KindName;
  opts: ListOptions & { name?: string };
  error?: string;
};

/**
 * useDeleteReads returns the failed reads of a delete page and the kinds
 * that it cannot rely on. A current snapshot of a watch replaces the error
 * of initial for its kind. claimReads are the reads of useClaimDelete.
 */
export function useDeleteReads(
  watched: readonly DeleteWatch[],
  initial: readonly ReadError[],
  claimReads: { errors: ReadError[]; stale: string[] },
): { errors: ReadError[]; unread: string[] } {
  const fresh = useWatchesFresh(watched);
  const replaced = watched
    .filter((w, i) => fresh[i] && !w.error)
    .map((w): string => w.kind);
  const errors: ReadError[] = [
    ...initial.filter((e) => !replaced.includes(e.kind)),
    ...watched.flatMap((w) =>
      w.error ? [{ kind: w.kind, message: w.error }] : [],
    ),
    ...claimReads.errors,
  ];
  const unread = unreadKinds({
    errors,
    stale: [
      ...watched.filter((_, i) => !fresh[i]).map((w) => w.kind),
      ...claimReads.stale,
    ],
  });
  return { errors, unread };
}

/**
 * useDeleteStage returns the stage of a delete page from the stage of
 * useClaimDelete. While the delete runs, it gives the state of each of
 * targets from the live objects of live, and a target stays once the
 * delete started, so that an object that is gone shows as Deleted.
 */
export function useDeleteStage(
  run: { stage: ClaimDeleteStage; claim?: DeleteClaim },
  targets: readonly DeleteTarget[],
  live: Parameters<typeof deleteProgress>[1],
  unread: readonly string[],
): DeleteStage {
  const [kept, setKept] = useState<DeleteTarget[]>();
  if (run.stage.kind !== "deleting") return run.stage;
  const merged = mergeTargets(kept ?? [], targets);
  if (JSON.stringify(merged) !== JSON.stringify(kept)) setKept(merged);
  return {
    kind: "deleting",
    progress: deleteProgress(merged, live, unread),
    gitWaiting: run.stage.gitWaiting,
    ...(run.claim && { claim: run.claim }),
    ...(run.stage.keepRefused && { keepRefused: run.stage.keepRefused }),
  };
}

/**
 * ClaimDeleteView is the frame of the delete page of a claim of either
 * kind. It never writes: the confirm of its review calls apply with
 * remove, and the review previews remove once for each remove object.
 * Without a mode it shows CHANGES_OFF and effects, and no review. While
 * unread has a kind it shows effects and no review, unless a write runs.
 * While keepRefused is set, it shows keepRefused in place of the review,
 * unless a write runs. keepRefusedOnDelete says that the claim keeps its
 * resources and an option cannot. policyStep says that the delete writes
 * the deletion policy first. policyOwned says what the delete does while
 * another field manager sets the deletion policy. apply gets the ownership
 * that the take-over of the review chose. deletedText is the line once the
 * claim is gone.
 */
export function ClaimDeleteView({
  subject,
  name,
  back,
  done,
  deletedText,
  choice,
  keepRefusedOnDelete,
  unread,
  effects,
  caution,
  mode,
  stage,
  errors,
  remove,
  policyStep,
  keepRefused,
  policyOwned,
  onPick,
  preview,
  apply,
  show,
  onWritten,
  onStopWaiting,
}: {
  subject: "cluster" | "management plane";
  name: string;
  back: { href: string; label: string };
  done: { href: string; label: string };
  deletedText: string;
  choice: DeletionChoice;
  keepRefusedOnDelete: boolean;
  unread: readonly string[];
  effects: ReactNode[];
  caution?: ReactNode;
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
  const [writing, setWriting] = useState(false);
  const reviewed = writing || (unread.length === 0 && !keepRefused);
  const takes = (
    <section aria-label="What the delete takes">
      <ul className="flex list-disc flex-col gap-1 pl-5 text-sm text-text-2">
        {effects.map((effect, i) => (
          <li key={i}>{effect}</li>
        ))}
      </ul>
    </section>
  );
  const backLink = (className: string) => (
    <Link href={back.href} className={`relative w-fit hit-area ${className}`}>
      {back.label}
    </Link>
  );
  return (
    <div className="flex max-w-3xl min-w-0 flex-col gap-6">
      <div className="flex flex-col gap-2">
        {backLink("text-sm text-muted hover:text-text")}
        <h1 className="font-heading text-2xl font-bold tracking-[-0.02em] break-all">
          Delete the {subject} {name}
        </h1>
      </div>
      {stage.kind === "deleted" ? (
        <div className="flex flex-col gap-2 text-sm">
          <p role="status" className="text-text">
            {deletedText}
          </p>
          <Link
            href={done.href}
            className="relative w-fit text-accent hit-area hover:underline"
          >
            {done.label}
          </Link>
        </div>
      ) : stage.kind === "replaced" ? (
        <div className="flex flex-col gap-2 text-sm">
          <p role="status" className="text-text">
            The claim {name} was made again after this page opened. Open the
            delete page again to delete the new claim.
          </p>
          {backLink("text-accent hover:underline")}
        </div>
      ) : stage.kind === "waiting" ? (
        <>
          <ReadErrors errors={errors} />
          {stage.blocked && (
            <p role="status" className="text-sm text-warn">
              {stage.blocked}
            </p>
          )}
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
          <DeleteProgress name={name} stage={stage} />
        </>
      ) : (
        <>
          <DeletionChoiceField
            subject={subject}
            choice={choice}
            onChange={onPick}
            disabled={writing}
          />
          {keepRefusedOnDelete && choice.policy !== "Orphan" && (
            <p className="-mt-3 text-sm text-warn">{KEEP_REFUSED_ON_DELETE}</p>
          )}
          {policyOwned && mode === "direct" && reviewed && (
            <p className="text-sm text-warn">{policyOwned}</p>
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
              {policyNote(subject, name, stage.afterPolicyWrite)}
            </p>
          )}
          {mode === undefined ? (
            <>
              <p role="alert" className="text-sm text-bad">
                {CHANGES_OFF}
              </p>
              {takes}
            </>
          ) : unread.length > 0 && !writing ? (
            <>
              <p className="text-sm text-muted">{unreadDeleteText(unread)}</p>
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
              caution={caution}
              confirm={{
                label: `Delete the ${subject}`,
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
              otherWrite
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

/**
 * DeleteProgress shows a delete of the claim name that is under way: the
 * wait for GitOps, why the claim controller refuses the delete, and the
 * state of each target.
 */
export function DeleteProgress({
  name,
  stage,
}: {
  name: string;
  stage: {
    progress: { target: DeleteTarget; state: DeleteState }[];
    gitWaiting: boolean;
    claim?: DeleteClaim;
    keepRefused?: string;
  };
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

function policyNote(
  subject: string,
  name: string,
  after: PolicyAfterWrite,
): string {
  const label = (policy: DeletionPolicy) =>
    policy === "Orphan" ? "Keep" : "Delete";
  const held = `The claim ${name} has the deletion policy ${label(after.held)}.`;
  return after.unapplied
    ? `The ${subject} is not deleted. The repository holds the commit that sets the deletion policy of ${name} to ${label(after.unapplied)}, and GitOps has not applied it yet. ${held}`
    : `The ${subject} is not deleted. ${held}`;
}

/** ReadErrors names each read that failed, as an alert. */
export function ReadErrors({ errors }: { errors: readonly ReadError[] }) {
  return errors.map((e, i) => (
    <p key={i} role="alert" className="text-sm text-bad">
      Could not read {e.kind}: {e.message}
    </p>
  ));
}

/** DeleteAlert shows the refusals of a write as one alert. */
export function DeleteAlert({ texts }: { texts: string[] }) {
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
