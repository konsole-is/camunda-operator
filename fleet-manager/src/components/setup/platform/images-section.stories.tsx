import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import { defaultRepositories } from "@/lib/merge";
import { twoMinorTable } from "./fixtures";
import { ImagesSection } from "./images-section";

const meta = {
  title: "setup/platform/ImagesSection",
  component: ImagesSection,
  args: {
    images: { camunda: "registry.example.com/camunda/camunda" },
    defaults: defaultRepositories(twoMinorTable, ["8.9"]),
    errors: [],
    onChange: fn(),
  },
} satisfies Meta<typeof ImagesSection>;
export default meta;
type Story = StoryObj<typeof meta>;

export const OneMinor: Story = { name: "One minor" };

export const TwoMinors: Story = {
  name: "Two minors",
  args: { defaults: defaultRepositories(twoMinorTable, ["8.9", "8.10"]) },
};

export const MinorWithoutDefaults: Story = {
  name: "A minor without defaults",
  args: { defaults: defaultRepositories(twoMinorTable, ["8.8", "8.9"]) },
};

export const MinorsIncomplete: Story = {
  name: "Minors that can be incomplete",
  args: { unreadMinorKinds: ["CamundaCluster"] },
};
