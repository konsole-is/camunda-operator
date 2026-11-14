import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { StatusDot } from "./status-dot";

const meta = {
  title: "fleet/StatusDot",
  component: StatusDot,
  args: { label: "Zeebe" },
} satisfies Meta<typeof StatusDot>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Ready: Story = {
  name: "Ready",
  args: { condition: { status: "True" } },
};

export const NotReady: Story = {
  name: "Not ready",
  args: { condition: { status: "False", reason: "PodsNotReady" } },
};

export const Unknown: Story = {
  name: "Unknown",
  args: { condition: { status: "Unknown" } },
};

export const NoStatus: Story = { name: "No status" };

export const NotUsed: Story = {
  name: "Not used",
  args: { label: "Backups", off: true },
};
