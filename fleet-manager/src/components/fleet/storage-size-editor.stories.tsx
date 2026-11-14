import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import { StorageSizeEditor } from "./storage-size-editor";

const meta = {
  title: "fleet/StorageSizeEditor",
  component: StorageSizeEditor,
  args: { label: "storageSize", onChange: fn() },
} satisfies Meta<typeof StorageSizeEditor>;
export default meta;
type Story = StoryObj<typeof meta>;

export const BelowTheCurrentSize: Story = {
  name: "Below the current size",
  args: { defaultValue: "16Gi", current: "32Gi" },
};

export const NotSet: Story = { name: "Not set" };
