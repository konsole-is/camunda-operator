import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import { Button } from "./button";

const meta = {
  title: "ui/Button",
  component: Button,
  args: { children: "Create cluster", onClick: fn() },
} satisfies Meta<typeof Button>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Variants: Story = {
  name: "Variants",
  render: (args) => (
    <div className="flex flex-wrap gap-3">
      <Button {...args}>Create cluster</Button>
      <Button {...args} variant="secondary">
        Cancel
      </Button>
      <Button {...args} variant="outline">
        Edit
      </Button>
      <Button {...args} variant="ghost">
        More
      </Button>
      <Button {...args} variant="destructive">
        Delete
      </Button>
      <Button {...args} variant="link">
        View
      </Button>
    </div>
  ),
};

export const Disabled: Story = {
  name: "Disabled",
  args: { children: "Saving", disabled: true },
};

export const Sizes: Story = {
  name: "Sizes",
  render: (args) => (
    <div className="flex flex-wrap items-center gap-3">
      <Button {...args} size="xs">
        Extra small
      </Button>
      <Button {...args} size="sm">
        Small
      </Button>
      <Button {...args}>Default</Button>
      <Button {...args} size="lg">
        Large
      </Button>
    </div>
  ),
};
