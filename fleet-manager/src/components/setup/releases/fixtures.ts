import type { V1Namespace } from "@kubernetes/client-node";
import type { CamundaCluster } from "@/generated/crds/camundacluster";
import type { CamundaClusterClaim } from "@/generated/crds/camundaclusterclaim";
import type {
  CamundaRelease,
  CamundaReleaseSpec,
} from "@/generated/crds/camundarelease";
import type { ElasticsearchCluster } from "@/generated/crds/elasticsearchcluster";
import type { Environment } from "@/generated/crds/environment";
import { ENVIRONMENT_LABEL } from "@/lib/labels";
import type { ImagesTable } from "@/startup/bundled-data";
import type { ReleasesData } from "./sources";

/** release returns a CamundaRelease named name with spec, for stories and tests. */
export function release(name: string, spec: CamundaReleaseSpec): CamundaRelease {
  return {
    apiVersion: "core.camunda.io/v1",
    kind: "CamundaRelease",
    metadata: {
      name,
      uid: `uid-${name}`,
      resourceVersion: "41",
      labels: { "app.kubernetes.io/name": "camunda-operator" },
    },
    spec,
  };
}

function environment(
  name: string,
  displayName: string,
  releases: { allowed: string[]; default?: string },
): Environment {
  return {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "Environment",
    metadata: { name, resourceVersion: "12" },
    spec: { displayName, releases },
  };
}

function namespace(name: string, env: string): V1Namespace {
  return {
    apiVersion: "v1",
    kind: "Namespace",
    metadata: { name, labels: { [ENVIRONMENT_LABEL]: env } },
  };
}

/** claim returns a CamundaClusterClaim in namespace that names releaseRef. */
export function claim(
  namespace: string,
  name: string,
  releaseRef: string,
): CamundaClusterClaim {
  return {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "CamundaClusterClaim",
    metadata: { namespace, name },
    spec: { presetRef: "small", releaseRef },
  };
}

/**
 * releaseFixtures holds four releases: camunda-8-9-0 is the default of
 * Production and used by a cluster, its claim, and an Elasticsearch cluster;
 * camunda-8-9-1 is unused and allowed nowhere; camunda-8-10-0 is named only
 * by a claim; camunda-8-8-5 is unused and allowed in Staging.
 */
export function releaseFixtures(): ReleasesData {
  return {
    releases: [
      release("camunda-8-9-0", {
        version: "8.9.0",
        connectors: { version: "8.9.7" },
        elasticsearch: { version: "9.2.4" },
        databaseServer: { version: "17" },
        extraEnv: [
          { name: "CAMUNDA_FEATURE_FLAG", value: "true" },
          {
            name: "ORDERS_DB_PASSWORD",
            valueFrom: {
              secretKeyRef: { name: "orders-db", key: "password" },
            },
          },
        ],
        zeebe: {
          extraEnvFrom: [{ configMapRef: { name: "zeebe-tuning" } }],
        },
      }),
      release("camunda-8-9-1", { version: "8.9.1" }),
      release("camunda-8-10-0", { version: "8.10.0" }),
      release("camunda-8-8-5", { version: "8.8.5" }),
    ],
    environments: [
      environment("production", "Production", {
        allowed: ["camunda-8-9-0"],
        default: "camunda-8-9-0",
      }),
      environment("staging", "Staging", {
        allowed: ["camunda-8-9-0", "camunda-8-8-5"],
      }),
    ],
    namespaces: [
      namespace("team-orders", "production"),
      namespace("team-new", "production"),
    ],
    clusters: [
      {
        apiVersion: "core.camunda.io/v1",
        kind: "CamundaCluster",
        metadata: { namespace: "team-orders", name: "orders" },
        spec: { releaseRef: "camunda-8-9-0" },
      } as CamundaCluster,
    ],
    elasticsearchClusters: [
      {
        apiVersion: "core.camunda.io/v1",
        kind: "ElasticsearchCluster",
        metadata: { namespace: "team-orders", name: "orders-search" },
        spec: { releaseRef: "camunda-8-9-0" },
      } as ElasticsearchCluster,
    ],
    databaseServers: [],
    claims: [
      claim("team-orders", "orders", "camunda-8-9-0"),
      claim("team-new", "billing", "camunda-8-10-0"),
    ],
  };
}

/** imagesFixture is an images table that holds the minor 8.9 only. */
export const imagesFixture: ImagesTable = {
  schemaVersion: 1,
  minors: {
    "8.9": {
      camunda: { repository: "camunda/camunda", version: "8.9.28" },
      optimize: { repository: "camunda/optimize", version: "8.9.28" },
      connectors: {
        repository: "camunda/connectors-bundle",
        version: "8.9.19",
      },
      elasticsearch: { version: "9.2.8" },
      postgres: {
        repository: "ghcr.io/cloudnative-pg/postgresql",
        version: "17",
      },
      identity: { repository: "camunda/identity", version: "8.9.14" },
    },
  },
};
