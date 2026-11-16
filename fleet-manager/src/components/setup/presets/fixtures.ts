import type { Resource } from "@/lib/k8s/kinds";
import {
  CLAIM_LABEL,
  DESCRIPTION_ANNOTATION,
  ENVIRONMENT_LABEL,
} from "@/lib/labels";
import type { PresetSources } from "./sources";

const CORE = "core.camunda.io/v1";
const FLEET = "fleet.camunda.io/v1alpha1";
const PLUGINS = "plugins.camunda.io/v1alpha1";
const SINCE = "2026-11-10T09:00:00Z";

function preset<
  K extends
    | "CamundaClusterPreset"
    | "ElasticsearchClusterPreset"
    | "DatabaseServerPreset",
>(
  kind: K,
  name: string,
  resourceVersion: string,
  spec: Resource<K>["spec"],
  metadata: Partial<Resource<K>["metadata"]> = {},
): Resource<K> {
  return {
    apiVersion: CORE,
    kind,
    metadata: { name, resourceVersion, uid: `uid-${name}`, ...metadata },
    spec,
  } as Resource<K>;
}

function section(
  name: string,
  label: string,
  slot: string,
): Resource<"PluginSection"> {
  return {
    apiVersion: PLUGINS,
    kind: "PluginSection",
    metadata: { name },
    spec: {
      label,
      slot,
      appliesTo: ["CamundaClusterClaim"],
      selection: "One",
    },
  } as Resource<"PluginSection">;
}

/**
 * presetFixtures returns the sources of the preset pages for the stories and
 * the component tests. The cluster preset medium is used by the cluster
 * team-orders/orders and its claim and is allowed in production. Every
 * other preset is allowed nowhere. spare, the Elasticsearch preset
 * standard, and the PostgreSQL preset standard are unused. The PostgreSQL
 * preset small is named only by the selection of the plane claim
 * platform/plane.
 */
export function presetFixtures(): PresetSources {
  return {
    clusterPresets: [
      preset(
        "CamundaClusterPreset",
        "medium",
        "10",
        {
          cluster: {
            podLabels: { "company.com/team": "automation-ops" },
            zeebe: {
              replicas: 3,
              partitions: 3,
              resources: {
                requests: { cpu: "1", memory: "2Gi" },
                limits: { cpu: "2", memory: "4Gi" },
              },
              storageClassName: "ssd",
              storageSize: "32Gi",
              extraEnv: [
                {
                  name: "JAVA_TOOL_OPTIONS",
                  value: "-XX:+ExitOnOutOfMemoryError",
                },
              ],
            },
            auth: { basic: { adminEmail: "admin@example.com" } },
          },
        },
        {
          annotations: {
            [DESCRIPTION_ANNOTATION]:
              "For production workloads with three brokers and replication.",
          },
          managedFields: [
            {
              manager: "kubectl-client-side-apply",
              operation: "Update",
              time: "2026-11-01T08:00:00Z",
            },
            {
              manager: "fleet-manager",
              operation: "Apply",
              time: "2026-11-14T09:30:00Z",
            },
          ],
        },
      ),
      preset("CamundaClusterPreset", "spare", "11", {
        cluster: { zeebe: { replicas: 1, storageSize: "8Gi" } },
      }),
    ],
    elasticsearchPresets: [
      preset(
        "ElasticsearchClusterPreset",
        "standard",
        "12",
        {
          cluster: {
            replicas: 3,
            resources: { requests: { cpu: "1", memory: "4Gi" } },
            storageSize: "64Gi",
          },
        },
        { uid: "uid-es" },
      ),
    ],
    databasePresets: [
      preset("DatabaseServerPreset", "small", "13", {
        server: { instances: 1, storageSize: "10Gi" },
      }),
      preset(
        "DatabaseServerPreset",
        "standard",
        "14",
        {
          server: {
            instances: 2,
            resources: { requests: { cpu: "500m", memory: "1Gi" } },
            storageSize: "20Gi",
            walStorageSize: "5Gi",
          },
        },
        { uid: "uid-pg" },
      ),
    ],
    environments: [
      {
        apiVersion: FLEET,
        kind: "Environment",
        metadata: { name: "production", resourceVersion: "20" },
        spec: {
          displayName: "Production",
          presets: { camundaCluster: ["medium"] },
        },
      },
    ],
    namespaces: [
      {
        apiVersion: "v1",
        kind: "Namespace",
        metadata: {
          name: "team-orders",
          labels: { [ENVIRONMENT_LABEL]: "production" },
        },
      },
      { apiVersion: "v1", kind: "Namespace", metadata: { name: "platform" } },
    ],
    clusters: [
      {
        apiVersion: CORE,
        kind: "CamundaCluster",
        metadata: {
          namespace: "team-orders",
          name: "orders",
          labels: { [CLAIM_LABEL]: "orders" },
        },
        spec: {
          presetRef: "medium",
          releaseRef: "camunda-8-9-0",
          platformConfigRef: "shared",
          zeebe: { replicas: 1, storageClassName: "encrypted" },
        },
        status: {
          conditions: [
            {
              type: "Ready",
              status: "True",
              reason: "Ready",
              message: "",
              lastTransitionTime: SINCE,
            },
          ],
        },
      } as Resource<"CamundaCluster">,
    ],
    elasticsearchClusters: [],
    databaseServers: [],
    clusterClaims: [
      {
        apiVersion: FLEET,
        kind: "CamundaClusterClaim",
        metadata: { namespace: "team-orders", name: "orders" },
        spec: {
          presetRef: "medium",
          plugins: [
            {
              section: "secondary-storage",
              registration: "existing-secondary-storage",
              parameters: { name: "orders-storage" },
            },
            { section: "volume-encryption", registration: "aws-ebs-kms" },
          ],
        },
      } as Resource<"CamundaClusterClaim">,
    ],
    planeClaims: [
      {
        apiVersion: FLEET,
        kind: "CamundaManagementClusterClaim",
        metadata: { namespace: "platform", name: "plane" },
        spec: {
          plugins: [
            {
              section: "management-database",
              registration: "management-database-server",
              parameters: { preset: "small" },
            },
          ],
        },
      } as Resource<"CamundaManagementClusterClaim">,
    ],
    requests: [],
    sections: [
      section("secondary-storage", "Secondary storage", "secondaryStorage"),
      section("volume-encryption", "Volume encryption", "volumeEncryption"),
    ],
    registrations: [],
  };
}
