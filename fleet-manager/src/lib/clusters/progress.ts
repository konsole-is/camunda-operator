// This module runs in the browser and on the server.
import type { ProgressRow } from "@/components/fleet/progress-rows";
import type { ConditionLike } from "@/components/fleet/status";
import type { FleetCamundaClusterClaimConditionType } from "@/generated/conditions";
import type { CamundaClusterClaim } from "@/generated/crds/camundaclusterclaim";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import type { PluginRequest } from "@/generated/crds/pluginrequest";
import type { PluginSection } from "@/generated/crds/pluginsection";

const COMPONENTS: readonly {
  id: string;
  label: string;
  type: FleetCamundaClusterClaimConditionType;
}[] = [
  { id: "requests", label: "Requests", type: "RequestsReady" },
  { id: "cluster", label: "Cluster", type: "ClusterReady" },
  { id: "attachments", label: "Attachments", type: "AttachmentsReady" },
];

/**
 * clusterProgressRows returns the progress rows of a cluster claim: one row
 * for each selection of claim, in claim order, then the rows Requests,
 * Cluster, and Attachments of the conditions RequestsReady, ClusterReady,
 * and AttachmentsReady. A selection row shows the option label and its
 * Outputs and Ready checks and, once status.requests names it, the request
 * name and the first summary item of that request in requests. A
 * condition row carries the condition and shows its message. Without a
 * status each check is pending.
 */
export function clusterProgressRows(
  claim: CamundaClusterClaim | undefined,
  sections: readonly PluginSection[],
  registrations: readonly PluginRegistration[],
  requests: readonly PluginRequest[],
): ProgressRow[] {
  const status = claim?.status;
  const selections = (claim?.spec.plugins ?? []).map(
    ({ section, registration }): ProgressRow => {
      const entry = status?.requests?.find(
        (r) => r.section === section && r.registration === registration,
      );
      const optionLabel =
        registrations.find((r) => r.metadata.name === registration)?.spec
          .label || registration;
      const headline = requests.find(
        (r) =>
          r.metadata.namespace === claim?.metadata.namespace &&
          r.metadata.name === entry?.name,
      )?.status?.summary?.[0];
      return {
        id: `section:${section}/${registration}`,
        label:
          sections.find((s) => s.metadata.name === section)?.spec.label ||
          section,
        detail: entry ? `${optionLabel}, ${entry.name}` : optionLabel,
        ...(headline && { summary: `${headline.label}: ${headline.value}` }),
        checks: [
          { label: "Outputs", condition: statusOf(entry?.outputsReady) },
          { label: "Ready", condition: statusOf(entry?.ready) },
        ],
      };
    },
  );
  return [
    ...selections,
    ...COMPONENTS.map(({ id, label, type }): ProgressRow => {
      const condition = status?.conditions?.find((c) => c.type === type);
      return {
        id,
        label,
        ...(condition?.message && { summary: condition.message }),
        checks: [
          {
            label: "Ready",
            condition: condition && {
              status: condition.status,
              reason: condition.reason,
              message: condition.message,
            },
          },
        ],
      };
    }),
  ];
}

function statusOf(value: string | undefined): ConditionLike | undefined {
  return value === "True" || value === "False" || value === "Unknown"
    ? { status: value }
    : undefined;
}
