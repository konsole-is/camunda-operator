import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import { TypedConfirm } from "./typed-confirm";

const meta = {
  title: "fleet/TypedConfirm",
  component: TypedConfirm,
  args: {
    name: "orders",
    confirmLabel: "Delete cluster",
    children:
      "The cluster orders and its data go. The backups in the bucket stay.",
    onConfirm: fn(),
  },
} satisfies Meta<typeof TypedConfirm>;
export default meta;
type Story = StoryObj<typeof meta>;

export const WaitingForTheName: Story = { name: "Waiting for the name" };
