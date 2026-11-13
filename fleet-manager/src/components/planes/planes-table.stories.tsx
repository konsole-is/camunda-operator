import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { planes } from "./fixtures";
import { PlanesTable } from "./planes-table";

const meta = {
  title: "planes/PlanesTable",
  component: PlanesTable,
} satisfies Meta<typeof PlanesTable>;
export default meta;
type Story = StoryObj<typeof meta>;

export const PlanesOfProduction: Story = {
  name: "Planes of production",
  args: { planes, environment: "production" },
};
