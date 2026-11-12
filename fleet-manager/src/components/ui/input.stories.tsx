import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import { Input } from "./input";

const meta = {
  title: "ui/Input",
  component: Input,
  args: { onChange: fn() },
  decorators: [
    (Story) => (
      <div className="flex max-w-sm flex-col gap-2">
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof Input>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Enabled: Story = {
  name: "Enabled",
  args: { id: "story-name", defaultValue: "orders" },
  render: (args) => (
    <>
      <label htmlFor={args.id} className="text-sm font-medium text-muted">
        Name
      </label>
      <Input {...args} />
    </>
  ),
};

export const Disabled: Story = {
  name: "Disabled",
  args: { id: "story-ns", defaultValue: "team-orders", disabled: true },
  render: (args) => (
    <>
      <label htmlFor={args.id} className="text-sm font-medium text-muted">
        Namespace
      </label>
      <Input {...args} />
    </>
  ),
};
