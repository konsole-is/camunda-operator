import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import { Button } from "./button";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "./tooltip";

const meta = {
  title: "ui/Tooltip",
  component: Tooltip,
  args: { defaultOpen: true, onOpenChange: fn() },
  decorators: [
    (Story) => (
      <div className="pt-12">
        <TooltipProvider>
          <Story />
        </TooltipProvider>
      </div>
    ),
  ],
} satisfies Meta<typeof Tooltip>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Open: Story = {
  name: "Open",
  render: (args) => (
    <Tooltip {...args}>
      <TooltipTrigger asChild>
        <Button variant="outline">Version</Button>
      </TooltipTrigger>
      <TooltipContent>Set by the release</TooltipContent>
    </Tooltip>
  ),
};
