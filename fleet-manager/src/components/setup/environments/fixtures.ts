import type { V1Namespace } from "@kubernetes/client-node";
import type { CamundaCluster } from "@/generated/crds/camundacluster";
import type { CamundaClusterClaim } from "@/generated/crds/camundaclusterclaim";
import type { CamundaClusterPreset } from "@/generated/crds/camundaclusterpreset";
import type { CamundaManagementClusterClaim } from "@/generated/crds/camundamanagementclusterclaim";
import type { CamundaPlatformConfig } from "@/generated/crds/camundaplatformconfig";
import type { CamundaRelease } from "@/generated/crds/camundarelease";
import type { Environment } from "@/generated/crds/environment";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import type { PluginSection } from "@/generated/crds/pluginsection";
import { ENVIRONMENT_LABEL } from "@/lib/labels";
import type { EnvironmentSources } from "./sources";

const SINCE = "2026-11-10T09:00:00Z";
const FLEET = "fleet.camunda.io/v1alpha1";
const PLUGINS = "plugins.camunda.io/v1alpha1";

function ready(status: "True" | "False", reason: string, message = "") {
  return {
    type: "Ready",
    status,
    reason,
    message,
    lastTransitionTime: SINCE,
  };
}

function environment(
  name: string,
  spec: Environment["spec"],
  status?: Environment["status"],
): Environment {
  return {
    apiVersion: FLEET,
    kind: "Environment",
    metadata: { name, resourceVersion: "12" },
    spec,
    ...(status && { status }),
  };
}

function namespace(name: string, env?: string): V1Namespace {
  return {
    apiVersion: "v1",
    kind: "Namespace",
    metadata: {
      name,
      resourceVersion: "7",
      ...(env && { labels: { [ENVIRONMENT_LABEL]: env } }),
    },
  };
}

function section(
  name: string,
  label: string,
  appliesTo: PluginSection["spec"]["appliesTo"],
  order: number,
): PluginSection {
  return {
    apiVersion: PLUGINS,
    kind: "PluginSection",
    metadata: { name },
    spec: { label, appliesTo, order, selection: "One" },
  };
}

function registration(
  name: string,
  label: string,
  sections: string[],
): PluginRegistration {
  return {
    apiVersion: PLUGINS,
    kind: "PluginRegistration",
    metadata: { name },
    spec: {
      label,
      sections,
      mode: "Optional",
      deletionPolicies: ["Delete", "Orphan"],
      serviceAccount: { name: "camunda-operator", namespace: "camunda-system" },
    },
    status: {
      conditions: [
        { ...ready("True", "Accepted"), type: "Accepted" },
        ready("True", "Ready"),
      ],
    },
  };
}

function clusterClaim(
  ns: string,
  name: string,
  spec: Partial<CamundaClusterClaim["spec"]>,
): CamundaClusterClaim {
  return {
    apiVersion: FLEET,
    kind: "CamundaClusterClaim",
    metadata: { namespace: ns, name },
    spec: { presetRef: "small", ...spec },
    status: { resolvedPolicy: { deletionPolicy: "Delete" } },
  };
}

/**
 * environmentFixtures returns the sources of the Environments page in the
 * stories and the component tests: the environments production, staging
 * (no status yet), and platform (the plane sections access and
 * management-database), the namespaces legacy (a cluster without an
 * environment) and sandbox (empty, without an environment), and the
 * objects they name.
 */
export function environmentFixtures(): EnvironmentSources {
  return {
    environments: [
      environment(
        "production",
        {
          displayName: "Production",
          color: "#C62828",
          platformConfigRef: "shared",
          presets: { camundaCluster: ["small", "large"] },
          releases: { allowed: ["8.9.22", "8.9.23"], default: "8.9.23" },
          deletionPolicy: "Delete",
        },
        { namespaces: 2, conditions: [ready("True", "Ready")] },
      ),
      environment("staging", {
        displayName: "Staging",
        color: "#F9A825",
        presets: { camundaCluster: ["small"] },
        releases: { allowed: ["8.9.23"], default: "8.9.23" },
        namespaceCreation: {
          allowed: true,
          labels: { team: "payments" },
          requiredLabelKeys: ["cost-center"],
        },
      }),
      environment(
        "platform",
        {
          displayName: "Platform",
          color: "#1565C0",
          platformConfigRef: "shared",
          sections: {
            access: {
              allowed: ["gateway-internal"],
              default: "gateway-internal",
            },
            "management-database": {
              required: true,
              allowed: ["management-database-server"],
              default: "management-database-server",
            },
          },
          deletionPolicy: "Orphan",
        },
        { namespaces: 1, conditions: [ready("True", "Ready")] },
      ),
    ],
    namespaces: [
      namespace("team-a", "production"),
      namespace("team-b", "production"),
      namespace("team-c", "staging"),
      namespace("platform", "platform"),
      namespace("legacy"),
      namespace("sandbox"),
    ],
    clusterClaims: [
      clusterClaim("team-a", "orders", { releaseRef: "8.9.22" }),
      clusterClaim("team-a", "billing", {
        presetRef: "large",
        releaseRef: "8.9.23",
      }),
    ],
    planeClaims: [
      {
        apiVersion: FLEET,
        kind: "CamundaManagementClusterClaim",
        metadata: { namespace: "platform", name: "plane-a" },
        spec: { environments: ["production", "staging"] },
        status: { resolvedPolicy: { deletionPolicy: "Orphan" } },
      } as unknown as CamundaManagementClusterClaim,
    ],
    clusters: [
      {
        apiVersion: "core.camunda.io/v1",
        kind: "CamundaCluster",
        metadata: { namespace: "legacy", name: "old-orders" },
        spec: {},
      } as CamundaCluster,
    ],
    planes: [],
    sections: [
      section(
        "access",
        "Access",
        ["CamundaClusterClaim", "CamundaManagementClusterClaim"],
        10,
      ),
      section(
        "management-database",
        "Management database",
        ["CamundaManagementClusterClaim"],
        20,
      ),
      section("backup-storage", "Backup storage", ["CamundaClusterClaim"], 30),
    ],
    registrations: [
      registration("gateway-internal", "Internal gateway", ["access"]),
      registration("management-database-server", "PostgreSQL in the cluster", [
        "management-database",
      ]),
      registration("rustfs", "RustFS", ["backup-storage"]),
    ],
    requests: [],
    clusterPresets: (["small", "large"] as const).map(
      (name): CamundaClusterPreset =>
        ({
          apiVersion: "core.camunda.io/v1",
          kind: "CamundaClusterPreset",
          metadata: { name },
          spec: {},
        }) as CamundaClusterPreset,
    ),
    elasticsearchPresets: [],
    databasePresets: [],
    releases: ["8.9.22", "8.9.23"].map(
      (name): CamundaRelease =>
        ({
          apiVersion: "core.camunda.io/v1",
          kind: "CamundaRelease",
          metadata: { name },
          spec: { version: name },
        }) as CamundaRelease,
    ),
    platformConfigs: [
      {
        apiVersion: "core.camunda.io/v1",
        kind: "CamundaPlatformConfig",
        metadata: { name: "shared" },
        spec: {},
      } as CamundaPlatformConfig,
    ],
  };
}
