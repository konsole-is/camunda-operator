import type { KindName } from "@/lib/k8s/kinds";

/** PRESET_SOURCE_KINDS is the kind of each list that the preset pages read. */
export const PRESET_SOURCE_KINDS = {
  clusterPresets: "CamundaClusterPreset",
  elasticsearchPresets: "ElasticsearchClusterPreset",
  databasePresets: "DatabaseServerPreset",
  environments: "Environment",
  namespaces: "Namespace",
  clusters: "CamundaCluster",
  elasticsearchClusters: "ElasticsearchCluster",
  databaseServers: "DatabaseServer",
  clusterClaims: "CamundaClusterClaim",
  planeClaims: "CamundaManagementClusterClaim",
  requests: "PluginRequest",
  sections: "PluginSection",
  registrations: "PluginRegistration",
} as const satisfies Record<string, KindName>;
