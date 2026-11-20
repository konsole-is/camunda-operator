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
import {
  WAITING_FOR_GITOPS,
  WAITING_ON_LIVE,
  WaitingPanel,
} from "@/components/cluster/edit/waiting-panel";
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
import { pathFrom, type FieldLock, type Step } from "@/lib/fields";
import type { KindName } from "@/lib/k8s/kinds";
import { useGitSync } from "@/lib/k8s/hooks";
import { isManaged, REVISION_ANNOTATION } from "@/lib/labels";
import type { FieldPath } from "@/lib/merge";
import { equalJSON } from "@/lib/policy";
import {
  claimAfter,
  openSession,
  rebase,
  resetValue,
  setValue,
  type ClaimObject,
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
import { ChangeBar } from "./change-bar";
import { ClaimReview, type ClaimReviewInput } from "./claim-review";
import { useGitPushes } from "./git-pushes";
import { Toast, useWriteToast } from "./toast";

/** ClaimActions are the two server actions of a claim write. */
export type ClaimActions = {
  preview(change: Change): Promise<Preview>;
  apply(change: Change): Promise<WriteResult>;
};

export type { ClaimReviewInput } from "./claim-review";

/**
 * ClaimEditHost is what a claim page gives ClaimEditProvider. claim is the
 * live claim. resource is the object that the claim made: the session opens
 * only while it carries the claim label. locks can change between renders,
 * and the next change uses the newest one. validate runs only while a change
 * is pending. prepare runs when Review is clicked, and the review shows the
 * session that it returns. review runs only while the review is open, and
 * close closes it. saved is a session whose change gives the claim that
 * the save leaves with the take-over choice of the review: a cleared field
 * of another tool stays in it until the take-over.
 */
export type ClaimEditHost<C extends ClaimObject> = {
  claim: C | undefined;
  resource: { metadata?: { labels?: Record<string, string> } } | undefined;
  writeMode: WriteMode | undefined;
  locks: LockProvider;
  validate(s: Session<C>): readonly FieldError[];
  prepare?(s: Session<C>): Session<C>;
  review(
    s: Session<C>,
    review: { mode: WriteMode; close(): void; saved: Session<C> },
  ): ClaimReviewInput;
};

/**
 * ClaimEdit is the edit state of a claim page. session is undefined when
 * the page has none: no write mode, no claim, a resource without the claim
 * label, or a claim without a name or a resourceVersion. set and reset
 * return the lock that refused the change, else undefined, and do nothing
 * without a session. Two calls in one event handler build on each other.
 * invalid says that the editor of steps shows message: an error that it did
 * not pass on, so that the session can still hold an older value there, or
 * an error of the merged value that the session holds there; an undefined
 * message clears it. An editor clears its error when it
 * unmounts. While an editor shows an error, the change bar blocks Review
 * with the message. discard drops every pending change. review opens the
 * review on the next render when a change is pending, errors is empty, and
 * no editor shows an error, and returns whether it opens it. review first
 * applies prepare of the host. pending is the count of pending changes.
 * show is the show function of useWriteToast of the page: a write that is
 * not a claim change reports its result through it, so that the page has
 * one toast. errors are the errors of the session before review, then the
 * field errors of a refused preview or write of that session.
 */
export type ClaimEdit<C extends ClaimObject = ClaimObject> = {
  session: Session<C> | undefined;
  set(steps: Step[], value: unknown): FieldLock | undefined;
  reset(steps: Step[]): FieldLock | undefined;
  invalid(steps: Step[], message: string | undefined): void;
  discard(): void;
  review(): boolean;
  pending: number;
  show(result: WriteResult, message: string): void;
  errors: readonly FieldError[];
};

const ClaimEditContext = createContext<ClaimEdit | undefined>(undefined);

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
type Pushed<C extends ClaimObject> = {
  key: RefKey;
  revision: string;
  spec: C["spec"];
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
type Edit<C extends ClaimObject> = {
  seen?: string;
  seenPush?: string;
  session?: Session<C>;
  refused?: { session: Session<C>; fields: FieldError[] };
  reviewing: boolean;
  pushed?: Pushed<C>;
  note?: { session?: Session<C>; text: string };
  dropped?: boolean;
  ended?: RefKey;
};

/**
 * ClaimEditProvider holds the one session of a claim page, over the live
 * claim of host, with the locks of host. It rebases the session on each new
 * resourceVersion of the claim. It shows the change bar while a change is
 * pending, the review in a dialog, and the toast of the last write. After an
 * ok write the session goes on from the written claim with no pending
 * change, until the next claim update rebases it. In Git mode the session
 * keeps the pushed claim spec, and Review waits, until the claim shows the
 * push or the push is dropped. It never writes on its own: only the confirm
 * of the review writes. It needs a GitPushesProvider above it.
 */
export function ClaimEditProvider<C extends ClaimObject>({
  host,
  actions,
  children,
}: {
  host: ClaimEditHost<C>;
  actions: ClaimActions;
  children: ReactNode;
}): ReactNode {
  const { toast, show, dismiss } = useWriteToast();
  const { pushes, settle } = useGitPushes();
  const hostRef = useRef(host);
  const [locks] = useState(() => followingLocks(host.locks));
  // The session follows no claim without a write mode, because then no
  // write can happen.
  const live = host.writeMode ? host.claim : undefined;
  const version = live?.metadata?.resourceVersion;

  const [writing, setWriting] = useState(false);
  const [shown, setShown] = useState<ReadonlyMap<FieldPath, string>>(
    () => new Map(),
  );
  const [state, setState] = useState<Edit<C>>(() => ({
    seen: version,
    session: openSession({ claim: live, resource: host.resource ?? {}, locks }),
    reviewing: false,
  }));
  const synced = syncedEdit(state, {
    host,
    live,
    locks,
    following: (p) =>
      pushes.some(
        (q) => q.key === p.key && "revision" in q && q.revision === p.revision,
      ),
  });
  const validation = useMemo(
    () => validationOf(synced.session, host.validate),
    [synced.session, host.validate],
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

  const claimKey = host.claim?.metadata?.name
    ? refKey({
        kind: host.claim.kind,
        namespace: host.claim.metadata.namespace,
        name: host.claim.metadata.name,
      })
    : undefined;
  // The pushes outlive the page, so a push of an earlier visit waits here
  // too, without its spec.
  const waiting = edit.pushed
    ? WAITING_FOR_GITOPS
    : pushes.some(
          (p) =>
            p.key === claimKey &&
            !("revision" in p && p.revision === revisionOf(host.claim)),
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
    hostRef.current = host;
    stateRef.current = edit;
    waitingRef.current = waiting;
    locks.follow(host.locks);
  });

  const update = useCallback((next: Edit<C>) => {
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
      const before = revisionOf(hostRef.current.claim);
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
    (apply: (s: Session<C>) => SetResult<C>): FieldLock | undefined => {
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
    const host = hostRef.current;
    if (!current.session || current.session.pending.size === 0) return false;
    if (waitingRef.current || shownRef.current.size > 0) return false;
    const s = host.prepare?.(current.session) ?? current.session;
    if (
      errorsOf({ ...current, session: s }, validationOf(s, host.validate))
        .length
    )
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
  const value: ClaimEdit<C> = {
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
    host.writeMode === undefined &&
    host.resource !== undefined &&
    isManaged(host.resource) &&
    host.claim !== undefined;
  const note =
    edit.note && edit.note.session !== session ? edit.note.text : undefined;

  return (
    <ClaimEditContext.Provider value={value}>
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
      {session && host.writeMode && (
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
                review={host.review}
                session={session}
                mode={host.writeMode}
                actions={{ preview: actions.preview, apply }}
                show={show}
                close={close}
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
    </ClaimEditContext.Provider>
  );
}

/** useClaimEdit returns the edit state of the page. It throws without a ClaimEditProvider above it. */
export function useClaimEdit<
  C extends ClaimObject = ClaimObject,
>(): ClaimEdit<C> {
  const edit = useContext(ClaimEditContext);
  if (!edit) throw new Error("useClaimEdit needs a ClaimEditProvider.");
  return edit as ClaimEdit<C>;
}

function ReviewOf<C extends ClaimObject>({
  review,
  session,
  mode,
  actions,
  show,
  close,
  onRefused,
  onCancel,
  onWritten,
}: {
  review: ClaimEditHost<C>["review"];
  session: Session<C>;
  mode: WriteMode;
  actions: ClaimActions;
  show: (result: WriteResult, message: string) => void;
  close: () => void;
  onRefused: (fields: FieldError[]) => void;
  onCancel?: () => void;
  onWritten: () => void;
}) {
  return (
    <ClaimReview
      input={(saved) => review(session, { mode, close, saved })}
      session={session}
      mode={mode}
      intent={`Change the claim ${session.claim.metadata.name}`}
      preview={actions.preview}
      apply={actions.apply}
      show={show}
      onRefused={onRefused}
      onCancel={onCancel}
      onWritten={onWritten}
    />
  );
}

function syncedEdit<C extends ClaimObject>(
  edit: Edit<C>,
  page: {
    host: ClaimEditHost<C>;
    live: C | undefined;
    locks: LockProvider;
    following: (p: Pushed<C>) => boolean;
  },
): Edit<C> {
  const { host, live, locks, following } = page;
  const version = live?.metadata?.resourceVersion;
  const pushed = edit.pushed;
  let next = edit;
  if (pushed) {
    const liveRevision = revisionOf(live);
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
        ? { ...rebase(next.session, base, host.locks), locks }
        : openSession({ claim: base, resource: host.resource ?? {}, locks }),
  };
}

// FollowPush calls onApplied once the watch of claim sees the revision of
// the push. The Git waiting line of the page settles on the same watch.
function FollowPush({
  claim,
  revision,
  onApplied,
}: {
  claim: ClaimObject;
  revision: string;
  onApplied: () => void;
}) {
  const { namespace, name } = claim.metadata ?? {};
  const sync = useGitSync(
    // Each claim kind that a page edits is a kind of the watch. kinds.ts
    // cannot load in the browser to check it.
    { kind: claim.kind as KindName, namespace, name: name ?? "" },
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

// revisionOf returns the revision annotation of claim. ClaimObject does not
// type the annotations.
function revisionOf(claim: ClaimObject | undefined): string | undefined {
  const metadata = claim?.metadata as
    { annotations?: Record<string, string> } | undefined;
  return metadata?.annotations?.[REVISION_ANNOTATION];
}

// pushedOf returns the push of the claim of saved in result, or undefined
// for a direct write.
function pushedOf<C extends ClaimObject>(
  result: WriteResult | undefined,
  saved: Session<C>,
  before: string | undefined,
): Pushed<C> | undefined {
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
// newer claim. pushedSpec is the spec of a Git push, which the live claim
// does not show yet.
function written<C extends ClaimObject>(
  current: Session<C> | undefined,
  saved: Session<C>,
  pushedSpec: C["spec"] | undefined,
): Session<C> | undefined {
  if (!current) return undefined;
  const version = (s: Session<C>) => s.claim.metadata.resourceVersion;
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

function errorsOf<C extends ClaimObject>(
  edit: Edit<C>,
  invalid: FieldError[],
): FieldError[] {
  const s = edit.session;
  return [
    ...invalid,
    ...(s && edit.refused?.session === s ? edit.refused.fields : []),
  ];
}

function validationOf<C extends ClaimObject>(
  s: Session<C> | undefined,
  validate: (s: Session<C>) => readonly FieldError[],
): FieldError[] {
  if (!s || s.pending.size === 0) return [];
  return [...validate(s)];
}

// FollowingLocks is one LockProvider for the life of the page. follow
// gives it the lock provider of a newer render, so that a lock that
// appears later applies to the next change.
type FollowingLocks = LockProvider & {
  follow(next: LockProvider): void;
};

function followingLocks(initial: LockProvider): FollowingLocks {
  let provider = initial;
  const locks = ((path) => provider(path)) as FollowingLocks;
  locks.follow = (next) => {
    provider = next;
  };
  return locks;
}
