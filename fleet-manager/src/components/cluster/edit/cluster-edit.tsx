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
import { pathFrom, type Step } from "@/lib/fields";
import type { Resource } from "@/lib/k8s/kinds";
import {
  clusterLocks,
  resetValue,
  validateClusterClaim,
  type Session,
} from "@/lib/session";
import type { WriteMode } from "@/lib/write/types";
import { mergedSpecs, showsMergedDiff } from "../impacts";
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
 * live claim of context, with clusterLocks of context.fields and
 * validateClusterClaim. Review adds the reset of allowVersionDowngrade once
 * the downgrade is done, and shows the impacts, cautions, confirm, and
 * merged diff of the cluster. It needs a GitPushesProvider above it.
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
  } = context;
  const registrationsRead = !context.unread.includes("PluginRegistration");
  const locks = useMemo(() => clusterLocks(fields), [fields]);
  const validate = useCallback(
    (s: Session<ClusterClaim>) =>
      validateClusterClaim(s, {
        environment,
        sections,
        registrations,
        releases,
        cluster,
        backups,
        requests,
        registrationsRead,
      }),
    [
      environment,
      sections,
      registrations,
      releases,
      cluster,
      backups,
      requests,
      registrationsRead,
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

/** useClusterEdit returns the edit state of the page. It needs a ClusterEditProvider above it. */
export function useClusterEdit(): ClusterEdit {
  return useClaimEdit<ClusterClaim>();
}

// clusterReview reads saved, the claim that the save leaves, so a reset
// field that another tool keeps changes nothing in the impacts and the
// merged diff.
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
  };
}

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
