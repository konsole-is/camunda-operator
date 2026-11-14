import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import { scaleDiff } from "./fixtures";
import { ReviewPanel } from "./review-panel";

const meta = {
  title: "fleet/ReviewPanel",
  component: ReviewPanel,
  args: {
    lines: scaleDiff,
    mode: "direct",
    confirmLabel: "Change the claim",
    effects: ["Zeebe gets two more brokers."],
    onConfirm: fn(),
  },
} satisfies Meta<typeof ReviewPanel>;
export default meta;
type Story = StoryObj<typeof meta>;

export const DirectMode: Story = {
  name: "Direct mode",
  args: {
    effects: [
      "Zeebe gets two more brokers.",
      "The partitions move to the new brokers.",
    ],
  },
};

export const GitMode: Story = { name: "Git mode", args: { mode: "git" } };

export const WithCaution: Story = {
  name: "With caution",
  args: {
    danger: true,
    confirmLabel: "Change anyway",
    caution:
      "A backup of orders runs now. The rollout makes it fail. A backup that starts after the change waits until the rollout is done.",
  },
};
