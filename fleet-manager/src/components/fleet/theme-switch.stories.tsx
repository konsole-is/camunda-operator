import type { Decorator, Meta, StoryObj } from "@storybook/nextjs-vite";
import type { ThemeChoice } from "@/lib/theme";
import { ThemeProvider, ThemeSwitch } from "./theme-switch";

// The choice that the switch marks. The theme of the canvas stays the theme
// of the toolbar.
function chosen(choice: ThemeChoice): Decorator {
  return function Chosen(Story) {
    return (
      <ThemeProvider initial={choice}>
        <Story />
      </ThemeProvider>
    );
  };
}

const meta = {
  title: "fleet/ThemeSwitch",
  component: ThemeSwitch,
} satisfies Meta<typeof ThemeSwitch>;
export default meta;
type Story = StoryObj<typeof meta>;

export const System: Story = {
  name: "System chosen",
  decorators: [chosen("system")],
};

export const Light: Story = {
  name: "Light chosen",
  decorators: [chosen("light")],
};

export const Dark: Story = {
  name: "Dark chosen",
  decorators: [chosen("dark")],
};
