import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { degraded } from "./fixtures";
import { PartitionTable } from "./partition-table";

const meta = {
  title: "cluster/PartitionTable",
  component: PartitionTable,
} satisfies Meta<typeof PartitionTable>;
export default meta;
type Story = StoryObj<typeof meta>;

export const HealthFromPrometheus: Story = {
  name: "Health from Prometheus",
  args: { partitions: degraded },
};

export const NoPrometheus: Story = {
  name: "No Prometheus",
  args: { partitions: degraded.map((p) => ({ ...p, health: "unknown" })) },
};
