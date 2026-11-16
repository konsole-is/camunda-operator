import { LAST_APPLIED_ANNOTATION, REVISION_ANNOTATION } from "@/lib/labels";
import {
  APP_FIELD_MANAGER,
  child,
  fieldManagers,
  type KeptField,
  keptRemovals,
  type ManagedFieldsEntry,
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
 * that applied the revision annotation of claim, as the Git writer finds
 * it. It is undefined in Git mode for a claim that the GitOps tool applies
 * client side, which keeps no field for another manager.
 */
export function saveManager(
  claim: ClaimObject,
  writeMode: WriteMode | undefined,
): string | undefined {
  if (writeMode !== "git") return APP_FIELD_MANAGER;
  const metadata = claim.metadata as ManagedObject["metadata"] & {
    annotations?: Record<string, string>;
  };
  const applied = (metadata?.managedFields ?? []).filter(
    (e: ManagedFieldsEntry) =>
      e.operation === "Apply" && e.manager !== APP_FIELD_MANAGER,
  );
  const gitOps = fieldManagers({ metadata: { managedFields: applied } })[
    child(child("metadata", "annotations"), REVISION_ANNOTATION)
  ];
  if (gitOps !== undefined) return gitOps;
  return metadata?.annotations?.[LAST_APPLIED_ANNOTATION] === undefined
    ? APP_FIELD_MANAGER
    : undefined;
}

function kept<C extends ClaimObject>(s: Session<C>, applier: string) {
  return keptRemovals(
    s.claim as ManagedObject,
    { spec: s.claim.spec },
    { spec: claimAfter(s).spec },
    applier,
  );
}
