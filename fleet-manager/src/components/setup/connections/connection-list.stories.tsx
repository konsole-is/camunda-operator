import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import {
  ConnectionList,
  type ContractRow,
  contractRows,
} from "./connection-list";
import { connectionFixtures } from "./fixtures";
import type { ConnectionSources } from "./sources";
import { CONNECTION_TABS, type ConnectionTab } from "./tabs";

const rowsOf = (sources: ConnectionSources) =>
  Object.fromEntries(
    CONNECTION_TABS.map((t) => [t.value, contractRows(sources, t.kind)]),
  ) as Record<ConnectionTab, ContractRow[]>;

const meta = {
  title: "setup/connections/ConnectionList",
  component: ConnectionList,
  args: {
    tab: "object-storage",
    rows: rowsOf(connectionFixtures()),
    errors: [],
    onTab: () => {},
  },
} satisfies Meta<typeof ConnectionList>;
export default meta;
type Story = StoryObj<typeof meta>;

export const ObjectStorageWithEveryWriter: Story = {
  name: "Object storage with every writer",
};

export const SecondaryStorageWithAnOwner: Story = {
  name: "Secondary storage with an owner",
  args: { tab: "secondary-storage" },
};

export const ManagementIdentityWrittenByAPlane: Story = {
  name: "Management Identity written by a plane",
  args: { tab: "management-identity" },
};

export const AnEmptyTab: Story = {
  name: "An empty tab",
  args: {
    tab: "databases",
    rows: rowsOf({ ...connectionFixtures(), databaseConfigs: [] }),
  },
};

export const AReadError: Story = {
  name: "A read error",
  args: {
    errors: [
      {
        kind: "PluginRequest",
        message:
          'pluginrequests.plugins.camunda.io is forbidden: User "system:serviceaccount:camunda-system:fleet-manager" cannot list resource "pluginrequests"',
      },
    ],
  },
};
