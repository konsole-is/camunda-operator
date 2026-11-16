import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { expect, fn, userEvent, within } from "storybook/test";
import {
  SecretRefsReaderContext,
  staticSecretRefs,
} from "@/components/fleet/namespaced-secret-ref-picker";
import type { Change } from "@/lib/write/types";
import { ConnectionDetail } from "./connection-detail";
import { connectionFixtures } from "./fixtures";

// A story must not read live Secrets, so the pickers read fixed lists.
const SECRETS = staticSecretRefs({
  "team-a": [
    { name: "postgres-admin", keys: ["username", "password"] },
    { name: "orders-db", keys: ["user", "pass"] },
  ],
});

// The dry run keeps exactly the applied spec, with one changed line.
const actions = {
  previewChange: fn(async (change: Change) => ({
    objects: [...change.apply, ...(change.delete ?? [])].map((object) => {
      const ref =
        "metadata" in object
          ? {
              ...object.metadata,
              apiVersion: object.apiVersion,
              kind: object.kind,
            }
          : object;
      return {
        ref: {
          apiVersion: ref.apiVersion,
          kind: ref.kind,
          namespace: ref.namespace,
          name: ref.name,
        },
        diff: [{ op: "added" as const, text: "host: pg-2.team-a" }],
      };
    }),
    errors: [],
  })),
  applyChange: fn(async () => ({ ok: true as const, mode: "direct" as const })),
};

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
  decorators: [
    (Story) => (
      <SecretRefsReaderContext.Provider value={SECRETS}>
        <Story />
      </SecretRefsReaderContext.Provider>
    ),
  ],
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

const unusedServer = {
  ...connectionFixtures(),
  databaseConfigs: [],
  errors: [],
};

export const EditingAContractCreatedByHand: Story = {
  name: "Editing a contract created by hand",
  args: {
    target: {
      kind: "DatabaseServerConfig",
      namespace: "team-a",
      name: "postgres",
    },
    mode: "direct",
    actions,
  },
};

export const ReviewOfASave: Story = {
  name: "Review of a save",
  args: {
    target: {
      kind: "DatabaseServerConfig",
      namespace: "team-a",
      name: "postgres",
    },
    mode: "direct",
    actions,
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const host = canvas.getByLabelText("Host");
    await userEvent.clear(host);
    await userEvent.type(host, "pg-2.team-a");
    await userEvent.click(canvas.getByRole("button", { name: "Review" }));
    await expect(
      await canvas.findByRole("region", { name: "Review the contract" }),
    ).toBeVisible();
  },
};

export const DeleteOfAnUnusedContract: Story = {
  name: "Delete of an unused contract",
  args: {
    target: {
      kind: "DatabaseServerConfig",
      namespace: "team-a",
      name: "postgres",
    },
    sources: unusedServer,
    mode: "direct",
    actions,
  },
};

export const DeleteRefusedForAUsedContract: Story = {
  name: "Delete refused for a used contract",
  args: {
    target: { kind: "DatabaseConfig", namespace: "team-a", name: "orders-db" },
    sources: {
      ...connectionFixtures(),
      secondaryStorageConfigs: [
        ...connectionFixtures().secondaryStorageConfigs,
        {
          apiVersion: "core.camunda.io/v1",
          kind: "SecondaryStorageConfig",
          metadata: { namespace: "team-a", name: "orders-rdbms" },
          spec: { type: "rdbms", rdbms: { databaseConfigRef: "orders-db" } },
        },
      ],
      errors: [],
    },
    mode: "direct",
    actions,
  },
};
