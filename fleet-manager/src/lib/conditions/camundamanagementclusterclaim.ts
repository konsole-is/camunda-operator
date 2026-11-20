import type { FleetCamundaManagementClusterClaimConditionType } from "@/generated/conditions";
import type { PlacementMap } from "./placement";

export const PLANE_CLAIM_PLACEMENT: PlacementMap<FleetCamundaManagementClusterClaimConditionType> =
  {
    Ready: "header",
    RequestsReady: "claim",
    ManagementClusterReady: "claim",
  };
