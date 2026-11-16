import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { presetFixtures } from "./fixtures";
import { inheritingRows } from "./inheriting";
import { InheritingTable } from "./inheriting-table";

const sources = presetFixtures();

const meta = {
  title: "setup/presets/InheritingTable",
  component: InheritingTable,
  args: {
    rows: inheritingRows(sources.clusterPresets[0], sources),
    unread: [],
  },
} satisfies Meta<typeof InheritingTable>;
export default meta;
type Story = StoryObj<typeof meta>;

export const AClusterWithASlotFilledField: Story = {
  name: "A cluster with a slot-filled field",
};

export const NoInheritingResource: Story = {
  name: "No inheriting resource",
  args: { rows: [] },
};

export const InheritingResourcesNotKnown: Story = {
  name: "Inheriting resources not known",
  args: { rows: [], unread: ["CamundaCluster"] },
};
