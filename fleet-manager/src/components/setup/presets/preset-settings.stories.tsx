import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { presetFixtures } from "./fixtures";
import { PresetSettings } from "./preset-settings";

const sources = presetFixtures();

const meta = {
  title: "setup/presets/PresetSettings",
  component: PresetSettings,
  args: {
    kind: "CamundaClusterPreset",
    body: sources.clusterPresets[0].spec.cluster,
  },
} satisfies Meta<typeof PresetSettings>;
export default meta;
type Story = StoryObj<typeof meta>;

export const ClusterPreset: Story = { name: "Cluster preset" };

export const ElasticsearchPreset: Story = {
  name: "Elasticsearch preset",
  args: {
    kind: "ElasticsearchClusterPreset",
    body: sources.elasticsearchPresets[0].spec.cluster,
  },
};

export const PostgreSQLPreset: Story = {
  name: "PostgreSQL preset",
  args: {
    kind: "DatabaseServerPreset",
    body: sources.databasePresets[1].spec.server,
  },
};
