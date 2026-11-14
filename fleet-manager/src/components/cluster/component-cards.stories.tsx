import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import type { CamundaClusterSpec } from "@/generated/crds/camundacluster";
import type { Resource } from "@/lib/k8s/kinds";
import { COMPONENT_LABEL, OPTIMIZE_UID_LABEL } from "@/lib/labels";
import type { PodUsage } from "@/lib/observe/types";
import { componentCards } from "./cards";
import { ComponentCards } from "./component-card";

function pod(name: string, component: string, isReady = true): Resource<"Pod"> {
  return {
    kind: "Pod",
    metadata: {
      name,
      labels: {
        [COMPONENT_LABEL]: component,
        ...(component.startsWith("optimize-") && {
          [OPTIMIZE_UID_LABEL]: "uid-orders-optimize",
        }),
      },
    },
    status: isReady
      ? { phase: "Running", conditions: [{ type: "Ready", status: "True" }] }
      : {
          phase: "Running",
          containerStatuses: [
            {
              name: "zeebe",
              image: "camunda/zeebe",
              imageID: "",
              ready: false,
              restartCount: 4,
              state: { waiting: { reason: "CrashLoopBackOff" } },
            },
          ],
        },
  } as Resource<"Pod">;
}

function usage(name: string, cpu: number, memory: number): PodUsage {
  return {
    pod: name,
    cpu: { usedCores: cpu, limitCores: 1 },
    memory: { usedBytes: memory, limitBytes: 2 * 1024 ** 3 },
  };
}

const healthy = (type: string) =>
  ({ type, status: "True", reason: "Healthy" }) as const;

const standalone = {
  operate: { mode: "Standalone" },
  tasklist: { mode: "Standalone" },
  admin: { mode: "Standalone" },
  connectors: { enabled: true },
} as CamundaClusterSpec;

const allPods = [
  pod("orders-zeebe-0", "zeebe"),
  pod("orders-zeebe-1", "zeebe"),
  pod("orders-zeebe-2", "zeebe"),
  pod("orders-gateway-7c9f-a", "gateway"),
  pod("orders-operate-5d8b-a", "operate"),
  pod("orders-tasklist-6b4c-a", "tasklist"),
  pod("orders-admin-8f2d-a", "admin"),
  pod("orders-connectors-4a1e-a", "connectors"),
  pod("orders-optimize-webapp-a", "optimize-webapp"),
  pod("orders-optimize-importer-a", "optimize-importer"),
];

const optimize = {
  kind: "CamundaOptimize",
  metadata: {
    name: "orders-optimize",
    namespace: "team-a",
    uid: "uid-orders-optimize",
  },
  spec: { clusterRef: { name: "orders" } },
  status: { conditions: [healthy("Ready")] },
} as unknown as Resource<"CamundaOptimize">;

const threePartitions = [
  {
    partition: 1,
    leader: "orders-zeebe-0",
    followers: ["orders-zeebe-1", "orders-zeebe-2"],
    health: "unknown",
  },
  {
    partition: 2,
    leader: "orders-zeebe-1",
    followers: ["orders-zeebe-0", "orders-zeebe-2"],
    health: "unknown",
  },
  {
    partition: 3,
    leader: "orders-zeebe-2",
    followers: ["orders-zeebe-0", "orders-zeebe-1"],
    health: "unknown",
  },
] as const;

const meta = {
  title: "cluster/ComponentCards",
  component: ComponentCards,
  args: { onOpen: fn() },
} satisfies Meta<typeof ComponentCards>;
export default meta;
type Story = StoryObj<typeof meta>;

export const AllUsedAndReady: Story = {
  name: "All used and ready",
  args: {
    cards: componentCards({
      spec: standalone,
      pods: allPods,
      conditions: [
        "ZeebeReady",
        "GatewayReady",
        "OperateReady",
        "TasklistReady",
        "AdminReady",
        "ConnectorsReady",
      ].map(healthy),
      optimize,
      usage: allPods.map((p, i) =>
        usage(p.metadata?.name ?? "", 0.1 * (i + 1), 0.8 * 1024 ** 3),
      ),
      partitions: threePartitions.map((p) => ({
        ...p,
        followers: [...p.followers],
      })),
    }),
    hrefOf: (id) => `?tab=overview&component=${id}`,
  },
};

export const DegradedBrokers: Story = {
  name: "Degraded brokers",
  args: {
    cards: componentCards({
      spec: {} as CamundaClusterSpec,
      pods: [
        pod("orders-zeebe-0", "zeebe"),
        pod("orders-zeebe-1", "zeebe"),
        pod("orders-zeebe-2", "zeebe", false),
        pod("orders-gateway-7c9f-a", "gateway"),
      ],
      conditions: [
        {
          type: "ZeebeReady",
          status: "False",
          reason: "Degraded",
          message: "2 of 3 pods are ready",
        },
        healthy("GatewayReady"),
      ],
      usage: [usage("orders-gateway-7c9f-a", 0.35, 1.1 * 1024 ** 3)],
      partitions: [
        {
          partition: 1,
          leader: "orders-zeebe-0",
          followers: ["orders-zeebe-1"],
          health: "unknown",
        },
        {
          partition: 2,
          leader: "orders-zeebe-1",
          followers: ["orders-zeebe-0"],
          health: "unknown",
        },
        { partition: 3, followers: ["orders-zeebe-0"], health: "unknown" },
      ],
    }),
  },
};

export const EmbeddedAppsConnectorsAndOptimizeNotUsed: Story = {
  name: "Embedded apps, Connectors and Optimize not used",
  args: {
    cards: componentCards({
      spec: {} as CamundaClusterSpec,
      pods: [
        pod("orders-zeebe-0", "zeebe"),
        pod("orders-gateway-7c9f-a", "gateway"),
      ],
      conditions: [
        healthy("ZeebeReady"),
        healthy("GatewayReady"),
        { type: "OperateReady", status: "True", reason: "Disabled" },
      ],
      usage: [usage("orders-gateway-7c9f-a", 0.2, 0.5 * 1024 ** 3)],
      partitions: [
        {
          partition: 1,
          leader: "orders-zeebe-0",
          followers: [],
          health: "unknown",
        },
      ],
    }),
  },
};

export const UsageNotAvailable: Story = {
  name: "Usage not available",
  args: {
    cards: componentCards({
      spec: standalone,
      pods: allPods,
      conditions: [healthy("ZeebeReady"), healthy("GatewayReady")],
      optimize,
      usage: "unavailable",
      partitions: "unavailable",
    }),
  },
};
