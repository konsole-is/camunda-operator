import { EventList } from "@/components/fleet/event-list";
import type { Story } from "@/gallery/registry";
import type { Resource } from "@/lib/k8s/kinds";
import { parseLogLine } from "@/lib/observe/logs";
import type {
  LogLine,
  MetricChartData,
  PartitionHealth,
  Series,
} from "@/lib/observe/types";
import { ComponentDetailView } from "./component-detail";
import { DetailLogsView } from "./detail-logs";
import { DetailMetricsView } from "./detail-metrics";
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

const start = Date.parse("2026-10-30T11:00:00Z");
const GiB = 1024 ** 3;

// sine gives one series for each broker, 30 points 2 minutes apart.
function sine(base: number, swing: number): Series[] {
  return brokers.map((p, n) => ({
    name: p.metadata.name,
    points: Array.from({ length: 30 }, (_, i): [number, number] => [
      start + i * 120_000,
      base * (1 + n * 0.15) + swing * Math.sin((i + n * 3) / 4),
    ]),
  }));
}

const brokerUsage = [
  usage("orders-zeebe-0", 0.8, 2.6),
  usage("orders-zeebe-1", 1.1, 3.1),
  usage("orders-zeebe-2", 0.4, 1.4),
].map((u) => ({
  ...u,
  cpu: { ...u.cpu, requestCores: 1 },
  memory: { ...u.memory, requestBytes: 2 * GiB },
}));

const brokerCharts: MetricChartData[] = [
  { label: "CPU", unit: "cores", source: "", series: sine(0.8, 0.2) },
  {
    label: "Disk used",
    unit: "bytes",
    source: "",
    series: sine(12 * GiB, 0.4 * GiB).map((s) => ({
      ...s,
      name: `data-${s.name}`,
    })),
  },
  {
    label: "Process instances activated",
    unit: "instances/s",
    source: "",
    series: sine(40, 12),
  },
];

// logLine gives one JSON line of a broker, as Camunda writes it, from pod.
function logLine(
  pod: string,
  second: number,
  level: string,
  logger: string,
  message: string,
): LogLine {
  const timestamp = `2026-10-30T11:58:${String(second).padStart(2, "0")}.120Z`;
  return {
    ...parseLogLine(
      JSON.stringify({
        timestamp,
        severity: level,
        logger_name: logger,
        message,
      }),
    ),
    pod,
  };
}

const brokerLines = [
  logLine(
    "orders-zeebe-0",
    2,
    "INFO",
    "io.camunda.zeebe.broker.system",
    "Broker 0 is ready",
  ),
  logLine(
    "orders-zeebe-1",
    3,
    "INFO",
    "io.camunda.zeebe.broker.system",
    "Broker 1 is ready",
  ),
  logLine(
    "orders-zeebe-0",
    9,
    "INFO",
    "io.camunda.zeebe.raft",
    "Partition 1 transitioned to LEADER in term 4",
  ),
  logLine(
    "orders-zeebe-2",
    11,
    "WARN",
    "io.camunda.zeebe.broker.system",
    "Disk usage is above 80 percent of the data volume",
  ),
  logLine(
    "orders-zeebe-2",
    14,
    "ERROR",
    "io.camunda.zeebe.journal",
    "Could not append to the journal of partition 3, because the disk is full",
  ),
  logLine(
    "orders-zeebe-1",
    20,
    "INFO",
    "io.camunda.zeebe.raft",
    "Partition 2 transitioned to LEADER in term 7",
  ),
];

const controllerLines = [
  logLine(
    "rustfs-0",
    1,
    "INFO",
    "rustfs::server",
    "Bucket orders-backup-storage is ready",
  ),
  {
    ...parseLogLine(
      JSON.stringify({
        ts: "2026-10-30T11:58:04.000Z",
        level: "info",
        logger: "plugin",
        msg: "Reconciled the bucket",
        request: { namespace: "team-a", name: "orders-backup-storage" },
      }),
    ),
    pod: "rustfs-controller-6f9c-2xk4p",
  },
];

