import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import {
  SecretRefsReaderContext,
  staticSecretRefs,
} from "@/components/fleet/namespaced-secret-ref-picker";
import { AuthSection } from "./auth-section";
import { draftOf } from "./draft";
import { platformFixtures } from "./fixtures";

// A story must not read live Secrets, so the pickers read fixed lists.
const SECRETS = staticSecretRefs({
  "camunda-system": [
    { name: "oidc-credentials", keys: ["client-secret", "identity-secret"] },
    { name: "camunda-license", keys: ["license-key"] },
  ],
});

const draftNamed = (name: string) =>
  draftOf(
    platformFixtures().platformConfigs.find((c) => c.metadata.name === name),
  );

const meta = {
  title: "setup/platform/AuthSection",
  component: AuthSection,
  args: {
    draft: draftNamed("oidc-prod"),
    namespaces: ["camunda-system", "team-a"],
    errors: [],
    onChange: fn(),
  },
  decorators: [
    (Story) => (
      <SecretRefsReaderContext.Provider value={SECRETS}>
        <Story />
      </SecretRefsReaderContext.Provider>
    ),
  ],
} satisfies Meta<typeof AuthSection>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Oidc: Story = { name: "OIDC" };

export const Basic: Story = {
  name: "Basic",
  args: { draft: draftNamed("basic-lab") },
};
