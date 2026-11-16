// @vitest-environment jsdom
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { CHANGES_OFF } from "@/components/cluster/edit/cluster-edit";
import { GitPushesProvider } from "@/components/fleet/git-pushes";
import type { WriteMode } from "@/lib/write/types";
import { claim, releaseFixtures } from "./fixtures";
import { ReleaseList } from "./release-list";
import type { ReleaseActions } from "./release-detail";
import { releaseRows, type SourceReads } from "./rows";
import type { ReleasesData } from "./sources";

vi.mock("@/lib/k8s/hooks", () => ({
  useGitSync: () => "waiting",
  useGitGone: () => "waiting",
}));

afterEach(cleanup);

function setup(
  opts: { sources?: ReleasesData & SourceReads; mode?: WriteMode } = {},
) {
  const { sources = releaseFixtures() } = opts;
  const mode = "mode" in opts ? opts.mode : "direct";
  const actions = {
    previewChange: vi.fn<ReleaseActions["previewChange"]>(async () => ({
      objects: [],
      errors: [],
    })),
    applyChange: vi.fn<ReleaseActions["applyChange"]>(async () => ({
      ok: true as const,
      mode: "direct" as const,
    })),
  };
  const view = (s: ReleasesData & SourceReads) => (
    <GitPushesProvider>
      <ReleaseList
        rows={releaseRows(s)}
        releases={s.releases}
        environments={s.environments}
        errors={[...(s.errors ?? [])]}
        stale={s.stale}
        mode={mode}
        actions={actions}
      />
    </GitPushesProvider>
  );
  const { rerender } = render(view(sources));
  return {
    actions,
    rerender: (s: ReleasesData & SourceReads) => rerender(view(s)),
  };
}

const rowOf = (name: string) =>
  screen.getByRole("link", { name }).closest("tr") as HTMLElement;

const names = () =>
  screen
    .getAllByRole("row")
    .slice(1)
    .map((row) => within(row).getByRole("link").textContent);

