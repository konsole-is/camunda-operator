import type { KindName } from "@/lib/k8s/kinds";
import type { PlatformSources } from "./sources";

/** PLATFORM_SOURCE_KINDS is the kind of each list of PlatformSources. */
export const PLATFORM_SOURCE_KINDS = {
  platformConfigs: "CamundaPlatformConfig",
  environments: "Environment",
  namespaces: "Namespace",
  clusters: "CamundaCluster",
  planes: "CamundaManagementCluster",
  databaseServers: "DatabaseServer",
  clusterPresets: "CamundaClusterPreset",
  databasePresets: "DatabaseServerPreset",
  releases: "CamundaRelease",
  clusterClaims: "CamundaClusterClaim",
  planeClaims: "CamundaManagementClusterClaim",
} as const satisfies Record<keyof PlatformSources, KindName>;
