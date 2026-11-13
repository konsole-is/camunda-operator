import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import type { Resource } from "@/lib/k8s/kinds";
import { CLAIM_LABEL } from "@/lib/labels";
import { mergeCluster } from "@/lib/merge";
import { claimView } from "./claim";
import { ClaimTab } from "./claim-tab";

const now = "2026-10-30T12:00:00Z";

const condition = (
  type: string,
  status: "True" | "False",
  reason: string,
  message: string,
) => ({ type, status, reason, message, lastTransitionTime: now });

function claimOf(
  conditions: ReturnType<typeof condition>[],
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

const cluster = {
  kind: "CamundaCluster",
  metadata: {
    namespace: "team-a",
    name: "orders",
    labels: { [CLAIM_LABEL]: "orders" },
  },
  spec: {
    presetRef: "medium",
    externalUrl: "https://orders.example.com",
    releaseRef: "camunda-8.9",
  },
} as unknown as Resource<"CamundaCluster">;

// argsOf gives the props of the Claim tab of the cluster orders for claim.
function argsOf(claim: Resource<"CamundaClusterClaim">) {
  return {
    view: claimView({
      claim,
      cluster,
      merged: mergeCluster(undefined, undefined, cluster.spec),
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
    }),
    stored: claim,
    now,
  };
}

const meta = {
  title: "cluster/ClaimTab",
  component: ClaimTab,
} satisfies Meta<typeof ClaimTab>;
export default meta;
type Story = StoryObj<typeof meta>;

export const WaitingForOutputs: Story = {
  name: "Waiting for outputs",
  args: argsOf(
    claimOf(
      [
        condition(
          "Ready",
          "False",
          "PrerequisiteNotMet",
          "waiting for the outputs of cloud-identity",
        ),
        condition(
          "RequestsReady",
          "False",
          "OperationPending",
          "waiting for the outputs of cloud-identity",
        ),
        condition(
          "ClusterReady",
          "False",
          "Creating",
          "CamundaCluster has not reported Ready yet",
        ),
      ],
      "False",
    ),
  ),
};

export const HealthyWithSections: Story = {
  name: "Healthy with sections",
  args: argsOf(
    claimOf(
      [
        condition("Ready", "True", "Healthy", ""),
        condition("RequestsReady", "True", "Healthy", ""),
        condition(
          "ClusterReady",
          "True",
          "Healthy",
          "Healthy: Component is healthy.",
        ),
        condition("AttachmentsReady", "True", "Healthy", ""),
      ],
      "True",
    ),
  ),
};
