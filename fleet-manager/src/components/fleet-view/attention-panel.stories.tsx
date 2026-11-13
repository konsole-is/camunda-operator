import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import type { Resource } from "@/lib/k8s/kinds";
import { attentionItems } from "./attention";
import { AttentionPanel } from "./attention-panel";
import { NOW, fleet, planes } from "./fixtures";
import { clusterRows } from "./model";

const downPlanes = planes.map((plane) => ({
  ...plane,
  ready: {
    status: "False" as const,
    reason: "Degraded",
    message: "The Keycloak pod is not ready",
  },
}));

const failingRegistration = {
  kind: "PluginRegistration",
  metadata: { name: "aws-s3-bucket" },
  spec: { label: "Bucket on Amazon S3" },
  status: {
    conditions: [
      {
        type: "Ready",
        status: "False",
        reason: "ControllerNotRunning",
        message: "No controller pod of the plugin is ready",
      },
    ],
  },
} as unknown as Resource<"PluginRegistration">;

const meta = {
  title: "fleet-view/AttentionPanel",
  component: AttentionPanel,
} satisfies Meta<typeof AttentionPanel>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Mixed: Story = {
  name: "Mixed",
  args: {
    items: attentionItems({
      rows: clusterRows(fleet, NOW),
      backups: fleet.backups,
      registrations: [failingRegistration],
      planes: downPlanes,
      now: NOW,
    }),
  },
};
