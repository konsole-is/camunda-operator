import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "./select";

const meta = {
  title: "ui/Select",
  component: Select,
  args: { defaultValue: "prod", onValueChange: fn() },
  decorators: [
    (Story) => (
      <div className="flex max-w-sm flex-col gap-2">
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof Select>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Closed: Story = {
  name: "Closed",
  render: (args) => (
    <>
      <span id="story-env" className="text-sm font-medium text-muted">
        Environment
      </span>
      <Select {...args}>
        <SelectTrigger aria-labelledby="story-env">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="prod">prod</SelectItem>
          <SelectItem value="staging">staging</SelectItem>
        </SelectContent>
      </Select>
    </>
  ),
};
