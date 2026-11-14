import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { LockedField } from "./locked-field";

const meta = {
  title: "fleet/LockedField",
  component: LockedField,
  args: { label: "Version", value: "8.9.23", reason: "Set by the release" },
} satisfies Meta<typeof LockedField>;
export default meta;
type Story = StoryObj<typeof meta>;

export const ShortValue: Story = { name: "Short value" };

export const LongValue: Story = {
  name: "Long value",
  args: {
    label: "Image",
    value:
      "registry.example.com/camunda/zeebe@sha256:4f1c3a7e9b2d8c6f0a5e1b3d7c9f2a4e6b8d0c2f4a6e8b0d2c4f6a8e0b2d4c6f",
    reason: "Set by the environment policy",
  },
};
