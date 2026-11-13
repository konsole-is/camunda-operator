import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import {
  WAITING_FOR_GITOPS,
  WAITING_ON_LIVE,
  WaitingPanel,
} from "./waiting-panel";

const meta = {
  title: "cluster/edit/WaitingPanel",
  component: WaitingPanel,
  args: { onStopWaiting: fn() },
  decorators: [
    (Story) => (
      <div className="max-w-3xl">
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof WaitingPanel>;
export default meta;
type Story = StoryObj<typeof meta>;

export const APushOfThisPage: Story = {
  name: "A push of this page",
  args: { text: WAITING_FOR_GITOPS },
};

export const APushOfAnEarlierVisit: Story = {
  name: "A push of an earlier visit",
  args: { text: WAITING_ON_LIVE },
};
