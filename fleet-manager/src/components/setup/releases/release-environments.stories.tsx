import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { releaseFixtures } from "./fixtures";
import { ReleaseEnvironments } from "./release-environments";

function three() {
  const { environments } = releaseFixtures();
  return [
    ...environments,
    {
      ...environments[1],
      metadata: { name: "sandbox" },
      spec: { displayName: "Sandbox" },
    },
  ];
}

const meta = {
  title: "setup/releases/ReleaseEnvironments",
  component: ReleaseEnvironments,
  args: { release: "camunda-8-9-0", environments: three() },
} satisfies Meta<typeof ReleaseEnvironments>;
export default meta;
type Story = StoryObj<typeof meta>;

export const DefaultAllowedNotAllowed: Story = {
  name: "Default, allowed, not allowed",
};

export const NoEnvironment: Story = {
  name: "No environment",
  args: { environments: [] },
};
