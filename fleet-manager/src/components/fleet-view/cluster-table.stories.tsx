import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { ClusterTable } from "./cluster-table";
import { NOW, fleet } from "./fixtures";
import { clusterRows } from "./model";

const meta = {
  title: "fleet-view/ClusterTable",
  component: ClusterTable,
} satisfies Meta<typeof ClusterTable>;
export default meta;
type Story = StoryObj<typeof meta>;

export const MixedStates: Story = {
  name: "Mixed states",
  args: {
    label: "Production",
    rows: clusterRows(fleet, NOW),
    warnings: new Map(),
    now: NOW,
  },
};
