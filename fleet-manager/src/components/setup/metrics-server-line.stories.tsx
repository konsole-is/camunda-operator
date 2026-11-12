import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { MetricsServerLine } from "./metrics-server-line";

const meta = {
  title: "setup/MetricsServerLine",
  component: MetricsServerLine,
  args: { state: { found: true } },
} satisfies Meta<typeof MetricsServerLine>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Found: Story = { name: "Found" };

export const NotFound: Story = {
  name: "Not found",
  args: { state: { found: false } },
};

export const CheckFailed: Story = {
  name: "Check failed",
  args: { state: { error: "connect ECONNREFUSED 10.96.0.1:443" } },
};
