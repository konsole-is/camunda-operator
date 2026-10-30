import { pathFrom, type FieldLock, type Step } from "@/lib/fields";
import { isManaged } from "@/lib/labels";
import type { FieldPath } from "@/lib/merge";
import { equalJSON } from "@/lib/policy";
import { refKey } from "@/lib/write/refs";
import type { Change, KubeObject } from "@/lib/write/types";
import type { LockProvider } from "./locks";
import { valueAt, withChanges, without, withValue } from "./steps";

/** ClaimObject is a claim of any kind, as the form read it. */
export type ClaimObject = {
  apiVersion: string;
  kind: string;
  metadata: { name?: string; namespace?: string; resourceVersion?: string };
  spec?: object;
};

/**
 * PendingChange is one change of a session at path: a new value, or a
 * reset that removes the claim value so that the value below the claim
 * applies.
 */
export type PendingChange =
  | { path: FieldPath; steps: Step[]; value: unknown }
  | { path: FieldPath; steps: Step[]; reset: true };

/**
 * Session holds the pending changes of one live claim, by claim path, in
 * the order of their first change. A session never changes: each function
 * returns a new one.
 */
export type Session<C extends ClaimObject = ClaimObject> = {
  claim: C;
  pending: ReadonlyMap<FieldPath, PendingChange>;
  locks: LockProvider;
};

/** SetResult is the new session, or the lock that refused the change. */
export type SetResult<C extends ClaimObject> =
  { session: Session<C> } | { refused: FieldLock };

/**
 * openSession opens a session over claim with no pending changes. There is
 * none when claim is undefined, when resource does not carry the claim
 * label, or when claim has no name or no resourceVersion, because a write
 * needs both for its precondition.
 */
export function openSession<C extends ClaimObject>(input: {
  claim: C | undefined;
  resource: { metadata?: { labels?: Record<string, string> } };
  locks: LockProvider;
}): Session<C> | undefined {
  const { claim, resource, locks } = input;
  if (
    !claim ||
    !isManaged(resource) ||
    !claim.metadata.name ||
    !claim.metadata.resourceVersion
  )
    return undefined;
  return { claim, pending: new Map(), locks };
}

/**
 * setValue sets value at steps. A value equal to the claim value removes
 * the change, and undefined is resetValue. A locked path, or a path below a
 * locked one, refuses. A change below a pending change goes into that
 * change, and a change above pending changes replaces them.
 */
export function setValue<C extends ClaimObject>(
  s: Session<C>,
  steps: Step[],
  value: unknown,
): SetResult<C> {
  if (value === undefined) return resetValue(s, steps);
  const path = pathFrom("", steps);
  const lock = lockOf(s.locks, steps);
  if (lock) return { refused: lock };
  const parent = pendingParent(s, steps);
  if (parent)
    return setValue(
      s,
      parent.entry.steps,
      withValue(
        "reset" in parent.entry ? {} : parent.entry.value,
        parent.rest,
        value,
      ),
    );
  return {
    session: withEntry(
      s,
      steps,
      equalJSON(value, valueAt(s.claim, steps))
        ? undefined
        : {
            path,
            steps: structuredClone(steps),
            value: structuredClone(value),
          },
    ),
  };
}

/**
 * resetValue removes the claim value at steps, as "Reset to preset" and
 * "Reset to environment" do. When the claim holds no value there, it only
 * drops a pending change. A locked path, or a path below a locked one,
 * refuses.
 */
export function resetValue<C extends ClaimObject>(
  s: Session<C>,
  steps: Step[],
): SetResult<C> {
  const path = pathFrom("", steps);
  const lock = lockOf(s.locks, steps);
  if (lock) return { refused: lock };
  const parent = pendingParent(s, steps);
  if (parent) {
    if (
      "reset" in parent.entry ||
      valueAt(parent.entry.value, parent.rest) === undefined
    )
      return { session: s };
    const rest = without(parent.entry.value, parent.rest);
    // An empty value would stay in the claim as an empty field.
    return isEmpty(rest)
      ? resetValue(s, parent.entry.steps)
      : setValue(s, parent.entry.steps, rest);
  }
  return {
    session: withEntry(
      s,
      steps,
      valueAt(s.claim, steps) === undefined
        ? undefined
        : { path, steps: structuredClone(steps), reset: true },
    ),
  };
}

