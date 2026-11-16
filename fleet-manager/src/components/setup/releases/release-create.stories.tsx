import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import { imagesFixture, releaseFixtures } from "./fixtures";
import { ReleaseCreateView } from "./release-create";

const meta = {
  title: "setup/releases/ReleaseCreateView",
  component: ReleaseCreateView,
  args: {
    sources: { ...releaseFixtures(), errors: [] },
    imagesTable: imagesFixture,
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
} satisfies Meta<typeof ReleaseCreateView>;
export default meta;
type Story = StoryObj<typeof meta>;

export const EmptyRelease: Story = { name: "Empty release" };

export const CopyOfARelease: Story = {
  name: "Copy of a release",
  args: { from: "camunda-8-9-0" },
};

export const ChangesOff: Story = {
  name: "Changes off",
  args: { mode: undefined },
};
