import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { StatusChip } from "./status-chip";

const meta = {
  title: "fleet/StatusChip",
  component: StatusChip,
} satisfies Meta<typeof StatusChip>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Ready: Story = {
  name: "Ready",
  args: { condition: { status: "True", reason: "Healthy" } },
};

export const NotReady: Story = {
  name: "Not ready",
  args: {
    condition: {
      status: "False",
      reason: "ZeebeNotReady",
      message: "1 of 3 Zeebe brokers are ready",
    },
  },
};

export const NotReadyWithoutReason: Story = {
  name: "Not ready without reason",
  args: { condition: { status: "False" } },
};

export const Unknown: Story = {
  name: "Unknown",
  args: { condition: { status: "Unknown", reason: "Reconciling" } },
};

export const NoStatus: Story = { name: "No status", args: {} };

export const Loading: Story = { name: "Loading", args: { loading: true } };
