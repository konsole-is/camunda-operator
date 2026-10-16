import type { Story } from "@/gallery/registry";
import type { LogLine, Series } from "@/lib/observe/types";
import { EventList } from "./event-list";
import { LogViewer } from "./log-viewer";
import { MetricChart } from "./metric-chart";
import { PodTable } from "./pod-table";
import { ProgressRows } from "./progress-rows";
import { UsageBars } from "./usage-bars";

const now = "2026-10-15T12:00:00Z";
const GiB = 2 ** 30;
const pods = ["orders-zeebe-0", "orders-zeebe-1", "orders-zeebe-2"].map(
  (name, i) => ({
    metadata: { name, creationTimestamp: "2026-10-15T09:00:00Z" },
    status: {
      phase: "Running",
      containerStatuses: [{ ready: i !== 2, restartCount: i }],
    },
  }),
);
const usage = pods.map((p, i) => ({
  pod: p.metadata.name,
  cpu: { usedCores: 0.3 + i * 0.3, requestCores: 0.5, limitCores: 1 },
  memory: { usedBytes: (1.5 + i * 1.2) * GiB, limitBytes: 4 * GiB },
}));
const start = Date.parse("2026-10-15T11:00:00Z");
const series: Series[] = [
  {
    name: "orders-zeebe-0",
    points: Array.from({ length: 30 }, (_, i) => [
      start + i * 120_000,
      400 + Math.round(120 * Math.sin(i / 4)),
    ]),
  },
  {
    name: "orders-zeebe-1",
    points: Array.from({ length: 30 }, (_, i) => [
      start + i * 120_000,
      300 + Math.round(90 * Math.cos(i / 5)),
    ]),
  },
];
const logs: LogLine[] = [
  {
    raw: "",
    time: "2026-10-15T11:58:01Z",
    level: "INFO",
    message: "Partition 1 became leader",
    logger: "io.camunda.zeebe.broker",
    fields: { partitionId: 1, term: 4 },
  },
  {
    raw: "",
    time: "2026-10-15T11:58:05Z",
    level: "WARN",
    message: "Exporter is behind by 1200 records",
    fields: {},
  },
  {
    raw: "",
    time: "2026-10-15T11:58:09Z",
    level: "ERROR",
    message: "Failed to export a batch to Elasticsearch: connection refused",
    fields: { exporter: "camundaexporter" },
  },
].map((l) => ({ ...l, raw: `${l.level} ${l.message}` }));

export const observeStories: Story[] = [
  {
    component: "PodTable",
    name: "With usage",
    render: () => <PodTable pods={pods} usage={usage} now={now} />,
  },
  {
    component: "PodTable",
    name: "Usage unavailable",
    render: () => <PodTable pods={pods} usage="unavailable" now={now} />,
  },
  {
    component: "UsageBars",
    name: "Near the limit",
    render: () => (
      <UsageBars
        usage={{
          pod: "x",
          cpu: { usedCores: 0.95, limitCores: 1 },
          memory: { usedBytes: 3.8 * GiB, limitBytes: 4 * GiB },
        }}
      />
    ),
  },
  {
    component: "UsageBars",
    name: "No limit",
    render: () => (
      <UsageBars
        usage={{
          pod: "x",
          cpu: { usedCores: 0.2 },
          memory: { usedBytes: 0.5 * GiB },
        }}
      />
    ),
  },
  {
    component: "MetricChart",
    name: "Series",
    render: () => (
      <MetricChart title="Exported records per second" series={series} />
    ),
  },
  {
    component: "MetricChart",
    name: "No data",
    render: () => (
      <MetricChart title="Exported records per second" series={[]} />
    ),
  },
  {
    component: "LogViewer",
    name: "Following",
    render: () => <LogViewer lines={logs} />,
  },
  {
    component: "LogViewer",
    name: "No lines",
    render: () => <LogViewer lines={[]} />,
  },
  {
    component: "EventList",
    name: "Events",
    render: () => (
      <EventList
        now={now}
        events={[
          {
            type: "Normal",
            reason: "Created",
            note: "Created the StatefulSet orders-zeebe",
            eventTime: "2026-10-15T09:00:00Z",
          },
          {
            type: "Warning",
            reason: "BackOff",
            note: "Back-off restarting the failed container",
            eventTime: "2026-10-15T11:50:00Z",
            series: { count: 3, lastObservedTime: "2026-10-15T11:59:00Z" },
          },
        ]}
      />
    ),
  },
  {
    component: "EventList",
    name: "Empty",
    render: () => <EventList now={now} events={[]} />,
  },
  {
    component: "ProgressRows",
    name: "Waiting for outputs",
    render: () => (
      <ProgressRows
        ready={{
          status: "False",
          reason: "WaitingForOutputs",
          message: "Waiting for Backup storage and Secondary storage",
        }}
        rows={[
          {
            id: "backup",
            label: "Backup storage",
            detail: "New S3 bucket",
            summary: "Creating the bucket orders-backups",
            checks: [
              {
                label: "Outputs",
                condition: { status: "False", reason: "Provisioning" },
              },
              {
                label: "Ready",
                condition: { status: "False", reason: "Provisioning" },
              },
            ],
          },
          {
            id: "storage",
            label: "Secondary storage",
            detail: "New Elasticsearch",
            checks: [
              { label: "Outputs", condition: { status: "True" } },
              {
                label: "Ready",
                condition: { status: "Unknown", reason: "Converging" },
              },
            ],
          },
          {
            id: "cluster",
            label: "Cluster",
            checks: [
              {
                label: "RequestsReady",
                condition: { status: "False", reason: "Waiting" },
              },
              { label: "ClusterReady" },
            ],
          },
        ]}
      />
    ),
  },
  {
    component: "ProgressRows",
    name: "Ready",
    render: () => (
      <ProgressRows
        ready={{ status: "True", reason: "Healthy" }}
        rows={[
          {
            id: "cluster",
            label: "Cluster",
            checks: [
              { label: "RequestsReady", condition: { status: "True" } },
              { label: "ClusterReady", condition: { status: "True" } },
            ],
          },
        ]}
        done={
          <a href="/fleet/clusters/orders" className="text-sm text-info">
            Open the cluster orders
          </a>
        }
      />
    ),
  },
];
