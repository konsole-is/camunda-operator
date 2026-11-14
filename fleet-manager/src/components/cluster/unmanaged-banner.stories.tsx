import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { UnmanagedBanner } from "./banners";

const meta = {
  title: "cluster/UnmanagedBanner",
  component: UnmanagedBanner,
} satisfies Meta<typeof UnmanagedBanner>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = { name: "Default" };
