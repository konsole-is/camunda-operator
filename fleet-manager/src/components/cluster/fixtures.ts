import type { PartitionHealth } from "@/lib/observe/types";

/** now is the time of the stories of the component detail and its tabs. */
export const now = "2026-10-30T12:00:00Z";

/** pod gives a running pod with one container on node. */
export function pod(name: string, node: string, ready = true, restarts = 0) {
  return {
    metadata: { name, creationTimestamp: "2026-10-29T08:00:00Z" },
    spec: { containers: [{}], nodeName: node },
    status: {
      phase: "Running",
      containerStatuses: [{ ready, restartCount: restarts }],
    },
  };
}

/** usage gives the usage of pod name, with a limit of 2 cores and 4 GiB. */
export function usage(name: string, cpu: number, memoryGiB: number) {
  return {
    pod: name,
    cpu: { usedCores: cpu, limitCores: 2 },
    memory: { usedBytes: memoryGiB * 1024 ** 3, limitBytes: 4 * 1024 ** 3 },
  };
}

/**
 * brokers has the three brokers of the cluster orders. The third is not
 * ready and restarted 4 times.
 */
export const brokers = [
  pod("orders-zeebe-0", "pool-a-1"),
  pod("orders-zeebe-1", "pool-a-2"),
  pod("orders-zeebe-2", "pool-a-3", false, 4),
];

/** degraded has a healthy, an unhealthy, and a dead partition. */
export const degraded: PartitionHealth[] = [
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

/**
 * secrets has the Secrets that the secret pickers of the settings editors
 * offer.
 */
export const secrets = [
  { name: "orders-db", keys: ["password", "username"] },
  { name: "orders-oidc", keys: ["secret"] },
];
