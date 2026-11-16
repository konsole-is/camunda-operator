import type { KindName } from "@/lib/k8s/kinds";
import type { ConnectionSources } from "./sources";

/** CONNECTION_SOURCE_KINDS is the kind of each list of ConnectionSources. */
export const CONNECTION_SOURCE_KINDS = {
  namespaces: "Namespace",
  environments: "Environment",
  clusters: "CamundaCluster",
  clusterClaims: "CamundaClusterClaim",
  elasticsearchClusters: "ElasticsearchCluster",
  elasticsearchPresets: "ElasticsearchClusterPreset",
  databaseServers: "DatabaseServer",
  databasePresets: "DatabaseServerPreset",
  databases: "Database",
  databaseConfigs: "DatabaseConfig",
  secondaryStorageConfigs: "SecondaryStorageConfig",
  optimizes: "CamundaOptimize",
  planes: "CamundaManagementCluster",
  elasticsearchBackups: "LogicalBackupElasticsearch",
  rdbmsBackups: "LogicalBackupRDBMS",
  pointInTimeRestores: "PointInTimeRestore",
  objectStorageConfigs: "ObjectStorageConfig",
  databaseServerConfigs: "DatabaseServerConfig",
  managementAuthConfigs: "ManagementAuthConfig",
  requests: "PluginRequest",
} as const satisfies Record<keyof ConnectionSources, KindName>;
