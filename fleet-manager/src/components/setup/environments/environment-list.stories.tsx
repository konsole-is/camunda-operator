import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { EnvironmentList } from "./environment-list";
import { environmentFixtures } from "./fixtures";
import { environmentRows, unassignedNamespaces } from "./rows";
import type { EnvironmentSources } from "./sources";

const listOf = (sources: EnvironmentSources) => ({
  rows: environmentRows(sources),
  unassigned: unassignedNamespaces(sources),
});

const meta = {
  title: "setup/environments/EnvironmentList",
  component: EnvironmentList,
  args: { ...listOf(environmentFixtures()), errors: [] },
} satisfies Meta<typeof EnvironmentList>;
export default meta;
type Story = StoryObj<typeof meta>;

export const ThreeEnvironmentsAndTheBanner: Story = {
  name: "Three environments and the banner",
};

export const NoEnvironment: Story = {
  name: "No environment",
  args: listOf({ ...environmentFixtures(), environments: [] }),
};
