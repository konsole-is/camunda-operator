import type { ReactNode } from "react";
import type { ConditionRow } from "@/components/fleet/conditions-table";
import {
  CAMUNDA_CLUSTER_PLACEMENT,
  CAMUNDA_OPTIMIZE_PLACEMENT,
  conditionsAt,
} from "@/lib/conditions";
import type { Resource } from "@/lib/k8s/kinds";
import {
  COMPONENT_LABEL,
  OPTIMIZE_COMPONENTS,
  OPTIMIZE_UID_LABEL,
} from "@/lib/labels";
import type { PartitionHealth, Unavailable } from "@/lib/observe/types";
import type { CardId, ComponentCardModel } from "./cards";
import { ComponentDetail } from "./component-detail";
import { PartitionTable } from "./partition-table";

/**
 * ClusterComponentDetail is the component detail of one card of the
 * cluster page, or of an app embedded in a card. podSelector is the pod
 * selector of the page. An embedded app shows the pods of its host. id must
 * be a used card or an app embedded in one, as componentOf gives.
 */
export function ClusterComponentDetail({
  id,
  cards,
  namespace,
  podSelector,
  conditions,
  optimize,
  partitions,
  settings,
  now,
}: {
  id: CardId;
  cards: readonly ComponentCardModel[];
  namespace: string;
  podSelector: string;
  conditions: readonly ConditionRow[] | undefined;
  optimize?: Resource<"CamundaOptimize">;
  partitions: PartitionHealth[] | Unavailable | undefined;
  settings: ReactNode;
  now: string;
}) {
  const own = cards.find((c) => c.id === id);
  const host = own
    ? undefined
    : cards.find((c) => c.embedded.some((e) => e.process === id));
  const title =
    own?.label ?? host?.embedded.find((e) => e.process === id)?.label ?? id;

  let selector: string;
  let rows: ConditionRow[];
  if (id === "optimize") {
    selector = [
      podSelector,
      `${COMPONENT_LABEL} in (${OPTIMIZE_COMPONENTS.join(",")})`,
      `${OPTIMIZE_UID_LABEL}=${optimize?.metadata?.uid ?? ""}`,
    ].join(",");
    rows = conditionsAt(
      CAMUNDA_OPTIMIZE_PLACEMENT,
      optimize?.status?.conditions as ConditionRow[] | undefined,
      "optimize",
    );
  } else {
    selector = `${podSelector},${COMPONENT_LABEL}=${host?.id ?? id}`;
    rows = conditionsAt(CAMUNDA_CLUSTER_PLACEMENT, conditions, id);
  }

  return (
    <ComponentDetail
      title={title}
      subtitle={host && `Embedded in ${host.label}`}
      namespace={namespace}
      podSelector={selector}
      conditions={rows}
      settings={settings}
      now={now}
    >
      {id === "zeebe" && <PartitionTable partitions={partitions} />}
    </ComponentDetail>
  );
}
