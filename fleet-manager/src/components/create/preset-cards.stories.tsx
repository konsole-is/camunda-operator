import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { type ComponentProps, useState } from "react";
import { fn } from "storybook/test";
import { presetUsage, presets } from "./fixtures";
import { PresetCards } from "./preset-cards";

// The create flow holds the picked preset, so the story holds it too.
function PresetCardsHarness(props: ComponentProps<typeof PresetCards>) {
  const [value, setValue] = useState(props.value);
  return (
    <PresetCards
      {...props}
      value={value}
      onChange={(name) => {
        props.onChange?.(name);
        setValue(name);
      }}
    />
  );
}

const meta = {
  title: "create/PresetCards",
  component: PresetCards,
  args: {
    presets,
    usage: new Map(Object.entries(presetUsage)),
    onChange: fn(),
  },
  render: (args) => <PresetCardsHarness key={args.value} {...args} />,
} satisfies Meta<typeof PresetCards>;
export default meta;
type Story = StoryObj<typeof meta>;

export const ThreePresets: Story = {
  name: "Three presets",
  args: { value: "medium" },
};
