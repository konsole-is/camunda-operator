import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import { presetFixtures } from "./fixtures";
import { PresetList } from "./preset-list";
import { presetRows } from "./rows";

const meta = {
  title: "setup/presets/PresetList",
  component: PresetList,
  args: {
    rows: presetRows(presetFixtures(), []),
    errors: [],
    mode: "direct",
    actions: {
      previewChange: fn(async () => ({ objects: [], errors: [] })),
      applyChange: fn(async () => ({
        ok: true as const,
        mode: "direct" as const,
      })),
    },
  },
} satisfies Meta<typeof PresetList>;
export default meta;
type Story = StoryObj<typeof meta>;

export const ThreeKindsUsedAndUnused: Story = {
  name: "Three kinds, used and unused",
};

export const NoPreset: Story = {
  name: "No preset",
  args: { rows: [] },
};

const unreadable = [{ kind: "ElasticsearchCluster", message: "forbidden" }];

export const AListThatCannotBeRead: Story = {
  name: "A list that cannot be read",
  args: { rows: presetRows(presetFixtures(), unreadable), errors: unreadable },
};

export const GitMode: Story = {
  name: "Git mode",
  args: { mode: "git" },
};
