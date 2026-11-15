import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { UsersTable } from "./users-table";

const meta = {
  title: "setup/connections/UsersTable",
  component: UsersTable,
  args: {
    users: [
      {
        kind: "CamundaCluster",
        namespace: "team-a",
        name: "billing",
        fields: ["spec.storageRef"],
        ready: {
          status: "False",
          reason: "StorageAlreadyAttached",
          message: "Another cluster holds this storage backend: team-a/orders",
        },
        environment: "production",
      },
      {
        kind: "CamundaCluster",
        namespace: "team-a",
        name: "orders",
        fields: ["spec.backupStorageRef", "spec.storageRef"],
        ready: {
          status: "True",
          reason: "Healthy",
          message: "All checks passed",
        },
        environment: "production",
      },
    ],
  },
} satisfies Meta<typeof UsersTable>;
export default meta;
type Story = StoryObj<typeof meta>;

export const UsersWithARefusedCluster: Story = {
  name: "Users with a refused cluster",
};

export const AReferenceFromAPreset: Story = {
  name: "A reference from a preset",
  args: {
    users: [
      {
        kind: "ElasticsearchCluster",
        namespace: "team-a",
        name: "orders-es",
        fields: ["spec.snapshotStorageRef"],
        preset: "es-small",
        ready: {
          status: "True",
          reason: "Healthy",
          message: "All checks passed",
        },
      },
      {
        kind: "DatabaseServer",
        namespace: "team-a",
        name: "orders-pg",
        fields: [
          "spec.archive.objectStorageRef",
          "status.archive.history.objectStorageRef",
        ],
      },
    ],
  },
};

export const NoUser: Story = { name: "No user", args: { users: [] } };

export const AKindThatCannotBeRead: Story = {
  name: "A kind that cannot be read",
  args: { users: [], unread: ["CamundaCluster", "DatabaseServerPreset"] },
};
