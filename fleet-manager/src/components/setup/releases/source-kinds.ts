import type { KindName } from "@/lib/k8s/kinds";
import type { RolloutLists } from "./rollout-sources";
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
} as const satisfies Record<keyof ReleasesData, KindName>;

/** ROLLOUT_SOURCE_KINDS is the kind of each list of RolloutLists. */
export const ROLLOUT_SOURCE_KINDS = {
  clusterPresets: "CamundaClusterPreset",
  sections: "PluginSection",
  registrations: "PluginRegistration",
  requests: "PluginRequest",
  elasticsearchBackups: "LogicalBackupElasticsearch",
  rdbmsBackups: "LogicalBackupRDBMS",
} as const satisfies Record<keyof RolloutLists, KindName>;
