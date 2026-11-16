import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { ConnectionDetail } from "./connection-detail";
import { connectionFixtures } from "./fixtures";

const meta = {
  title: "setup/connections/ConnectionDetail",
  component: ConnectionDetail,
  args: {
    target: {
      kind: "SecondaryStorageConfig",
      namespace: "team-a",
      name: "shared-es",
    },
    sources: { ...connectionFixtures(), errors: [] },
  },
} satisfies Meta<typeof ConnectionDetail>;
export default meta;
type Story = StoryObj<typeof meta>;

export const CreatedByHand: Story = { name: "Created by hand" };

export const ManagedByAnElasticsearchCluster: Story = {
  name: "Managed by an Elasticsearch cluster",
  args: {
    target: {
      kind: "SecondaryStorageConfig",
      namespace: "team-a",
      name: "orders-es",
    },
  },
};

export const WrittenByARegistration: Story = {
  name: "Written by a registration",
  args: {
    target: {
      kind: "ObjectStorageConfig",
      namespace: "team-a",
      name: "orders-backup",
    },
  },
};

export const KeptFromARegistration: Story = {
  name: "Kept from a registration",
  args: {
    target: {
      kind: "ObjectStorageConfig",
      namespace: "team-a",
      name: "old-backup",
    },
  },
};

export const ManagementIdentityOfAPlane: Story = {
  name: "Management Identity of a plane",
  args: { target: { kind: "ManagementAuthConfig", name: "plane-a-optimize" } },
};

export const DatabaseServerWithDefaultKeys: Story = {
  name: "Database server with default keys",
  args: {
    target: {
      kind: "DatabaseServerConfig",
      namespace: "team-a",
      name: "postgres",
    },
  },
};

export const NotReady: Story = {
  name: "Not ready",
  args: {
    target: { kind: "DatabaseConfig", namespace: "team-a", name: "orders-db" },
    sources: {
      ...connectionFixtures(),
      databaseConfigs: connectionFixtures().databaseConfigs.map((c) =>
        c.metadata.name === "orders-db"
          ? {
              ...c,
              status: {
                conditions: [
                  {
                    type: "Ready",
                    status: "False" as const,
                    reason: "MissingSecret",
                    message: 'Secret "orders-db" has no key "pass"',
                    lastTransitionTime: "2026-11-10T09:00:00Z",
                  },
                ],
              },
            }
          : c,
      ),
      errors: [],
    },
  },
};
