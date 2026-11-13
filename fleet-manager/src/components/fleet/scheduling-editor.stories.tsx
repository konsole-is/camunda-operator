import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import { SchedulingEditor } from "./scheduling-editor";

const meta = {
  title: "fleet/SchedulingEditor",
  component: SchedulingEditor,
  args: { label: "Tolerations", onChange: fn() },
} satisfies Meta<typeof SchedulingEditor>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Tolerations: Story = {
  name: "Tolerations",
  args: {
    defaultValue: [
      {
        key: "dedicated",
        operator: "Equal",
        value: "camunda",
        effect: "NoSchedule",
      },
      {
        key: "node.kubernetes.io/unreachable",
        operator: "Exists",
        effect: "NoExecute",
        tolerationSeconds: 300,
      },
    ],
  },
};

export const Empty: Story = { name: "Empty" };