describe("ReleaseList", () => {
  it("shows the name, the versions, the facts, and the usage of each release", () => {
    setup();
    const row = within(rowOf("camunda-8-9-0"));
    for (const text of [
      "8.9.0",
      "8.9.7",
      "9.2.4",
      "17",
      "Default in Production",
      "Allowed in Production, Staging",
      "1 cluster, 1 Elasticsearch cluster",
    ])
      expect(row.getByText(text)).toBeTruthy();
    expect(within(rowOf("camunda-8-10-0")).getByText("1 claim")).toBeTruthy();
    expect(
      screen.getByRole("link", { name: "camunda-8-9-0" }).getAttribute("href"),
    ).toBe("/setup/releases/camunda-8-9-0");
    expect(
      screen.getByRole("link", { name: "New release" }).getAttribute("href"),
    ).toBe("/setup/new-release");
  });

  it("offers a checkbox only for a release that can be deleted", async () => {
    setup();
    const enabled = screen
      .getAllByRole("checkbox")
      .filter((box) => !box.hasAttribute("disabled"))
      .map((box) => box.getAttribute("aria-label"));
    expect(enabled).toEqual(["Select camunda-8-9-1"]);
    await userEvent.click(
      screen.getByRole("button", { name: "Select all that can be deleted" }),
    );
    expect(screen.getByText("1 selected")).toBeTruthy();
  });

  it("filters by an environment that allows the release", async () => {
    setup();
    await userEvent.selectOptions(
      screen.getByRole("combobox", { name: "Environment" }),
      "staging",
    );
    expect(names()).toEqual(["camunda-8-8-5", "camunda-8-9-0"]);
  });

  it("filters by usage", async () => {
    setup();
    await userEvent.selectOptions(
      screen.getByRole("combobox", { name: "Usage" }),
      "unused",
    );
    expect(names()).toEqual(["camunda-8-8-5", "camunda-8-9-1"]);
  });

  it("deletes the selected releases in one change with a precondition and a UID each", async () => {
    const { actions } = setup();
    await userEvent.click(
      screen.getByRole("checkbox", { name: "Select camunda-8-9-1" }),
    );
    await userEvent.click(
      screen.getByRole("button", { name: "Delete 1 release" }),
    );
    const review = screen.getByRole("region", { name: "Review the delete" });
    expect(
      within(review).getByText("The release camunda-8-9-1 is deleted."),
    ).toBeTruthy();
    await waitFor(() => expect(actions.previewChange).toHaveBeenCalled());
    const change = actions.previewChange.mock.calls[0][0];
    expect(change.delete).toEqual([
      {
        apiVersion: "core.camunda.io/v1",
        kind: "CamundaRelease",
        name: "camunda-8-9-1",
      },
    ]);
    expect(change.preconditions).toEqual({
      "CamundaRelease//camunda-8-9-1": "41",
    });
    expect(change.uids).toEqual({
      "CamundaRelease//camunda-8-9-1": "uid-camunda-8-9-1",
    });
    const confirm = within(review).getByRole("button", { name: "Delete" });
    await waitFor(() => expect(confirm).toHaveProperty("disabled", false));
    fireEvent.click(confirm);
    await waitFor(() => expect(actions.applyChange).toHaveBeenCalledTimes(1));
    expect(actions.applyChange.mock.calls[0][0]).toEqual(change);
  });

  it("cancels the delete review when a selected release becomes used", async () => {
    const { rerender } = setup();
    await userEvent.click(
      screen.getByRole("checkbox", { name: "Select camunda-8-9-1" }),
    );
    await userEvent.click(
      screen.getByRole("button", { name: "Delete 1 release" }),
    );
    expect(
      screen.getByRole("region", { name: "Review the delete" }),
    ).toBeTruthy();
    const sources = releaseFixtures();
    sources.claims.push(claim("team-new", "late", "camunda-8-9-1"));
    rerender(sources);
    expect(
      screen.queryByRole("region", { name: "Review the delete" }),
    ).toBeNull();
  });

  it("offers no delete while a list cannot be read and closes an open delete review", async () => {
    const { rerender } = setup();
    await userEvent.click(
      screen.getByRole("checkbox", { name: "Select camunda-8-9-1" }),
    );
    await userEvent.click(
      screen.getByRole("button", { name: "Delete 1 release" }),
    );
    rerender({
      ...releaseFixtures(),
      errors: [{ kind: "CamundaClusterClaim", message: "forbidden" }],
    });
    expect(
      screen.queryByRole("region", { name: "Review the delete" }),
    ).toBeNull();
    expect(
      screen.getByText(
        "Delete is possible when the fleet manager can read CamundaClusterClaim.",
      ),
    ).toBeTruthy();
    expect(
      screen
        .getAllByRole("checkbox")
        .filter((box) => !box.hasAttribute("disabled")),
    ).toEqual([]);
  });

  it("offers no delete while a list has no snapshot on the connection", () => {
    setup({ sources: { ...releaseFixtures(), stale: ["Environment"] } });
    expect(
      screen.getByText(
        "Delete is possible when the fleet manager can read Environment.",
      ),
    ).toBeTruthy();
    expect(
      screen.getByRole("checkbox", { name: "Select camunda-8-9-1" }),
    ).toHaveProperty("disabled", true);
  });

  it("offers no delete while changes are off", async () => {
    setup({ mode: undefined });
    expect(screen.getByText(CHANGES_OFF)).toBeTruthy();
    await userEvent.click(
      screen.getByRole("checkbox", { name: "Select camunda-8-9-1" }),
    );
    expect(screen.queryByRole("button", { name: /^Delete/ })).toBeNull();
  });

  it("says so when no release exists", () => {
    setup({ sources: { ...releaseFixtures(), releases: [] } });
    expect(
      screen.getByText("No release yet. Create one with New release."),
    ).toBeTruthy();
  });
});
