import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import { presetFixtures } from "./fixtures";
import { PresetDetail } from "./preset-detail";

const meta = {
  title: "setup/presets/PresetDetail",
  component: PresetDetail,
  args: {
    sources: { ...presetFixtures(), errors: [] },
    kind: "CamundaClusterPreset",
    name: "medium",
    mode: "direct",
    actions: {
      previewChange: fn(async () => ({ objects: [], errors: [] })),
      applyChange: fn(async () => ({
        ok: true as const,
        mode: "direct" as const,
      })),
    },
  },
} satisfies Meta<typeof PresetDetail>;
export default meta;
type Story = StoryObj<typeof meta>;

export const UsedClusterPreset: Story = { name: "Used cluster preset" };

export const UnusedElasticsearchPresetWithDelete: Story = {
  name: "Unused Elasticsearch preset with delete",
  args: { kind: "ElasticsearchClusterPreset", name: "standard" },
};

export const GitMode: Story = {
  name: "Git mode",
  args: { mode: "git", name: "spare" },
};
