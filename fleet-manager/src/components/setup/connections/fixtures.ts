import type { CamundaCluster } from "@/generated/crds/camundacluster";
import type { CamundaOptimize } from "@/generated/crds/camundaoptimize";
import type { DatabaseConfig } from "@/generated/crds/databaseconfig";
import type { DatabaseServerConfig } from "@/generated/crds/databaseserverconfig";
import type { ManagementAuthConfig } from "@/generated/crds/managementauthconfig";
import type { ObjectStorageConfig } from "@/generated/crds/objectstorageconfig";
import type { PluginRequest } from "@/generated/crds/pluginrequest";
import type { SecondaryStorageConfig } from "@/generated/crds/secondarystorageconfig";
import {
  ENVIRONMENT_LABEL,
  MANAGEMENT_CLUSTER_LABEL,
  MANAGEMENT_CLUSTER_NAMESPACE_LABEL,
  REGISTRATION_LABEL,
  REQUEST_LABEL,
} from "@/lib/labels";
import type { ConnectionSources } from "./sources";

const SINCE = "2026-11-10T09:00:00Z";
const CORE = "core.camunda.io/v1";

function ready(status: "True" | "False", reason: string, message: string) {
  return [
    { type: "Ready", status, reason, message, lastTransitionTime: SINCE },
  ];
}

const healthy = { conditions: ready("True", "Healthy", "All checks passed") };

function cluster(
  name: string,
  spec: Partial<CamundaCluster["spec"]>,
  status: CamundaCluster["status"],
): CamundaCluster {
  return {
    apiVersion: CORE,
    kind: "CamundaCluster",
    metadata: { namespace: "team-a", name },
    spec: spec as CamundaCluster["spec"],
    status,
  };
}

function objectStorage(
  name: string,
  labels: Record<string, string> | undefined,
  status = healthy,
): ObjectStorageConfig {
  return {
    apiVersion: CORE,
    kind: "ObjectStorageConfig",
    metadata: { namespace: "team-a", name, ...(labels && { labels }) },
    spec: {
      type: "S3",
      s3: {
        bucketName: `acme-${name}`,
        region: "eu-west-1",
        auth: {
          type: "workloadIdentity",
          workloadIdentity: {
            roleArn: `arn:aws:iam::123456789012:role/${name}`,
          },
        },
      },
    },
    status,
  };
}

function databaseConfig(name: string, database: string): DatabaseConfig {
  return {
    apiVersion: CORE,
    kind: "DatabaseConfig",
    metadata: { namespace: "team-a", name },
    spec: {
      databaseName: database,
      serverRef: "postgres",
      credentialsSecretRef: { name, usernameKey: "user", passwordKey: "pass" },
    },
    status: healthy,
  };
}

/**
 * connectionFixtures holds a contract of each writer and each kind in the
 * namespace team-a of the Environment production: backups created by hand,
 * orders-es owned by an ElasticsearchCluster, shared-es created by hand and
 * refused to the cluster billing, orders-backup written by the registration
 * aws-s3 for a live request, old-backup kept from aws-s3, postgres that two
 * DatabaseConfigs use and that is not ready, and plane-a-optimize of the
 * plane platform/plane-a.
 */
