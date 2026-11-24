import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { expect, fn, userEvent, within } from "storybook/test";
import {
  SecretRefsReaderContext,
  staticSecretRefs,
} from "@/components/fleet/namespaced-secret-ref-picker";
import { platformFixtures, twoMinorTable } from "./fixtures";
import { PlatformCreate } from "./platform-create";

// A story must not read live Secrets, so the pickers read fixed lists.
// camunda-system has no camunda-license, so a copy of broken names a
// Secret that does not exist.
const SECRETS = staticSecretRefs({
  "camunda-system": [
    { name: "oidc-credentials", keys: ["client-secret", "identity-secret"] },
  ],
});

const meta = {
  title: "setup/platform/PlatformCreate",
  component: PlatformCreate,
  args: {
    sources: { ...platformFixtures(), errors: [] },
    imagesTable: twoMinorTable,
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
} satisfies Meta<typeof PlatformCreate>;
export default meta;
type Story = StoryObj<typeof meta>;

export const EmptyConfig: Story = {
  name: "Empty config",
};

export const CopyWithMissingSecret: Story = {
  name: "Copy with a missing Secret",
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.selectOptions(canvas.getByLabelText("Copy from"), "broken");
    await expect(
      canvas.getByText(
        "The Secret camunda-license does not exist in namespace camunda-system.",
      ),
    ).toBeVisible();
  },
};

export const ChangesOff: Story = {
  name: "Changes off",
  args: { mode: undefined },
};
