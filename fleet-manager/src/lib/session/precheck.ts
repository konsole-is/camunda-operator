import type { FleetCamundaClusterClaimConditionType } from "@/generated/conditions";
import type { RefusalReason } from "@/lib/policy";

/** ClaimCondition is the part of a condition of a claim that precheckOf reads. */
export type ClaimCondition = {
  type: string;
  status: "True" | "False" | "Unknown";
  reason?: string;
  message?: string;
  observedGeneration?: number;
};

/** PRECHECK_REASONS are the reasons of a Ready=False claim whose values the claim controller refused. */
export const PRECHECK_REASONS: readonly RefusalReason[] = [
  "NotAllowed",
  "SlotConflict",
  "MissingInput",
  "InvalidReference",
];

const READY: FleetCamundaClusterClaimConditionType = "Ready";

/**
 * precheckOf returns the Ready condition of a claim when it is False with a
 * precheck reason. It returns nothing while the condition is of an older
 * generation of the claim.
 */
export function precheckOf(claim: {
  metadata?: { generation?: number };
  status?: { conditions?: readonly ClaimCondition[] };
}): ClaimCondition | undefined {
  const ready = claim.status?.conditions?.find((c) => c.type === READY);
  const generation = claim.metadata?.generation;
  const current =
    generation === undefined ||
    ready?.observedGeneration === undefined ||
    ready.observedGeneration === generation;
  return current &&
    ready?.status === "False" &&
    PRECHECK_REASONS.some((r) => r === ready.reason)
    ? ready
    : undefined;
}
