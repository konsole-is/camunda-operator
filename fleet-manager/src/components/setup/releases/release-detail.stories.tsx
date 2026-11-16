import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import { imagesFixture, releaseFixtures } from "./fixtures";
import { ReleaseDetailView } from "./release-detail";

const meta = {
  title: "setup/releases/ReleaseDetailView",
  component: ReleaseDetailView,
  args: {
    sources: { ...releaseFixtures(), errors: [] },
    name: "camunda-8-9-0",
    imagesTable: imagesFixture,
    secrets: { "team-orders": [{ name: "orders-db", keys: ["password"] }] },
    tab: "settings",
    mode: "direct",
    actions: {
      previewChange: fn(async () => ({ objects: [], errors: [] })),
      applyChange: fn(async () => ({
        ok: true as const,
        mode: "direct" as const,
      })),
    },
  },
} satisfies Meta<typeof ReleaseDetailView>;
export default meta;
type Story = StoryObj<typeof meta>;

export const ReleaseInUse: Story = { name: "Release in use" };

export const UnusedRelease: Story = {
  name: "Unused release",
  args: { name: "camunda-8-9-1", secrets: {} },
};
