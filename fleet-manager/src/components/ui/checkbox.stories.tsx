import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import { Checkbox } from "./checkbox";

const meta = {
  title: "ui/Checkbox",
  component: Checkbox,
  args: { onCheckedChange: fn() },
} satisfies Meta<typeof Checkbox>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Checked: Story = {
  name: "Checked",
  args: { defaultChecked: true },
  render: (args) => (
    <label className="flex items-center gap-2">
      <Checkbox {...args} /> Keep the resources
    </label>
  ),
};

export const Unchecked: Story = {
  name: "Unchecked",
  render: (args) => (
    <label className="flex items-center gap-2">
      <Checkbox {...args} /> Delete the backups
    </label>
  ),
};
