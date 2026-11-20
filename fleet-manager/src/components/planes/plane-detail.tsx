import type { ReactNode } from "react";
import type { ComponentCardModel } from "@/components/cluster/cards";
import { ComponentDetail } from "@/components/cluster/component-detail";
import type { ConditionRow } from "@/components/fleet/conditions-table";
import { conditionsAt, MANAGEMENT_CLUSTER_PLACEMENT } from "@/lib/conditions";
import type { PlaneField } from "@/lib/fields";
import type { Resource } from "@/lib/k8s/kinds";
import {
  PLANE_CARD_CONDITION,
  type PlaneCardId,
  planeCardFields,
  planeCardSelector,
} from "./plane-cards";
import { PlaneSettings } from "./plane-settings";

/**
 * PlaneComponentDetail is the detail of one card of the plane page: the
 * pods of its component in the namespace of the plane, the conditions that
 * the map places at it, the events of the CamundaManagementCluster, and
 * the field states of the card in the Settings tab. It shows no metrics
 * charts. podSelector selects the pods of the plane. While unread holds
 * CamundaManagementCluster, it states no condition and names that kind.
 */
export function PlaneComponentDetail({
  id,
  cards,
  plane,
  podSelector,
  fields,
  unread = [],
  now,
}: {
  id: Exclude<PlaneCardId, "identityProvider">;
  cards: readonly ComponentCardModel<PlaneCardId>[];
  plane: Resource<"CamundaManagementCluster">;
  podSelector: string;
  fields: readonly PlaneField[];
  unread?: readonly string[];
  now: string;
}): ReactNode {
  const { namespace = "", name = "", uid } = plane.metadata ?? {};
  const planeUnread = unread.filter((k) => k === "CamundaManagementCluster");
  const rows = planeUnread.length
    ? []
    : conditionsAt(
        MANAGEMENT_CLUSTER_PLACEMENT,
        plane.status?.conditions as ConditionRow[] | undefined,
        PLANE_CARD_CONDITION[id]!,
      );
  return (
    <ComponentDetail
      title={cards.find((c) => c.id === id)?.label ?? id}
      namespace={namespace}
      podSelector={planeCardSelector(podSelector, id)}
      eventRefs={[{ kind: "CamundaManagementCluster", name, namespace, uid }]}
      conditions={rows}
      ready={rows[0]}
      unread={planeUnread[0]}
      settings={<PlaneSettings fields={planeCardFields(id, fields)} />}
      now={now}
    />
  );
}