const event = (
  reason: string,
  note: string,
  regarding: { kind: string; name: string },
  minutesAgo: number,
  type = "Normal",
) =>
  ({
    metadata: {
      name: `${regarding.name}.${reason}`,
      uid: `${regarding.name}/${reason}`,
    },
    type,
    reason,
    note,
    eventTime: new Date(Date.parse(now) - minutesAgo * 60_000).toISOString(),
    regarding: { ...regarding, namespace: "team-a" },
  }) as Resource<"Event">;

/** detailStories show the component detail, its partition table, and its Metrics, Logs, and Events tabs. */
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
  {
    component: "DetailMetrics",
    name: "Charts of the brokers",
    render: () => (
      <DetailMetricsView
        pods={brokers}
        usage={brokerUsage}
        charts={{ prometheus: "answered", charts: brokerCharts, errors: [] }}
        range="1h"
      />
    ),
  },
  {
    component: "DetailMetrics",
    name: "Prometheus not configured",
    render: () => (
      <DetailMetricsView
        pods={brokers}
        usage={brokerUsage}
        charts={{ prometheus: "notConfigured" }}
        range="1h"
      />
    ),
  },
  {
    component: "DetailMetrics",
    name: "No Camunda charts for the version",
    render: () => (
      <DetailMetricsView
        pods={brokers}
        usage={brokerUsage}
        charts={{
          prometheus: "answered",
          charts: brokerCharts.slice(0, 2),
          errors: [],
          hiddenForVersion: { version: "8.10.0" },
        }}
        range="1h"
      />
    ),
  },
  {
    component: "DetailMetrics",
    name: "No metrics server",
    render: () => (
      <DetailMetricsView
        pods={brokers}
        usage="unavailable"
        charts={{
          prometheus: "answered",
          charts: brokerCharts.slice(0, 1),
          errors: [],
        }}
        range="6h"
      />
    ),
  },
  {
    component: "DetailMetrics",
    name: "A request query that does not render",
    render: () => (
      <DetailMetricsView
        pods={[pod("rustfs-0", "pool-a-1")]}
        usage={[usage("rustfs-0", 0.2, 0.6)]}
        charts={{
          prometheus: "answered",
          charts: [
            {
              label: "Requests",
              unit: "req/s",
              source: "rustfs-bucket",
              declaredBy: "Bucket on RustFS",
              series: sine(120, 30)
                .slice(0, 1)
                .map((s) => ({
                  ...s,
                  name: "rustfs-0",
                })),
            },
          ],
          errors: [
            {
              label: "Latency",
              declaredBy: "Bucket on RustFS",
              error: 'output "bucket" has no value',
            },
          ],
        }}
        range="1h"
      />
    ),
  },
  {
    component: "DetailLogs",
    name: "All pods of the brokers",
    render: () => (
      <DetailLogsView
        pods={brokers.map((p) => p.metadata.name)}
        picked=""
        lines={brokerLines}
        errors={[]}
      />
    ),
  },
  {
    component: "DetailLogs",
    name: "A request with its controller",
    render: () => (
      <DetailLogsView
        pods={["rustfs-0"]}
        controllerPods={["rustfs-controller-6f9c-2xk4p"]}
        picked=""
        lines={controllerLines}
        errors={[
          {
            pod: "rustfs-0",
            error: "The connection to the log stream broke",
          },
        ]}
      />
    ),
  },
  {
    component: "DetailEvents",
    name: "Events of the brokers",
    render: () => (
      <EventList
        now={now}
        showObject
        events={[
          event(
            "BackOff",
            "Back-off restarting failed container zeebe",
            { kind: "Pod", name: "orders-zeebe-2" },
            3,
            "Warning",
          ),
          event(
            "SuccessfulCreate",
            "create Pod orders-zeebe-2 in StatefulSet orders-zeebe successful",
            { kind: "StatefulSet", name: "orders-zeebe" },
            14,
          ),
          event(
            "CreatedStatefulSet",
            "Created StatefulSet 'orders-zeebe'",
            { kind: "CamundaCluster", name: "orders" },
            62,
          ),
        ]}
      />
    ),
  },
];
