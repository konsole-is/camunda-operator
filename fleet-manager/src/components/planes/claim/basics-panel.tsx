"use client";

import { useId } from "react";
import { errorsAt } from "@/components/cluster/settings-editor";
import { LockedField } from "@/components/fleet/locked-field";
import {
  NativeSelect,
  NativeSelectOption,
} from "@/components/ui/native-select";
import { lockLabel, type Step } from "@/lib/fields";
import { claimAfter } from "@/lib/session";
import { FieldErrors } from "./fields";
import { usePlaneClaimEdit } from "./plane-claim-edit";

const PLATFORM_CONFIG: Step[] = [
  { field: "spec" },
  { field: "platformConfigRef" },
];

/**
 * PlaneBasicsPanel shows the name and the namespace, set at creation, and
 * edits the platform config of the claim. The first option leaves the claim
 * without one, so that the plane takes the platform config of its
 * environment. It renders nothing without a session.
 */
export function PlaneBasicsPanel() {
  const plane = usePlaneClaimEdit();
  const id = useId();
  const edit = plane.edit;
  const session = edit?.session;
  if (!edit || !session) return null;
  const after = claimAfter(session);
  const ref =
    typeof after.spec?.platformConfigRef === "string"
      ? after.spec.platformConfigRef
      : "";
  const inherited = plane.environment?.spec.platformConfigRef;
  const names = [
    ...new Set([
      ...plane.platformConfigs.map((c) => c.metadata?.name ?? ""),
      ref,
    ]),
  ]
    .filter(Boolean)
    .sort();
  const atCreation = lockLabel({ reason: "atCreation" });
  return (
    <div className="flex min-w-0 flex-col gap-4">
      <LockedField label="Name" value={plane.name} reason={atCreation} />
      <LockedField
        label="Namespace"
        value={plane.namespace}
        reason={atCreation}
      />
      <div className="flex max-w-md flex-col gap-1">
        <label htmlFor={`${id}-config`} className="text-sm font-medium">
          Platform config
        </label>
        <p id={`${id}-config-hint`} className="text-sm text-muted">
          The apps of the plane restart on a move. OIDC sign-in needs a platform
          config with OIDC authentication.
        </p>
        <NativeSelect
          id={`${id}-config`}
          value={ref}
          aria-describedby={`${id}-config-hint`}
          onChange={(event) =>
            event.target.value === ""
              ? edit.reset(PLATFORM_CONFIG)
              : edit.set(PLATFORM_CONFIG, event.target.value)
          }
        >
          <NativeSelectOption value="">
            {inherited
              ? `From the environment (${inherited})`
              : "From the environment"}
          </NativeSelectOption>
          {names.map((name) => (
            <NativeSelectOption key={name} value={name}>
              {name}
            </NativeSelectOption>
          ))}
        </NativeSelect>
        <FieldErrors errors={errorsAt(edit.errors, PLATFORM_CONFIG)} />
      </div>
    </div>
  );
}
