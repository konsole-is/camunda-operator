import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { FactsList } from "./facts-list";
import { presetFixtures } from "./fixtures";
import { presetBody } from "./kinds";
import { presetFacts } from "./rows";

const sources = presetFixtures();

const meta = {
  title: "setup/presets/FactsList",
  component: FactsList,
  args: {
    facts: presetFacts(
      "CamundaClusterPreset",
      presetBody(sources.clusterPresets[0]),
    ),
  },
} satisfies Meta<typeof FactsList>;
export default meta;
type Story = StoryObj<typeof meta>;

export const ClusterPreset: Story = { name: "Cluster preset" };

export const PostgreSqlPreset: Story = {
  name: "PostgreSQL preset",
  args: {
    facts: presetFacts(
      "DatabaseServerPreset",
      presetBody(sources.databasePresets[1]),
    ),
  },
};
