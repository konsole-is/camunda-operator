import type { Resource } from "@/lib/k8s/kinds";
import { CLAIM_LABEL, ENVIRONMENT_LABEL } from "@/lib/labels";
import type { RolloutData } from "./rollout";

/** VERSION_SKIP_MESSAGE is the message of the VersionSkipRefused condition of the cluster ledger. */
export const VERSION_SKIP_MESSAGE =
  "The cluster runs 8.7.2. Move it to an 8.8 release before 8.9.1.";

function release(version: string): Resource<"CamundaRelease"> {
  return {
    apiVersion: "core.camunda.io/v1",
    kind: "CamundaRelease",
    metadata: {
      name: `camunda-${version.replaceAll(".", "-")}`,
      resourceVersion: "7",
    },
    spec: { version },
  } as Resource<"CamundaRelease">;
}

/** targetRelease is the release camunda-8-9-1, version 8.9.1, that the rows of rolloutData move to. */
export const targetRelease = release("8.9.1");

function environment(
  name: string,
  displayName: string,
  allowed: string[],
): Resource<"Environment"> {
  return {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "Environment",
    metadata: { name, resourceVersion: "12" },
    spec: {
      displayName,
      presets: { camundaCluster: ["medium"] },
      releases: { allowed, default: "camunda-8-9-0" },
    },
  } as Resource<"Environment">;
}

function namespace(name: string, env: string): Resource<"Namespace"> {
  return {
    apiVersion: "v1",
    kind: "Namespace",
    metadata: { name, labels: { [ENVIRONMENT_LABEL]: env } },
  };
}

type ClusterOptions = {
  version?: string;
  releaseRef?: string;
  managed?: boolean;
  suspend?: boolean;
  conditions?: object[];
};

/**
 * cluster returns the CamundaCluster name in namespace. version is
 * status.management.version, and managed adds the claim label that names
 * a claim of the same name.
 */
export function cluster(
  ns: string,
  name: string,
  {
    version,
    releaseRef = "camunda-8-9-0",
    managed = true,
    suspend,
    conditions,
  }: ClusterOptions = {},
): Resource<"CamundaCluster"> {
  return {
    apiVersion: "core.camunda.io/v1",
    kind: "CamundaCluster",
    metadata: {
      namespace: ns,
      name,
      resourceVersion: "30",
      ...(managed && { labels: { [CLAIM_LABEL]: name } }),
    },
    spec: {
      presetRef: "medium",
      releaseRef,
      zeebe: { replicas: 3 },
      ...(suspend && { suspend }),
    },
    status: {
      ...(version && { management: { version } }),
      ...(conditions && { conditions }),
    },
  } as Resource<"CamundaCluster">;
}

/** clusterClaim returns the CamundaClusterClaim name in namespace at resourceVersion 4, with spec merged over its default. */
export function clusterClaim(
  ns: string,
  name: string,
  spec: object = {},
): Resource<"CamundaClusterClaim"> {
  return {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "CamundaClusterClaim",
    metadata: { namespace: ns, name, resourceVersion: "4" },
    spec: { presetRef: "medium", releaseRef: "camunda-8-9-0", ...spec },
  } as Resource<"CamundaClusterClaim">;
}

/**
 * rolloutData holds one cluster for each eligibility of a move to
 * targetRelease, and the lists that the move checks, with over applied:
 * team-orders/orders can move, team-stage/stage is in Staging, which does
 * not allow the release, team-pay/payments runs 8.9.4, team-pay/ledger
 * runs 8.7.2 and reports VersionSkipRefused, team-pay/billing has the
 * Pending backup billing-nightly, team-legacy/legacy has no claim label,
 * team-pay/paused is suspended, team-pay/fresh reports no version,
 * team-pay/current is on the release, and team-pay/orphan names a claim
 * that does not exist. The claim orders selects the built-in Elasticsearch,
 * which takes the release of the claim, and attaches Optimize. The claim
 * payments holds allowVersionDowngrade 8.9.0.
 */
export function rolloutData(over: Partial<RolloutData> = {}): RolloutData {
  return {
    releases: [
      release("8.9.0"),
      targetRelease,
      release("8.8.0"),
      release("8.11.0"),
    ],
    environments: [
      environment("production", "Production", [
        "camunda-8-9-0",
        "camunda-8-9-1",
      ]),
      environment("staging", "Staging", ["camunda-8-9-0"]),
    ],
    namespaces: [
      namespace("team-orders", "production"),
      namespace("team-pay", "production"),
      namespace("team-stage", "staging"),
      namespace("team-legacy", "production"),
    ],
    clusters: [
      cluster("team-orders", "orders", { version: "8.9.0" }),
      cluster("team-stage", "stage", { version: "8.9.0" }),
      cluster("team-pay", "payments", { version: "8.9.4" }),
      cluster("team-pay", "ledger", {
        version: "8.7.2",
        conditions: [
          {
            type: "Ready",
            status: "False",
            reason: "VersionSkipRefused",
            message: VERSION_SKIP_MESSAGE,
          },
        ],
      }),
      cluster("team-pay", "billing", { version: "8.9.0" }),
      cluster("team-legacy", "legacy", { version: "8.9.0", managed: false }),
      cluster("team-pay", "paused", { suspend: true }),
      cluster("team-pay", "fresh"),
      cluster("team-pay", "current", {
        version: "8.9.1",
        releaseRef: "camunda-8-9-1",
      }),
      cluster("team-pay", "orphan", { version: "8.9.0" }),
    ],
    claims: [
      clusterClaim("team-orders", "orders", {
        plugins: [
          { section: "secondary-storage", registration: "elasticsearch" },
        ],
        attachments: { optimize: {} },
      }),
      clusterClaim("team-stage", "stage"),
      clusterClaim("team-pay", "payments", { allowVersionDowngrade: "8.9.0" }),
      clusterClaim("team-pay", "ledger"),
      clusterClaim("team-pay", "billing"),
      clusterClaim("team-pay", "paused"),
      clusterClaim("team-pay", "fresh"),
      clusterClaim("team-pay", "current", { releaseRef: "camunda-8-9-1" }),
    ],
    clusterPresets: [
      {
        apiVersion: "core.camunda.io/v1",
        kind: "CamundaClusterPreset",
        metadata: { name: "medium" },
        spec: { cluster: {} },
      } as Resource<"CamundaClusterPreset">,
    ],
    sections: [
      {
        apiVersion: "plugins.camunda.io/v1alpha1",
        kind: "PluginSection",
        metadata: { name: "secondary-storage" },
        spec: {
          label: "Secondary storage",
          appliesTo: ["CamundaClusterClaim"],
          selection: "One",
        },
      } as Resource<"PluginSection">,
    ],
    registrations: [
      {
        apiVersion: "plugins.camunda.io/v1alpha1",
        kind: "PluginRegistration",
        metadata: { name: "elasticsearch" },
        spec: {
          label: "Elasticsearch",
          section: "secondary-storage",
          mode: "Default",
          deletionPolicies: ["Delete", "Orphan"],
          parameters: {
            type: "object",
            properties: { releaseRef: { type: "string" } },
          },
        },
      } as unknown as Resource<"PluginRegistration">,
    ],
    requests: [],
    elasticsearchBackups: [
      {
        apiVersion: "core.camunda.io/v1",
        kind: "LogicalBackupElasticsearch",
        metadata: { namespace: "team-pay", name: "billing-nightly" },
        spec: { clusterRef: { name: "billing" } },
        status: { phase: "Pending" },
      } as Resource<"LogicalBackupElasticsearch">,
    ],
    rdbmsBackups: [],
    ...over,
  };
}
