import type { V1Namespace } from "@kubernetes/client-node";
import type { CamundaCluster } from "@/generated/crds/camundacluster";
import type { CamundaClusterClaim } from "@/generated/crds/camundaclusterclaim";
import type { CamundaManagementCluster } from "@/generated/crds/camundamanagementcluster";
import type { CamundaPlatformConfig } from "@/generated/crds/camundaplatformconfig";
import type { DatabaseServer } from "@/generated/crds/databaseserver";
import type { DatabaseServerPreset } from "@/generated/crds/databaseserverpreset";
import type { Environment } from "@/generated/crds/environment";
import { parseImagesTable, type ImagesTable } from "@/startup/bundled-data";
import type { PlatformSources } from "./sources";

const SINCE = "2026-11-10T09:00:00Z";
const CORE = "core.camunda.io/v1";

function ready(status: "True" | "False", reason: string, message = "") {
  return {
    type: "Ready",
    status,
    reason,
    message,
    lastTransitionTime: SINCE,
  };
}

function config(
  name: string,
  spec: CamundaPlatformConfig["spec"],
  condition: ReturnType<typeof ready>,
): CamundaPlatformConfig {
  return {
    apiVersion: CORE,
    kind: "CamundaPlatformConfig",
    metadata: { name, uid: `uid-${name}`, resourceVersion: "21" },
    spec,
    status: { conditions: [condition] },
  };
}

function namespaced<T>(
  kind: string,
  namespace: string,
  name: string,
  spec: object,
  status?: object,
): T {
  return {
    apiVersion: CORE,
    kind,
    metadata: { namespace, name },
    spec,
    ...(status && { status }),
  } as unknown as T;
}

function namespace(name: string): V1Namespace {
  return { apiVersion: "v1", kind: "Namespace", metadata: { name } };
}

function environment(name: string, platformConfigRef: string): Environment {
  return {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "Environment",
    metadata: { name },
    spec: { platformConfigRef },
  } as Environment;
}

/**
 * platformFixtures are the sources of the Platform pages: oidc-prod with a
 * user of each kind, basic-lab with no user, and broken with a missing
 * license Secret and one Environment that defaults to it.
 */
export function platformFixtures(): PlatformSources {
  return {
    platformConfigs: [
      config(
        "oidc-prod",
        {
          auth: {
            method: "oidc",
            oidc: {
              issuerUrl: "https://login.example.com/realms/camunda",
              clientId: "orchestration",
              clientSecretRef: {
                namespace: "camunda-system",
                name: "oidc-credentials",
                key: "client-secret",
              },
              management: {
                clients: {
                  identity: {
                    clientId: "management-identity",
                    clientSecretRef: {
                      namespace: "camunda-system",
                      name: "oidc-credentials",
                      key: "identity-secret",
                    },
                  },
                  console: { clientId: "console" },
                },
              },
            },
          },
          licenseSecretRef: {
            namespace: "camunda-system",
            name: "camunda-license",
            key: "license-key",
          },
          images: { camunda: "registry.example.com/camunda/camunda" },
        },
        ready("True", "Healthy"),
      ),
      config("basic-lab", {}, ready("True", "Healthy")),
      config(
        "broken",
        {
          auth: { method: "basic" },
          licenseSecretRef: {
            namespace: "camunda-system",
            name: "camunda-license",
            key: "license-key",
          },
        },
        ready(
          "False",
          "MissingSecret",
          "spec.licenseSecretRef: Secret camunda-system/camunda-license not found",
        ),
      ),
    ],
    environments: [
      environment("production", "oidc-prod"),
      environment("staging", "broken"),
    ],
    namespaces: ["camunda-system", "platform", "team-a", "team-b"].map(
      namespace,
    ),
    clusters: [
      namespaced<CamundaCluster>(
        "CamundaCluster",
        "team-a",
        "orders",
        { platformConfigRef: "oidc-prod" },
        { management: { version: "8.9.3" } },
      ),
    ],
    planes: [
      namespaced<CamundaManagementCluster>(
        "CamundaManagementCluster",
        "platform",
        "main",
        { platformConfigRef: "oidc-prod", identity: { version: "8.9.14" } },
      ),
    ],
    databaseServers: [
      namespaced<DatabaseServer>("DatabaseServer", "team-a", "orders-db", {
        presetRef: "pg-standard",
      }),
    ],
    clusterPresets: [],
    databasePresets: [
      {
        apiVersion: "fleet.camunda.io/v1alpha1",
        kind: "DatabaseServerPreset",
        metadata: { name: "pg-standard" },
        spec: { server: { platformConfigRef: "oidc-prod" } },
      } as unknown as DatabaseServerPreset,
    ],
    releases: [],
    clusterClaims: [
      namespaced<CamundaClusterClaim>(
        "CamundaClusterClaim",
        "team-b",
        "billing",
        { platformConfigRef: "oidc-prod", presetRef: "small" },
      ),
    ],
    planeClaims: [],
  };
}

/** withNextMinor adds the cluster team-c/next on Camunda 8.10.1 to oidc-prod. */
export function withNextMinor(sources: PlatformSources): PlatformSources {
  return {
    ...sources,
    clusters: [
      ...sources.clusters,
      namespaced<CamundaCluster>(
        "CamundaCluster",
        "team-c",
        "next",
        { platformConfigRef: "oidc-prod" },
        { management: { version: "8.10.1" } },
      ),
    ],
  };
}

/**
 * twoMinorTable holds Camunda 8.9 and 8.10. From 8.10 the Web Modeler
 * images move to camunda/hub and camunda/hub-websockets.
 */
export const twoMinorTable: ImagesTable = parseImagesTable({
  schemaVersion: 1,
  minors: {
    "8.9": {
      camunda: { repository: "camunda/camunda", version: "8.9.28" },
      connectors: {
        repository: "camunda/connectors-bundle",
        version: "8.9.19",
      },
      optimize: { repository: "camunda/optimize", version: "8.9.28" },
      identity: { repository: "camunda/identity", version: "8.9.14" },
      console: { repository: "camunda/console", version: "8.9.133" },
      "web-modeler-restapi": {
        repository: "camunda/web-modeler-restapi",
        version: "8.9.15",
      },
      "web-modeler-websockets": {
        repository: "camunda/web-modeler-websockets",
        version: "8.9.15",
      },
      keycloak: {
        repository: "camunda/keycloak",
        tagPrefix: "quay-optimized-",
        version: "26.6.4",
      },
      postgres: {
        repository: "ghcr.io/cloudnative-pg/postgresql",
        version: "17",
      },
      elasticsearch: { version: "9.2.8" },
    },
    "8.10": {
      camunda: { repository: "camunda/camunda", version: "8.10.2" },
      connectors: {
        repository: "camunda/connectors-bundle",
        version: "8.10.1",
      },
      optimize: { repository: "camunda/optimize", version: "8.10.2" },
      identity: { repository: "camunda/identity", version: "8.10.1" },
      console: { repository: "camunda/console", version: "8.10.1" },
      "web-modeler-restapi": { repository: "camunda/hub", version: "8.10.1" },
      "web-modeler-websockets": {
        repository: "camunda/hub-websockets",
        version: "8.10.1",
      },
      keycloak: {
        repository: "camunda/keycloak",
        tagPrefix: "quay-optimized-",
        version: "26.6.4",
      },
      postgres: {
        repository: "ghcr.io/cloudnative-pg/postgresql",
        version: "17",
      },
      elasticsearch: { version: "9.2.8" },
    },
  },
});
