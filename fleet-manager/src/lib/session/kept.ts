import { itemStep, pathFrom, type Step } from "@/lib/fields";
import {
  APP_FIELD_MANAGER,
  type FieldPath,
  gitOpsManager,
  type KeptField,
  keptRemovals,
  type LiveObject,
  type ManagedObject,
} from "@/lib/merge";
import { equalJSON } from "@/lib/policy";
import { offered, withoutFields, withValuesOf } from "@/lib/write/ownership";
import type { ObjectOwnership, WriteMode } from "@/lib/write/types";
import {
  claimAfter,
  type ClaimObject,
  type PendingChange,
  type Session,
} from "./session";
import { valueAt, withValue } from "./steps";

/**
 * keptFields returns each field of the spec of the claim of s that the
 * change of s leaves out while a field manager other than applier owns it,
 * with each such manager. The apply keeps such a field, because it does
 * not take a field over. The result is in the order of the claim, and
 * empty when the claim holds no managedFields.
 */
export function keptFields<C extends ClaimObject>(
  s: Session<C>,
  applier: string = APP_FIELD_MANAGER,
): KeptField[] {
  return kept(s, applier).kept;
}

/**
 * appliedSession returns a session over the claim of s whose change gives
 * the claim after the change of s with each field of keptFields back: what
 * the claim holds after the save.
 */
export function appliedSession<C extends ClaimObject>(
  s: Session<C>,
  applier: string = APP_FIELD_MANAGER,
): Session<C> {
  const { spec } = kept(s, applier).result as { spec?: object };
  return specSession(s, spec);
}

/**
 * savedSession returns a session over the claim of s whose change gives
 * the claim that a save of s in direct mode leaves, from ownership, the
 * fields of other tools that the review of the claim names: each cleared
 * field stays, unless takeOver is true and the review offers it. final is
 * the claim of the preview with the take-over. A removed field that final
 * holds, other than a list item, gets the value of final when the save
 * keeps its parent. It returns s when ownership names no cleared field.
 */
export function savedSession<C extends ClaimObject>(
  s: Session<C>,
  ownership: ObjectOwnership | undefined,
  takeOver: boolean,
  final?: object,
): Session<C> {
  const cleared = ownership?.cleared ?? [];
  if (cleared.length === 0) return s;
  const applied = appliedSession(s);
  if (!takeOver) return applied;
  const removed = cleared.filter(offered);
  const saved = withoutFields(claimAfter(applied), removed);
  const { spec } = final ? withValuesOf(saved, removed, final) : saved;
  return specSession(s, spec);
}

/**
 * dryRunKept returns each field of the spec of the claim of s that the
 * change of s removes and that dryRun, the claim of a dry run of that
 * change, still holds with the value of the claim, in the order of the
 * claim. A field is a map key at the deepest level where dryRun still
 * holds a value, or a list item that the change removes whole. applied is
 * a session whose change gives the claim after the change with each
 * removed field that dryRun holds back, as dryRun holds it, or s when
 * dryRun holds none.
 */
