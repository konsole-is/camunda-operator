import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { ClusterWaiting } from "./cluster-waiting";

const meta = {
  title: "cluster/ClusterWaiting",
  component: ClusterWaiting,
  args: { unread: [] },
} satisfies Meta<typeof ClusterWaiting>;
export default meta;
type Story = StoryObj<typeof meta>;

export const WaitingForTwoRequests: Story = {
  name: "Waiting for two requests",
  args: {
    waits: [
      {
        section: "secondary-storage",
        label: "Secondary storage",
        request: "orders-secondary-storage",
      },
      { section: "access", label: "Access" },
    ],
  },
};

export const WaitingForNothing: Story = {
  name: "Waiting for nothing",
  args: { waits: [] },
};

export const RequestsNotKnown: Story = {
  name: "Requests not known",
  args: { waits: [], unread: ["PluginRequest"] },
};

export const ClaimBeingDeleted: Story = {
  name: "Claim being deleted",
  args: { waits: [], deleting: true },
};
