import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { ConditionList } from "./condition-list";

const meta = {
  title: "fleet/ConditionList",
  component: ConditionList,
  args: {
    now: "2026-10-15T12:00:00Z",
    label: "Optimize conditions",
    conditions: [],
  },
  decorators: [
    (Story) => (
      <div className="max-w-80">
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof ConditionList>;
export default meta;
type Story = StoryObj<typeof meta>;

export const OneAppDegraded: Story = {
  name: "One app degraded",
  args: {
    conditions: [
      {
        type: "Ready",
        status: "False",
        reason: "Degraded",
        message: "The importer has 0 of 1 pods ready.",
        lastTransitionTime: "2026-10-15T11:48:00Z",
      },
      {
        type: "WebappReady",
        status: "True",
        reason: "Healthy",
        message: "",
        lastTransitionTime: "2026-10-14T09:00:00Z",
      },
      {
        type: "ImporterReady",
        status: "False",
        reason: "Degraded",
        message: "0 of 1 pods ready",
        lastTransitionTime: "2026-10-15T11:48:00Z",
      },
    ],
  },
};

export const NoConditions: Story = { name: "No conditions" };
