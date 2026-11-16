import {
  APP_FIELD_MANAGER,
  gitOpsManager,
  type KeptField,
  keptRemovals,
  type LiveObject,
  type ManagedObject,
} from "@/lib/merge";
import type { WriteMode } from "@/lib/write/types";
import {
  claimAfter,
  type ClaimObject,
  type PendingChange,
  type Session,
} from "./session";

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
 * the save leaves. firstSync says that the manager of the save is not
 * known yet. kept then names every owned field that the change leaves out,
 * because the first sync of the GitOps tool can keep each of them.
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
    applied: appliedSession(s, applier),
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
