import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import { imagesFixture, releaseFixtures } from "./fixtures";
import { ReleaseSettings } from "./release-settings";

const meta = {
  title: "setup/releases/ReleaseSettings",
  component: ReleaseSettings,
  args: {
    sources: { ...releaseFixtures(), errors: [] },
    name: "camunda-8-9-0",
    imagesTable: imagesFixture,
    secrets: {
      "team-orders": [{ name: "orders-db", keys: ["password"] }],
      "team-billing": [{ name: "billing-db", keys: ["password"] }],
    },
    mode: "direct",
    actions: {
      previewChange: fn(async () => ({ objects: [], errors: [] })),
      applyChange: fn(async () => ({
        ok: true as const,
        mode: "direct" as const,
      })),
      previewMove: fn(async () => ({ objects: [], errors: [] })),
      applyMove: fn(async () => ({
        ok: true as const,
        mode: "direct" as const,
      })),
    },
  },
} satisfies Meta<typeof ReleaseSettings>;
export default meta;
type Story = StoryObj<typeof meta>;

export const ExistingRelease: Story = { name: "Existing release" };

export const ReleaseThatCanBeDeleted: Story = {
  name: "Release that can be deleted",
  args: { name: "camunda-8-9-1", secrets: {} },
};

export const NewReleaseFromACopy: Story = {
  name: "New release from a copy",
  args: { name: undefined, copyFrom: "camunda-8-9-0", secrets: {} },
};
