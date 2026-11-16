import { type KeptField, keptRemovals, type ManagedObject } from "@/lib/merge";
import {
  claimAfter,
  type ClaimObject,
  type PendingChange,
  type Session,
} from "./session";

/**
 * keptFields returns each field of the spec of the claim of s that the
 * change of s leaves out while a field manager other than the fleet
 * manager owns it, with each such manager. The apply keeps such a field,
 * because the fleet manager does not take a field over. The result is in
 * the order of the claim, and empty when the claim holds no managedFields.
 */
export function keptFields<C extends ClaimObject>(s: Session<C>): KeptField[] {
  return kept(s).kept;
}

/**
 * appliedSession returns a session over the claim of s whose change gives
 * the claim after the change of s with each field of keptFields back: what
 * the claim holds after the save.
 */
export function appliedSession<C extends ClaimObject>(
  s: Session<C>,
): Session<C> {
  const { spec } = kept(s).result as { spec?: object };
  const steps = [{ field: "spec" }];
  const change: PendingChange =
    spec === undefined
      ? { path: "spec", steps, reset: true }
      : { path: "spec", steps, value: spec };
  return { ...s, pending: new Map([["spec", change]]) };
}

function kept<C extends ClaimObject>(s: Session<C>) {
  return keptRemovals(
    s.claim as ManagedObject,
    { spec: s.claim.spec },
    { spec: claimAfter(s).spec },
  );
}
