import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import { Radio } from "./radio";

const meta = {
  title: "ui/Radio",
  component: Radio,
  args: { onChange: fn(), name: "deletion" },
} satisfies Meta<typeof Radio>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Group: Story = {
  name: "Group",
  render: (args) => (
    <fieldset className="flex flex-col gap-2">
      <legend className="mb-1 text-sm font-medium">
        When the cluster is deleted
      </legend>
      <label className="flex items-center gap-2">
        <Radio {...args} value="keep" defaultChecked /> Keep the resources
      </label>
      <label className="flex items-center gap-2">
        <Radio {...args} value="delete" /> Delete the resources
      </label>
    </fieldset>
  ),
};

export const Disabled: Story = {
  name: "Disabled",
  render: (args) => (
    <label className="flex items-center gap-2">
      <Radio {...args} disabled /> Keep the resources
    </label>
  ),
};
