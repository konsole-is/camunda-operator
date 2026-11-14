import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { SetupNav } from "./setup-nav";

const meta = {
  title: "fleet/SetupNav",
  component: SetupNav,
} satisfies Meta<typeof SetupNav>;
export default meta;
type Story = StoryObj<typeof meta>;

export const PresetsCurrent: Story = {
  name: "Presets current",
  args: { pathname: "/setup/presets" },
};
