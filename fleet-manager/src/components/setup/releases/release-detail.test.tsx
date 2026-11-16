// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { GitPushesProvider } from "@/components/fleet/git-pushes";
import {
  imagesFixture,
  releaseFixtures,
  rolloutListsFixture,
} from "./fixtures";
import { type ReleaseActions, ReleaseDetailView } from "./release-detail";
import type { ReleaseTab } from "./release-tabs";
import type { RolloutLists } from "./rollout-sources";
import type { SourceReads } from "./rows";
import type { ReleasesData } from "./sources";

vi.mock("next/navigation", () => ({ useRouter: () => ({ push: vi.fn() }) }));
vi.mock("@/lib/k8s/hooks", () => ({
  useGitSync: () => "waiting",
  useGitGone: () => "waiting",
}));

afterEach(cleanup);

const actions: ReleaseActions = {
  previewChange: vi.fn(async () => ({ objects: [], errors: [] })),
  applyChange: vi.fn(async () => ({
    ok: true as const,
    mode: "direct" as const,
  })),
  previewMove: vi.fn(async () => ({ objects: [], errors: [] })),
  applyMove: vi.fn(async () => ({
    ok: true as const,
    mode: "direct" as const,
  })),
};

function show(
  name = "camunda-8-9-0",
  sources: ReleasesData = releaseFixtures(),
  tab: ReleaseTab = "settings",
  rollout: RolloutLists & SourceReads = rolloutListsFixture(),
) {
  render(
    <GitPushesProvider>
      <ReleaseDetailView
        sources={{ ...sources, errors: [] }}
        rollout={rollout}
        name={name}
        imagesTable={imagesFixture}
        secrets={{}}
        tab={tab}
        mode="direct"
        actions={actions}
      />
    </GitPushesProvider>,
  );
}

describe("ReleaseDetailView", () => {
  it("shows the Settings tab and the Environments card", () => {
    show();
    expect(
      screen.getByRole("heading", { level: 1, name: "camunda-8-9-0" }),
    ).toBeTruthy();
    expect(screen.getByRole("tab", { name: "Settings" })).toBeTruthy();
    expect(screen.getByLabelText("Camunda version")).toBeTruthy();
    expect(screen.getByRole("region", { name: "Environments" })).toBeTruthy();
  });

  it("opens the tab of the query", () => {
    show();
    expect(
      screen
        .getByRole("tab", { name: "Settings" })
        .getAttribute("aria-selected"),
    ).toBe("true");
  });

  it("links Copy this release to the create page with the release as the source", () => {
    show();
    expect(
      screen
        .getByRole("link", { name: "Copy this release" })
        .getAttribute("href"),
    ).toBe("/setup/new-release?from=camunda-8-9-0");
  });

  it("says that a release missing from the live list does not exist", () => {
    const sources = releaseFixtures();
    show("camunda-8-9-1", sources);
    cleanup();
    show("camunda-8-9-1", {
      ...sources,
      releases: sources.releases.filter(
        (r) => r.metadata.name !== "camunda-8-9-1",
      ),
    });
    expect(
      screen.getByText("The release camunda-8-9-1 does not exist."),
    ).toBeTruthy();
  });

  it("shows the Rollout tab and its clusters on ?tab=rollout", () => {
    show("camunda-8-9-1", releaseFixtures(), "rollout");
    expect(
      screen
        .getByRole("tab", { name: "Rollout" })
        .getAttribute("aria-selected"),
    ).toBe("true");
    const table = screen.getByRole("region", { name: "Clusters" });
    expect(within(table).getByRole("link", { name: "orders" })).toBeTruthy();
  });

  it("says on the Rollout tab that a release missing from the live list does not exist", () => {
    const sources = releaseFixtures();
    show(
      "camunda-8-9-1",
      {
        ...sources,
        releases: sources.releases.filter(
          (r) => r.metadata.name !== "camunda-8-9-1",
        ),
      },
      "rollout",
    );
    expect(
      screen.getByText("The release camunda-8-9-1 does not exist."),
    ).toBeTruthy();
  });

  it("keeps the delete of the Settings tab when only a list of the Rollout tab is not read", () => {
    show("camunda-8-9-1", releaseFixtures(), "settings", {
      ...rolloutListsFixture(),
      stale: ["PluginRequest"],
    });
    expect(
      screen.queryByText(/Delete is possible when the fleet manager can read/),
    ).toBeNull();
  });

  it("names a list of the Rollout tab that is not read and offers no move", () => {
    show("camunda-8-9-1", releaseFixtures(), "rollout", {
      ...rolloutListsFixture(),
      errors: [{ kind: "LogicalBackupRDBMS", message: "forbidden" }],
    });
    expect(
      screen.getByText("Could not read LogicalBackupRDBMS: forbidden"),
    ).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Move" })).toBeNull();
  });
});
