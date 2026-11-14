import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { PrecheckLine } from "./precheck-line";

const meta = {
  title: "fleet/PrecheckLine",
  component: PrecheckLine,
} satisfies Meta<typeof PrecheckLine>;
export default meta;
type Story = StoryObj<typeof meta>;

export const SlotConflict: Story = {
  name: "SlotConflict",
  args: {
    claim: {
      status: {
        conditions: [
          {
            type: "Ready",
            status: "False",
            reason: "SlotConflict",
            message:
              'spec.cluster.externalUrl is filled by section "access", so the claim cannot set it',
          },
        ],
      },
    },
  },
};
