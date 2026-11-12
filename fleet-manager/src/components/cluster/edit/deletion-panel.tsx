"use client";

import { DeletionChoiceField } from "@/components/create/deletion-choice";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import type { DeletionChoice } from "@/lib/clusters/storage";
import type { Step } from "@/lib/fields";
import { checkDeletionPolicy, type PluginSelection } from "@/lib/policy";
import { claimAfter } from "@/lib/session";
import type { ClaimView } from "../claim";
import type { ClusterSlotContext } from "../slots";
import { useClusterEdit } from "./cluster-edit";

const POLICY: Step[] = [{ field: "spec" }, { field: "deletionPolicy" }];

/**
 * DeletionPanel edits the deletion policy of the claim with the deletion
 * choice of the create flow. The default is the policy that the claim
 * resolved, else Delete. Keep is disabled while checkDeletionPolicy refuses
 * Orphan for the selections of the claim after the pending changes.
 * Choosing the default while the claim sets no policy removes the change.
 * view is the claim after the pending changes. It renders nothing without
 * a session.
 */
export function DeletionPanel({
  context,
  view,
}: {
  context: ClusterSlotContext;
  view: ClaimView;
}) {
  const { session, set, reset } = useClusterEdit();
  if (!session) return null;
  const after = claimAfter(session);
  const selections = (after.spec?.plugins ?? []) as PluginSelection[];
  // The cache objects differ from the generated types only in their
  // timestamps, which checkDeletionPolicy does not read.
  const registrations =
    context.registrations as unknown as PluginRegistration[];
  const defaultPolicy =
    after.status?.resolvedPolicy?.deletionPolicy || "Delete";
  const choice: DeletionChoice = {
    defaultPolicy,
    keepAllowed:
      "value" in checkDeletionPolicy("Orphan", selections, registrations),
    alwaysDeleted: view.sections.flatMap((s) => {
      const registration = registrations.find(
        (r) => r.metadata.name === s.registration,
      );
      return registration &&
        !registration.spec.deletionPolicies.includes("Orphan")
        ? [
            {
              section: s.section,
              label: `${s.optionLabel} (${s.sectionLabel})`,
            },
          ]
        : [];
    }),
    policy: after.spec?.deletionPolicy ?? defaultPolicy,
  };
  return (
    <DeletionChoiceField
      choice={choice}
      onChange={(policy) =>
        policy === defaultPolicy && !session.claim.spec?.deletionPolicy
          ? reset(POLICY)
          : set(POLICY, policy)
      }
    />
  );
}
