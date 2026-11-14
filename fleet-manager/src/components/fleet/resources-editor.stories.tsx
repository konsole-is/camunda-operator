import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import { ResourcesEditor } from "./resources-editor";

const meta = {
  title: "fleet/ResourcesEditor",
  component: ResourcesEditor,
  args: { label: "Gateway resources", onChange: fn() },
} satisfies Meta<typeof ResourcesEditor>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Set: Story = {
  name: "Set",
  args: {
    label: "Zeebe resources",
    defaultValue: {
      requests: { cpu: "500m", memory: "2Gi" },
      limits: { cpu: "2", memory: "4Gi" },
    },
  },
};

export const Empty: Story = { name: "Empty" };
