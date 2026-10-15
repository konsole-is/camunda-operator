import "server-only";
import type {
  EventsV1Event,
  KubernetesObject,
  V1Namespace,
  V1PersistentVolumeClaim,
  V1Pod,
  V1Service,
} from "@kubernetes/client-node";
import type { BackupSchedule } from "@/generated/crds/backupschedule";
import type { CamundaCluster } from "@/generated/crds/camundacluster";
import type { CamundaClusterPreset } from "@/generated/crds/camundaclusterpreset";
import type { CamundaManagementCluster } from "@/generated/crds/camundamanagementcluster";
import type { CamundaOptimize } from "@/generated/crds/camundaoptimize";
import type { CamundaPlatformConfig } from "@/generated/crds/camundaplatformconfig";
import type { CamundaRelease } from "@/generated/crds/camundarelease";
import type { Database } from "@/generated/crds/database";
import type { DatabaseConfig } from "@/generated/crds/databaseconfig";
import type { DatabaseServer } from "@/generated/crds/databaseserver";
import type { DatabaseServerConfig } from "@/generated/crds/databaseserverconfig";
import type { DatabaseServerPreset } from "@/generated/crds/databaseserverpreset";
import type { ElasticsearchCluster } from "@/generated/crds/elasticsearchcluster";
import type { ElasticsearchClusterPreset } from "@/generated/crds/elasticsearchclusterpreset";
import type { Environment } from "@/generated/crds/environment";
import type { Grant } from "@/generated/crds/grant";
import type { LogicalBackupElasticsearch } from "@/generated/crds/logicalbackupelasticsearch";
import type { LogicalBackupRDBMS } from "@/generated/crds/logicalbackuprdbms";
import type { LogicalRestoreElasticsearch } from "@/generated/crds/logicalrestoreelasticsearch";
import type { LogicalRestoreRDBMS } from "@/generated/crds/logicalrestorerdbms";
import type { ManagementAuthConfig } from "@/generated/crds/managementauthconfig";
import type { ObjectStorageConfig } from "@/generated/crds/objectstorageconfig";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import type { PluginRequest } from "@/generated/crds/pluginrequest";
import type { PluginSection } from "@/generated/crds/pluginsection";
import type { PointInTimeRestore } from "@/generated/crds/pointintimerestore";
import type { SecondaryStorageConfig } from "@/generated/crds/secondarystorageconfig";
import type { SecretSummary } from "./strip-secret";

export type Scope = "Namespaced" | "Cluster";

/** KindInfo is where the API server serves a kind. */
export interface KindInfo {
  /** group is "" for the core group. */
  group: string;
  version: string;
  plural: string;
  scope: Scope;
}

/**
 * KindTypes maps each kind that the fleet manager reads to its type. A
 * Secret is a SecretSummary, because the cache never holds Secret values.
 */
export interface KindTypes {
  BackupSchedule: BackupSchedule;
  CamundaCluster: CamundaCluster;
  CamundaClusterPreset: CamundaClusterPreset;
  CamundaManagementCluster: CamundaManagementCluster;
  CamundaOptimize: CamundaOptimize;
  CamundaPlatformConfig: CamundaPlatformConfig;
  CamundaRelease: CamundaRelease;
  Database: Database;
  DatabaseConfig: DatabaseConfig;
  DatabaseServer: DatabaseServer;
  DatabaseServerConfig: DatabaseServerConfig;
  DatabaseServerPreset: DatabaseServerPreset;
  ElasticsearchCluster: ElasticsearchCluster;
  ElasticsearchClusterPreset: ElasticsearchClusterPreset;
  LogicalBackupElasticsearch: LogicalBackupElasticsearch;
  LogicalBackupRDBMS: LogicalBackupRDBMS;
  LogicalRestoreElasticsearch: LogicalRestoreElasticsearch;
  LogicalRestoreRDBMS: LogicalRestoreRDBMS;
  ManagementAuthConfig: ManagementAuthConfig;
  ObjectStorageConfig: ObjectStorageConfig;
  PointInTimeRestore: PointInTimeRestore;
  SecondaryStorageConfig: SecondaryStorageConfig;
  Environment: Environment;
  PluginSection: PluginSection;
  PluginRegistration: PluginRegistration;
  PluginRequest: PluginRequest;
  Grant: Grant;
  Namespace: V1Namespace;
  Pod: V1Pod;
  Service: V1Service;
  PersistentVolumeClaim: V1PersistentVolumeClaim;
  Secret: SecretSummary;
  Event: EventsV1Event;
  HTTPRoute: KubernetesObject;
  GRPCRoute: KubernetesObject;
  ExternalSecret: KubernetesObject;
}

export type KindName = keyof KindTypes;
export type Resource<K extends KindName> = KindTypes[K];

const core = "core.camunda.io";
const plugins = "plugins.camunda.io";
const gateway = "gateway.networking.k8s.io";

/**
 * KindMap is every kind that the fleet manager reads. A new kind needs an
 * entry here, a type in KindTypes, and get, list, and watch in the
 * ClusterRole of deploy/chart. kinds.test.ts checks the CRDs and the role.
 */
