import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { TopNav } from "./top-nav";

const meta = {
  title: "fleet/TopNav",
  component: TopNav,
} satisfies Meta<typeof TopNav>;
export default meta;
type Story = StoryObj<typeof meta>;

export const FleetActive: Story = {
  name: "Fleet active",
  args: { pathname: "/fleet" },
};

export const SetupActive: Story = {
  name: "Setup active",
  args: { pathname: "/setup/presets" },
};
