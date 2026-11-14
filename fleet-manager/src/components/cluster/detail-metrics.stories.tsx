import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { type ComponentProps, useState } from "react";
import { fn } from "storybook/test";
import type { MetricChartData, Series } from "@/lib/observe/types";
import { DetailMetricsView } from "./detail-metrics";
import { brokers, pod, usage } from "./fixtures";

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

// DetailMetricsViewHarness keeps the picked range, as the Metrics tab does.
// The charts stay the same, because the story has no Prometheus to ask.
function DetailMetricsViewHarness(
  props: ComponentProps<typeof DetailMetricsView>,
) {
  const [range, setRange] = useState(props.range);
  return (
    <DetailMetricsView
      {...props}
      range={range}
      onRange={(value) => {
        props.onRange?.(value);
        setRange(value);
      }}
    />
  );
}

const meta = {
  title: "cluster/DetailMetrics",
  component: DetailMetricsView,
  args: { pods: brokers, usage: brokerUsage, range: "1h", onRange: fn() },
  render: (args) => <DetailMetricsViewHarness key={args.range} {...args} />,
} satisfies Meta<typeof DetailMetricsView>;
export default meta;
type Story = StoryObj<typeof meta>;

export const ChartsOfTheBrokers: Story = {
  name: "Charts of the brokers",
  args: {
    charts: { prometheus: "answered", charts: brokerCharts, errors: [] },
  },
};

export const PrometheusNotConfigured: Story = {
  name: "Prometheus not configured",
  args: { charts: { prometheus: "notConfigured" } },
};

export const NoCamundaChartsForTheVersion: Story = {
  name: "No Camunda charts for the version",
  args: {
    charts: {
      prometheus: "answered",
      charts: brokerCharts.slice(0, 2),
      errors: [],
      hiddenForVersion: { version: "8.10.0" },
    },
  },
};

export const NoMetricsServer: Story = {
  name: "No metrics server",
  args: {
    usage: "unavailable",
    charts: {
      prometheus: "answered",
      charts: brokerCharts.slice(0, 1),
      errors: [],
    },
    range: "6h",
  },
};

export const ARequestQueryThatDoesNotRender: Story = {
  name: "A request query that does not render",
  args: {
    pods: [pod("rustfs-0", "pool-a-1")],
    usage: [usage("rustfs-0", 0.2, 0.6)],
    charts: {
      prometheus: "answered",
      charts: [
        {
          label: "Requests",
          unit: "req/s",
          source: "rustfs-bucket",
          declaredBy: "Bucket on RustFS",
          series: sine(120, 30)
            .slice(0, 1)
            .map((s) => ({ ...s, name: "rustfs-0" })),
        },
      ],
      errors: [
        {
          label: "Latency",
          declaredBy: "Bucket on RustFS",
          error: 'output "bucket" has no value',
        },
      ],
    },
  },
};
