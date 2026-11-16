import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { ConditionBadge } from "./condition-badge";
import { condition } from "./fixtures";

const meta = {
  title: "setup/plugins/ConditionBadge",
  component: ConditionBadge,
  args: {
    type: "Accepted",
    condition: condition("Accepted", "True", "Healthy", "All checks passed"),
  },
} satisfies Meta<typeof ConditionBadge>;
export default meta;
type Story = StoryObj<typeof meta>;

export const True: Story = { name: "True" };

export const FalseWithAReason: Story = {
  name: "False with a reason",
  args: {
    condition: condition(
      "Accepted",
      "False",
      "SectionMissing",
      "These sections do not exist: gone",
    ),
  },
};

export const NoStatus: Story = {
  name: "No status",
  args: { type: "Ready", condition: undefined },
};

export const NotKnown: Story = {
  name: "Not known",
  args: { type: "Ready", unknown: true },
};
