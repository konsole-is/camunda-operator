import type { PendingPush } from "@/components/fleet/git-pushes";
import { planesOf } from "@/components/planes/plane";
import type { Story } from "@/gallery/registry";
import type { Resource } from "@/lib/k8s/kinds";
import { CLAIM_LABEL, ENVIRONMENT_LABEL } from "@/lib/labels";
import { ClusterTable } from "./cluster-table";
import { ComponentDots } from "./component-dots";
import { FleetView } from "./fleet-view";
import { type FleetSources, clusterRows } from "./model";

const NOW = "2026-11-10T12:00:00Z";

type Condition = {
  type: string;
  status: string;
  reason: string;
  message?: string;
};

function cluster(
  namespace: string,
  name: string,
  {
    managed = true,
    spec = {},
    conditions = [
      { type: "Ready", status: "True", reason: "Healthy" },
      { type: "ZeebeReady", status: "True", reason: "Healthy" },
      { type: "GatewayReady", status: "True", reason: "Healthy" },
      { type: "ConnectorsReady", status: "True", reason: "Healthy" },
    ],
    version = "8.9.23",
  }: {
    managed?: boolean;
    spec?: Record<string, unknown>;
    conditions?: Condition[];
    version?: string;
  } = {},
) {
  return {
    apiVersion: "core.camunda.io/v1",
    kind: "CamundaCluster",
    metadata: {
      namespace,
      name,
      ...(managed ? { labels: { [CLAIM_LABEL]: name } } : {}),
    },
    spec: {
      presetRef: "medium",
      gateway: { mode: "Standalone" },
      connectors: { enabled: true },
      ...spec,
    },
    status: { conditions, management: { version } },
  } as unknown as Resource<"CamundaCluster">;
}

function claim(namespace: string, name: string, conditions?: Condition[]) {
  return {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "CamundaClusterClaim",
    metadata: { namespace, name },
    spec: {
      presetRef: "medium",
      plugins: [
        { section: "secondary-storage", registration: "elasticsearch" },
      ],
    },
    status: { conditions },
  } as unknown as Resource<"CamundaClusterClaim">;
}

function environment(name: string, displayName: string, color: string) {
  return {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "Environment",
    metadata: { name },
    spec: { displayName, color },
  } as unknown as Resource<"Environment">;
}

function namespace(name: string, env?: string): Resource<"Namespace"> {
  return {
    apiVersion: "v1",
    kind: "Namespace",
    metadata: {
      name,
      ...(env ? { labels: { [ENVIRONMENT_LABEL]: env } } : {}),
    },
  };
}

function backup(namespace: string, clusterName: string, phase: string) {
  return {
    kind: "LogicalBackupElasticsearch",
    metadata: {
      namespace,
      name: `${clusterName}-nightly`,
      creationTimestamp: "2026-11-10T02:00:00Z",
    },
    spec: { clusterRef: { name: clusterName } },
    status: { phase, completionTime: "2026-11-10T02:12:00Z" },
  } as unknown as FleetSources["backups"][number];
}

const empty: FleetSources = {
  clusters: [],
  claims: [],
  environments: [],
  namespaces: [],
  presets: [],
  releases: [],
  sections: [],
  registrations: [],
  requests: [],
  storage: [],
  optimizes: [],
  backups: [],
};

const shared: Partial<FleetSources> = {
  sections: [
    {
      kind: "PluginSection",
      metadata: { name: "secondary-storage" },
      spec: {
        appliesTo: ["CamundaClusterClaim"],
        label: "Secondary storage",
        order: 50,
        selection: "One",
      },
    } as Resource<"PluginSection">,
  ],
  registrations: [
    {
      kind: "PluginRegistration",
      metadata: { name: "elasticsearch" },
      spec: { label: "New Elasticsearch cluster" },
    } as unknown as Resource<"PluginRegistration">,
  ],
};

