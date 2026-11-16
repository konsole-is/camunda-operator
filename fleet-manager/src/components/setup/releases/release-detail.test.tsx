// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { GitPushesProvider } from "@/components/fleet/git-pushes";
import { imagesFixture, releaseFixtures } from "./fixtures";
import { type ReleaseActions, ReleaseDetailView } from "./release-detail";
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
};

function show(
  name = "camunda-8-9-0",
  sources: ReleasesData = releaseFixtures(),
) {
  render(
    <GitPushesProvider>
      <ReleaseDetailView
        sources={{ ...sources, errors: [] }}
        name={name}
        imagesTable={imagesFixture}
        secrets={{}}
        tab="settings"
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
});
