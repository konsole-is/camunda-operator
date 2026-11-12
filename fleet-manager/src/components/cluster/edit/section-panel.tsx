"use client";

import { LockedField } from "@/components/fleet/locked-field";
import { secretRefsOf } from "@/components/fleet/secret-ref-picker";
import { type ClusterField, lockLabel } from "@/lib/fields";
import { useResources } from "@/lib/k8s/hooks";
import { equalJSON } from "@/lib/policy";
import type { OpenAPISchema } from "@/lib/schema";
import { claimAfter } from "@/lib/session";
import type { ClaimSection } from "../claim";
import { SelectionSettingsEditor } from "../selection-settings-editor";
import type { ClusterSlotContext } from "../slots";
import { useClusterEdit } from "./cluster-edit";
import { useEditorGeneration } from "./editor-generation";

type Choice = { name: string; value: unknown };

/**
 * SectionPanel edits one selection of the claim: its option is locked as
 * chosen at creation, and the parameters that the claim may set have
 * editors. section comes from the claim after the pending changes. After
 * a discard or a write, the editors show the values of the session again.
 * It follows the Secrets of the namespace while it is open, and renders
 * nothing without a session.
 */
export function SectionPanel({
  context,
  section,
}: {
  context: ClusterSlotContext;
  section: ClaimSection;
}) {
  const { session, pending, set, reset, invalid } = useClusterEdit();
  const { generation, own } = useEditorGeneration(pending);
  const namespace = context.cluster.metadata?.namespace ?? "";
  const secrets = secretRefsOf(useResources("Secret", { namespace }, []).items);
  if (!session) return null;
  const registration = context.registrations.find(
    (r) => r.metadata?.name === section.registration,
  );
  const policies =
    claimAfter(session).status?.resolvedPolicy?.parameters?.[
      section.registration
    ] ?? {};
  const choices = Object.fromEntries(
    Object.entries(policies).flatMap(([name, p]) =>
      p.allowed?.length ? [[name, p.allowed as Choice[]]] : [],
    ),
  );
  const editable = section.parameters.some((f) => !f.lock && f.claimSteps);
  return (
    <div className="flex flex-col gap-4">
      <LockedField
        label="Option"
        value={section.optionLabel}
        reason={lockLabel({ reason: "chosenAtCreation" })}
      />
      {editable ? (
        <SelectionSettingsEditor
          key={generation}
          fields={section.parameters.map((f) => withChoiceName(f, choices))}
          schema={registration?.spec.parameters as OpenAPISchema | undefined}
          choices={Object.fromEntries(
            Object.entries(choices).map(([name, list]) => [
              name,
              list.map((c) => c.name),
            ]),
          )}
          secrets={secrets}
          set={own(set)}
          reset={own(reset)}
          invalid={invalid}
        />
      ) : (
        <p className="text-sm text-muted">No parameters you can change.</p>
      )}
    </div>
  );
}

// withChoiceName gives a parameter with choices the name of its choice, or
// the names of a list, as its value: the claim stores names, while
// field.value is the value of the request.
function withChoiceName(
  field: ClusterField,
  choices: Record<string, Choice[]>,
): ClusterField {
  const list = Object.hasOwn(choices, field.label)
    ? choices[field.label]
    : undefined;
  if (!list) return field;
  const nameOf = (value: unknown) =>
    (
      list.find((c) => c.name === value) ??
      list.find((c) => equalJSON(c.value, value))
    )?.name;
  return {
    ...field,
    value: Array.isArray(field.value)
      ? field.value.map(nameOf).filter((n) => n !== undefined)
      : nameOf(field.value),
  };
}
