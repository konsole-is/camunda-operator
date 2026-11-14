import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import type { Series as MetricSeries } from "@/lib/observe/types";
import { MetricChart } from "./metric-chart";

const start = Date.parse("2026-10-15T11:00:00Z");
const series: MetricSeries[] = [
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

const meta = {
  title: "fleet/MetricChart",
  component: MetricChart,
  args: { title: "Exported records per second", series: [] },
} satisfies Meta<typeof MetricChart>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Series: Story = { name: "Series", args: { series } };

export const NoData: Story = { name: "No data" };
