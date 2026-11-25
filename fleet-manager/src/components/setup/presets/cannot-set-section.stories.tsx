import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { CannotSetSection } from "./cannot-set-section";

const meta = {
  title: "setup/presets/CannotSetSection",
  component: CannotSetSection,
  args: { kind: "CamundaClusterPreset" },
} satisfies Meta<typeof CannotSetSection>;
export default meta;
type Story = StoryObj<typeof meta>;

export const ClusterPreset: Story = { name: "Cluster preset" };

export const ElasticsearchPreset: Story = {
  name: "Elasticsearch preset",
  args: { kind: "ElasticsearchClusterPreset" },
};

export const PostgreSQLPreset: Story = {
  name: "PostgreSQL preset",
  args: { kind: "DatabaseServerPreset" },
};
