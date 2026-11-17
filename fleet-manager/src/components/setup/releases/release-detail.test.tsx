// @vitest-environment jsdom
import {
  cleanup,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { GitPushesProvider } from "@/components/fleet/git-pushes";
import {
  imagesFixture,
  releaseFixtures,
  rolloutListsFixture,
} from "./fixtures";
import { type ReleaseActions, ReleaseDetailView } from "./release-detail";
import type { ReleaseTab } from "./release-tabs";
import { rolloutData } from "./rollout-fixtures";
import type { RolloutLists } from "./rollout-sources";
import type { ReleasesData } from "./sources";
import type { SourceReads } from "@/lib/k8s/unread";

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

  it("keeps a failed move across a switch to Settings and back until Dismiss", async () => {
    const data = rolloutData();
    vi.mocked(actions.previewMove).mockImplementationOnce(async (c) => ({
      objects: c.apply.map((o) => ({
        ref: {
          apiVersion: o.apiVersion,
          kind: o.kind,
          namespace: o.metadata.namespace,
          name: o.metadata.name,
        },
        diff: [{ op: "added" as const, text: "releaseRef: camunda-8-9-1" }],
      })),
      errors: [],
    }));
    vi.mocked(actions.applyMove).mockResolvedValueOnce({
      ok: false,
      errors: [
        { code: "Unavailable", message: "The API server did not answer." },
      ],
    });
    show(
      "camunda-8-9-1",
      {
        releases: data.releases,
        environments: data.environments,
        namespaces: data.namespaces,
        clusters: data.clusters,
        elasticsearchClusters: [],
        databaseServers: data.databaseServers,
        claims: data.claims,
      } as unknown as ReleasesData,
      "rollout",
      {
        clusterPresets: data.clusterPresets,
        sections: data.sections,
        registrations: data.registrations,
        requests: data.requests,
        elasticsearchBackups: data.elasticsearchBackups,
        rdbmsBackups: data.rdbmsBackups,
      } as unknown as RolloutLists,
    );
    const orders = screen
      .getByRole("link", { name: "orders" })
      .closest("tr") as HTMLElement;
    await userEvent.click(within(orders).getByRole("button", { name: "Move" }));
    const confirm = await screen.findByRole("button", { name: "Move" });
    await waitFor(() => expect(confirm).toHaveProperty("disabled", false));
    await userEvent.click(confirm);
    await userEvent.click(await screen.findByRole("button", { name: "Back" }));
    await screen.findByRole("alert", { name: "The move did not finish" });

    await userEvent.click(screen.getByRole("tab", { name: "Settings" }));
    expect(
      screen.queryByRole("alert", { name: "The move did not finish" }),
    ).toBeNull();
    await userEvent.click(screen.getByRole("tab", { name: "Rollout" }));
    const note = screen.getByRole("alert", { name: "The move did not finish" });
    expect(
      within(note).getByText("The API server did not answer."),
    ).toBeTruthy();

    await userEvent.click(
      within(note).getByRole("button", { name: "Dismiss" }),
    );
    expect(
      screen.queryByRole("alert", { name: "The move did not finish" }),
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
