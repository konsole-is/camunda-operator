import {
  type ConditionRow,
  ConditionsTable,
} from "@/components/fleet/conditions-table";
import {
  MANAGEMENT_CLUSTER_PLACEMENT,
  PLANE_CLAIM_PLACEMENT,
  conditionsAt,
  placementOf,
} from "@/lib/conditions";

/**
 * PlaneConditions is the Conditions section of a plane: the conditions of
 * the CamundaManagementCluster that the map places at resource, then the
 * group "Claim" with the claim conditions at claim or resource. A group
 * shows only when it has rows. now is the time that the ages count to.
 */
export function PlaneConditions({
  plane,
  claim,
  now,
}: {
  plane?: ConditionRow[];
  claim?: ConditionRow[];
  now: string;
}) {
  const groups = [
    {
      label: "Claim",
      conditions: (claim ?? []).filter((c) =>
        ["claim", "resource"].includes(
          placementOf(PLANE_CLAIM_PLACEMENT, c.type),
        ),
      ),
    },
  ].filter((g) => g.conditions.length > 0);
  return (
    <section
      aria-label="Conditions"
      className="flex flex-col gap-3 rounded-lg border border-border bg-panel p-4"
    >
      <h2 className="font-heading text-base font-semibold">Conditions</h2>
      <ConditionsTable
        conditions={conditionsAt(
          MANAGEMENT_CLUSTER_PLACEMENT,
          plane,
          "resource",
        )}
        groups={groups}
        now={now}
        label="Conditions outside the components"
      />
    </section>
  );
}