export const KindMap = {
  BackupSchedule: {
    group: core,
    version: "v1",
    plural: "backupschedules",
    scope: "Namespaced",
  },
  CamundaCluster: {
    group: core,
    version: "v1",
    plural: "camundaclusters",
    scope: "Namespaced",
  },
  CamundaClusterPreset: {
    group: core,
    version: "v1",
    plural: "camundaclusterpresets",
    scope: "Cluster",
  },
  CamundaManagementCluster: {
    group: core,
    version: "v1",
    plural: "camundamanagementclusters",
    scope: "Namespaced",
  },
  CamundaOptimize: {
    group: core,
    version: "v1",
    plural: "camundaoptimizes",
    scope: "Namespaced",
  },
  CamundaPlatformConfig: {
    group: core,
    version: "v1",
    plural: "camundaplatformconfigs",
    scope: "Cluster",
  },
  CamundaRelease: {
    group: core,
    version: "v1",
    plural: "camundareleases",
    scope: "Cluster",
  },
  Database: {
    group: core,
    version: "v1",
    plural: "databases",
    scope: "Namespaced",
  },
  DatabaseConfig: {
    group: core,
    version: "v1",
    plural: "databaseconfigs",
    scope: "Namespaced",
  },
  DatabaseServer: {
    group: core,
    version: "v1",
    plural: "databaseservers",
    scope: "Namespaced",
  },
  DatabaseServerConfig: {
    group: core,
    version: "v1",
    plural: "databaseserverconfigs",
    scope: "Namespaced",
  },
  DatabaseServerPreset: {
    group: core,
    version: "v1",
    plural: "databaseserverpresets",
    scope: "Cluster",
  },
  ElasticsearchCluster: {
    group: core,
    version: "v1",
    plural: "elasticsearchclusters",
    scope: "Namespaced",
  },
  ElasticsearchClusterPreset: {
    group: core,
    version: "v1",
    plural: "elasticsearchclusterpresets",
    scope: "Cluster",
  },
  LogicalBackupElasticsearch: {
    group: core,
    version: "v1",
    plural: "logicalbackupelasticsearches",
    scope: "Namespaced",
  },
  LogicalBackupRDBMS: {
    group: core,
    version: "v1",
    plural: "logicalbackuprdbmses",
    scope: "Namespaced",
  },
  LogicalRestoreElasticsearch: {
    group: core,
    version: "v1",
    plural: "logicalrestoreelasticsearches",
    scope: "Namespaced",
  },
  LogicalRestoreRDBMS: {
    group: core,
    version: "v1",
    plural: "logicalrestorerdbmses",
    scope: "Namespaced",
  },
  ManagementAuthConfig: {
    group: core,
    version: "v1",
    plural: "managementauthconfigs",
    scope: "Cluster",
  },
  ObjectStorageConfig: {
    group: core,
    version: "v1",
    plural: "objectstorageconfigs",
    scope: "Namespaced",
  },
  PointInTimeRestore: {
    group: core,
    version: "v1",
    plural: "pointintimerestores",
    scope: "Namespaced",
  },
  SecondaryStorageConfig: {
    group: core,
    version: "v1",
    plural: "secondarystorageconfigs",
    scope: "Namespaced",
  },
  Environment: {
    group: "fleet.camunda.io",
    version: "v1alpha1",
    plural: "environments",
    scope: "Cluster",
  },
  PluginSection: {
    group: plugins,
    version: "v1alpha1",
    plural: "pluginsections",
    scope: "Cluster",
  },
  PluginRegistration: {
    group: plugins,
    version: "v1alpha1",
    plural: "pluginregistrations",
    scope: "Cluster",
  },
  PluginRequest: {
    group: plugins,
    version: "v1alpha1",
    plural: "pluginrequests",
    scope: "Namespaced",
  },
  Grant: {
    group: plugins,
    version: "v1alpha1",
    plural: "grants",
    scope: "Namespaced",
  },
  Namespace: {
    group: "",
    version: "v1",
    plural: "namespaces",
    scope: "Cluster",
  },
  Pod: { group: "", version: "v1", plural: "pods", scope: "Namespaced" },
  Service: {
    group: "",
    version: "v1",
    plural: "services",
    scope: "Namespaced",
  },
  PersistentVolumeClaim: {
    group: "",
    version: "v1",
    plural: "persistentvolumeclaims",
    scope: "Namespaced",
  },
  Secret: { group: "", version: "v1", plural: "secrets", scope: "Namespaced" },
  Event: {
    group: "events.k8s.io",
    version: "v1",
    plural: "events",
    scope: "Namespaced",
  },
  HTTPRoute: {
    group: gateway,
    version: "v1",
    plural: "httproutes",
    scope: "Namespaced",
  },
  GRPCRoute: {
    group: gateway,
    version: "v1",
    plural: "grpcroutes",
    scope: "Namespaced",
  },
  ExternalSecret: {
    group: "external-secrets.io",
    version: "v1",
    plural: "externalsecrets",
    scope: "Namespaced",
  },
} as const satisfies Record<KindName, KindInfo>;

/** isKindName reports whether value names a kind of KindMap. */
export function isKindName(value: string): value is KindName {
  return Object.hasOwn(KindMap, value);
}

/** apiVersionOf returns the apiVersion of the objects of a kind. */
export function apiVersionOf(info: KindInfo): string {
  return info.group === "" ? info.version : `${info.group}/${info.version}`;
}

/** collectionPath returns the path of all objects of a kind, in every namespace. */
export function collectionPath(info: KindInfo): string {
  const base =
    info.group === ""
      ? `/api/${info.version}`
      : `/apis/${info.group}/${info.version}`;
  return `${base}/${info.plural}`;
}
