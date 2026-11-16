"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { ReadError } from "@/components/planes/planes-table";
import { deleteClaimSteps } from "@/lib/claims/deletion";
import { useGitSync, useResource, useWatchesFresh } from "@/lib/k8s/hooks";
import {
  type DeletableClaim,
  type DeletionPolicy,
  equalJSON,
} from "@/lib/policy";
import { refKey } from "@/lib/write/refs";
import type {
  ApiError,
  Change,
  Preview,
  WriteMode,
  WriteResult,
} from "@/lib/write/types";

/**
 * ClaimDeleteActions are the server actions of the delete of a claim: the
 * dry run and the write of the delete, and the write of the deletion
 * policy before it.
 */
export type ClaimDeleteActions = {
  preview(change: Change): Promise<Preview>;
  remove(change: Change): Promise<WriteResult>;
  applyPolicy(change: Change): Promise<WriteResult>;
};

/**
 * ClaimDeleteStage is where the delete of a claim is. choose: no delete is
 * under way, or a delete after the policy write was refused with errors or
 * stopped; policyWritten is then the deletion policy that the claim keeps.
 * waiting: the policy write waits for GitOps, or for the claim controller
 * to read it. deleting: the delete is sent or the claim is being deleted;
 * gitWaiting says that the delete commit waits for GitOps to delete the
 * claim. deleted: the claim is gone. replaced: a claim with another UID
 * has its name.
 */
export type ClaimDeleteStage =
  | { kind: "choose"; errors?: ApiError[]; policyWritten?: DeletionPolicy }
  | { kind: "waiting" }
  | { kind: "deleting"; gitWaiting: boolean }
  | { kind: "deleted" }
  | { kind: "replaced" };

type ClaimKind = "CamundaClusterClaim" | "CamundaManagementClusterClaim";

// Sent is the state that the hook keeps. waiting holds the policy write:
// the spec before and after it, and the revision of a Git commit.
type Sent =
  | { kind: "choose"; errors?: ApiError[]; policyWritten?: DeletionPolicy }
  | {
      kind: "waiting";
      policy: DeletionPolicy;
      before: unknown;
      spec: unknown;
      revision?: string;
    }
  | { kind: "deleting" };

/**
 * useClaimDelete runs the delete of a claim of either kind with the steps
 * of deleteClaimSteps, on the claim that it follows live from initial.
 * Give remove, apply, and onWritten to a ChangeReview. apply refuses
 * without a write when the preview of the delete fails. After a policy
 * write it returns that result, and the hook sends the delete once the
 * claim controller read the policy. It sends nothing for a claim with
 * another UID, and refuses with Stale when another spec change came in. reads holds the failed or stale watch of the claim: the
 * policy step needs a current claim. It needs a QueryClientProvider above
 * it.
 */
