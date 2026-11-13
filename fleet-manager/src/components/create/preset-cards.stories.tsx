import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import { presetUsage, presets } from "./fixtures";
import { PresetCards } from "./preset-cards";

const meta = {
  title: "create/PresetCards",
  component: PresetCards,
  args: {
    presets,
    usage: new Map(Object.entries(presetUsage)),
    onChange: fn(),
  },
} satisfies Meta<typeof PresetCards>;
export default meta;
type Story = StoryObj<typeof meta>;

export const ThreePresets: Story = {
  name: "Three presets",
  args: { value: "medium" },
};
