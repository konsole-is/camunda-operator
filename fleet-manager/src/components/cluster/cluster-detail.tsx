import type { ReactNode } from "react";
import type { ConditionRow } from "@/components/fleet/conditions-table";
import {
  CAMUNDA_CLUSTER_PLACEMENT,
  CAMUNDA_OPTIMIZE_PLACEMENT,
  conditionsAt,
} from "@/lib/conditions";
import type { Resource } from "@/lib/k8s/kinds";
import {
  componentObjectSelector,
  componentPodSelector,
} from "@/lib/observe/selectors";
import type {
  MetricsComponent,
  PartitionHealth,
  Unavailable,
} from "@/lib/observe/types";
import type { CardId, ComponentCardModel } from "./cards";
import { ComponentDetail } from "./component-detail";
import { PartitionTable } from "./partition-table";

/**
 * ClusterComponentDetail is the component detail of one card of the
 * cluster page, or of an app embedded in a card. podSelector is the pod
 * selector of the page. id must be a used card or an app embedded in one,
 * as componentOf gives.
 */
export function ClusterComponentDetail({
  id,
  cards,
  cluster,
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
  cluster: Resource<"CamundaCluster">;
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

  const component: MetricsComponent =
    id === "optimize" ? "optimize" : (host?.id ?? id);
  const selector = componentPodSelector(
    podSelector,
    component,
    optimize?.metadata?.uid,
  );
  const owner = id === "optimize" ? optimize : cluster;
  const eventRefs = owner
    ? [
        {
          kind: id === "optimize" ? "CamundaOptimize" : "CamundaCluster",
          name: owner.metadata?.name ?? "",
          namespace,
          uid: owner.metadata?.uid,
        },
      ]
    : [];
  const rows =
    id === "optimize"
      ? conditionsAt(
          CAMUNDA_OPTIMIZE_PLACEMENT,
          optimize?.status?.conditions as ConditionRow[] | undefined,
          "optimize",
        )
      : conditionsAt(CAMUNDA_CLUSTER_PLACEMENT, conditions, id);

  return (
    <ComponentDetail
      title={title}
      subtitle={host && `Embedded in ${host.label}`}
      namespace={namespace}
      podSelector={selector}
      relatedSelector={componentObjectSelector(podSelector, component)}
      metrics={{ cluster: cluster.metadata?.name ?? "", component }}
      eventRefs={eventRefs}
      conditions={rows}
      settings={settings}
      now={now}
    >
      {id === "zeebe" && <PartitionTable partitions={partitions} />}
    </ComponentDetail>
  );
}