export function useClaimDelete<C extends DeletableClaim>(input: {
  initial: C;
  policy: DeletionPolicy;
  mode: WriteMode | undefined;
  intent: string;
  pushedMessage: string;
  actions: ClaimDeleteActions;
  show(result: WriteResult, message: string): void;
}): {
  claim: C | undefined;
  stage: ClaimDeleteStage;
  remove: Change;
  policyStep: boolean;
  reads: { errors: ReadError[]; stale: string[] };
  apply(change: Change): Promise<WriteResult>;
  onWritten(): void;
  stopWaiting(): void;
} {
  const { initial, policy, mode, intent, pushedMessage, actions, show } = input;
  const kind = initial.kind as ClaimKind;
  const namespace = initial.metadata.namespace;
  const name = initial.metadata.name ?? "";
  const { item, error } = useResource(
    kind,
    { namespace, name },
    initial as never,
  );
  const [fresh] = useWatchesFresh([{ kind, opts: { namespace }, name }]);
  const live = item as C | undefined;
  const [sent, setSent] = useState<Sent>({ kind: "choose" });

  const uid = live?.metadata.uid;
  const version = live?.metadata.resourceVersion;
  const remove = useMemo(
    () => deleteClaimSteps(live ?? initial, policy, intent).remove,
    // ChangeReview keeps a refused write blocked for the same change
    // object, so only a new claim version or pick gives a new remove.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [uid, version, intent, policy],
  );
  const policyStep =
    deleteClaimSteps(live ?? initial, policy, intent).policy !== undefined;

  const latest = useRef({ live, policy, intent, actions, show, pushedMessage });
  useEffect(() => {
    latest.current = { live, policy, intent, actions, show, pushedMessage };
  });
  const claimRef = { kind, namespace, name };
  const original = initial.metadata.uid;
  const isOriginal = (claim: C) =>
    original === undefined || claim.metadata.uid === original;

  const apply = useCallback(
    async (): Promise<WriteResult> => {
      const { live, policy, intent, actions } = latest.current;
      setSent({ kind: "choose" });
      if (!live || !isOriginal(live))
        return { ok: false, errors: [gone(name)] };
      const steps = deleteClaimSteps(live, policy, intent);
      if (!steps.policy) return actions.remove(steps.remove).catch(unavailable);
      // A delete that would be refused must not leave the claim with a
      // new policy.
      const checked = await actions
        .preview(steps.remove)
        .catch((e: unknown) => ({
          errors: [{ code: "Unavailable" as const, message: String(e) }],
        }));
      if (checked.errors.length > 0)
        return { ok: false, errors: checked.errors };
      const result = await actions.applyPolicy(steps.policy).catch(unavailable);
      if (!result.ok) return result;
      setSent({
        kind: "waiting",
        policy,
        before: live.spec,
        spec: steps.policy.apply[0].spec,
        ...(result.mode === "git" && {
          revision: result.revisions[refKey(claimRef)] ?? "",
        }),
      });
      return result;
    },
    // claimRef and isOriginal follow initial, which does not change.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [],
  );
  const onWritten = useCallback(
    () => setSent((s) => (s.kind === "waiting" ? s : { kind: "deleting" })),
    [],
  );
  const stopWaiting = useCallback(
    () =>
      setSent((s) =>
        s.kind === "waiting" ? { kind: "choose", policyWritten: s.policy } : s,
      ),
    [],
  );

  const sync = useGitSync(
    claimRef,
    sent.kind === "waiting" ? (sent.revision ?? "") : "",
  );
  const written =
    sent.kind === "waiting" &&
    live !== undefined &&
    equalJSON(live.spec, sent.spec);
  // The review showed the claim before or after the policy write. Any
  // other spec needs a new review.
  const changedSince =
    sent.kind === "waiting" &&
    live !== undefined &&
    !written &&
    !equalJSON(live.spec, sent.before);
  const settled =
    written &&
    sent.kind === "waiting" &&
    live !== undefined &&
    (sent.revision === undefined || sync === "applied") &&
    observed(live);
  if (changedSince && live !== undefined && isOriginal(live))
    setSent({
      kind: "choose",
      errors: [changed(name)],
      policyWritten: sent.policy,
    });
  const sentFor = useRef<Sent | undefined>(undefined);
  useEffect(() => {
    if (!settled || sent.kind !== "waiting" || sentFor.current === sent) return;
    sentFor.current = sent;
    const { live, intent, actions, show, pushedMessage } = latest.current;
    if (!live || !isOriginal(live)) return;
    setSent({ kind: "deleting" });
    void actions
      .remove(deleteClaimSteps(live, sent.policy, intent).remove)
      .catch(unavailable)
      .then((result) => {
        if (result.ok) {
          if (result.mode === "git") show(result, pushedMessage);
        } else
          setSent({
            kind: "choose",
            errors: result.errors,
            policyWritten: sent.policy,
          });
      });
    // isOriginal follows initial, which does not change.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [settled, sent]);

  const deleting = (live?.metadata as { deletionTimestamp?: string })
    ?.deletionTimestamp;
  let stage: ClaimDeleteStage;
  if (!live) stage = { kind: "deleted" };
  else if (!isOriginal(live)) stage = { kind: "replaced" };
  else if (sent.kind === "deleting" || deleting)
    stage = { kind: "deleting", gitWaiting: mode === "git" && !deleting };
  else if (sent.kind === "waiting") stage = { kind: "waiting" };
  else stage = sent;

  return {
    claim: live,
    stage,
    remove,
    policyStep,
    reads: {
      errors: error === undefined ? [] : [{ kind, message: error }],
      stale: fresh ? [] : [kind],
    },
    apply,
    onWritten,
    stopWaiting,
  };
}

// observed says that the claim controller read the current generation of
// claim. A claim that lacks one of the numbers counts as read.
function observed(claim: DeletableClaim): boolean {
  const generation = (claim.metadata as { generation?: number }).generation;
  const seen = (claim.status as { observedGeneration?: number } | undefined)
    ?.observedGeneration;
  return generation === undefined || seen === undefined || generation === seen;
}

function unavailable(error: unknown): WriteResult {
  return {
    ok: false,
    errors: [{ code: "Unavailable", message: String(error) }],
  };
}

function changed(name: string): ApiError {
  return {
    code: "Stale",
    message: `The claim ${name} changed after the review. Review the delete again.`,
  };
}

function gone(name: string): ApiError {
  return { code: "Stale", message: `The claim ${name} is gone.` };
}
