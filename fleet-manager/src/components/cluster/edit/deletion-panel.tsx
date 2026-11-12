"use client";

import { DeletionChoiceField } from "@/components/fleet/deletion-choice";
import type { Environment } from "@/generated/crds/environment";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import type { PluginSection } from "@/generated/crds/pluginsection";
import type { Step } from "@/lib/fields";
import {
  type DeletionChoice,
  deletionChoice,
  offeredSection,
  type PluginSelection,
} from "@/lib/policy";
import { claimAfter } from "@/lib/session";
import type { ClusterSlotContext } from "../slots";
import { type ClusterEdit, useClusterEdit } from "./cluster-edit";

const POLICY: Step[] = [{ field: "spec" }, { field: "deletionPolicy" }];

const KEEP_REFUSED =
  "The claim keeps its resources, but an option below cannot keep them. Choose Delete, or remove that option.";

/**
 * DeletionPanel edits the deletion policy of the claim. Choosing the
 * default while the claim sets no policy removes the change. While the
 * claim keeps its resources and an option cannot, the panel shows Keep as
 * chosen. It renders nothing without a session.
 */
export function DeletionPanel({ context }: { context: ClusterSlotContext }) {
  const edit = useClusterEdit();
  if (!edit.session) return null;
  const choice = shownChoice(context, edit.session);
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

function shownChoice(
  context: ClusterSlotContext,
  session: NonNullable<ClusterEdit["session"]>,
): DeletionChoice {
  const after = claimAfter(session);
  const selections = (after.spec?.plugins ?? []) as PluginSelection[];
  // The cache objects differ from the generated types only in their
  // timestamps, which deletionChoice and offeredSection do not read.
  const registrations =
    context.registrations as unknown as PluginRegistration[];
  const environment = context.environment as unknown as Environment | undefined;
  const views = (context.sections as unknown as PluginSection[]).map((s) =>
    offeredSection(s, registrations, environment, "CamundaClusterClaim"),
  );
  const choice = deletionChoice(
    after.spec?.deletionPolicy,
    selections,
    environment,
    registrations,
    views,
    after.status?.resolvedPolicy?.deletionPolicy || undefined,
  );
  return {
    ...choice,
    policy: after.spec?.deletionPolicy ?? choice.defaultPolicy,
  };
}
