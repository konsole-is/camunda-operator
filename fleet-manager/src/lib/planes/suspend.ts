// This module runs in the browser and on the server.
import {
  type ClaimObject,
  changeOf,
  type Session,
  setValue,
} from "@/lib/session";
import type { Change } from "@/lib/write/types";

/**
 * suspendChange returns the write that sets spec.suspend of the plane claim
 * to suspend: the claim with that value, and the resourceVersion that the
 * page read as its precondition, so that a write on an older copy fails
 * with Stale. The intent names the plane.
 */
export function suspendChange<C extends ClaimObject>(
  claim: C,
  suspend: boolean,
): Change {
  const open: Session<C> = {
    claim,
    pending: new Map(),
    locks: () => undefined,
  };
  // A session without locks never refuses a change.
  const { session } = setValue(
    open,
    [{ field: "spec" }, { field: "suspend" }],
    suspend,
  ) as { session: Session<C> };
  const name = claim.metadata.name ?? "";
  return changeOf(
    session,
    `${suspend ? "Suspend" : "Resume"} the management plane ${name}`,
  );
}
