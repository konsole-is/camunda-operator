import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import { ChangeBar } from "./change-bar";

const meta = {
  title: "fleet/ChangeBar",
  component: ChangeBar,
  args: { count: 1, onDiscard: fn(), onReview: fn() },
} satisfies Meta<typeof ChangeBar>;
export default meta;
type Story = StoryObj<typeof meta>;

export const OneChange: Story = { name: "One change" };

export const ThreeChanges: Story = {
  name: "Three changes",
  args: { count: 3 },
};

export const BlockedByErrors: Story = {
  name: "Blocked by errors",
  args: {
    count: 2,
    errors: [
      {
        path: "spec.releaseRef",
        message: "Release camunda-8-10-0 does not exist.",
      },
      {
        message: "The section access does not allow the registration aws-alb.",
      },
    ],
  },
};

export const Blocked: Story = {
  name: "Blocked",
  args: {
    blocked: "The default release must be one of the allowed releases.",
  },
};
