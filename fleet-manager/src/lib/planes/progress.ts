// This module runs in the browser and on the server.
import type { ProgressRow } from "@/components/fleet/progress-rows";
import type { ConditionLike } from "@/components/fleet/status";
import type { FleetCamundaManagementClusterClaimConditionType } from "@/generated/conditions";
import type { CamundaManagementClusterClaim } from "@/generated/crds/camundamanagementclusterclaim";
import type { PluginSection } from "@/generated/crds/pluginsection";
import { precheckOf } from "@/lib/session/precheck";

const MANAGEMENT_CLUSTER_READY: FleetCamundaManagementClusterClaimConditionType =
  "ManagementClusterReady";

/**
 * planeProgressRows returns the progress rows of a plane claim in the
 * expansion order of the claim controller: environment and policy, the
 * database request, the access request when the claim selects one, and
 * the management cluster. A section of a row is the section of sections
 * with the slot of the row. Without a status each row is pending.
 */
export function planeProgressRows(
  claim: CamundaManagementClusterClaim | undefined,
  sections: readonly PluginSection[],
): ProgressRow[] {
  const status = claim?.status;
  const refusal = claim && precheckOf(claim);
  const environment: ProgressRow = {
    id: "environment",
    label: "Environment and policy",
    ...(status?.environment && { detail: status.environment }),
    checks: [
      {
        label: "Policy",
        condition: refusal
          ? conditionOf(refusal)
          : status?.environment
            ? { status: "True" }
            : undefined,
      },
    ],
  };
  const access = sectionOf(sections, "externalAccess");
  const selectsAccess = (claim?.spec.plugins ?? []).some(
    (p) => p.section === access,
  );
  const managementCluster = status?.conditions?.find(
    (c) => c.type === MANAGEMENT_CLUSTER_READY,
  );
  return [
    environment,
    requestRow(
      "database",
      "Databases",
      sectionOf(sections, "managementDatabases"),
      claim,
    ),
    ...(selectsAccess ? [requestRow("access", "Access", access, claim)] : []),
    {
      id: "managementCluster",
      label: "Management cluster",
      checks: [
        {
          label: "Ready",
          condition: managementCluster && conditionOf(managementCluster),
        },
      ],
    },
  ];
}

function requestRow(
  id: string,
  label: string,
  section: string | undefined,
  claim: CamundaManagementClusterClaim | undefined,
): ProgressRow {
  const request = claim?.status?.requests?.find((r) => r.section === section);
  return {
    id,
    label,
    ...(request && { detail: request.name }),
    checks: [
      { label: "Outputs", condition: statusOf(request?.outputsReady) },
      { label: "Ready", condition: statusOf(request?.ready) },
    ],
  };
}

function sectionOf(
  sections: readonly PluginSection[],
  slot: string,
): string | undefined {
  return sections.find((s) => s.spec.slot === slot)?.metadata.name;
}

function statusOf(value: string | undefined): ConditionLike | undefined {
  return value === "True" || value === "False" || value === "Unknown"
    ? { status: value }
    : undefined;
}

function conditionOf(condition: {
  status: "True" | "False" | "Unknown";
  reason?: string;
  message?: string;
}): ConditionLike {
  return {
    status: condition.status,
    ...(condition.reason !== undefined && { reason: condition.reason }),
    ...(condition.message !== undefined && { message: condition.message }),
  };
}
