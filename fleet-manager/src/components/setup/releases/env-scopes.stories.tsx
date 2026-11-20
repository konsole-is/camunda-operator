import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import type { CamundaReleaseSpec } from "@/generated/crds/camundarelease";
import { EnvScopes } from "./env-scopes";
import { releaseFixtures } from "./fixtures";

const noneMissing: (name: string, key: string) => readonly string[] = () => [];

const meta = {
  title: "setup/releases/EnvScopes",
  component: EnvScopes,
  args: {
    spec: { version: "8.9.0" } as CamundaReleaseSpec,
    secrets: [{ name: "orders-db", keys: ["password"] }],
    missingIn: noneMissing,
    onChange: fn(),
  },
} satisfies Meta<typeof EnvScopes>;
export default meta;
type Story = StoryObj<typeof meta>;

export const NoEntries: Story = { name: "No entries" };

export const EntriesInTwoScopes: Story = {
  name: "Entries in two scopes",
  args: { spec: releaseFixtures().releases[0].spec },
};

export const SecretMissingInANamespace: Story = {
  name: "Secret missing in a namespace",
  args: {
    spec: releaseFixtures().releases[0].spec,
    missingIn: (name: string) => (name === "orders-db" ? ["team-billing"] : []),
  },
};
