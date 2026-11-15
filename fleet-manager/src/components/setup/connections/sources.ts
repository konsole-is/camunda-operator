"use client";

import type { ReadError } from "@/components/planes/planes-table";
import type { DatabaseServerConfig } from "@/generated/crds/databaseserverconfig";
import type { ManagementAuthConfig } from "@/generated/crds/managementauthconfig";
import type { ObjectStorageConfig } from "@/generated/crds/objectstorageconfig";
import type { PluginRequest } from "@/generated/crds/pluginrequest";
import { useResources } from "@/lib/k8s/hooks";
import type { KindName, Resource } from "@/lib/k8s/kinds";
import { useAnswered } from "@/lib/k8s/use-answered";
import type { ContractUserSources } from "@/lib/usage";
import { CONNECTION_SOURCE_KINDS } from "./source-kinds";

/**
 * ConnectionSources holds every list that the Connections pages read, in all
 * namespaces: the contracts, the objects that use them, and the requests
 * that a plugin label can name.
 */
export type ConnectionSources = ContractUserSources & {
  objectStorageConfigs: ObjectStorageConfig[];
  databaseServerConfigs: DatabaseServerConfig[];
  managementAuthConfigs: ManagementAuthConfig[];
  requests: PluginRequest[];
};

/**
 * ConnectionPageData is the first snapshot of a Connections page. readErrors
 * holds the error of each kind that the server could not read.
 */
export type ConnectionPageData = ConnectionSources & {
  readErrors?: ReadError[];
};

/**
 * useConnectionSources follows each list of initial live. errors holds the
 * error of each failed watch, with the kind of CONNECTION_SOURCE_KINDS, and
 * the error of initial.readErrors of each other kind until the watch of that
 * kind answers. The list of a failed watch keeps its last items, which can be
 * out of date.
 */
export function useConnectionSources(
  initial: ConnectionPageData,
): ConnectionSources & { errors: ReadError[] } {
  const namespaces = useLive("Namespace", initial.namespaces);
  const environments = useLive("Environment", initial.environments);
  const clusters = useLive("CamundaCluster", initial.clusters);
  const clusterClaims = useLive("CamundaClusterClaim", initial.clusterClaims);
  const elasticsearchClusters = useLive(
    "ElasticsearchCluster",
    initial.elasticsearchClusters,
  );
  const elasticsearchPresets = useLive(
    "ElasticsearchClusterPreset",
    initial.elasticsearchPresets,
  );
  const databaseServers = useLive("DatabaseServer", initial.databaseServers);
  const databasePresets = useLive(
    "DatabaseServerPreset",
    initial.databasePresets,
  );
  const databases = useLive("Database", initial.databases);
  const databaseConfigs = useLive("DatabaseConfig", initial.databaseConfigs);
  const secondaryStorageConfigs = useLive(
    "SecondaryStorageConfig",
    initial.secondaryStorageConfigs,
  );
  const optimizes = useLive("CamundaOptimize", initial.optimizes);
  const planes = useLive("CamundaManagementCluster", initial.planes);
  const elasticsearchBackups = useLive(
    "LogicalBackupElasticsearch",
    initial.elasticsearchBackups,
  );
  const rdbmsBackups = useLive("LogicalBackupRDBMS", initial.rdbmsBackups);
  const pointInTimeRestores = useLive(
    "PointInTimeRestore",
    initial.pointInTimeRestores,
  );
  const objectStorageConfigs = useLive(
    "ObjectStorageConfig",
    initial.objectStorageConfigs,
  );
  const databaseServerConfigs = useLive(
    "DatabaseServerConfig",
    initial.databaseServerConfigs,
  );
  const managementAuthConfigs = useLive(
    "ManagementAuthConfig",
    initial.managementAuthConfigs,
  );
  const requests = useLive("PluginRequest", initial.requests);
  const results = {
    namespaces,
    environments,
    clusters,
    clusterClaims,
    elasticsearchClusters,
    elasticsearchPresets,
    databaseServers,
    databasePresets,
    databases,
    databaseConfigs,
    secondaryStorageConfigs,
    optimizes,
    planes,
    elasticsearchBackups,
    rdbmsBackups,
    pointInTimeRestores,
    objectStorageConfigs,
    databaseServerConfigs,
    managementAuthConfigs,
    requests,
  };
  const liveErrors = (
    Object.keys(results) as (keyof ConnectionSources)[]
  ).flatMap((key) => {
    const error = results[key].error;
    return error
      ? [{ kind: CONNECTION_SOURCE_KINDS[key], message: error }]
      : [];
  });
  const answered = useAnswered(Object.values(CONNECTION_SOURCE_KINDS));
  const firstErrors = (initial.readErrors ?? []).filter(
    (e) => !answered.has(e.kind) && !liveErrors.some((l) => l.kind === e.kind),
  );
  const errors = [...liveErrors, ...firstErrors];
  return {
    namespaces: namespaces.items,
    environments: environments.items,
    clusters: clusters.items,
    clusterClaims: clusterClaims.items,
    elasticsearchClusters: elasticsearchClusters.items,
    elasticsearchPresets: elasticsearchPresets.items,
    databaseServers: databaseServers.items,
    databasePresets: databasePresets.items,
    databases: databases.items,
    databaseConfigs: databaseConfigs.items,
    secondaryStorageConfigs: secondaryStorageConfigs.items,
    optimizes: optimizes.items,
    planes: planes.items,
    elasticsearchBackups: elasticsearchBackups.items,
    rdbmsBackups: rdbmsBackups.items,
    pointInTimeRestores: pointInTimeRestores.items,
    objectStorageConfigs: objectStorageConfigs.items,
    databaseServerConfigs: databaseServerConfigs.items,
    managementAuthConfigs: managementAuthConfigs.items,
    requests: requests.items,
    errors,
  };
}

// A Resource is plain JSON, so it differs from its model type only in its
// timestamps, which the Connections pages do not read.
function useLive<K extends KindName, T>(
  kind: K,
  initial: readonly T[],
): { items: T[]; error?: string } {
  const { items, error } = useResources(
    kind,
    {},
    initial as unknown as Resource<K>[],
  );
  return { items: items as unknown as T[], ...(error && { error }) };
}
