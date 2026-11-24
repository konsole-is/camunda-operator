import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import type { CamundaClusterClaim } from "@/generated/crds/camundaclusterclaim";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import type { PluginRequest } from "@/generated/crds/pluginrequest";
import type { PluginSection } from "@/generated/crds/pluginsection";
import { ClusterProgressView } from "./cluster-progress";

const sections = [
  { metadata: { name: "access" }, spec: { label: "Access", selection: "One" } },
  {
    metadata: { name: "secondary-storage" },
    spec: { label: "Secondary storage", selection: "One" },
  },
  {
    metadata: { name: "encryption-key" },
    spec: { label: "Encryption key", selection: "One" },
  },
] as PluginSection[];

const registrations = [
  { metadata: { name: "gateway" }, spec: { label: "Gateway" } },
  {
    metadata: { name: "elasticsearch" },
    spec: { label: "New Elasticsearch cluster" },
  },
  { metadata: { name: "kms" }, spec: { label: "KMS key" } },
] as PluginRegistration[];

const plugins = [
  { section: "access", registration: "gateway" },
  { section: "secondary-storage", registration: "elasticsearch" },
  { section: "encryption-key", registration: "kms" },
];

const condition = (
  type: string,
  status: "True" | "False",
  reason: string,
  message: string,
) => ({
  type,
  status,
  reason,
  message,
  observedGeneration: 1,
  lastTransitionTime: "2026-11-16T00:00:00Z",
});

const entry = (
  section: string,
  registration: string,
  outputsReady: string,
  ready: string,
) => ({
  section,
  registration,
  name: `orders-${section}`,
  outputsReady,
  ready,
});

const claim = (status?: CamundaClusterClaim["status"]) =>
  ({
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "CamundaClusterClaim",
    metadata: { namespace: "team-a", name: "orders", generation: 1 },
    spec: { presetRef: "small", plugins },
    ...(status && { status }),
  }) as unknown as CamundaClusterClaim;

const requests = [
  {
    metadata: { namespace: "team-a", name: "orders-access" },
    status: {
      summary: [{ label: "Host", value: "orders.team-a.example.com" }],
    },
  },
  {
    metadata: { namespace: "team-a", name: "orders-secondary-storage" },
    status: { summary: [{ label: "Nodes", value: "3 of 3 ready" }] },
  },
] as unknown as PluginRequest[];

const meta = {
  title: "create/ClusterProgressView",
  component: ClusterProgressView,
  args: {
    name: "orders",
    namespace: "team-a",
    sections,
    registrations,
    requests: [],
  },
} satisfies Meta<typeof ClusterProgressView>;
export default meta;
type Story = StoryObj<typeof meta>;

export const EveryRowPending: Story = {
  name: "Every row pending",
  args: { claim: claim() },
};

const waiting = "waiting for the outputs of encryption-key";

export const WaitingForOutputs: Story = {
  name: "Waiting for outputs",
  args: {
    claim: claim({
      conditions: [
        condition("Ready", "False", "PrerequisiteNotMet", waiting),
        condition("RequestsReady", "False", "Blocked", waiting),
      ],
      requests: [
        entry("access", "gateway", "True", "True"),
        entry("secondary-storage", "elasticsearch", "True", "True"),
        entry("encryption-key", "kms", "False", "False"),
      ],
    }),
    requests,
  },
};

export const RefusedByThePrecheck: Story = {
  name: "Refused by the precheck",
  args: {
    claim: claim({
      conditions: [
        condition(
          "Ready",
          "False",
          "NoEnvironment",
          "the namespace team-a has no fleet.camunda.io/environment label",
        ),
      ],
    }),
  },
};

const held =
  'StorageAlreadyAttached: CamundaCluster "fleet-smoke/camundacluster-sample" already holds the backend';

export const ClusterHeldAtZero: Story = {
  name: "Cluster held at zero",
  args: {
    claim: claim({
      cluster: { name: "orders" },
      conditions: [
        condition("Ready", "False", "ClusterNotReady", held),
        condition("RequestsReady", "True", "Ready", "All requests are ready"),
        condition("ClusterReady", "False", "StorageAlreadyAttached", held),
        condition("AttachmentsReady", "True", "Ready", "No attachments"),
      ],
      requests: [
        entry("access", "gateway", "True", "True"),
        entry("secondary-storage", "elasticsearch", "True", "True"),
        entry("encryption-key", "kms", "True", "True"),
      ],
    }),
    requests,
  },
};

export const ReadyWithTheClusterLink: Story = {
  name: "Ready with the cluster link",
  args: {
    claim: claim({
      cluster: { name: "orders" },
      conditions: [
        condition("Ready", "True", "Ready", "The cluster is ready"),
        condition("RequestsReady", "True", "Ready", "All requests are ready"),
        condition(
          "ClusterReady",
          "True",
          "Ready",
          "Ready: The cluster is ready",
        ),
        condition("AttachmentsReady", "True", "Ready", "No attachments"),
      ],
      requests: [
        entry("access", "gateway", "True", "True"),
        entry("secondary-storage", "elasticsearch", "True", "True"),
        entry("encryption-key", "kms", "True", "True"),
      ],
    }),
    requests,
  },
};
