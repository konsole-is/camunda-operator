"use client";

import {
  createContext,
  type ReactNode,
  useCallback,
  useContext,
  useEffect,
  useRef,
  useState,
} from "react";
import { ChangeBar } from "@/components/fleet/change-bar";
import { ClaimReview } from "@/components/fleet/claim-review";
import { Toast, useWriteToast } from "@/components/fleet/toast";
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
import { objectDiff } from "@/lib/diff";
import type { ClusterField, FieldLock, Step } from "@/lib/fields";
import type { Resource } from "@/lib/k8s/kinds";
import { isManaged } from "@/lib/labels";
import {
  claimAfter,
  clusterLocks,
  openSession,
  rebase,
  resetValue,
  setValue,
  validateClusterClaim,
  type FieldError,
  type LockProvider,
  type Session,
  type SetResult,
} from "@/lib/session";
import type {
  Change,
  Preview,
  WriteMode,
  WriteResult,
} from "@/lib/write/types";
import { clusterImpacts, mergedSpecs, showsMergedDiff } from "../impacts";
import type { ClusterSlotContext } from "../slots";
import { changeInputOf } from "./change-input";

export type ClusterClaim = Resource<"CamundaClusterClaim">;

/** ClaimActions are the two server actions of a claim write. */
export type ClaimActions = {
  preview(change: Change): Promise<Preview>;
  apply(change: Change): Promise<WriteResult>;
};

/**
 * ClusterEdit is the edit state of the cluster page. session is undefined
 * when the page has none: an unmanaged cluster, a cluster without its claim,
 * a claim without a resourceVersion, or no write mode. set and reset return
 * the lock that refused the change, else undefined, and do nothing without a
 * session. Two calls in one event handler build on each other. discard drops
 * every pending change. review opens the review on the next render when a
 * change is pending and errors is empty. pending is the count of pending
 * changes. show is the show function of useWriteToast of the page: a write
 * that is not a claim change, such as a backup or a restore, reports its
 * result through it, so that the page has one toast. errors are the errors
 * of the session before review, then the field errors of a refused preview
 * or write of that session.
 */
export type ClusterEdit = {
  session: Session<ClusterClaim> | undefined;
  set(steps: Step[], value: unknown): FieldLock | undefined;
  reset(steps: Step[]): FieldLock | undefined;
  discard(): void;
  review(): void;
  pending: number;
  show(result: WriteResult, message: string): void;
  errors: readonly FieldError[];
};

const ClusterEditContext = createContext<ClusterEdit | undefined>(undefined);

// Edit is the state of the host. seen is the resourceVersion of the claim
// that session was last opened or rebased on. refused holds the field
// errors of a refused preview or write of one session.
type Edit = {
  seen?: string;
  session?: Session<ClusterClaim>;
  refused?: { session: Session<ClusterClaim>; fields: FieldError[] };
  reviewing: boolean;
};

/**
 * ClusterEditProvider holds the one session of the cluster page, over the
 * live claim of context, with clusterLocks of context.fields. It rebases the
 * session on each new resourceVersion of the claim. It shows the change bar
 * while a change is pending, the review in a dialog, and the toast of the
 * last write. After an ok write the session goes on from the written claim
 * with no pending change, until the next claim update rebases it. It never
 * writes on its own: only the confirm of the review writes.
 */
export function ClusterEditProvider({
  context,
  actions,
  children,
}: {
  context: ClusterSlotContext;
  actions: ClaimActions;
  children: ReactNode;
}): ReactNode {
  const { toast, show, dismiss } = useWriteToast();
  const contextRef = useRef(context);
  const [locks] = useState(() => followingLocks(context.fields));
  const live = liveClaim(context);
  const version = live?.metadata?.resourceVersion;

  const [state, setState] = useState<Edit>(() => ({
    seen: version,
    session: open(context, live, locks),
    reviewing: false,
  }));
  let edit = state;
  if (version !== state.seen) {
    edit = {
      ...state,
      seen: version,
      session:
        state.session && live && version
          ? {
              ...rebase(state.session, live, clusterLocks(context.fields)),
              locks,
            }
          : open(context, live, locks),
    };
    setState(edit);
  }

  // Handlers read and write the latest state through stateRef, so that two
  // calls in one handler build on each other.
  const stateRef = useRef(edit);
  useEffect(() => {
    contextRef.current = context;
    stateRef.current = edit;
    locks.follow(context.fields);
  });

  const update = useCallback((next: Edit) => {
    stateRef.current = next;
    setState(next);
  }, []);
  const change = useCallback(
    (
      apply: (s: Session<ClusterClaim>) => SetResult<ClusterClaim>,
    ): FieldLock | undefined => {
      const current = stateRef.current;
      if (!current.session) return undefined;
      const result = apply(current.session);
      if ("refused" in result) return result.refused;
      update({ ...current, session: result.session });
      return undefined;
    },
    [update],
  );
  const set = useCallback(
    (steps: Step[], value: unknown) => change((s) => setValue(s, steps, value)),
    [change],
  );
  const reset = useCallback(
    (steps: Step[]) => change((s) => resetValue(s, steps)),
    [change],
  );
  const discard = useCallback(() => {
    const current = stateRef.current;
    if (!current.session) return;
    update({
      ...current,
      session: { ...current.session, pending: new Map() },
      reviewing: false,
    });
  }, [update]);
  const review = useCallback(() => {
    const current = stateRef.current;
    const s = current.session;
    if (!s || s.pending.size === 0) return;
    if (errorsOf(current, contextRef.current).length > 0) return;
    update({ ...current, reviewing: true });
  }, [update]);

  const session = edit.session;
  const pending = session?.pending.size ?? 0;
  const invalid = validationOf(edit.session, context);
  const errors = errorsOf(edit, context);
  const value: ClusterEdit = {
    session,
    set,
    reset,
    discard,
    review,
    pending,
    show,
    errors,
  };

  const reviewing =
    edit.reviewing &&
    session !== undefined &&
    pending > 0 &&
    invalid.length === 0;
  const close = () => update({ ...stateRef.current, reviewing: false });
  const off =
    context.writeMode === undefined &&
    isManaged(context.cluster) &&
    context.claim !== undefined;

  return (
    <ClusterEditContext.Provider value={value}>
      {off && (
        <p role="alert" className="text-sm text-bad">
          Changes are off: the fleet manager could not read its settings.
        </p>
      )}
      {children}
      {(pending > 0 || toast) && (
        <div className="sticky bottom-4 z-30 flex flex-col items-end gap-2">
          {toast && <Toast result={toast} onDismiss={dismiss} />}
          <div className="w-full">
            <ChangeBar
              count={pending}
              errors={errors}
              onDiscard={discard}
              onReview={review}
            />
          </div>
        </div>
      )}
      {session && context.writeMode && (
        <Dialog open={reviewing} onOpenChange={(o) => !o && close()}>
          <DialogContent
            aria-describedby={undefined}
            className="max-h-[85vh] overflow-y-auto sm:max-w-3xl"
          >
            <DialogTitle>Review the change</DialogTitle>
            {reviewing && (
              <ReviewOf
                context={context}
                session={session}
                mode={context.writeMode}
                actions={actions}
                show={show}
                onRefused={(fields) =>
                  update({ ...stateRef.current, refused: { session, fields } })
                }
                onCancel={close}
                onWritten={() => {
                  const current = stateRef.current;
                  update({
                    ...current,
                    session: written(current.session, session, locks),
                    refused: undefined,
                    reviewing: false,
                  });
                }}
              />
            )}
          </DialogContent>
        </Dialog>
      )}
    </ClusterEditContext.Provider>
  );
}

