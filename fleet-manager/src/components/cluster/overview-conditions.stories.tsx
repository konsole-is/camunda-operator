import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { OverviewConditions } from "./overview-conditions";

const now = "2026-10-18T12:00:00Z";

const healthy = (type: string, lastTransitionTime = now) =>
  ({ type, status: "True", reason: "Healthy", lastTransitionTime }) as const;

const meta = {
  title: "cluster/OverviewConditions",
  component: OverviewConditions,
} satisfies Meta<typeof OverviewConditions>;
export default meta;
type Story = StoryObj<typeof meta>;

export const WithOptimizeAndClaim: Story = {
  name: "With Optimize and claim",
  args: {
    cluster: [
      healthy("Ready"),
      {
        type: "AdminSecretReady",
        status: "False",
        reason: "MissingSecret",
        message: "Secret team-a/orders-admin does not exist",
        lastTransitionTime: "2026-10-18T11:40:00Z",
      },
      healthy("MirroredSecretsReady", "2026-10-17T09:00:00Z"),
    ],
    optimize: [healthy("MirroredSecretsReady")],
    claim: [
      {
        type: "RequestsReady",
        status: "False",
        reason: "Blocked",
        message: "The access request waits for its plugin",
        lastTransitionTime: "2026-10-18T11:55:00Z",
      },
      healthy("ClusterReady"),
      healthy("AttachmentsReady"),
    ],
    now,
  },
};
