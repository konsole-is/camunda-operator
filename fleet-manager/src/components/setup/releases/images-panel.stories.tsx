import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import { imagesFixture } from "./fixtures";
import { releaseImages } from "./images";
import { ImagesPanel } from "./images-panel";

const meta = {
  title: "setup/releases/ImagesPanel",
  component: ImagesPanel,
  args: {
    rows: releaseImages(
      { version: "8.9.0", connectors: { version: "8.9.7" } },
      imagesFixture,
    ),
    minor: "8.9",
    onPin: fn(),
  },
} satisfies Meta<typeof ImagesPanel>;
export default meta;
type Story = StoryObj<typeof meta>;

export const DefaultsFor89: Story = { name: "Defaults for 8.9" };

export const PinnedImages: Story = {
  name: "Pinned images",
  args: {
    pinned: {
      camunda: "registry.example.com/camunda/camunda:8.9.0-patched",
      connectors: "registry.example.com/camunda/connectors-bundle:8.9.7",
    },
  },
};

export const MinorNotInTheTable: Story = {
  name: "Minor not in the table",
  args: { rows: undefined, minor: "8.10" },
};
