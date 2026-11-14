import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { Toast } from "./toast";

const meta = {
  title: "fleet/Toast",
  component: Toast,
} satisfies Meta<typeof Toast>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Saved: Story = {
  name: "Saved",
  args: { result: { kind: "saved", message: "Saved the claim orders" } },
};

export const WaitingForGitOps: Story = {
  name: "Waiting for GitOps",
  args: { result: { kind: "waiting", commit: "4f1c3a7e9b2d8c6f0a5e1b3d" } },
};

export const Applied: Story = {
  name: "Applied",
  args: {
    result: {
      kind: "applied",
      message: "GitOps applied the claim orders",
      commit: "4f1c3a7e",
    },
  },
};

export const Error: Story = {
  name: "Error",
  args: {
    result: {
      kind: "error",
      message:
        "The claim changed since you opened it. Reload it to see the change.",
    },
  },
};