/** useClusterEdit returns the edit state of the page. It throws without a ClusterEditProvider above it. */
export function useClusterEdit(): ClusterEdit {
  const edit = useContext(ClusterEditContext);
  if (!edit) throw new Error("useClusterEdit needs a ClusterEditProvider.");
  return edit;
}

function ReviewOf({
  context,
  session,
  mode,
  actions,
  show,
  onRefused,
  onCancel,
  onWritten,
}: {
  context: ClusterSlotContext;
  session: Session<ClusterClaim>;
  mode: WriteMode;
  actions: ClaimActions;
  show: (result: WriteResult, message: string) => void;
  onRefused: (fields: FieldError[]) => void;
  onCancel: () => void;
  onWritten: () => void;
}) {
  const input = changeInputOf(context, session);
  const merged = showsMergedDiff(session) ? mergedSpecs(input) : undefined;
  return (
    <ClaimReview
      session={session}
      mode={mode}
      intent={`Change the claim ${session.claim.metadata.name}`}
      impacts={clusterImpacts(input)}
      mergedLines={merged && objectDiff(merged.before.spec, merged.after.spec)}
      preview={actions.preview}
      apply={actions.apply}
      show={show}
      onRefused={onRefused}
      onCancel={onCancel}
      onWritten={onWritten}
    />
  );
}

// liveClaim is the claim that the session follows: none without a write
// mode, because then no write can happen.
function liveClaim(context: ClusterSlotContext): ClusterClaim | undefined {
  return context.writeMode ? context.claim : undefined;
}

function open(
  context: ClusterSlotContext,
  claim: ClusterClaim | undefined,
  locks: LockProvider,
): Session<ClusterClaim> | undefined {
  return openSession({ claim, resource: context.cluster, locks });
}

// written returns the session after an ok write of saved. A claim update
// can arrive before the write returns, and then current is already on the
// newer claim.
function written(
  current: Session<ClusterClaim> | undefined,
  saved: Session<ClusterClaim>,
  locks: LockProvider,
): Session<ClusterClaim> | undefined {
  if (!current) return undefined;
  const version = (s: Session<ClusterClaim>) =>
    s.claim.metadata.resourceVersion;
  if (version(current) !== version(saved)) return current;
  return rebase(current, claimAfter(saved), locks);
}

function errorsOf(edit: Edit, context: ClusterSlotContext): FieldError[] {
  const s = edit.session;
  return [
    ...validationOf(s, context),
    ...(s && edit.refused?.session === s ? edit.refused.fields : []),
  ];
}

function validationOf(
  s: Session<ClusterClaim> | undefined,
  context: ClusterSlotContext,
): FieldError[] {
  if (!s || s.pending.size === 0) return [];
  return [
    ...validateClusterClaim(s, {
      environment: context.environment,
      sections: context.sections,
      registrations: context.registrations,
      releases: context.releases,
      cluster: context.cluster,
      backups: context.backups,
    }),
  ];
}

// FollowingLocks is one LockProvider for the life of the page. follow
// gives it the fields of a newer render, so that a lock that appears later
// applies to the next change.
type FollowingLocks = LockProvider & {
  follow(fields: readonly ClusterField[]): void;
};

function followingLocks(initial: readonly ClusterField[]): FollowingLocks {
  let fields = initial;
  let provider = clusterLocks(initial);
  const locks = ((path) => provider(path)) as FollowingLocks;
  locks.follow = (next) => {
    if (next === fields) return;
    fields = next;
    provider = clusterLocks(next);
  };
  return locks;
}
