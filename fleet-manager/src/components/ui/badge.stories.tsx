import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { Badge } from "./badge";

const meta = {
  title: "ui/Badge",
  component: Badge,
} satisfies Meta<typeof Badge>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Variants: Story = {
  name: "Variants",
  render: (args) => (
    <div className="flex flex-wrap gap-2">
      <Badge {...args}>Default</Badge>
      <Badge {...args} variant="secondary">
        Optional
      </Badge>
      <Badge {...args} variant="outline">
        Required
      </Badge>
      <Badge {...args} variant="destructive">
        Refused
      </Badge>
      <Badge {...args} variant="ghost">
        Draft
      </Badge>
      <Badge {...args} variant="link">
        Details
      </Badge>
    </div>
  ),
};
