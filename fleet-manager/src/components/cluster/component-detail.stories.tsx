import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import type { ConditionRow } from "@/components/fleet/conditions-table";
import { ComponentDetailView } from "./component-detail";
import { brokers, degraded, now, pod, usage } from "./fixtures";
import { PartitionTable } from "./partition-table";

const meta = {
  title: "cluster/ComponentDetail",
  component: ComponentDetailView,
  args: { now },
} satisfies Meta<typeof ComponentDetailView>;
export default meta;
type Story = StoryObj<typeof meta>;

const zeebeDegraded: ConditionRow = {
  type: "ZeebeReady",
  status: "False",
  reason: "Degraded",
  message: "2 of 3 pods ready",
  lastTransitionTime: "2026-10-30T11:52:00Z",
};

const operateDisabled: ConditionRow = {
  type: "OperateReady",
  status: "True",
  reason: "Disabled",
  message: "",
  lastTransitionTime: "2026-10-29T08:00:00Z",
};

const tasklistHealthy: ConditionRow = {
  type: "TasklistReady",
  status: "True",
  reason: "Healthy",
  message: "",
  lastTransitionTime: "2026-10-29T08:00:00Z",
};

export const BrokersDegraded: Story = {
  name: "Brokers degraded",
  args: {
    title: "Brokers",
    conditions: [zeebeDegraded],
    ready: zeebeDegraded,
    pods: brokers,
    usage: [
      usage("orders-zeebe-0", 0.8, 2.6),
      usage("orders-zeebe-1", 1.1, 3.1),
    ],
    children: <PartitionTable partitions={degraded} />,
  },
};

export const EmbeddedOperateInTheGateway: Story = {
  name: "Embedded Operate in the gateway",
  args: {
    title: "Operate",
    subtitle: "Embedded in Gateway",
    conditions: [operateDisabled],
    ready: operateDisabled,
    pods: [pod("orders-gateway-0", "pool-a-1")],
    usage: [usage("orders-gateway-0", 0.3, 1.2)],
  },
};

export const UsageNotAvailable: Story = {
  name: "Usage not available",
  args: {
    title: "Tasklist",
    conditions: [tasklistHealthy],
    ready: tasklistHealthy,
    pods: [pod("orders-tasklist-0", "pool-a-2")],
    usage: "unavailable",
  },
};

const accessWaiting: ConditionRow = {
  type: "Ready",
  status: "False",
  reason: "WaitingForEndpoints",
  message: "The gateway has no address yet",
  lastTransitionTime: "2026-10-30T11:58:00Z",
};

export const ComponentWithoutPods: Story = {
  name: "Component without pods",
  args: {
    title: "Access",
    subtitle: "Gateway access",
    conditions: [accessWaiting],
    ready: accessWaiting,
    pods: [],
    usage: [],
    podless: true,
  },
};
