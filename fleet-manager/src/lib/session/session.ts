import { itemStep, pathFrom, type FieldLock, type Step } from "@/lib/fields";
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
 * locked one, refuses, and so does a value that changes a locked field
 * below steps. A change below a pending change goes into that change, and a
 * change above pending changes replaces them. An index in steps names the
 * item of the list in the claim, as in withChanges.
 */
export function setValue<C extends ClaimObject>(
  s: Session<C>,
  steps: Step[],
  value: unknown,
): SetResult<C> {
  if (value === undefined) return resetValue(s, steps);
  const path = pathFrom("", steps);
  const lock = lockAt(s.locks, s.claim, steps, value);
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
 * drops a pending change. A locked path, a path below a locked one, or a
 * path above a locked field that the claim holds, refuses. An index in steps
 * names the item of the list in the claim, as in withChanges.
 */
export function resetValue<C extends ClaimObject>(
  s: Session<C>,
  steps: Step[],
): SetResult<C> {
  const path = pathFrom("", steps);
  const lock = lockAt(s.locks, s.claim, steps, undefined);
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
  if (valueAt(s.claim, steps) === undefined)
    return { session: compactAppended(withEntry(s, steps, undefined)) };
  return {
    session: withEntry(s, steps, {
      path,
      steps: structuredClone(steps),
      reset: true,
    }),
  };
}

/**
 * discard drops the pending change at path. An index in path names the item
 * of the list in the claim, as in withChanges.
 */
export function discard<C extends ClaimObject>(
  s: Session<C>,
  path: FieldPath,
): Session<C> {
  const entry = s.pending.get(path);
  if (!entry) return s;
  const pending = new Map(s.pending);
  pending.delete(path);
  return compactAppended({ ...s, pending });
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
    const current = valueAt(claim, entry.steps);
    const changes =
      "reset" in entry
        ? current !== undefined
        : !equalJSON(entry.value, current);
    if (
      changes &&
      !movedIndex(s.claim, claim, entry.steps) &&
      !lockAt(locks, claim, entry.steps, valueOf(entry))
    )
      pending.set(path, entry);
  }
  // Closing a hole moves later appended items, so their locks are checked
  // again at their new places.
  let next: Session<C> = { claim, pending, locks };
  for (;;) {
    next = compactAppended(next);
    const locked = [...next.pending].filter(([, e]) =>
      lockAt(locks, claim, e.steps, valueOf(e)),
    );
    if (locked.length === 0) return next;
    const kept = new Map(next.pending);
    for (const [path] of locked) kept.delete(path);
    next = { ...next, pending: kept };
  }
}

function valueOf(entry: PendingChange): unknown {
  return "reset" in entry ? undefined : entry.value;
}

// rekeyed gives each list step of steps the itemStep of the item that it
// names in object, and keeps a step that names no item.
function rekeyed(object: unknown, steps: readonly Step[]): Step[] {
  let at = object;
  return steps.map((step) => {
    if ("field" in step || !Array.isArray(at)) {
      at = valueAt(at, [step]);
      return step;
    }
    const list: unknown[] = at;
    const index =
      "index" in step
        ? step.index
        : list.findIndex((item) => valueAt(list, [step]) === item);
    at = list[index];
    return at === undefined ? step : itemStep(at, index);
  });
}

// lockAt returns the lock that refuses a change at steps to after. A lock
// can name a list item by its keys while steps name it by its index.
function lockAt(
  locks: LockProvider,
  claim: ClaimObject,
  steps: readonly Step[],
  after: unknown,
): FieldLock | undefined {
  const keyedIn = (object: unknown) => rekeyed(object, steps);
  const before = valueAt(claim, steps);
  const target = after === undefined ? claim : withValue(claim, steps, after);
  return (
    lockOf(locks, steps) ??
    [keyedIn(claim), keyedIn(target)]
      .map(
        (keyed) =>
          lockOf(locks, keyed) ?? lockedChange(locks, keyed, before, after),
      )
      .find((lock) => lock !== undefined)
  );
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

// lockedChange returns the lock of a field at or below steps whose value
// differs between before and after.
function lockedChange(
  locks: LockProvider,
  steps: readonly Step[],
  before: unknown,
  after: unknown,
): FieldLock | undefined {
  if (equalJSON(before, after)) return undefined;
  const lock = locks(pathFrom("", steps));
  if (lock) return lock;
  for (const [step, b, a] of childrenOf(before, after)) {
    const found = lockedChange(locks, [...steps, step], b, a);
    if (found) return found;
  }
  return undefined;
}

// childrenOf pairs the children of before and after by their step. A list
// item has the keys that the field paths use: section and registration for
// a plugin selection, its name for a named item, else its index.
function childrenOf(
  before: unknown,
  after: unknown,
): [Step, unknown, unknown][] {
  const pairs = new Map<string, [Step, unknown, unknown]>();
  const add = (step: Step, value: unknown, side: 1 | 2) => {
    const key = JSON.stringify(step);
    const pair = pairs.get(key) ?? [step, undefined, undefined];
    pair[side] = value;
    pairs.set(key, pair);
  };
  for (const [value, side] of [
    [before, 1],
    [after, 2],
  ] as const) {
    if (Array.isArray(value))
      value.forEach((item, index) => add(itemStep(item, index), item, side));
    else if (isObject(value))
      for (const [field, child] of Object.entries(value))
        add({ field }, child, side);
  }
  return [...pairs.values()];
}

function isObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
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
  for (const [key, other] of pending)
    if (isBelow(other.steps, steps)) pending.delete(key);
  if (entry === undefined) pending.delete(pathFrom("", steps));
  else pending.set(entry.path, entry);
  return { ...s, pending };
}

// compactAppended moves the appended items of each list, at indexes past
// the end of the list in the claim, to the indexes right after it, so that
// a dropped item leaves no hole.
function compactAppended<C extends ClaimObject>(s: Session<C>): Session<C> {
  let pending = new Map(s.pending);
  for (let level = 0; ; level++) {
    const at = (steps: readonly Step[]) =>
      appendedIndexes(s.claim, steps)[level];
    const lists = new Map<string, number[]>();
    for (const entry of pending.values()) {
      const j = at(entry.steps);
      if (j === undefined) continue;
      const key = JSON.stringify(entry.steps.slice(0, j));
      const index = (entry.steps[j] as { index: number }).index;
      lists.set(key, [...new Set([...(lists.get(key) ?? []), index])]);
    }
    if (lists.size === 0) return { ...s, pending };
    const next = new Map<FieldPath, PendingChange>();
    for (const [key, entry] of pending) {
      const j = at(entry.steps);
      if (j === undefined) {
        next.set(key, entry);
        continue;
      }
      const list = entry.steps.slice(0, j);
      const items = valueAt(s.claim, list);
      const start = Array.isArray(items) ? items.length : 0;
      const order = lists.get(JSON.stringify(list))!.sort((a, b) => a - b);
      const index =
        start + order.indexOf((entry.steps[j] as { index: number }).index);
      const moved = [...list, { index }, ...entry.steps.slice(j + 1)];
      const path = pathFrom("", moved);
      next.set(path, { ...entry, path, steps: moved });
    }
    pending = next;
  }
}

function appendedIndexes(claim: ClaimObject, steps: readonly Step[]): number[] {
  return steps.flatMap((step, j) =>
    "index" in step && valueAt(claim, steps.slice(0, j + 1)) === undefined
      ? [j]
      : [],
  );
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
