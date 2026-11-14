import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { UsageBars } from "./usage-bars";

const GiB = 2 ** 30;

const meta = {
  title: "fleet/UsageBars",
  component: UsageBars,
} satisfies Meta<typeof UsageBars>;
export default meta;
type Story = StoryObj<typeof meta>;

export const NearTheLimit: Story = {
  name: "Near the limit",
  args: {
    usage: {
      pod: "x",
      cpu: { usedCores: 0.95, limitCores: 1 },
      memory: { usedBytes: 3.8 * GiB, limitBytes: 4 * GiB },
    },
  },
};

export const RequestAndLimit: Story = {
  name: "Request and limit",
  args: {
    usage: {
      pod: "x",
      cpu: { usedCores: 0.8, requestCores: 0.5, limitCores: 1 },
      memory: {
        usedBytes: 1.2 * GiB,
        requestBytes: 1 * GiB,
        limitBytes: 2 * GiB,
      },
    },
  },
};

export const NoLimit: Story = {
  name: "No limit",
  args: {
    usage: {
      pod: "x",
      cpu: { usedCores: 0.2 },
      memory: { usedBytes: 0.5 * GiB },
    },
  },
};
