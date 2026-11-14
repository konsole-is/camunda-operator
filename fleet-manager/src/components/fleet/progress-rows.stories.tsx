import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import Link from "next/link";
import { ProgressRows } from "./progress-rows";

const meta = {
  title: "fleet/ProgressRows",
  component: ProgressRows,
} satisfies Meta<typeof ProgressRows>;
export default meta;
type Story = StoryObj<typeof meta>;

export const WaitingForOutputs: Story = {
  name: "Waiting for outputs",
  args: {
    ready: {
      status: "False",
      reason: "WaitingForOutputs",
      message: "Waiting for Backup storage and Secondary storage",
    },
    rows: [
      {
        id: "backup",
        label: "Backup storage",
        detail: "New S3 bucket",
        summary: "Creating the bucket orders-backups",
        checks: [
          {
            label: "Outputs",
            condition: { status: "False", reason: "Provisioning" },
          },
          {
            label: "Ready",
            condition: { status: "False", reason: "Provisioning" },
          },
        ],
      },
      {
        id: "storage",
        label: "Secondary storage",
        detail: "New Elasticsearch",
        checks: [
          { label: "Outputs", condition: { status: "True" } },
          {
            label: "Ready",
            condition: { status: "Unknown", reason: "Converging" },
          },
        ],
      },
      {
        id: "cluster",
        label: "Cluster",
        checks: [
          {
            label: "RequestsReady",
            condition: { status: "False", reason: "Waiting" },
          },
          { label: "ClusterReady" },
        ],
      },
    ],
  },
};

export const Ready: Story = {
  name: "Ready",
  args: {
    ready: { status: "True", reason: "Healthy" },
    rows: [
      {
        id: "cluster",
        label: "Cluster",
        checks: [
          { label: "RequestsReady", condition: { status: "True" } },
          { label: "ClusterReady", condition: { status: "True" } },
        ],
      },
    ],
    done: (
      <Link href="/fleet/clusters/team-a/orders" className="text-sm text-info">
        Open the cluster orders
      </Link>
    ),
  },
};
