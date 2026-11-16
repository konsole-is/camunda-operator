import type { KindName } from "@/lib/k8s/kinds";
import type { ReleasesData } from "./sources";

/** RELEASE_SOURCE_KINDS is the kind of each list of ReleasesData. */
export const RELEASE_SOURCE_KINDS = {
  releases: "CamundaRelease",
  environments: "Environment",
  namespaces: "Namespace",
  clusters: "CamundaCluster",
  elasticsearchClusters: "ElasticsearchCluster",
  databaseServers: "DatabaseServer",
  claims: "CamundaClusterClaim",
  clusterPresets: "CamundaClusterPreset",
  sections: "PluginSection",
  registrations: "PluginRegistration",
  requests: "PluginRequest",
  elasticsearchBackups: "LogicalBackupElasticsearch",
  rdbmsBackups: "LogicalBackupRDBMS",
} as const satisfies Record<keyof ReleasesData, KindName>;
