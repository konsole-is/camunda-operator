import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import { releaseFixtures } from "./fixtures";
import { ReleaseList } from "./release-list";
import { releaseRows } from "./rows";
import type { ReleasesData } from "./sources";

const listOf = (sources: ReleasesData) => ({
  rows: releaseRows(sources),
  releases: sources.releases,
  environments: sources.environments,
});

function claimOnly(): ReleasesData {
  const sources = releaseFixtures();
  return {
    ...sources,
    releases: sources.releases.filter(
      (r) => r.metadata.name === "camunda-8-10-0",
    ),
  };
}

const meta = {
  title: "setup/releases/ReleaseList",
  component: ReleaseList,
  args: {
    ...listOf(releaseFixtures()),
    errors: [],
    mode: "direct",
    actions: {
      previewChange: fn(async () => ({ objects: [], errors: [] })),
      applyChange: fn(async () => ({
        ok: true as const,
        mode: "direct" as const,
      })),
    },
  },
} satisfies Meta<typeof ReleaseList>;
export default meta;
type Story = StoryObj<typeof meta>;

export const FourFactsAndTheFilters: Story = {
  name: "Four facts and the filters",
};

export const ReleaseThatOnlyAClaimUses: Story = {
  name: "A release that only a claim uses",
  args: listOf(claimOnly()),
};

export const NoRelease: Story = {
  name: "No release",
  args: listOf({ ...releaseFixtures(), releases: [] }),
};

export const ChangesOff: Story = {
  name: "Changes off",
  args: { mode: undefined },
};
