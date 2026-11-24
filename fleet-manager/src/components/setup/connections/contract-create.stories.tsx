import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { expect, fn, userEvent, within } from "storybook/test";
import {
  SecretRefsReaderContext,
  staticSecretRefs,
} from "@/components/fleet/namespaced-secret-ref-picker";
import { ContractCreate } from "./contract-create";
import { connectionFixtures } from "./fixtures";

// A story must not read live Secrets, so the pickers read fixed lists.
const SECRETS = staticSecretRefs({
  "team-a": [
    { name: "gcs", keys: ["key.json"] },
    { name: "postgres-admin", keys: ["username", "password"] },
  ],
  platform: [{ name: "plane-a-optimize-client", keys: ["client-secret"] }],
});

const meta = {
  title: "setup/connections/ContractCreate",
  component: ContractCreate,
  args: {
    tab: "object-storage",
    sources: { ...connectionFixtures(), errors: [] },
    mode: "direct",
    actions: {
      previewChange: fn(async () => ({ objects: [], errors: [] })),
      applyChange: fn(async () => ({
        ok: true as const,
        mode: "direct" as const,
      })),
    },
  },
  decorators: [
    (Story) => (
      <SecretRefsReaderContext.Provider value={SECRETS}>
        <Story />
      </SecretRefsReaderContext.Provider>
    ),
  ],
} satisfies Meta<typeof ContractCreate>;
export default meta;
type Story = StoryObj<typeof meta>;

// inTeamA picks the namespace team-a, so the form of the contract shows.
const inTeamA = async ({ canvasElement }: { canvasElement: HTMLElement }) => {
  await userEvent.selectOptions(
    within(canvasElement).getByLabelText("Namespace", { exact: true }),
    "team-a",
  );
};

export const ObjectStorage: Story = {
  name: "Object storage",
  play: async (context) => {
    await inTeamA(context);
    const canvas = within(context.canvasElement);
    await userEvent.click(canvas.getByRole("radio", { name: "GCS" }));
    await expect(canvas.getByLabelText("Bucket name")).toBeVisible();
    await expect(canvas.queryByLabelText("Region")).toBeNull();
  },
};

export const SecondaryStorage: Story = {
  name: "Secondary storage",
  args: { tab: "secondary-storage" },
  play: inTeamA,
};

export const DatabaseServer: Story = {
  name: "Database server",
  args: { tab: "database-servers" },
  play: inTeamA,
};

export const Database: Story = {
  name: "Database",
  args: { tab: "databases" },
  play: inTeamA,
};

export const ManagementIdentity: Story = {
  name: "Management Identity",
  args: { tab: "management-identity" },
};

export const ChangesOff: Story = {
  name: "Changes off",
  args: { mode: undefined },
};
