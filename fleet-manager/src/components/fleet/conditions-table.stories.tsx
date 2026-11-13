import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { ConditionsTable } from "./conditions-table";

const meta = {
  title: "fleet/ConditionsTable",
  component: ConditionsTable,
  args: { now: "2026-10-15T12:00:00Z", conditions: [] },
} satisfies Meta<typeof ConditionsTable>;
export default meta;
type Story = StoryObj<typeof meta>;

export const NotReadyCondition: Story = {
  name: "Not ready condition",
  args: {
    conditions: [
      {
        type: "AdminSecretReady",
        status: "False",
        reason: "SecretMissing",
        message: "The Secret orders-admin does not exist.",
        lastTransitionTime: "2026-10-15T11:48:00Z",
      },
      {
        type: "MirroredSecretsReady",
        status: "True",
        reason: "Healthy",
        message: "",
        lastTransitionTime: "2026-10-14T09:00:00Z",
      },
    ],
  },
};

export const Groups: Story = {
  name: "Groups",
  args: {
    conditions: [
      {
        type: "MirroredSecretsReady",
        status: "True",
        reason: "Healthy",
        lastTransitionTime: "2026-10-14T09:00:00Z",
      },
    ],
    groups: [
      {
        label: "Claim",
        conditions: [
          {
            type: "RequestsReady",
            status: "True",
            reason: "Healthy",
            lastTransitionTime: "2026-10-15T11:00:00Z",
          },
          {
            type: "ClusterReady",
            status: "False",
            reason: "Converging",
            message: "Zeebe has 1 of 3 brokers ready.",
            lastTransitionTime: "2026-10-15T11:58:00Z",
          },
          {
            type: "AttachmentsReady",
            status: "Unknown",
            reason: "Waiting",
            lastTransitionTime: "2026-10-15T11:58:00Z",
          },
        ],
      },
    ],
  },
};

export const Empty: Story = { name: "Empty" };
