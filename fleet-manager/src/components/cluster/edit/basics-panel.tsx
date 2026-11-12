"use client";

import { useId } from "react";
import { LockedField } from "@/components/fleet/locked-field";
import { Checkbox } from "@/components/ui/checkbox";
import { lockLabel, type Step } from "@/lib/fields";
import { fieldLock } from "@/lib/session";
import type { ClaimView } from "../claim";
import { useClusterEdit } from "./cluster-edit";

const SUSPEND: Step[] = [{ field: "spec" }, { field: "suspend" }];

/**
 * BasicsPanel edits the Basics of the claim: the name, the namespace, and
 * the environment are locked as set at creation, and Suspend is a toggle.
 * view is the claim after the pending changes. It renders nothing without
 * a session.
 */
export function BasicsPanel({ view }: { view: ClaimView }) {
  const { session, set, reset } = useClusterEdit();
  const id = useId();
  if (!session) return null;
  const created = lockLabel({ reason: "atCreation" });
  const lock = fieldLock(session, SUSPEND);
  const toggle = (checked: boolean) => {
    // Unchecking a claim without the field removes the pending change, so
    // the claim does not get suspend: false.
    if (!checked && session.claim.spec?.suspend === undefined) reset(SUSPEND);
    else set(SUSPEND, checked);
  };
  return (
    <div className="flex flex-col gap-4">
      <div className="grid gap-4 sm:grid-cols-3">
        <LockedField label="Name" value={view.name} reason={created} />
        <LockedField
          label="Namespace"
          value={view.namespace}
          reason={created}
        />
        <LockedField
          label="Environment"
          value={view.environment ?? "No environment"}
          reason={created}
        />
      </div>
      {lock ? (
        <LockedField
          label="Suspend the cluster"
          value={view.suspend ? "Yes" : "No"}
          reason={lockLabel(lock)}
        />
      ) : (
        <div className="flex items-start gap-3">
          <Checkbox
            id={id}
            className="mt-0.5"
            checked={view.suspend}
            aria-describedby={`${id}-hint`}
            onCheckedChange={(checked) => toggle(checked === true)}
          />
          <div className="flex flex-col gap-0.5">
            <label htmlFor={id} className="text-sm font-medium text-text">
              Suspend the cluster
            </label>
            <p id={`${id}-hint`} className="text-xs text-muted">
              Every workload goes to zero. The volumes stay.
            </p>
          </div>
        </div>
      )}
    </div>
  );
}
