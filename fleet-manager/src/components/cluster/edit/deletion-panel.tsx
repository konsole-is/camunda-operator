"use client";

import { DeletionChoiceField } from "@/components/fleet/deletion-choice";
import type { Environment } from "@/generated/crds/environment";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import type { PluginSection } from "@/generated/crds/pluginsection";
import type { Step } from "@/lib/fields";
import {
  deletionChoice,
  offeredSection,
  type PluginSelection,
} from "@/lib/policy";
import { claimAfter } from "@/lib/session";
import type { ClusterSlotContext } from "../slots";
import { useClusterEdit } from "./cluster-edit";

const POLICY: Step[] = [{ field: "spec" }, { field: "deletionPolicy" }];

/**
 * DeletionPanel edits the deletion policy of the claim with deletionChoice,
 * for the selections of the claim after the pending changes. The default
 * is the policy that the claim resolved, else the default of
 * deletionChoice. Choosing the default while the claim sets no policy
 * removes the change. It renders nothing without a session.
 */
export function DeletionPanel({ context }: { context: ClusterSlotContext }) {
  const { session, set, reset } = useClusterEdit();
  if (!session) return null;
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
  return (
    <DeletionChoiceField
      subject="cluster"
      choice={choice}
      onChange={(policy) =>
        policy === choice.defaultPolicy && !session.claim.spec?.deletionPolicy
          ? reset(POLICY)
          : set(POLICY, policy)
      }
    />
  );
}
