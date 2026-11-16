"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { ReadError } from "@/components/planes/planes-table";
import { deleteClaimSteps } from "@/lib/claims/deletion";
import { useGitSync, useResource, useWatchesFresh } from "@/lib/k8s/hooks";
import {
  type DeletableClaim,
  deletionPolicyOf,
  type DeletionPolicy,
  equalJSON,
  orphanRefusedBy,
} from "@/lib/policy";
import { precheckOf } from "@/lib/session";
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
 * PolicyAfterWrite is the deletion policy of a claim after a policy write
 * that no delete followed. held is the policy of the live claim. unapplied
 * is the policy of the Git commit while the claim does not carry that
 * commit.
 */
export type PolicyAfterWrite = {
  held: DeletionPolicy;
  unapplied?: DeletionPolicy;
};

/**
 * ClaimDeleteStage is where the delete of a claim is. choose: no delete is
 * under way, or a delete after the policy write was refused or stopped;
 * afterPolicyWrite is then set. waiting: the delete waits until the live
 * claim carries the policy write, which GitOps applies in Git mode.
 * deleting: the delete is sent or the claim is being deleted; gitWaiting
 * says that the delete commit waits for GitOps to delete the claim;
 * keepRefused says what to do when the claim controller refuses the delete
 * because a registration cannot keep. deleted: the claim is gone.
 * replaced: a claim with another UID has its name.
 */
export type ClaimDeleteStage =
  | {
      kind: "choose";
      errors?: ApiError[];
      afterPolicyWrite?: PolicyAfterWrite;
    }
  | { kind: "waiting" }
  | { kind: "deleting"; gitWaiting: boolean; keepRefused?: string }
  | { kind: "deleted" }
  | { kind: "replaced" };

type ClaimKind = "CamundaClusterClaim" | "CamundaManagementClusterClaim";

/** KeepRegistration is the part of a PluginRegistration that the Keep check reads. */
export type KeepRegistration = Parameters<typeof orphanRefusedBy>[1][number];

type Written = { policy: DeletionPolicy; revision?: string };

// Sent is the state that the hook keeps.
type Sent =
  | { kind: "choose"; errors?: ApiError[]; written?: Written }
  | (Written & {
      kind: "waiting";
      before: unknown;
      spec: unknown;
      seen: boolean;
    })
  | { kind: "deleting" };

/**
 * useClaimDelete runs the delete of a claim of either kind with the steps
 * of deleteClaimSteps, on the claim that it follows live from initial.
 * Give remove, apply, and onWritten to a ChangeReview. remove names the UID
 * and the generation of the claim, so the server refuses its delete with
 * Stale after a spec change. Without a policy step, apply sends the change
 * that it gets. With one, apply refuses without a write when the preview
 * of the delete fails, and returns the result of the policy write. The
 * hook then sends the delete itself. It sends nothing for a claim with
 * another UID, and refuses with Stale when another spec change came in. keepRefused names the registrations of the claim that cannot keep
 * while policy is Orphan. apply then refuses with Invalid and writes
 * nothing, and the delete after the policy write does too. registrations
 * are the live PluginRegistrations. reads holds the failed or stale watch
 * of the claim: the policy step needs a current claim. It needs a
 * QueryClientProvider above it.
 */
