import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { AppShell } from "./app-shell";
import { claimPush, heading, now } from "./fixtures";
import { GitBannerView } from "./git-banner";
import { GitWaitingLine } from "./git-waiting-line";

const meta = {
  title: "fleet/GitBannerView",
  component: GitBannerView,
  args: {
    count: 1,
    children: <GitWaitingLine pushed={claimPush} now={now} />,
  },
} satisfies Meta<typeof GitBannerView>;
export default meta;
type Story = StoryObj<typeof meta>;

export const OneChangeWaiting: Story = { name: "One change waiting" };

export const NothingWaiting: Story = {
  name: "Nothing waiting",
  args: { count: 0, children: null },
  render: (args) => (
    <AppShell pathname="/fleet" banner={<GitBannerView {...args} />}>
      {heading("Fleet")}
    </AppShell>
  ),
};
