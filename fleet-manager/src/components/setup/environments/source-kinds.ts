import type { KindName } from "@/lib/k8s/kinds";
import type { EnvironmentSources } from "./sources";

/** SOURCE_KINDS is the kind of each list of EnvironmentSources. */
export const SOURCE_KINDS = {
  environments: "Environment",
  namespaces: "Namespace",
  clusterClaims: "CamundaClusterClaim",
  planeClaims: "CamundaManagementClusterClaim",
  clusters: "CamundaCluster",
  planes: "CamundaManagementCluster",
  sections: "PluginSection",
  registrations: "PluginRegistration",
  requests: "PluginRequest",
  clusterPresets: "CamundaClusterPreset",
  elasticsearchPresets: "ElasticsearchClusterPreset",
  databasePresets: "DatabaseServerPreset",
  releases: "CamundaRelease",
  platformConfigs: "CamundaPlatformConfig",
} as const satisfies Record<keyof EnvironmentSources, KindName>;
