import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import { InlineConfirm } from "./inline-confirm";

const meta = {
  title: "fleet/InlineConfirm",
  component: InlineConfirm,
  args: {
    triggerLabel: "Delete backup",
    confirmLabel: "Delete",
    goes: ["The stored artifacts of the backup"],
    onConfirm: fn(),
  },
} satisfies Meta<typeof InlineConfirm>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Closed: Story = { name: "Closed" };

export const Open: Story = {
  name: "Open",
  args: {
    defaultOpen: true,
    stays: [
      "The other backups of orders",
      "The schedule, which prunes its own backups",
    ],
  },
};
