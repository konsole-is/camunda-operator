import type { CoreCamundaClusterConditionType } from "@/generated/conditions";
import type { PlacementMap } from "./placement";

export const CAMUNDA_CLUSTER_PLACEMENT: PlacementMap<CoreCamundaClusterConditionType> =
  {
    Ready: "header",
    ZeebeReady: "zeebe",
    GatewayReady: "gateway",
    OperateReady: "operate",
    TasklistReady: "tasklist",
    AdminReady: "admin",
    ConnectorsReady: "connectors",
    AdminSecretReady: "resource",
    MirroredSecretsReady: "resource",
  };
