"use client";

import { LockedField } from "@/components/fleet/locked-field";
import { secretRefsOf } from "@/components/fleet/secret-ref-picker";
import { type ClusterField, lockLabel } from "@/lib/fields";
import { useResources } from "@/lib/k8s/hooks";
import { namesOf } from "@/lib/policy/options";
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
  const { session, pending, set, reset, invalid, errors } = useClusterEdit();
  const { generation, own } = useEditorGeneration(pending);
  const namespace = context.cluster.metadata?.namespace ?? "";
  const secrets = secretRefsOf(useResources("Secret", { namespace }, []).items);
  if (!session) return null;
  const registration = context.registrations.find(
    (r) => r.metadata?.name === section.registration,
  );
  const after = claimAfter(session);
  const policies =
    after.status?.resolvedPolicy?.parameters?.[section.registration] ?? {};
  const selection = (after.spec?.plugins ?? []).find(
    (p) =>
      p.section === section.section && p.registration === section.registration,
  );
  const claimed = asObject(selection?.parameters);
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
          fields={section.parameters.map((f) =>
            withChoiceName(f, choices, claimed),
          )}
          schema={schemaWithNames(
            registration?.spec.parameters as OpenAPISchema | undefined,
            choices,
          )}
          choices={Object.fromEntries(
            Object.entries(choices).map(([name, list]) => [
              name,
              list.map((c) => c.name),
            ]),
          )}
          secrets={secrets}
          errors={errors}
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

// withChoiceName gives a parameter with choices the names that the claim
// stores as its value, else the names of the value of the request.
function withChoiceName(
  field: ClusterField,
  choices: Record<string, Choice[]>,
  claimed: Record<string, unknown>,
): ClusterField {
  const list = Object.hasOwn(choices, field.label)
    ? choices[field.label]
    : undefined;
  if (!list) return field;
  return {
    ...field,
    value: Object.hasOwn(claimed, field.label)
      ? claimed[field.label]
      : namesOf(field.value, list),
  };
}

// schemaWithNames gives a list parameter whose choices are each a whole
// list the schema of one name: the claim stores one name for it.
function schemaWithNames(
  schema: OpenAPISchema | undefined,
  choices: Record<string, Choice[]>,
): OpenAPISchema | undefined {
  const properties = schema?.properties;
  if (!properties) return schema;
  const names = Object.keys(choices).filter((name) =>
    choices[name].every((c) => Array.isArray(c.value)),
  );
  if (names.length === 0) return schema;
  return {
    ...schema,
    properties: {
      ...properties,
      ...Object.fromEntries(names.map((name) => [name, { type: "string" }])),
    },
  };
}

function asObject(value: unknown): Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : {};
}