export function connectionFixtures(): ConnectionSources {
  const objectStorageConfigs: ObjectStorageConfig[] = [
    objectStorage("backups", undefined),
    objectStorage("orders-backup", {
      [REGISTRATION_LABEL]: "aws-s3",
      [REQUEST_LABEL]: "orders-backup",
    }),
    objectStorage("old-backup", {
      [REGISTRATION_LABEL]: "aws-s3",
      [REQUEST_LABEL]: "old-backup",
    }),
  ];
  const secondaryStorageConfigs: SecondaryStorageConfig[] = [
    {
      apiVersion: CORE,
      kind: "SecondaryStorageConfig",
      metadata: {
        namespace: "team-a",
        name: "orders-es",
        ownerReferences: [
          {
            apiVersion: CORE,
            kind: "ElasticsearchCluster",
            name: "orders-es",
            uid: "0b6f",
            controller: true,
          },
        ],
      },
      spec: {
        type: "elasticsearch",
        elasticsearch: {
          endpoint: "https://orders-es-es-http.team-a:9200",
          credentialsSecretRef: { name: "orders-es-es-elastic-user" },
          caSecretRef: {
            name: "orders-es-es-http-certs-public",
            key: "ca.crt",
          },
          snapshotRepository: "orders-es",
        },
      },
      status: healthy,
    },
    {
      apiVersion: CORE,
      kind: "SecondaryStorageConfig",
      metadata: { namespace: "team-a", name: "shared-es" },
      spec: {
        type: "elasticsearch",
        elasticsearch: {
          endpoint: "https://search.example.com:9243",
          nodeCount: 3,
          credentialsSecretRef: {
            name: "shared-es-user",
            usernameKey: "user",
            passwordKey: "pass",
          },
          caSecretRef: { name: "shared-es-ca", key: "ca.crt" },
        },
      },
      status: healthy,
    },
  ];
  const databaseServerConfigs: DatabaseServerConfig[] = [
    {
      apiVersion: CORE,
      kind: "DatabaseServerConfig",
      metadata: { namespace: "team-a", name: "postgres" },
      spec: {
        engine: "postgres",
        host: "pg.team-a",
        port: 5432,
        adminCredentialsSecretRef: { name: "postgres-admin" },
      },
      status: {
        conditions: ready(
          "False",
          "ConnectionFailed",
          "Connecting to pg.team-a:5432: connection refused",
        ),
      },
    },
  ];
  const managementAuthConfigs: ManagementAuthConfig[] = [
    {
      apiVersion: CORE,
      kind: "ManagementAuthConfig",
      metadata: {
        name: "plane-a-optimize",
        labels: {
          [MANAGEMENT_CLUSTER_LABEL]: "plane-a",
          [MANAGEMENT_CLUSTER_NAMESPACE_LABEL]: "platform",
        },
      },
      spec: {
        issuerUrl: "https://id.example.com/auth/realms/camunda-platform",
        authUrl:
          "https://id.example.com/auth/realms/camunda-platform/protocol/openid-connect/auth",
        tokenUrl:
          "https://id.example.com/auth/realms/camunda-platform/protocol/openid-connect/token",
        jwksUrl:
          "https://id.example.com/auth/realms/camunda-platform/protocol/openid-connect/certs",
        baseUrl: "https://id.example.com/identity",
        clientId: "optimize",
        audience: "optimize-api",
        clientSecretRef: {
          namespace: "platform",
          name: "plane-a-optimize-client",
          key: "client-secret",
        },
      },
      status: healthy,
    },
  ];
  const requests = [
    {
      apiVersion: "plugins.camunda.io/v1alpha1",
      kind: "PluginRequest",
      metadata: { namespace: "team-a", name: "orders-backup" },
      spec: {},
    } as unknown as PluginRequest,
  ];
  const optimizes = [
    {
      apiVersion: CORE,
      kind: "CamundaOptimize",
      metadata: { namespace: "team-a", name: "optimize" },
      spec: { managementAuthRef: "plane-a-optimize" },
      status: healthy,
    } as unknown as CamundaOptimize,
  ];
  return {
    namespaces: [
      {
        metadata: {
          name: "team-a",
          labels: { [ENVIRONMENT_LABEL]: "production" },
        },
      },
    ],
    environments: [
      {
        apiVersion: "fleet.camunda.io/v1alpha1",
        kind: "Environment",
        metadata: { name: "production" },
        spec: {},
      },
    ],
    clusters: [
      cluster(
        "orders",
        { storageRef: "shared-es", backupStorageRef: "backups" },
        healthy,
      ),
      cluster(
        "billing",
        { storageRef: "shared-es" },
        {
          conditions: ready(
            "False",
            "StorageAlreadyAttached",
            "Another cluster holds this storage backend: team-a/orders",
          ),
        },
      ),
    ],
    clusterClaims: [],
    elasticsearchClusters: [],
    elasticsearchPresets: [],
    databaseServers: [],
    databasePresets: [],
    databases: [],
    databaseConfigs: [
      databaseConfig("orders-db", "orders"),
      databaseConfig("billing-db", "billing"),
    ],
    secondaryStorageConfigs,
    optimizes,
    planes: [],
    elasticsearchBackups: [],
    rdbmsBackups: [],
    pointInTimeRestores: [],
    objectStorageConfigs,
    databaseServerConfigs,
    managementAuthConfigs,
    requests,
    planeClaims: [],
    registrations: [],
  };
}
