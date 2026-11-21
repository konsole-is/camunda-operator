"use client";

import { type ReactNode, useCallback, useMemo } from "react";
import {
  ClaimEditProvider,
  useClaimEdit,
  type ClaimActions,
  type ClaimEdit,
  type ClaimEditHost,
  type ClaimReviewInput,
} from "@/components/fleet/claim-edit";
import { objectDiff } from "@/lib/diff";
import { clusterFields, pathFrom, type Step } from "@/lib/fields";
import type { Resource } from "@/lib/k8s/kinds";
import {
  claimAfter,
  clusterLocks,
  type FieldError,
  resetValue,
  validateClusterClaim,
  type Session,
} from "@/lib/session";
import type { WriteMode } from "@/lib/write/types";
import { blockingMergedErrors } from "../editable-settings";
import { mergedSpecs, showsMergedDiff } from "../impacts";
import { clusterFieldsInputOf } from "../settings";
import type { ClusterSlotContext } from "../slots";
import { changeInputOf } from "./change-input";
import { downgradeDone } from "./downgrade";
import { reviewGuards } from "./review-guards";

export { CHANGES_OFF, type ClaimActions } from "@/components/fleet/claim-edit";

export type ClusterClaim = Resource<"CamundaClusterClaim">;

/** ClusterEdit is the claim edit state of the cluster page. */
export type ClusterEdit = ClaimEdit<ClusterClaim>;

/**
 * ClusterEditProvider is the claim edit host of the cluster page, over the
 * live claim of context, with clusterLocks of context.fields, and with
 * validateClusterClaim and the merged errors that block as its
 * validation. Review adds the reset of allowVersionDowngrade once the
 * downgrade is done, and shows the impacts, cautions, confirm, and merged
 * diff of the cluster. It needs a GitPushesProvider above it.
 */
export function ClusterEditProvider({
  context,
  actions,
  children,
}: {
  context: ClusterSlotContext;
  actions: ClaimActions;
  children: ReactNode;
}): ReactNode {
  const {
    fields,
    environment,
    sections,
    registrations,
    releases,
    cluster,
    backups,
    requests,
    presets,
    platformConfigs,
    storage,
    optimize,
    schedules,
  } = context;
  const registrationsRead = !context.unread.includes("PluginRegistration");
  const locks = useMemo(() => clusterLocks(fields), [fields]);
  const validate = useCallback(
    (s: Session<ClusterClaim>) => [
      ...validateClusterClaim(s, {
        environment,
        sections,
        registrations,
        releases,
        cluster,
        backups,
        requests,
        registrationsRead,
      }),
      ...mergedErrorsAfter(
        {
          cluster,
          presets,
          releases,
          sections,
          environment,
          platformConfigs,
          storage,
          registrations,
          optimize,
          schedules,
        },
        s,
      ),
    ],
    // The page builds a new context on each render, as each second for its
    // clock, so the check depends on the lists that it reads.
    [
      environment,
      sections,
      registrations,
      releases,
      cluster,
      backups,
      requests,
      registrationsRead,
      presets,
      platformConfigs,
      storage,
      optimize,
      schedules,
    ],
  );
  const host: ClaimEditHost<ClusterClaim> = {
    claim: context.claim,
    resource: cluster,
    writeMode: context.writeMode,
    locks,
    validate,
    prepare: (s) => withDowngradeReset(s, cluster),
    review: (s, review) => clusterReview(context, s, review),
  };
  return (
    <ClaimEditProvider host={host} actions={actions}>
      {children}
    </ClaimEditProvider>
  );
}

// mergedErrorsAfter returns the merged errors that block after the pending
// changes of session. The editors that show them can be closed, and the
// preset can change while they are.
function mergedErrorsAfter(
  context: Parameters<typeof changeInputOf>[0] &
    Pick<ClusterSlotContext, "registrations" | "optimize" | "schedules">,
  session: Session<ClusterClaim>,
): FieldError[] {
  const { after } = mergedSpecs(changeInputOf(context, session));
  const claim = claimAfter(session);
  const fields = clusterFields(
    clusterFieldsInputOf({
      cluster: context.cluster,
      merged: after,
      claim,
      sections: context.sections,
      registrations: context.registrations,
      optimize: context.optimize,
      schedules: context.schedules,
    }),
  );
  // Without a path, the change bar shows the message also while the
  // editors of the setting are closed.
  return blockingMergedErrors(fields, after.spec, claim.spec.cluster ?? {}).map(
    (message) => ({ message }),
  );
}

/** useClusterEdit returns the edit state of the page. It needs a ClusterEditProvider above it. */
export function useClusterEdit(): ClusterEdit {
  return useClaimEdit<ClusterClaim>();
}

// clusterReview reads saved, the claim that the save leaves, so a reset
// field that another tool keeps changes nothing in the impacts and the
// merged diff. validate passed on session, so a merged error of saved
// comes from a cleared field that the save keeps or sets to its default.
function clusterReview(
  context: ClusterSlotContext,
  session: Session<ClusterClaim>,
  {
    mode,
    close,
    saved,
  }: { mode: WriteMode; close(): void; saved: Session<ClusterClaim> },
): ClaimReviewInput {
  const merged = showsMergedDiff(session)
    ? mergedSpecs(changeInputOf(context, saved))
    : undefined;
  return {
    ...reviewGuards(context, saved, mode, {
      backupsHref: "?tab=backups",
      onOpenBackups: () => {
        close();
        window.history.replaceState(null, "", "?tab=backups");
      },
    }),
    mergedLines: merged && objectDiff(merged.before.spec, merged.after.spec),
    blocking:
      saved === session
        ? []
        : mergedErrorsAfter(context, saved).map(
            (e) => `${e.message} ${KEPT_CAUSE}`,
          ),
  };
}

const KEPT_CAUSE =
  "A field that you cleared stays in the claim that the save leaves, or gets its default value again. That causes this error. Go back and change the edit, or take the fields over when the review offers that.";

const ALLOW_DOWNGRADE: Step[] = [
  { field: "spec" },
  { field: "allowVersionDowngrade" },
];

// withDowngradeReset adds the reset of allowVersionDowngrade to s once
// downgradeDone, unless s changes the field itself.
function withDowngradeReset(
  s: Session<ClusterClaim>,
  cluster: ClusterSlotContext["cluster"],
): Session<ClusterClaim> {
  if (!downgradeDone(cluster, s.claim)) return s;
  if (s.pending.has(pathFrom("", ALLOW_DOWNGRADE))) return s;
  const result = resetValue(s, ALLOW_DOWNGRADE);
  return "session" in result ? result.session : s;
}
