import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { NotRead } from "./not-read";

const meta = {
  title: "fleet/NotRead",
  component: NotRead,
} satisfies Meta<typeof NotRead>;
export default meta;
type Story = StoryObj<typeof meta>;

export const OneKind: Story = {
  name: "One kind",
  args: { kinds: ["PluginRequest"] },
};

export const TwoKinds: Story = {
  name: "Two kinds",
  args: { kinds: ["Namespace", "Environment"] },
};

export const Tab: Story = {
  name: "A tab that still shows what it has",
  args: { kinds: ["CamundaOptimize"], tab: true },
};
