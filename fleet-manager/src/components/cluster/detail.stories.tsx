import type { Story } from "@/gallery/registry";
import type { PartitionHealth } from "@/lib/observe/types";
import { ComponentDetailView } from "./component-detail";
import { PartitionTable } from "./partition-table";

const now = "2026-10-30T12:00:00Z";

function pod(name: string, node: string, ready = true, restarts = 0) {
  return {
    metadata: { name, creationTimestamp: "2026-10-29T08:00:00Z" },
    spec: { containers: [{}], nodeName: node },
    status: {
      phase: "Running",
      containerStatuses: [{ ready, restartCount: restarts }],
    },
  };
}

function usage(name: string, cpu: number, memoryGiB: number) {
  return {
    pod: name,
    cpu: { usedCores: cpu, limitCores: 2 },
    memory: { usedBytes: memoryGiB * 1024 ** 3, limitBytes: 4 * 1024 ** 3 },
  };
}

const brokers = [
  pod("orders-zeebe-0", "pool-a-1"),
  pod("orders-zeebe-1", "pool-a-2"),
  pod("orders-zeebe-2", "pool-a-3", false, 4),
];

const degraded: PartitionHealth[] = [
  {
    partition: 1,
    leader: "orders-zeebe-0",
    followers: ["orders-zeebe-1"],
    processorPhase: "PROCESSING",
    health: "healthy",
  },
  {
    partition: 2,
    leader: "orders-zeebe-1",
    followers: ["orders-zeebe-0"],
    processorPhase: "PROCESSING",
    health: "unhealthy",
  },
  {
    partition: 3,
    followers: ["orders-zeebe-0"],
    health: "dead",
  },
];

/** detailStories show the component detail and its partition table. */
export const detailStories: Story[] = [
  {
    component: "ComponentDetail",
    name: "Brokers degraded",
    render: () => (
      <ComponentDetailView
        title="Brokers"
        conditions={[
          {
            type: "ZeebeReady",
            status: "False",
            reason: "Degraded",
            message: "2 of 3 pods ready",
            lastTransitionTime: "2026-10-30T11:52:00Z",
          },
        ]}
        pods={brokers}
        usage={[
          usage("orders-zeebe-0", 0.8, 2.6),
          usage("orders-zeebe-1", 1.1, 3.1),
        ]}
        now={now}
      >
        <PartitionTable partitions={degraded} />
      </ComponentDetailView>
    ),
  },
  {
    component: "ComponentDetail",
    name: "Embedded Operate in the gateway",
    render: () => (
      <ComponentDetailView
        title="Operate"
        subtitle="Embedded in Gateway"
        conditions={[
          {
            type: "OperateReady",
            status: "True",
            reason: "Disabled",
            message: "",
            lastTransitionTime: "2026-10-29T08:00:00Z",
          },
        ]}
        pods={[pod("orders-gateway-0", "pool-a-1")]}
        usage={[usage("orders-gateway-0", 0.3, 1.2)]}
        now={now}
      />
    ),
  },
  {
    component: "ComponentDetail",
    name: "Usage not available",
    render: () => (
      <ComponentDetailView
        title="Tasklist"
        conditions={[
          {
            type: "TasklistReady",
            status: "True",
            reason: "Healthy",
            message: "",
            lastTransitionTime: "2026-10-29T08:00:00Z",
          },
        ]}
        pods={[pod("orders-tasklist-0", "pool-a-2")]}
        usage="unavailable"
        now={now}
      />
    ),
  },
  {
    component: "PartitionTable",
    name: "Health from Prometheus",
    render: () => <PartitionTable partitions={degraded} />,
  },
  {
    component: "PartitionTable",
    name: "No Prometheus",
    render: () => (
      <PartitionTable
        partitions={degraded.map((p) => ({ ...p, health: "unknown" }))}
      />
    ),
  },
];
