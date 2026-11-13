import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { GitWaitingLine } from "./git-waiting-line";

const meta = {
  title: "fleet/GitWaitingLine",
  component: GitWaitingLine,
  args: { now: "2026-10-18T12:00:00Z" },
} satisfies Meta<typeof GitWaitingLine>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Waiting: Story = {
  name: "Waiting",
  args: {
    pushed: {
      path: "clusters/team-a/camundaclusterclaims/orders.yaml",
      revision: "bbb",
      pushedAt: "2026-10-18T11:58:00Z",
    },
    liveRevision: "aaa",
  },
};

export const WaitingForADelete: Story = {
  name: "Waiting for a delete",
  args: {
    pushed: {
      path: "clusters/team-a/camundaclusterclaims/orders.yaml",
      pushedAt: "2026-10-18T11:58:00Z",
      deleted: true,
    },
  },
};
