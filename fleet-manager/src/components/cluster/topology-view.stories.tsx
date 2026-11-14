import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import type { CamundaClusterSpec } from "@/generated/crds/camundacluster";
import type { Resource } from "@/lib/k8s/kinds";
import { COMPONENT_LABEL } from "@/lib/labels";
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

const meta = {
  title: "cluster/TopologyView",
  component: TopologyView,
} satisfies Meta<typeof TopologyView>;
export default meta;
type Story = StoryObj<typeof meta>;

export const StandaloneGateway: Story = {
  name: "Standalone gateway",
  args: {
    boxes: topologyOf({} as CamundaClusterSpec, [
      pod("orders-zeebe-0", "zeebe", ready),
      pod("orders-zeebe-1", "zeebe", ready),
      pod("orders-zeebe-2", "zeebe", ready),
      pod("orders-gateway-7d9f-x2", "gateway", ready),
    ]),
  },
};

export const EmbeddedGateway: Story = {
  name: "Embedded gateway",
  args: {
    boxes: topologyOf(
      {
        gateway: { mode: "Embedded" },
        connectors: { enabled: true },
      } as CamundaClusterSpec,
      [
        pod("orders-zeebe-0", "zeebe", ready),
        pod("orders-connectors-5c-a1", "connectors", ready),
      ],
    ),
  },
};

export const PodsInEveryState: Story = {
  name: "Pods in every state",
  args: {
    boxes: topologyOf(
      { operate: { mode: "Standalone" } } as CamundaClusterSpec,
      [
        pod("orders-zeebe-0", "zeebe", ready),
        pod("orders-zeebe-1", "zeebe", waiting("CrashLoopBackOff")),
        pod("orders-zeebe-2", "zeebe", waiting("ContainerCreating")),
        pod("orders-gateway-7d9f-x2", "gateway", { phase: "Failed" }),
        pod("orders-gateway-7d9f-y3", "gateway", ready, true),
      ],
    ),
  },
};
