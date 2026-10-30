import type { CoreCamundaOptimizeConditionType } from "@/generated/conditions";
import type { PlacementMap } from "./placement";

export const CAMUNDA_OPTIMIZE_PLACEMENT: PlacementMap<CoreCamundaOptimizeConditionType> =
  {
    Ready: "optimize",
    WebappReady: "optimize",
    ImporterReady: "optimize",
    MirroredSecretsReady: "resource",
  };
