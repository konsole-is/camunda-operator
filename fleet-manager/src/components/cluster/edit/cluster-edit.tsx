"use client";

import {
  createContext,
  type ReactNode,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { ChangeBar } from "@/components/fleet/change-bar";
import { ClaimReview } from "@/components/fleet/claim-review";
import { useGitPushes } from "@/components/fleet/git-pushes";
import { Toast, useWriteToast } from "@/components/fleet/toast";
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
import { objectDiff } from "@/lib/diff";
import {
  pathFrom,
  type ClusterField,
  type FieldLock,
  type Step,
} from "@/lib/fields";
import type { Resource } from "@/lib/k8s/kinds";
import { useGitSync } from "@/lib/k8s/hooks";
import { isManaged, REVISION_ANNOTATION } from "@/lib/labels";
import type { FieldPath } from "@/lib/merge";
import { equalJSON } from "@/lib/policy";
import {
  claimAfter,
  clusterLocks,
  openSession,
  rebase,
  resetValue,
  setValue,
  validateClusterClaim,
  type ClusterClaimContext,
  type FieldError,
  type PendingChange,
  type LockProvider,
  type Session,
  type SetResult,
} from "@/lib/session";
import { refKey } from "@/lib/write/refs";
import type {
  Change,
  Preview,
  RefKey,
  WriteMode,
  WriteResult,
} from "@/lib/write/types";
import { mergedSpecs, showsMergedDiff } from "../impacts";
import type { ClusterSlotContext } from "../slots";
import { changeInputOf } from "./change-input";
import { downgradeDone } from "./downgrade";
import { reviewGuards } from "./review-guards";
import {
  WAITING_FOR_GITOPS,
  WAITING_ON_LIVE,
  WaitingPanel,
} from "./waiting-panel";

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
 * session. Two calls in one event handler build on each other. invalid
 * says that the editor of steps shows message, an error that it did not
 * pass on, so that the session can still hold an older value there; an
 * undefined message clears it. An editor clears its error when it
 * unmounts. While an editor shows an error, the change bar blocks Review
 * with the message. discard drops every pending change. review opens the
 * review on the next render when a change is pending, errors is empty, and
 * no editor shows an error, and returns whether it opens it. Once
 * downgradeDone, review also resets allowVersionDowngrade, unless a change
 * sets it. pending is the count of pending changes. show is the show
 * function of useWriteToast of the page: a write that is not a claim change, such as a backup or a
 * restore, reports its result through it, so that the page has one toast.
 * errors are the errors of the session before review, then the field
 * errors of a refused preview or write of that session.
 */
export type ClusterEdit = {
  session: Session<ClusterClaim> | undefined;
  set(steps: Step[], value: unknown): FieldLock | undefined;
  reset(steps: Step[]): FieldLock | undefined;
  invalid(steps: Step[], message: string | undefined): void;
  discard(): void;
  review(): boolean;
  pending: number;
  show(result: WriteResult, message: string): void;
  errors: readonly FieldError[];
};

const ClusterEditContext = createContext<ClusterEdit | undefined>(undefined);

/**
 * CHANGES_OFF is the reason that the page shows when it cannot write: the
 * fleet manager could not read its settings, so the write mode is unknown.
 */
export const CHANGES_OFF =
  "Changes are off: the fleet manager could not read its settings.";

const PUSH_DROPPED =
  "GitOps did not apply your last change. The page shows the live claim again.";
const STALE_WRITE =
  "The claim changed while the save ran. Review the change and save again.";

// Pushed is the claim spec of a push of this page that GitOps has not
// applied yet. before is the revision of the live claim when the push
// left. applied is set once the watch of the push sees its revision,
// which can come before the claim of the page shows it.
type Pushed = {
  key: RefKey;
  revision: string;
  spec: ClusterClaim["spec"];
  before?: string;
  applied?: boolean;
};

// Edit is the state of the host. seen is the resourceVersion of the claim,
// and seenPush the revision of the push, that session was last opened or
// rebased on. refused holds the field errors of a refused preview or write
// of one session. note is the refusal of the last write, with the session
// that the write sent. dropped says that the page stopped following a push
// that GitOps did not apply. ended is the key of a push that this page
// stopped following, because the claim shows its revision or left for
// another. The tab still follows it until the effect settles it.
type Edit = {
  seen?: string;
  seenPush?: string;
  session?: Session<ClusterClaim>;
  refused?: { session: Session<ClusterClaim>; fields: FieldError[] };
  reviewing: boolean;
  pushed?: Pushed;
  note?: { session?: Session<ClusterClaim>; text: string };
  dropped?: boolean;
  ended?: RefKey;
};

/**
 * ClusterEditProvider holds the one session of the cluster page, over the
 * live claim of context, with clusterLocks of context.fields. It rebases the
 * session on each new resourceVersion of the claim. It shows the change bar
 * while a change is pending, the review in a dialog, and the toast of the
 * last write. After an ok write the session goes on from the written claim
 * with no pending change, until the next claim update rebases it. In Git
 * mode the session keeps the pushed claim spec, and Review waits, until the
 * claim shows the push or the push is dropped. It never writes on its own:
 * only the confirm of the review writes. It needs a GitPushesProvider above
 * it.
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
  const { pushes, settle } = useGitPushes();
  const contextRef = useRef(context);
  const [locks] = useState(() => followingLocks(context.fields));
  const live = liveClaim(context);
  const version = live?.metadata?.resourceVersion;

  const [writing, setWriting] = useState(false);
  const [shown, setShown] = useState<ReadonlyMap<FieldPath, string>>(
    () => new Map(),
  );
  const [state, setState] = useState<Edit>(() => ({
    seen: version,
    session: open(context, live, locks),
    reviewing: false,
  }));
  const synced = syncedEdit(state, {
    context,
    live,
    locks,
    following: (p) =>
      pushes.some(
        (q) => q.key === p.key && "revision" in q && q.revision === p.revision,
      ),
  });
  const {
    environment,
    sections,
    registrations,
    releases,
    cluster,
    backups,
    requests,
  } = context;
  const validation = useMemo(
    () =>
      validationOf(synced.session, {
        environment,
        sections,
        registrations,
        releases,
        cluster,
        backups,
        requests,
      }),
    [
      synced.session,
      environment,
      sections,
      registrations,
      releases,
      cluster,
      backups,
      requests,
    ],
  );
  const reviewing =
    synced.reviewing &&
    synced.session !== undefined &&
    (writing || (synced.session.pending.size > 0 && validation.length === 0));
  // A review that an update closed opens again only on a click of Review.
  const edit =
    synced.reviewing && !reviewing
      ? { ...synced, reviewing: false, note: undefined }
      : synced;
  if (edit !== state) setState(edit);

  const claimKey = context.claim?.metadata?.name
    ? refKey({
        kind: "CamundaClusterClaim",
        namespace: context.claim.metadata.namespace,
        name: context.claim.metadata.name,
      })
    : undefined;
  // The pushes outlive the page, so a push of an earlier visit waits here
  // too, without its spec.
  const waiting = edit.pushed
    ? WAITING_FOR_GITOPS
    : pushes.some(
          (p) =>
            p.key === claimKey &&
            !(
              "revision" in p &&
              p.revision ===
                context.claim?.metadata?.annotations?.[REVISION_ANNOTATION]
            ),
        )
      ? WAITING_ON_LIVE
      : undefined;

  const shownError = shown.size > 0 ? [...shown.values()].join(" ") : undefined;

  // Handlers read and write the latest state through stateRef and shownRef,
  // so that two calls in one handler build on each other.
  const stateRef = useRef(edit);
  const shownRef = useRef(shown);
  const waitingRef = useRef(waiting);
  const reviewButton = useRef<HTMLButtonElement>(null);
  const lastWrite = useRef<
    { result: WriteResult; before: string | undefined } | undefined
  >(undefined);
  useEffect(() => {
    contextRef.current = context;
    stateRef.current = edit;
    waitingRef.current = waiting;
    locks.follow(context.fields);
  });

  const update = useCallback((next: Edit) => {
    stateRef.current = next;
    setState(next);
  }, []);
  // SettleWrite settles a push only when its watch sees the exact revision.
  // A push that the claim left for another revision never gets it, so settle
  // it here.
  useEffect(() => {
    if (!edit.ended) return;
    settle(edit.ended);
    update({ ...stateRef.current, ended: undefined });
  }, [edit.ended, settle, update]);
  // The review stays open while a write runs, so that no change can start
  // before the write returns.
  const apply = useCallback(
    async (change: Change) => {
      const sent = stateRef.current.session;
      const before =
        contextRef.current.claim?.metadata?.annotations?.[REVISION_ANNOTATION];
      update({ ...stateRef.current, note: undefined, dropped: false });
      setWriting(true);
      try {
        const result = await actions
          .apply(change)
          .catch((error: unknown): WriteResult => ({
            ok: false,
            errors: [{ code: "Unavailable", message: String(error) }],
          }));
        lastWrite.current = { result, before };
        if (!result.ok)
          update({
            ...stateRef.current,
            note: { session: sent, text: refusalOf(result) },
          });
        return result;
      } finally {
        setWriting(false);
      }
    },
    [actions, update],
  );
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
  const invalid = useCallback((steps: Step[], message: string | undefined) => {
    const path = pathFrom("", steps);
    const current = shownRef.current;
    if (current.get(path) === message) return;
    const next = new Map(current);
    if (message === undefined) next.delete(path);
    else next.set(path, message);
    shownRef.current = next;
    setShown(next);
  }, []);
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
    const context = contextRef.current;
    if (!current.session || current.session.pending.size === 0) return false;
    if (waitingRef.current || shownRef.current.size > 0) return false;
    const s = withDowngradeReset(current.session, context.cluster);
    if (errorsOf({ ...current, session: s }, validationOf(s, context)).length)
      return false;
    update({ ...current, session: s, reviewing: true });
    return true;
  }, [update]);
  const stopWaiting = () => {
    // The toast reads a push that leaves the tab as applied.
    if (toast?.kind === "waiting") dismiss();
    if (claimKey) settle(claimKey);
    update({ ...stateRef.current, pushed: undefined, dropped: true });
  };

  const session = edit.session;
  const pending = session?.pending.size ?? 0;
  const errors = errorsOf(edit, validation);
  const value: ClusterEdit = {
    session,
    set,
    reset,
    invalid,
    discard,
    review,
    pending,
    show,
    errors,
  };

  const close = () => {
    if (!writing)
      update({ ...stateRef.current, reviewing: false, note: undefined });
  };
  const off =
    context.writeMode === undefined &&
    isManaged(context.cluster) &&
    context.claim !== undefined;
  const note =
    edit.note && edit.note.session !== session ? edit.note.text : undefined;

  return (
    <ClusterEditContext.Provider value={value}>
      {off && (
        <p role="alert" className="text-sm text-bad">
          {CHANGES_OFF}
        </p>
      )}
      {edit.dropped && (
        <p role="status" className="text-sm text-warn">
          {PUSH_DROPPED}
        </p>
      )}
      {edit.pushed && !edit.pushed.applied && live && (
        <FollowPush
          claim={live}
          revision={edit.pushed.revision}
          onApplied={() => {
            const current = stateRef.current;
            if (current.pushed)
              update({
                ...current,
                pushed: { ...current.pushed, applied: true },
              });
          }}
        />
      )}
      {children}
      {(pending > 0 || toast || waiting) && (
        <div className="sticky bottom-4 z-30 flex flex-col items-end gap-2">
          {toast && <Toast result={toast} onDismiss={dismiss} />}
          {waiting && (
            <WaitingPanel text={waiting} onStopWaiting={stopWaiting} />
          )}
          <div className="w-full">
            <ChangeBar
              count={pending}
              blocked={waiting ?? shownError}
              errors={errors}
              reviewRef={reviewButton}
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
            onCloseAutoFocus={(event) => {
              // The dialog opens from the change bar or from an editor that
              // closed, so without this the focus goes to the page body.
              const review = reviewButton.current;
              const button = review?.disabled
                ? review
                    .closest("section")
                    ?.querySelector<HTMLButtonElement>("button:enabled")
                : review;
              if (!button) return;
              event.preventDefault();
              button.focus();
            }}
          >
            <DialogTitle>Review the change</DialogTitle>
            {note && (
              <p
                role="status"
                className="rounded-md border border-warn/40 bg-warn/10 p-3 text-sm text-text"
              >
                {note}
              </p>
            )}
            {reviewing && (
              <ReviewOf
                context={context}
                session={session}
                mode={context.writeMode}
                actions={{ preview: actions.preview, apply }}
                show={show}
                onOpenBackups={() => {
                  close();
                  window.history.replaceState(null, "", "?tab=backups");
                }}
                onRefused={(fields) =>
                  update({ ...stateRef.current, refused: { session, fields } })
                }
                onCancel={writing ? undefined : close}
                onWritten={() => {
                  const current = stateRef.current;
                  const next = pushedOf(
                    lastWrite.current?.result,
                    session,
                    lastWrite.current?.before,
                  );
                  update({
                    ...current,
                    session: written(current.session, session, next?.spec),
                    refused: undefined,
                    reviewing: false,
                    pushed: next,
                    seenPush: next?.revision,
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
  onOpenBackups,
}: {
  context: ClusterSlotContext;
  session: Session<ClusterClaim>;
  mode: WriteMode;
  actions: ClaimActions;
  show: (result: WriteResult, message: string) => void;
  onOpenBackups: () => void;
  onRefused: (fields: FieldError[]) => void;
  onCancel?: () => void;
  onWritten: () => void;
}) {
  const input = changeInputOf(context, session);
  const merged = showsMergedDiff(session) ? mergedSpecs(input) : undefined;
  return (
    <ClaimReview
      {...reviewGuards(context, session, mode, {
        backupsHref: "?tab=backups",
        onOpenBackups,
      })}
      session={session}
      mode={mode}
      intent={`Change the claim ${session.claim.metadata.name}`}
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

function syncedEdit(
  edit: Edit,
  page: {
    context: ClusterSlotContext;
    live: ClusterClaim | undefined;
    locks: LockProvider;
    following: (p: Pushed) => boolean;
  },
): Edit {
  const { context, live, locks, following } = page;
  const version = live?.metadata?.resourceVersion;
  const pushed = edit.pushed;
  let next = edit;
  if (pushed) {
    const liveRevision = live?.metadata?.annotations?.[REVISION_ANNOTATION];
    if (liveRevision === pushed.revision || liveRevision !== pushed.before)
      next = { ...next, pushed: undefined, ended: pushed.key };
    else if (!following(pushed) && !pushed.applied)
      next = { ...next, pushed: undefined, dropped: true };
  }
  const pushRevision = next.pushed?.revision;
  if (version === next.seen && pushRevision === next.seenPush) return next;
  const base = live && next.pushed ? { ...live, spec: next.pushed.spec } : live;
  return {
    ...next,
    seen: version,
    seenPush: pushRevision,
    session:
      next.session && base && version
        ? {
            ...rebase(next.session, base, clusterLocks(context.fields)),
            locks,
          }
        : open(context, base, locks),
  };
}

// FollowPush calls onApplied once the watch of claim sees the revision of
// the push. The Git waiting line of the page settles on the same watch.
function FollowPush({
  claim,
  revision,
  onApplied,
}: {
  claim: ClusterClaim;
  revision: string;
  onApplied: () => void;
}) {
  const { namespace, name } = claim.metadata ?? {};
  const sync = useGitSync(
    { kind: "CamundaClusterClaim", namespace, name: name ?? "" },
    revision,
  );
  const onAppliedRef = useRef(onApplied);
  useEffect(() => {
    onAppliedRef.current = onApplied;
  });
  useEffect(() => {
    if (sync === "applied") onAppliedRef.current();
  }, [sync]);
  return null;
}

// pushedOf returns the push of the claim of saved in result, or undefined
// for a direct write.
function pushedOf(
  result: WriteResult | undefined,
  saved: Session<ClusterClaim>,
  before: string | undefined,
): Pushed | undefined {
  if (!result?.ok || result.mode !== "git") return undefined;
  const { kind, metadata } = saved.claim;
  const key = refKey({
    kind,
    namespace: metadata.namespace,
    name: metadata.name ?? "",
  });
  const revision = result.revisions[key];
  return revision === undefined
    ? undefined
    : { key, revision, spec: claimAfter(saved).spec, before };
}

function refusalOf(result: WriteResult): string {
  if (result.ok) return "";
  return result.errors.some((e) => e.code === "Stale")
    ? STALE_WRITE
    : result.errors.map((e) => e.message).join(" ");
}

// written returns the session after an ok write of saved. A claim update
// can arrive before the write returns, and then current is already on the
// newer claim. pushedSpec is the spec of a Git push, which the claim of the
// cluster does not show yet.
function written(
  current: Session<ClusterClaim> | undefined,
  saved: Session<ClusterClaim>,
  pushedSpec: ClusterClaim["spec"] | undefined,
): Session<ClusterClaim> | undefined {
  if (!current) return undefined;
  const version = (s: Session<ClusterClaim>) =>
    s.claim.metadata.resourceVersion;
  const pending = new Map(current.pending);
  for (const [path, entry] of current.pending)
    if (sameEntry(entry, saved.pending.get(path))) pending.delete(path);
  const claim =
    version(current) !== version(saved) ? current.claim : claimAfter(saved);
  return rebase(
    { ...current, pending },
    pushedSpec ? { ...claim, spec: pushedSpec } : claim,
  );
}

function sameEntry(a: PendingChange, b: PendingChange | undefined): boolean {
  if (b === undefined) return false;
  if ("reset" in a || "reset" in b) return "reset" in a && "reset" in b;
  return equalJSON(a.value, b.value);
}

const ALLOW_DOWNGRADE: Step[] = [
  { field: "spec" },
  { field: "allowVersionDowngrade" },
];

// withDowngradeReset adds the reset of allowVersionDowngrade to s once
// downgradeDone, unless s changes the field itself.
function withDowngradeReset(
  s: Session<ClusterClaim>,
  cluster: ClusterSlotContext["cluster"],
): Session<ClusterClaim> {
  if (!downgradeDone(cluster, s.claim)) return s;
  if (s.pending.has(pathFrom("", ALLOW_DOWNGRADE))) return s;
  const result = resetValue(s, ALLOW_DOWNGRADE);
  return "session" in result ? result.session : s;
}

function errorsOf(edit: Edit, invalid: FieldError[]): FieldError[] {
  const s = edit.session;
  return [
    ...invalid,
    ...(s && edit.refused?.session === s ? edit.refused.fields : []),
  ];
}

function validationOf(
  s: Session<ClusterClaim> | undefined,
  ctx: ClusterClaimContext,
): FieldError[] {
  if (!s || s.pending.size === 0) return [];
  return validateClusterClaim(s, ctx);
}

// FollowingLocks is one LockProvider for the life of the page. follow
// gives it the fields of a newer render, so that a lock that appears later
// applies to the next change.
type FollowingLocks = LockProvider & {
  follow(fields: readonly ClusterField[]): void;
};

function followingLocks(initial: readonly ClusterField[]): FollowingLocks {
  let fields = initial;
  let provider: LockProvider | undefined;
  const locks = ((path) =>
    (provider ??= clusterLocks(fields))(path)) as FollowingLocks;
  locks.follow = (next) => {
    if (next === fields) return;
    fields = next;
    provider = undefined;
  };
  return locks;
}