const fleet: FleetSources = {
  ...empty,
  ...shared,
  clusters: [
    cluster("team-orders", "orders"),
    cluster("team-orders", "returns", {
      conditions: [
        {
          type: "Ready",
          status: "False",
          reason: "Degraded",
          message: "1 of 3 brokers is not ready",
        },
        {
          type: "ZeebeReady",
          status: "False",
          reason: "Degraded",
          message: "1 of 3 brokers is not ready",
        },
        { type: "GatewayReady", status: "True", reason: "Healthy" },
        { type: "ConnectorsReady", status: "Unknown", reason: "Updating" },
      ],
    }),
    cluster("team-billing", "billing", {
      managed: false,
      spec: { connectors: { enabled: false } },
    }),
    cluster("team-sandbox", "sandbox", {
      spec: { suspend: true },
      conditions: [{ type: "Ready", status: "True", reason: "Suspended" }],
    }),
    cluster("scratch", "playground", { managed: false }),
  ],
  claims: [
    claim("team-orders", "orders"),
    claim("team-orders", "returns"),
    claim("team-sandbox", "sandbox"),
    claim("team-search", "search", [
      {
        type: "Ready",
        status: "False",
        reason: "NotAllowed",
        message: "The Environment does not allow the preset large",
      },
    ]),
  ],
  environments: [
    environment("production", "Production", "#3dd68c"),
    environment("staging", "Staging", "#ffc14d"),
    environment("development", "Development", "#5b9bff"),
  ],
  namespaces: [
    namespace("team-orders", "production"),
    namespace("team-billing", "production"),
    namespace("team-search", "staging"),
    namespace("team-sandbox", "development"),
    namespace("scratch"),
  ],
  backups: [
    backup("team-orders", "orders", "Completed"),
    backup("team-billing", "billing", "Failed"),
  ],
};

const planes = planesOf({
  clusters: [
    {
      apiVersion: "core.camunda.io/v1",
      kind: "CamundaManagementCluster",
      metadata: {
        namespace: "platform",
        name: "plane-production",
        labels: { [CLAIM_LABEL]: "plane-production" },
      },
      spec: { identityProvider: { keycloak: {} }, console: {} },
      status: {
        conditions: [{ type: "Ready", status: "True", reason: "Healthy" }],
        clusters: [
          { namespace: "team-orders", name: "orders", attached: true },
          { namespace: "team-orders", name: "returns", attached: true },
        ],
      },
    } as never,
  ],
  claims: [
    {
      apiVersion: "fleet.camunda.io/v1alpha1",
      kind: "CamundaManagementClusterClaim",
      metadata: { namespace: "platform", name: "plane-production" },
      spec: {
        environments: ["production"],
        management: { identityProvider: { keycloak: {} }, console: {} },
      },
    } as never,
  ],
  namespaces: fleet.namespaces,
  environments: fleet.environments,
});

const pushes: PendingPush[] = [
  {
    key: "CamundaClusterClaim/team-search/payments",
    ref: {
      kind: "CamundaClusterClaim",
      namespace: "team-search",
      name: "payments",
    },
    path: "clusters/team-search/payments.yaml",
    revision: "3f2c1a",
    pushedAt: "2026-11-10T11:58:00Z",
  },
];

export const fleetViewStories: Story[] = [
  {
    component: "FleetView",
    name: "Environments",
    render: () => (
      <FleetView sources={fleet} planes={planes} pushes={[]} now={NOW} />
    ),
  },
  {
    component: "FleetView",
    name: "No environments",
    render: () => (
      <FleetView
        sources={{ ...fleet, environments: [] }}
        planes={[]}
        pushes={[]}
        now={NOW}
      />
    ),
  },
  {
    component: "FleetView",
    name: "Waiting for GitOps",
    render: () => (
      <FleetView sources={fleet} planes={planes} pushes={pushes} now={NOW} />
    ),
  },
  {
    component: "FleetView",
    name: "Empty fleet",
    render: () => (
      <FleetView sources={empty} planes={[]} pushes={[]} now={NOW} />
    ),
  },
  {
    component: "ComponentDots",
    name: "Open",
    render: () => (
      <ComponentDots
        dots={
          clusterRows(fleet, NOW).find((r) => r.name === "returns")!.components
        }
        defaultOpen
      />
    ),
  },
  {
    component: "ClusterTable",
    name: "Mixed states",
    render: () => (
      <ClusterTable label="Production" rows={clusterRows(fleet, NOW)} />
    ),
  },
];
