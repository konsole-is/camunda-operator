import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import { ReviewPanel } from "@/components/fleet/review-panel";
import { objectDiff } from "@/lib/diff";

const meta = {
  title: "cluster/ReviewPanel",
  component: ReviewPanel,
  args: { onConfirm: fn() },
} satisfies Meta<typeof ReviewPanel>;
export default meta;
type Story = StoryObj<typeof meta>;

export const TypedConfirm: Story = {
  name: "Typed confirm",
  args: {
    lines: objectDiff(
      { spec: { cluster: { zeebe: { replicas: 3 } } } },
      { spec: { cluster: { zeebe: { replicas: 5 } } } },
    ),
    mode: "direct",
    confirmLabel: "Change anyway",
    danger: true,
    typed: "orders",
    effects: ["Broker pods restart."],
  },
};