export function useClaimDelete<C extends DeletableClaim>(input: {
  initial: C;
  policy: DeletionPolicy;
  registrations: readonly KeepRegistration[];
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
  keepRefused?: string;
  reads: { errors: ReadError[]; stale: string[] };
  apply(change: Change): Promise<WriteResult>;
  onWritten(): void;
  stopWaiting(): void;
} {
  const {
    initial,
    policy,
    registrations,
    mode,
    intent,
    pushedMessage,
    actions,
    show,
  } = input;
  const kind = initial.kind as ClaimKind;
  const namespace = initial.metadata.namespace;
  const name = initial.metadata.name ?? "";
  const { item, error } = useResource(
    kind,
    { namespace, name },
    initial as never,
  );
  const [fresh] = useWatchesFresh([{ kind, opts: { namespace, name } }]);
  const live = item as C | undefined;
  const [sent, setSent] = useState<Sent>({ kind: "choose" });

  const uid = live?.metadata.uid;
  const version = live?.metadata.resourceVersion;
  const generation = live?.metadata.generation;
  const spec = JSON.stringify((live ?? initial).spec);
  // ChangeReview keeps a refused write blocked for the same change object.
  // A refusal for a version that the watch has not shown yet ends with the
  // next version.
  const [refusal, setRefusal] = useState<{ at?: string; retry?: string }>({});
  if (refusal.at !== undefined && refusal.at !== version)
    setRefusal({ retry: version });
  const { retry } = refusal;
  const remove = useMemo(
    () => deleteClaimSteps(live ?? initial, policy, intent).remove,
    // A Git preview clones the repository, so a status write of the claim
    // must not start a new one.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [uid, generation, spec, intent, policy, retry],
  );
  const policyStep =
    deleteClaimSteps(live ?? initial, policy, intent).policy !== undefined;

  const keepRefused = keepRefusal(live ?? initial, policy, registrations);

  const latest = useRef({
    live,
    policy,
    registrations,
    intent,
    actions,
    show,
    pushedMessage,
  });
  useEffect(() => {
    latest.current = {
      live,
      policy,
      registrations,
      intent,
      actions,
      show,
      pushedMessage,
    };
  });
  const claimRef = { kind, namespace, name };
  const original = initial.metadata.uid;
  const isOriginal = (claim: C) =>
    original === undefined || claim.metadata.uid === original;

  const send = useCallback(
    async (change: Change): Promise<WriteResult> => {
      const { live, policy, registrations, intent, actions } = latest.current;
      setSent((s) =>
        s.kind === "choose" && s.written
          ? { kind: "choose", written: s.written }
          : { kind: "choose" },
      );
      if (!live || !isOriginal(live))
        return { ok: false, errors: [gone(name)] };
      const refused = keepRefusal(live, policy, registrations);
      if (refused)
        return { ok: false, errors: [{ code: "Invalid", message: refused }] };
      const steps = deleteClaimSteps(live, policy, intent);
      if (!steps.policy) return actions.remove(change).catch(unavailable);
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
        seen: false,
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
  const apply = useCallback(
    async (change: Change): Promise<WriteResult> => {
      const at = latest.current.live?.metadata.resourceVersion ?? "";
      const result = await send(change);
      if (!result.ok) setRefusal((r) => ({ ...r, at }));
      return result;
    },
    [send],
  );
  const onWritten = useCallback(
    () => setSent((s) => (s.kind === "waiting" ? s : { kind: "deleting" })),
    [],
  );
  const stopWaiting = useCallback(
    () =>
      setSent((s) =>
        s.kind === "waiting" ? { kind: "choose", written: writtenOf(s) } : s,
      ),
    [],
  );

  const sync = useGitSync(
    claimRef,
    sent.kind === "waiting"
      ? (sent.revision ?? "")
      : sent.kind === "choose"
        ? (sent.written?.revision ?? "")
        : "",
  );
  const written =
    sent.kind === "waiting" &&
    live !== undefined &&
    equalJSON(live.spec, sent.spec);
  if (written && sent.kind === "waiting" && !sent.seen)
    setSent({ ...sent, seen: true });
  // The review showed the claim before or after the policy write. Any
  // other spec change needs a new review.
  const changedSince =
    sent.kind === "waiting" &&
    live !== undefined &&
    !written &&
    (sent.seen || !equalJSON(live.spec, sent.before));
  const settled =
    written &&
    sent.kind === "waiting" &&
    live !== undefined &&
    // The claim finalizer reads spec.deletionPolicy when it deletes, so the
    // delete needs no wait for the claim controller.
    (sent.revision === undefined || sync === "applied");
  if (changedSince && live !== undefined && isOriginal(live))
    setSent({
      kind: "choose",
      errors: [changed(name)],
      written: writtenOf(sent),
    });
  const sentFor = useRef<Sent | undefined>(undefined);
  useEffect(() => {
    if (!settled || sent.kind !== "waiting" || sentFor.current === sent) return;
    sentFor.current = sent;
    const { live, registrations, intent, actions, show, pushedMessage } =
      latest.current;
    if (!live || !isOriginal(live)) return;
    const refused = keepRefusal(live, sent.policy, registrations);
    if (refused) {
      setSent({
        kind: "choose",
        errors: [{ code: "Invalid", message: refused }],
        written: writtenOf(sent),
      });
      return;
    }
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
            written: writtenOf(sent),
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
  else if (sent.kind === "deleting" || deleting) {
    const stuck = deleting ? deleteRefusal(live, registrations) : undefined;
    stage = {
      kind: "deleting",
      gitWaiting: mode === "git" && !deleting,
      ...(stuck && { keepRefused: stuck }),
    };
  } else if (sent.kind === "waiting") stage = { kind: "waiting" };
  else {
    const { written, ...rest } = sent;
    const held = deletionPolicyOf(
      live.spec?.deletionPolicy,
      live.status?.resolvedPolicy,
    );
    stage = written
      ? {
          ...rest,
          afterPolicyWrite: {
            held,
            ...(written.revision !== undefined &&
              sync !== "applied" &&
              held !== written.policy && { unapplied: written.policy }),
          },
        }
      : rest;
  }

  return {
    claim: live,
    stage,
    remove,
    policyStep,
    ...(keepRefused && { keepRefused }),
    reads: {
      errors: error === undefined ? [] : [{ kind, message: error }],
      stale: fresh ? [] : [kind],
    },
    apply,
    onWritten,
    stopWaiting,
  };
}

function keepRefusal(
  claim: DeletableClaim,
  policy: DeletionPolicy,
  registrations: readonly KeepRegistration[],
): string | undefined {
  if (policy !== "Orphan") return undefined;
  const names = orphanRefusedBy(claim.spec?.plugins ?? [], registrations);
  return names.length === 0
    ? undefined
    : `${names.join(", ")} cannot keep the resources. Choose Delete the resources.`;
}

// deleteRefusal says what to do when the claim finalizer refuses the
// deletion policy of claim, as CheckDeletionPolicy of pkg/claimpolicy does
// for a registration that cannot keep.
function deleteRefusal(
  claim: DeletableClaim,
  registrations: readonly KeepRegistration[],
): string | undefined {
  const held = deletionPolicyOf(
    claim.spec?.deletionPolicy,
    claim.status?.resolvedPolicy,
  );
  const names =
    held === "Orphan"
      ? orphanRefusedBy(claim.spec?.plugins ?? [], registrations)
      : [];
  return names.length > 0 &&
    precheckOf(claim as Parameters<typeof precheckOf>[0])?.reason ===
      "NotAllowed"
    ? `The delete waits, because ${names.join(", ")} cannot keep the resources. Set the deletion policy of the claim to Delete, or add Orphan back to the deletion policies of the registration.`
    : undefined;
}

function writtenOf(s: Written): Written {
  return {
    policy: s.policy,
    ...(s.revision !== undefined && { revision: s.revision }),
  };
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
