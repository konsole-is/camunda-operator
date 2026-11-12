"use client";

import { LockedField } from "@/components/fleet/locked-field";
import { secretRefsOf } from "@/components/fleet/secret-ref-picker";
import { lockLabel } from "@/lib/fields";
import { useResources } from "@/lib/k8s/hooks";
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
 * editors. section comes from the claim after the pending changes. It
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
  const allowed = Object.fromEntries(
    Object.entries(policies).map(([name, p]) => [
      name,
      p.allowed as Choice[] | undefined,
    ]),
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
          fields={section.parameters}
          schema={registration?.spec.parameters as OpenAPISchema | undefined}
          allowed={allowed}
          claimed={asObject(selection?.parameters)}
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

function asObject(value: unknown): Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : {};
}
