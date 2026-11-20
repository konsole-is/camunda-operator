import type {
  ClusterField,
  FieldLock,
  FieldTarget,
  PlaneField,
  Step,
} from "@/lib/fields";
import { child, type FieldPath } from "@/lib/merge";

/** LockProvider returns the lock of a claim path, or undefined when the claim can change it. */
export type LockProvider = (claimPath: FieldPath) => FieldLock | undefined;

// The claim path of the root of each target that a cluster claim sets in
// place.
const CLAIM_ROOTS: Partial<Record<FieldTarget["kind"], readonly string[]>> = {
  CamundaCluster: ["spec", "cluster"],
  CamundaOptimize: ["spec", "attachments", "optimize"],
  BackupSchedule: ["spec", "attachments", "backupSchedule"],
};

/**
 * clusterLocks is the LockProvider of a cluster claim, over clusterFields.
 * A field that a section fills is locked at the claim path of its slot
 * field, and its selection stays open for its parameters. A field of an
 * unmanaged cluster is never asked, because such a cluster has no session.
 */
export function clusterLocks(fields: readonly ClusterField[]): LockProvider {
  const locks = new Map<FieldPath, FieldLock>();
  for (const field of fields) {
    if (field.lock === undefined || field.lock.reason === "notManaged")
      continue;
    const path =
      field.lock.reason !== "filled" && field.claimPath !== undefined
        ? field.claimPath
        : slotPath(field);
    if (path !== undefined) locks.set(path, field.lock);
  }
  return (claimPath) => locks.get(claimPath);
}

/**
 * planeLocks is the LockProvider of a plane claim, over planeFields. It
 * returns the lock of each locked field at its claim path. The lock of an
 * option sits on its registration, so the parameters of the selection stay
 * open unless their own field is locked.
 */
export function planeLocks(fields: readonly PlaneField[]): LockProvider {
  const locks = new Map<FieldPath, FieldLock>();
  for (const field of fields)
    if (field.lock !== undefined) locks.set(field.claimPath, field.lock);
  return (claimPath) => locks.get(claimPath);
}

/** filledClaimSteps returns the claim steps of each field of fields that a section fills, from the root of the claim. */
export function filledClaimSteps(fields: readonly ClusterField[]): Step[][] {
  return fields.flatMap((field) => {
    const names =
      field.lock?.reason === "filled" ? slotNames(field) : undefined;
    return names ? [names.map((name) => ({ field: name }))] : [];
  });
}

function slotPath(field: ClusterField): FieldPath | undefined {
  return slotNames(field)?.reduce(child, "");
}

// docNames hold no list items. That is exact for the locked and filled
// fields, which are all plain fields.
function slotNames(field: ClusterField): string[] | undefined {
  const root = CLAIM_ROOTS[field.target.kind];
  if (root === undefined || field.docNames === undefined) return undefined;
  return [...root, ...field.docNames.slice(1)];
}
