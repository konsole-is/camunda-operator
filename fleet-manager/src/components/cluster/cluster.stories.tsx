import type { CamundaClusterSpec } from "@/generated/crds/camundacluster";
import type { Story } from "@/gallery/registry";
import type { Resource } from "@/lib/k8s/kinds";
import { CLAIM_LABEL, COMPONENT_LABEL } from "@/lib/labels";
import { mergeCluster } from "@/lib/merge";
import { ActivityBanner, UnmanagedBanner } from "./banners";
import { claimView } from "./claim";
import { ClaimTab } from "./claim-tab";
import { ClusterHeader } from "./cluster-header";
import { endpointRows } from "./endpoints";
import { EndpointsTab } from "./endpoints-tab";
import { launchLinks } from "./launch";
import { topologyOf } from "./topology";
import { TopologyView } from "./topology-view";

function pod(
  name: string,
  component: string,
  status: Resource<"Pod">["status"],
  terminating = false,
): Resource<"Pod"> {
  return {
    kind: "Pod",
    metadata: {
      name,
      labels: { [COMPONENT_LABEL]: component },
      deletionTimestamp: terminating ? "2026-10-18T10:00:00Z" : undefined,
    },
    status,
  } as Resource<"Pod">;
}

const ready = {
  phase: "Running",
  conditions: [{ type: "Ready", status: "True" }],
};
const waiting = (reason: string) => ({
  phase: "Pending",
  containerStatuses: [
    {
      name: "zeebe",
      image: "camunda/zeebe",
      imageID: "",
      ready: false,
      restartCount: 4,
      state: { waiting: { reason } },
    },
  ],
});

const route = (
  name: string,
  pathPrefix: string,
  service = "orders-gateway",
) => ({
  name,
  pathPrefix,
  port: 8080,
  protocol: "HTTP" as const,
  service,
});

const storyCluster = {
  kind: "CamundaCluster",
  metadata: { namespace: "team-a", name: "orders" },
  spec: { presetRef: "medium", externalUrl: "https://orders.example.com" },
  status: {
    endpoints: [
      route("admin", "/admin"),
      route("connectors", "/inbound", "orders-connectors"),
      route("default", "/"),
      {
        name: "grpc",
        port: 26500,
        protocol: "GRPC",
        service: "orders-gateway",
      },
      route("operate", "/operate"),
      route("rest", "/v2"),
      route("tasklist", "/tasklist"),
    ],
  },
} as Resource<"CamundaCluster">;

const storyOptimize = {
  kind: "CamundaOptimize",
  metadata: { namespace: "team-a", name: "orders-optimize" },
  spec: {
    clusterRef: { name: "orders" },
    externalUrl: "https://orders.example.com/optimize",
  },
  status: {
    endpoints: [route("optimize", "/optimize", "orders-optimize-webapp")],
  },
} as unknown as Resource<"CamundaOptimize">;

const storyNow = "2026-10-30T12:00:00Z";
const storyCondition = (
  type: string,
  status: "True" | "False",
  reason: string,
  message: string,
) => ({ type, status, reason, message, lastTransitionTime: storyNow });

function storyClaim(
  conditions: ReturnType<typeof storyCondition>[],
  outputsReady: string,
): Resource<"CamundaClusterClaim"> {
  return {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "CamundaClusterClaim",
    metadata: { namespace: "team-a", name: "orders" },
    spec: {
      presetRef: "medium",
      deletionPolicy: "Orphan",
      plugins: [
        { section: "cloud-identity", registration: "aws-identity" },
        {
          section: "access",
          registration: "gateway-access",
          parameters: { hostname: "orders.example.com" },
        },
      ],
      attachments: {
        optimize: {
          externalUrl: "https://orders.example.com/optimize",
          managementAuthConfigRef: "orders-auth",
        },
        backupSchedule: {
          schedule: "0 2 * * *",
          retained: { completed: 7, failed: 2 },
        },
      },
    },
    status: {
      environment: "prod",
      conditions,
      requests: [
        {
          name: "orders-cloud-identity",
          section: "cloud-identity",
          registration: "aws-identity",
          outputsReady,
          ready: outputsReady,
        },
        {
          name: "orders-access",
          section: "access",
          registration: "gateway-access",
          outputsReady: "False",
          ready: "False",
        },
      ],
      resolvedPolicy: {
        deletionPolicy: "Delete",
        parameters: {
          "aws-identity": { mode: { value: "pod-identity" } },
          "gateway-access": {
            hostname: { value: "orders.example.com", claimEditable: true },
            gatewayClass: { value: "public" },
          },
        },
      },
    },
  } as unknown as Resource<"CamundaClusterClaim">;
}

const storyClaimCluster = {
  ...storyCluster,
  metadata: { ...storyCluster.metadata, labels: { [CLAIM_LABEL]: "orders" } },
  spec: { ...storyCluster.spec, releaseRef: "camunda-8.9" },
} as Resource<"CamundaCluster">;