/** discard drops the pending change at path. */
export function discard<C extends ClaimObject>(
  s: Session<C>,
  path: FieldPath,
): Session<C> {
  const pending = new Map(s.pending);
  pending.delete(path);
  return { ...s, pending };
}

/** claimAfter returns a copy of the claim with each pending change applied. */
export function claimAfter<C extends ClaimObject>(s: Session<C>): C {
  return withChanges(s.claim, [...s.pending.values()]);
}

/**
 * changeOf returns the write of the session: the claim after the change,
 * with the resourceVersion that the form read as its precondition.
 */
export function changeOf<C extends ClaimObject>(
  s: Session<C>,
  intent: string,
): Change {
  const { kind, metadata } = s.claim;
  return {
    intent,
    apply: [claimAfter(s) as unknown as KubeObject],
    preconditions: {
      [refKey({ kind, namespace: metadata.namespace, name: metadata.name! })]:
        metadata.resourceVersion!,
    },
  };
}

/**
 * rebase moves the session to a newer copy of its claim. It keeps each
 * pending change that locks, by default the lock provider of s, do not
 * lock and that still changes claim, and drops the others. It also drops a
 * change at an index of a list that claim changed.
 */
export function rebase<C extends ClaimObject>(
  s: Session<C>,
  claim: C,
  locks: LockProvider = s.locks,
): Session<C> {
  const pending = new Map<FieldPath, PendingChange>();
  for (const [path, entry] of s.pending) {
    if (lockOf(locks, entry.steps)) continue;
    if (movedIndex(s.claim, claim, entry.steps)) continue;
    const current = valueAt(claim, entry.steps);
    const changes =
      "reset" in entry
        ? current !== undefined
        : !equalJSON(entry.value, current);
    if (changes) pending.set(path, entry);
  }
  return { claim, pending, locks };
}

function lockOf(
  locks: LockProvider,
  steps: readonly Step[],
): FieldLock | undefined {
  for (let i = 1; i <= steps.length; i++) {
    const lock = locks(pathFrom("", steps.slice(0, i)));
    if (lock) return lock;
  }
  return undefined;
}

// movedIndex says whether a list that steps names an item of by index
// differs between before and after, so that the index can name another item.
function movedIndex(
  before: unknown,
  after: unknown,
  steps: readonly Step[],
): boolean {
  return steps.some(
    (step, i) =>
      "index" in step &&
      !equalJSON(
        valueAt(before, steps.slice(0, i)),
        valueAt(after, steps.slice(0, i)),
      ),
  );
}

function withEntry<C extends ClaimObject>(
  s: Session<C>,
  steps: readonly Step[],
  entry: PendingChange | undefined,
): Session<C> {
  const pending = new Map(s.pending);
  pending.delete(pathFrom("", steps));
  for (const [key, other] of pending)
    if (isBelow(other.steps, steps)) pending.delete(key);
  if (entry !== undefined) pending.set(entry.path, entry);
  return { ...s, pending };
}

// pendingParent returns the pending change above steps, and the steps from
// it to steps. A change below a pending change goes into that change, so
// that the two never apply in the wrong order.
function pendingParent(
  s: Session,
  steps: readonly Step[],
): { entry: PendingChange; rest: Step[] } | undefined {
  for (const entry of s.pending.values())
    if (isBelow(steps, entry.steps))
      return { entry, rest: steps.slice(entry.steps.length) };
  return undefined;
}

function isEmpty(value: unknown): boolean {
  return (
    typeof value === "object" &&
    value !== null &&
    Object.keys(value).length === 0
  );
}

function isBelow(steps: readonly Step[], parent: readonly Step[]): boolean {
  return (
    steps.length > parent.length &&
    parent.every((step, i) => equalJSON(step, steps[i]))
  );
}
