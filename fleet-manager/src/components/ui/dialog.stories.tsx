import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "./dialog";

const meta = {
  title: "ui/Dialog",
  component: Dialog,
  args: { open: true, onOpenChange: fn() },
  parameters: {
    // An open modal dialog blocks the page it is on, so the docs page shows
    // it in a frame of its own.
    docs: { story: { inline: false, iframeHeight: 320 } },
  },
} satisfies Meta<typeof Dialog>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Open: Story = {
  name: "Open",
  render: (args) => (
    <Dialog {...args}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Delete cluster orders</DialogTitle>
          <DialogDescription>
            The claim goes. The backups stay.
          </DialogDescription>
        </DialogHeader>
      </DialogContent>
    </Dialog>
  ),
};
