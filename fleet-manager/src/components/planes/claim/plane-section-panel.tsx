"use client";

import { useContext } from "react";
import { useEditorGeneration } from "@/components/cluster/edit/editor-generation";
import { SelectionSettingsEditor } from "@/components/cluster/selection-settings-editor";
import { LockedField } from "@/components/fleet/locked-field";
import { SecretRefsReaderContext } from "@/components/fleet/namespaced-secret-ref-picker";
import { type ClusterField, lockLabel, type PlaneField } from "@/lib/fields";
import { child } from "@/lib/merge";
import type { OpenAPISchema } from "@/lib/schema";
import { claimAfter } from "@/lib/session";
import { planeFieldsOf, usePlaneClaimEdit } from "./plane-claim-edit";

type Choice = { name: string; value: unknown };

/**
 * PlaneSectionPanel edits the selections of the claim in group: the
 * option of each is shown as chosen at creation, and each parameter that
 * the claim may set has an editor. It says so when the claim selects no
 * option there. It renders nothing without a session.
 */
export function PlaneSectionPanel({
  group,
}: {
  group: "databases" | "access";
}) {
  const plane = usePlaneClaimEdit();
  const edit = plane.edit;
  const { generation, own } = useEditorGeneration(edit?.pending ?? 0);
  const { secrets } = useContext(SecretRefsReaderContext)(plane.namespace);
  const session = edit?.session;
  if (!edit || !session) return null;
  const after = claimAfter(session);
  const fields = planeFieldsOf(after, plane).filter((f) => f.group === group);
  const options = fields.filter((f) => f.label === "Option" && f.lock);
  if (options.length === 0)
    return (
      <p className="text-sm text-muted">
        {group === "access"
          ? "No access from outside. Set the URLs in the claim YAML."
          : "No database option."}
      </p>
    );
  return (
    <div className="flex min-w-0 flex-col gap-6">
      {options.map((option) => {
        const registrationName = String(option.value);
        const parameters = fields.filter((f) =>
          f.key.startsWith(`${option.key}.`),
        );
        const selection = (after.spec?.plugins ?? []).find(
          (p) => `plugins.${p.section}.${p.registration}` === option.key,
        );
        const registration = plane.registrations.find(
          (r) => r.metadata?.name === registrationName,
        );
        const policies =
          after.status?.resolvedPolicy?.parameters?.[registrationName] ?? {};
        const allowed = Object.fromEntries(
          Object.entries(policies).map(([name, p]) => [
            name,
            p.allowed as Choice[] | undefined,
          ]),
        );
        const claimed = asObject(selection?.parameters);
        const editable = parameters.some((f) => !f.lock);
        return (
          <section key={option.key} className="flex min-w-0 flex-col gap-3">
            <LockedField
              label="Option"
              value={registrationName}
              reason={lockLabel(option.lock!)}
            />
            {editable ? (
              <SelectionSettingsEditor
                key={generation}
                fields={parameters.map((f) =>
                  clusterFieldOf(f, {
                    section: selection?.section ?? "",
                    registration: registrationName,
                    claimed,
                  }),
                )}
                schema={
                  registration?.spec.parameters as OpenAPISchema | undefined
                }
                allowed={allowed}
                claimed={claimed}
                secrets={secrets}
                errors={edit.errors}
                set={own(edit.set)}
                reset={own(edit.reset)}
                invalid={edit.invalid}
              />
            ) : (
              <p className="text-sm text-muted">
                No parameters you can change.
              </p>
            )}
          </section>
        );
      })}
    </div>
  );
}

// clusterFieldOf gives a parameter of a plane selection the shape that the
// settings editor of a selection takes.
function clusterFieldOf(
  field: PlaneField,
  selection: {
    section: string;
    registration: string;
    claimed: Record<string, unknown>;
  },
): ClusterField {
  const { section, registration, claimed } = selection;
  return {
    key: field.key,
    target: { kind: "PluginRequest", section, registration },
    group: "parameters",
    path: child(child("spec", "parameters"), field.label),
    label: field.label,
    value: field.value,
    state: field.lock
      ? { kind: "admin" }
      : Object.hasOwn(claimed, field.label)
        ? { kind: "claim" }
        : { kind: "environment" },
    ...(field.lock && { lock: field.lock }),
    claimPath: field.claimPath,
    claimSteps: field.claimSteps,
  };
}

function asObject(value: unknown): Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : {};
}
