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

type Choice = { name: string; value: unknown };

/**
 * SectionPanel edits one selection of the claim: its option is locked as
 * chosen at creation, and the parameters that the claim may set have
 * editors. A parameter with choices of the Environment picks a choice by
 * its name. section comes from the claim after the pending changes. It
 * follows the Secrets of the namespace while it is open, and renders
 * nothing without a session.
 */
export function SectionPanel({
  context,
  section,
}: {
  context: ClusterSlotContext;
  section: ClaimSection;
}) {
  const { session, set, reset, invalid } = useClusterEdit();
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
          fields={section.parameters.map((f) => withChoiceName(f, choices))}
          schema={registration?.spec.parameters as OpenAPISchema | undefined}
          choices={Object.fromEntries(
            Object.entries(choices).map(([name, list]) => [
              name,
              list.map((c) => c.name),
            ]),
          )}
          secrets={secrets}
          set={set}
          reset={reset}
          invalid={invalid}
        />
      ) : (
        <p className="text-sm text-muted">No parameters you can change.</p>
      )}
    </div>
  );
}

// withChoiceName gives a parameter with choices the name of its choice as
// its value: the claim stores the name, while field.value is the value of
// the request.
function withChoiceName(
  field: ClusterField,
  choices: Record<string, Choice[]>,
): ClusterField {
  const list = Object.hasOwn(choices, field.label)
    ? choices[field.label]
    : undefined;
  if (!list) return field;
  const byName = list.find((c) => c.name === field.value);
  const byValue = list.find((c) => equalJSON(c.value, field.value));
  return { ...field, value: (byName ?? byValue)?.name };
}
