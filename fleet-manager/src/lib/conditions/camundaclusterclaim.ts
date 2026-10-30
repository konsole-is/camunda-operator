import type { FleetCamundaClusterClaimConditionType } from "@/generated/conditions";
import type { PlacementMap } from "./placement";

export const CLAIM_PLACEMENT: PlacementMap<FleetCamundaClusterClaimConditionType> =
  {
    Ready: "header",
    RequestsReady: "claim",
    ClusterReady: "claim",
    AttachmentsReady: "claim",
  };
