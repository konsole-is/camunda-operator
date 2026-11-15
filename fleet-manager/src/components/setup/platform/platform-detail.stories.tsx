import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import {
  SecretRefsReaderContext,
  staticSecretRefs,
} from "@/components/fleet/namespaced-secret-ref-picker";
import { platformFixtures, twoMinorTable, withNextMinor } from "./fixtures";
import { PlatformDetail } from "./platform-detail";

// A story must not read live Secrets, so the pickers read fixed lists.
const SECRETS = staticSecretRefs({
  "camunda-system": [
    { name: "oidc-credentials", keys: ["client-secret", "identity-secret"] },
  ],
});

const meta = {
  title: "setup/platform/PlatformDetail",
  component: PlatformDetail,
  args: {
    sources: { ...platformFixtures(), errors: [] },
    imagesTable: twoMinorTable,
    name: "oidc-prod",
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
  // The detail keeps the draft of the config that it opened with, so a
  // change of the config from the Controls panel remounts it.
  render: (args) => (
    <PlatformDetail
      key={`${args.name}:${JSON.stringify(
        args.sources.platformConfigs.find((c) => c.metadata.name === args.name),
      )}`}
      {...args}
    />
  ),
} satisfies Meta<typeof PlatformDetail>;
export default meta;
type Story = StoryObj<typeof meta>;

export const OidcWithClients: Story = {
  name: "OIDC with management plane clients",
};

export const MissingSecret: Story = {
  name: "Missing Secret",
  args: { name: "broken" },
};

export const UsersOnTwoMinors: Story = {
  name: "Users on two minors",
  args: { sources: { ...withNextMinor(platformFixtures()), errors: [] } },
};

export const UnusedReadyToDelete: Story = {
  name: "Unused, ready to delete",
  args: { name: "basic-lab" },
};
