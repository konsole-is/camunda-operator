"use client";

import {
  DeletionChoiceField,
  KEEP_REFUSED,
} from "@/components/fleet/deletion-choice";
import type { Environment } from "@/generated/crds/environment";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import type { PluginSection } from "@/generated/crds/pluginsection";
import type { Step } from "@/lib/fields";
import { claimDeletionChoice, type PluginSelection } from "@/lib/policy";
import { claimAfter } from "@/lib/session";
import type { ClusterSlotContext } from "../slots";
import { useClusterEdit } from "./cluster-edit";

const POLICY: Step[] = [{ field: "spec" }, { field: "deletionPolicy" }];

/**
 * DeletionPanel edits the deletion policy of the claim. Choosing the
 * default while the claim sets no policy removes the change. While the
 * claim keeps its resources and an option cannot, the panel shows Keep as
 * chosen. It renders nothing without a session.
 */
export function DeletionPanel({ context }: { context: ClusterSlotContext }) {
  const edit = useClusterEdit();
  if (!edit.session) return null;
  const after = claimAfter(edit.session);
  // The cache objects differ from the generated types only in their
  // timestamps, which claimDeletionChoice does not read.
  const choice = claimDeletionChoice({
    claim: {
      ...after,
      spec: {
        ...after.spec,
        plugins: after.spec?.plugins as PluginSelection[] | undefined,
      },
    },
    environment: context.environment as unknown as Environment | undefined,
    sections: context.sections as unknown as PluginSection[],
    registrations: context.registrations as unknown as PluginRegistration[],
    registrationsRead: !context.unread.includes("PluginRegistration"),
    kind: "CamundaClusterClaim",
  });
  const refused = choice.policy === "Orphan" && !choice.keepAllowed;
  const claimed = edit.session.claim.spec?.deletionPolicy;
  return (
    <div className="flex flex-col gap-3">
      <DeletionChoiceField
        subject="cluster"
        choice={choice}
        onChange={(policy) =>
          policy === choice.defaultPolicy && !claimed
            ? edit.reset(POLICY)
            : edit.set(POLICY, policy)
        }
      />
      {refused && <p className="text-sm text-warn">{KEEP_REFUSED}</p>}
    </div>
  );
}
