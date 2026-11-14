import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { planes } from "./fixtures";
import { PlaneList } from "./plane-list";

const meta = {
  title: "planes/PlaneList",
  component: PlaneList,
} satisfies Meta<typeof PlaneList>;
export default meta;
type Story = StoryObj<typeof meta>;

export const MixedPlanes: Story = { name: "Mixed planes", args: { planes } };

export const NoPlanes: Story = { name: "No planes", args: { planes: [] } };