export function dryRunKept<C extends ClaimObject>(
  s: Session<C>,
  dryRun: object,
): { kept: FieldPath[]; applied: Session<C> } {
  const intended = claimAfter(s).spec;
  const dry = (dryRun as { spec?: unknown }).spec;
  const restored: Step[][] = [];
  // A list inside an item comes before the list of that item, whose new
  // order can move the item that an index names.
  const reordered: Step[][] = [];
  const visit = (
    live: unknown,
    after: unknown,
    held: unknown,
    at: Step[],
    item: boolean,
  ): void => {
    if (live === undefined || held === undefined) return;
    if (
      isPlainObject(live) &&
      isPlainObject(held) &&
      !(item && after === undefined)
    ) {
      if (after !== undefined && !isPlainObject(after)) return;
      for (const key of Object.keys(live)) {
        const step = { field: key };
        visit(
          live[key],
          valueAt(after, [step]),
          valueAt(held, [step]),
          [...at, step],
          false,
        );
      }
      return;
    }
    if (Array.isArray(live) && Array.isArray(after) && Array.isArray(held)) {
      const before = restored.length;
      live.forEach((entry, index) => {
        const step = itemStep(entry, index);
        visit(
          entry,
          valueAt(after, [step]),
          valueAt(held, [step]),
          [...at, step],
          true,
        );
      });
      if (
        restored.slice(before).some((steps) => steps.length === at.length + 1)
      )
        reordered.push(at);
      return;
    }
    if (after === undefined) restored.push(at);
  };
  visit(s.claim.spec, intended, dry, [{ field: "spec" }], false);
  if (restored.length === 0) return { kept: [], applied: s };
  let claim = restored.reduce(
    (to, steps) => withValue(to, steps, valueAt(dryRun, steps)),
    { spec: intended } as object,
  );
  for (const at of reordered)
    claim = withValue(
      claim,
      at,
      inDryRunOrder(valueAt(claim, at), valueAt(dryRun, at)),
    );
  return {
    kept: restored
      .filter((steps) =>
        equalJSON(valueAt(dryRun, steps), valueAt(s.claim, steps)),
      )
      .map((steps) => pathFrom("", steps)),
    applied: specSession(s, (claim as { spec?: object }).spec),
  };
}

// inDryRunOrder returns the items of list in the order of the same items in
// dryRun, with the items that dryRun does not hold at the end.
function inDryRunOrder(list: unknown, dryRun: unknown): unknown {
  if (!Array.isArray(list) || !Array.isArray(dryRun)) return list;
  const keyOf = (item: unknown, index: number) =>
    JSON.stringify(itemStep(item, index));
  const keys = list.map(keyOf);
  const ordered = dryRun.flatMap((item, index) => {
    const at = keys.indexOf(keyOf(item, index));
    return at >= 0 ? [list[at]] : [];
  });
  return [...ordered, ...list.filter((item) => !ordered.includes(item))];
}

function isPlainObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function specSession<C extends ClaimObject>(
  s: Session<C>,
  spec: object | undefined,
): Session<C> {
  const steps = [{ field: "spec" }];
  const change: PendingChange =
    spec === undefined
      ? { path: "spec", steps, reset: true }
      : { path: "spec", steps, value: spec };
  return { ...s, pending: new Map([["spec", change]]) };
}

/**
 * saveManager returns the field manager of the apply that a save of claim
 * in writeMode makes: the fleet manager, or in Git mode the GitOps tool
 * that applied the revision annotation of claim with server-side apply. It
 * is undefined in Git mode while no GitOps tool did so yet.
 */
export function saveManager(
  claim: ClaimObject,
  writeMode: WriteMode | undefined,
): string | undefined {
  if (writeMode !== "git") return APP_FIELD_MANAGER;
  return gitOpsManager(claim as LiveObject);
}

/**
 * Saved is what a save keeps: the fields that it leaves out while other
 * field managers own them, and a session whose change gives the claim that
 * the save leaves. While firstSync, the manager of the save is not known:
 * kept names each owned field that a server-side sync keeps, and applied
 * is the change as written, which a client-side sync applies.
 */
export type Saved<C extends ClaimObject> = {
  kept: KeptField[];
  applied: Session<C>;
  firstSync: boolean;
};

/** savedOf returns what a save of s in writeMode keeps. */
export function savedOf<C extends ClaimObject>(
  s: Session<C>,
  writeMode: WriteMode | undefined,
): Saved<C> {
  const manager = saveManager(s.claim, writeMode);
  // An applier that owns no entry keeps every owned field.
  const applier = manager ?? "";
  return {
    kept: keptFields(s, applier),
    applied: manager === undefined ? s : appliedSession(s, applier),
    firstSync: manager === undefined,
  };
}

function kept<C extends ClaimObject>(s: Session<C>, applier: string) {
  return keptRemovals(
    s.claim as ManagedObject,
    { spec: s.claim.spec },
    { spec: claimAfter(s).spec },
    applier,
  );
}