function claimTabStory(claim: Resource<"CamundaClusterClaim">) {
  return (
    <ClaimTab
      view={claimView({
        claim,
        cluster: storyClaimCluster,
        merged: mergeCluster(undefined, undefined, storyClaimCluster.spec),
        environments: [
          {
            kind: "Environment",
            metadata: { name: "prod" },
            spec: { displayName: "Production" },
          } as unknown as Resource<"Environment">,
        ],
        sections: [
          ["cloud-identity", "Cloud identity", 10],
          ["access", "Access", 50],
        ].map(
          ([name, label, order]) =>
            ({
              kind: "PluginSection",
              metadata: { name },
              spec: { label, order, appliesTo: ["CamundaClusterClaim"] },
            }) as unknown as Resource<"PluginSection">,
        ),
        registrations: [
          ["aws-identity", "AWS identity"],
          ["gateway-access", "Gateway"],
        ].map(
          ([name, label]) =>
            ({
              kind: "PluginRegistration",
              metadata: { name },
              spec: { label, deletionPolicies: ["Delete", "Orphan"] },
            }) as unknown as Resource<"PluginRegistration">,
        ),
      })}
      stored={claim}
      now={storyNow}
    />
  );
}

export const clusterStories: Story[] = [
  {
    component: "ClusterHeader",
    name: "Ready with launch links",
    render: () => (
      <ClusterHeader
        name="orders"
        namespace="team-a"
        ready={{ status: "True", reason: "Healthy" }}
        environment="Production"
        version="8.9.23"
        preset="medium"
        storage="elasticsearch"
        links={launchLinks("https://orders.example.com", undefined)}
      />
    ),
  },
  {
    component: "ClusterHeader",
    name: "Not ready without external URL",
    render: () => (
      <ClusterHeader
        name="payments"
        namespace="team-b"
        ready={{
          status: "False",
          reason: "ZeebeNotReady",
          message: "1 of 3 Zeebe brokers are ready",
        }}
        storage="rdbms"
        links={[]}
      />
    ),
  },
  {
    component: "UnmanagedBanner",
    name: "Default",
    render: () => <UnmanagedBanner />,
  },
  {
    component: "ActivityBanner",
    name: "Backup running",
    render: () => (
      <ActivityBanner
        backup={{
          kind: "LogicalBackupRDBMS",
          namespace: "team-a",
          name: "nightly-1",
        }}
        backupsHref="?tab=backups"
      />
    ),
  },
  {
    component: "ActivityBanner",
    name: "Point-in-time restore running",
    render: () => (
      <ActivityBanner
        restore={{
          kind: "PointInTimeRestore",
          namespace: "team-a",
          name: "before-incident",
        }}
        backupsHref="?tab=backups"
      />
    ),
  },
  {
    component: "TopologyView",
    name: "Standalone gateway",
    render: () => (
      <TopologyView
        boxes={topologyOf({} as CamundaClusterSpec, [
          pod("orders-zeebe-0", "zeebe", ready),
          pod("orders-zeebe-1", "zeebe", ready),
          pod("orders-zeebe-2", "zeebe", ready),
          pod("orders-gateway-7d9f-x2", "gateway", ready),
        ])}
      />
    ),
  },
  {
    component: "TopologyView",
    name: "Embedded gateway",
    render: () => (
      <TopologyView
        boxes={topologyOf(
          {
            gateway: { mode: "Embedded" },
            connectors: { enabled: true },
          } as CamundaClusterSpec,
          [
            pod("orders-zeebe-0", "zeebe", ready),
            pod("orders-connectors-5c-a1", "connectors", ready),
          ],
        )}
      />
    ),
  },
  {
    component: "TopologyView",
    name: "Pods in every state",
    render: () => (
      <TopologyView
        boxes={topologyOf(
          { operate: { mode: "Standalone" } } as CamundaClusterSpec,
          [
            pod("orders-zeebe-0", "zeebe", ready),
            pod("orders-zeebe-1", "zeebe", waiting("CrashLoopBackOff")),
            pod("orders-zeebe-2", "zeebe", waiting("ContainerCreating")),
            pod("orders-gateway-7d9f-x2", "gateway", { phase: "Failed" }),
            pod("orders-gateway-7d9f-y3", "gateway", ready, true),
          ],
        )}
      />
    ),
  },
  {
    component: "EndpointsTab",
    name: "Embedded apps and an Optimize",
    render: () => (
      <EndpointsTab
        rows={endpointRows({
          cluster: storyCluster,
          spec: { connectors: { enabled: true } } as CamundaClusterSpec,
          optimizes: [storyOptimize],
          accessOutputs: { grpcHost: "grpc.orders.example.com:443" },
        })}
      />
    ),
  },
  {
    component: "EndpointsTab",
    name: "No endpoints yet",
    render: () => <EndpointsTab rows={[]} />,
  },
  {
    component: "ClaimTab",
    name: "Waiting for outputs",
    render: () =>
      claimTabStory(
        storyClaim(
          [
            storyCondition(
              "Ready",
              "False",
              "PrerequisiteNotMet",
              "waiting for the outputs of cloud-identity",
            ),
            storyCondition(
              "RequestsReady",
              "False",
              "OperationPending",
              "waiting for the outputs of cloud-identity",
            ),
            storyCondition(
              "ClusterReady",
              "False",
              "Creating",
              "CamundaCluster has not reported Ready yet",
            ),
          ],
          "False",
        ),
      ),
  },
  {
    component: "ClaimTab",
    name: "Healthy with sections",
    render: () =>
      claimTabStory(
        storyClaim(
          [
            storyCondition("Ready", "True", "Healthy", ""),
            storyCondition("RequestsReady", "True", "Healthy", ""),
            storyCondition(
              "ClusterReady",
              "True",
              "Healthy",
              "Healthy: Component is healthy.",
            ),
            storyCondition("AttachmentsReady", "True", "Healthy", ""),
          ],
          "True",
        ),
      ),
  },
];
