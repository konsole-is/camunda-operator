import type { CoreCamundaManagementClusterConditionType } from "@/generated/conditions";
import type { PlacementMap } from "./placement";

export const MANAGEMENT_CLUSTER_PLACEMENT: PlacementMap<CoreCamundaManagementClusterConditionType> =
  {
    Ready: "header",
    KeycloakReady: "keycloak",
    IdentityReady: "identity",
    ConsoleReady: "console",
    WebModelerReady: "webModeler",
    SecretsReady: "resource",
    MirroredSecretsReady: "resource",
    ServiceAccountsReady: "resource",
    ManagementAuthReady: "resource",
    OptimizeCallbacksReady: "resource",
  };
