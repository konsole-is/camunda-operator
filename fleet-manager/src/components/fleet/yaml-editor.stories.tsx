import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import { YamlEditor } from "./yaml-editor";

const meta = {
  title: "fleet/YamlEditor",
  component: YamlEditor,
  args: { label: "Extra environment sources", onChange: fn() },
} satisfies Meta<typeof YamlEditor>;
export default meta;
type Story = StoryObj<typeof meta>;

export const NodeAffinity: Story = {
  name: "Node affinity",
  args: {
    label: "Node affinity",
    defaultValue: {
      requiredDuringSchedulingIgnoredDuringExecution: {
        nodeSelectorTerms: [
          {
            matchExpressions: [
              { key: "pool", operator: "In", values: ["camunda"] },
            ],
          },
        ],
      },
    },
  },
};

export const Empty: Story = { name: "Empty" };
