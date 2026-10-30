import {
  type ConditionRow,
  ConditionsTable,
} from "@/components/fleet/conditions-table";
import {
  CAMUNDA_CLUSTER_PLACEMENT,
  CAMUNDA_OPTIMIZE_PLACEMENT,
  CLAIM_PLACEMENT,
  conditionsAt,
} from "@/lib/conditions";

/**
 * OverviewConditions lists the conditions that belong to no component: those
 * of the cluster and of its Optimize that the maps place at resource, and
 * the claim group. A group shows only when it has rows. now is the time that
 * the ages count to.
 */
export function OverviewConditions({
  cluster,
  optimize,
  claim,
  now,
}: {
  cluster: ConditionRow[] | undefined;
  optimize?: ConditionRow[];
  claim?: ConditionRow[];
  now: string;
}) {
  const groups = [
    {
      label: "Optimize",
      conditions: conditionsAt(
        CAMUNDA_OPTIMIZE_PLACEMENT,
        optimize,
        "resource",
      ),
    },
    {
      label: "Claim",
      conditions: conditionsAt(CLAIM_PLACEMENT, claim, "claim"),
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
          CAMUNDA_CLUSTER_PLACEMENT,
          cluster,
          "resource",
        )}
        groups={groups}
        now={now}
        label="Conditions outside the components"
      />
    </section>
  );
}
