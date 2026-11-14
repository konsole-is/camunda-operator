import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { objectDiff } from "@/lib/diff";
import { DiffView } from "./diff-view";
import { ordersClaim, scaleDiff } from "./fixtures";

const meta = {
  title: "fleet/DiffView",
  component: DiffView,
  args: { label: "Claim orders", lines: scaleDiff },
} satisfies Meta<typeof DiffView>;
export default meta;
type Story = StoryObj<typeof meta>;

export const ChangedField: Story = { name: "Changed field" };

export const NewObject: Story = {
  name: "New object",
  args: { lines: objectDiff(undefined, ordersClaim) },
};

export const NoChanges: Story = {
  name: "No changes",
  args: { lines: objectDiff(ordersClaim, ordersClaim) },
};
