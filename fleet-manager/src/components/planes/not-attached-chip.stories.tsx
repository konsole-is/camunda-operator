import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { notAttached } from "./fixtures";
import { NotAttachedChip } from "./not-attached-chip";

const meta = {
  title: "planes/NotAttachedChip",
  component: NotAttachedChip,
  args: { clusters: notAttached },
} satisfies Meta<typeof NotAttachedChip>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Open: Story = { name: "Open", args: { defaultOpen: true } };
