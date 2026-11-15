import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { PodTable } from "./pod-table";

const GiB = 2 ** 30;
const pods = ["orders-zeebe-0", "orders-zeebe-1", "orders-zeebe-2"].map(
  (name, i) => ({
    metadata: { name },
    spec: { nodeName: `pool-a-${i + 1}` },
    status: {
      phase: "Running",
      conditions: [{ type: "Ready", status: i === 2 ? "False" : "True" }],
      containerStatuses: [
        {
          ready: i !== 2,
          restartCount: i * 3,
          state:
            i === 2 ? { waiting: { reason: "CrashLoopBackOff" } } : undefined,
        },
      ],
    },
  }),
);
const usage = pods.map((p, i) => ({
  pod: p.metadata.name,
  cpu: { usedCores: 0.3 + i * 0.3, requestCores: 0.5, limitCores: 1 },
  memory: { usedBytes: (1.5 + i * 1.2) * GiB, limitBytes: 4 * GiB },
}));

const meta = {
  title: "cluster/PodTable",
  component: PodTable,
  args: { pods, usage },
} satisfies Meta<typeof PodTable>;
export default meta;
type Story = StoryObj<typeof meta>;

export const WithUsage: Story = { name: "With usage" };

export const UsageUnavailable: Story = {
  name: "Usage unavailable",
  args: { usage: "unavailable" },
};
