"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { deleteClaimSteps } from "@/lib/claims/deletion";
import { useGitSync, useResource } from "@/lib/k8s/hooks";
import type { DeletableClaim, DeletionPolicy } from "@/lib/policy";
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
 * ClaimDeleteStage is where the delete of a claim is. choose: nothing is
 * sent yet, or the delete after a Git wait was refused with errors.
 * waiting: in Git mode, the commit of the deletion policy waits for
 * GitOps. deleting: the delete is sent or the claim is being deleted;
 * gitWaiting says that the delete commit waits for GitOps to delete the
 * claim. deleted: the claim is gone. replaced: a claim with another UID
 * has its name.
 */
export type ClaimDeleteStage =
  | { kind: "choose"; errors?: ApiError[] }
  | { kind: "waiting" }
  | { kind: "deleting"; gitWaiting: boolean }
  | { kind: "deleted" }
  | { kind: "replaced" };

type ClaimKind = "CamundaClusterClaim" | "CamundaManagementClusterClaim";

// Sent is the state that the hook keeps. revision is the revision of the
// policy commit that the waiting stage follows.
type Sent =
  | { kind: "choose"; errors?: ApiError[] }
  | { kind: "waiting"; revision: string }
  | { kind: "deleting" };

/**
 * useClaimDelete runs the delete of a claim of either kind with the steps
 * of deleteClaimSteps. It follows the claim live from initial, the claim
 * that the page opened with, which needs a name and a namespace. It writes
 * only in apply, which the confirm of a ChangeReview calls, and in Git
 * mode once more when GitOps applied the policy commit.
 *
 * remove is the remove step of the live claim, a new object for each UID
 * and resourceVersion, so that the review previews it again. policyStep
 * says that the delete writes the deletion policy first. apply ignores
 * the change that it gets: it sends the steps of the live claim under
 * policy. A refused or Git-mode policy write is its result; otherwise it
 * sends remove and returns that result. onWritten moves to deleting
 * unless a Git policy commit waits. Once GitOps applied that commit, the
 * hook sends remove once, calls show with pushedMessage when it is ok,
 * and moves to deleting, or back to choose with the errors of a refusal.
 * stopWaiting moves back to choose and sends nothing. The stage opens on
 * deleting when initial has a deletionTimestamp, and is deleted or
 * replaced whenever the live claim is gone or has another UID. It needs a
 * QueryClientProvider above it.
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
  apply(change: Change): Promise<WriteResult>;
  onWritten(): void;
  stopWaiting(): void;
} {
  const { initial, policy, mode, intent, pushedMessage, actions, show } = input;
  const kind = initial.kind as ClaimKind;
  const namespace = initial.metadata.namespace;
  const name = initial.metadata.name ?? "";
  const { item } = useResource(kind, { namespace, name }, initial as never);
  const live = item as C | undefined;
  const [sent, setSent] = useState<Sent>(() =>
    (initial.metadata as { deletionTimestamp?: string }).deletionTimestamp
      ? { kind: "deleting" }
      : { kind: "choose" },
  );

  const uid = live?.metadata.uid;
  const version = live?.metadata.resourceVersion;
  const remove = useMemo(
    () => deleteClaimSteps(live ?? initial, policy, intent).remove,
    // ChangeReview keeps a refused write blocked for the same change
    // object, so only a new claim version gives a new remove.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [uid, version, intent],
  );
  const policyStep =
    deleteClaimSteps(live ?? initial, policy, intent).policy !== undefined;

  const latest = useRef({ live, policy, intent, actions, show, pushedMessage });
  useEffect(() => {
    latest.current = { live, policy, intent, actions, show, pushedMessage };
  });
  const claimRef = { kind, namespace, name };

  const apply = useCallback(
    async (): Promise<WriteResult> => {
      const { live, policy, intent, actions } = latest.current;
      if (!live) return { ok: false, errors: [gone(name)] };
      const steps = deleteClaimSteps(live, policy, intent);
      if (steps.policy) {
        const result = await actions
          .applyPolicy(steps.policy)
          .catch(unavailable);
        if (!result.ok) return result;
        if (result.mode === "git") {
          const revision = result.revisions[refKey(claimRef)] ?? "";
          setSent({ kind: "waiting", revision });
          return result;
        }
      }
      return actions.remove(steps.remove).catch(unavailable);
    },
    // claimRef follows initial, which does not change.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [],
  );
  const onWritten = useCallback(
    () => setSent((s) => (s.kind === "waiting" ? s : { kind: "deleting" })),
    [],
  );
  const stopWaiting = useCallback(() => setSent({ kind: "choose" }), []);

  const sync = useGitSync(
    claimRef,
    sent.kind === "waiting" ? sent.revision : "",
  );
  const sentFor = useRef<string | undefined>(undefined);
  useEffect(() => {
    if (sent.kind !== "waiting" || sync !== "applied") return;
    if (sentFor.current === sent.revision) return;
    sentFor.current = sent.revision;
    const { live, policy, intent, actions, show, pushedMessage } =
      latest.current;
    if (!live) return;
    void actions
      .remove(deleteClaimSteps(live, policy, intent).remove)
      .catch(unavailable)
      .then((result) => {
        if (result.ok) {
          show(result, pushedMessage);
          setSent({ kind: "deleting" });
        } else setSent({ kind: "choose", errors: result.errors });
      });
  }, [sent, sync]);

  let stage: ClaimDeleteStage;
  if (!live) stage = { kind: "deleted" };
  else if (initial.metadata.uid && uid !== initial.metadata.uid)
    stage = { kind: "replaced" };
  else if (sent.kind === "deleting") {
    const deleting = (live.metadata as { deletionTimestamp?: string })
      .deletionTimestamp;
    stage = { kind: "deleting", gitWaiting: mode === "git" && !deleting };
  } else if (sent.kind === "waiting") stage = { kind: "waiting" };
  else stage = sent.errors ? sent : { kind: "choose" };

  return {
    claim: live,
    stage,
    remove,
    policyStep,
    apply,
    onWritten,
    stopWaiting,
  };
}

function unavailable(error: unknown): WriteResult {
  return {
    ok: false,
    errors: [{ code: "Unavailable", message: String(error) }],
  };
}

function gone(name: string): ApiError {
  return { code: "Stale", message: `The claim ${name} is gone.` };
}
