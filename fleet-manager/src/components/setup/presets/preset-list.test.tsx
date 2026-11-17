// @vitest-environment jsdom
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { CHANGES_OFF } from "@/components/cluster/edit/cluster-edit";
import { GitPushesProvider } from "@/components/fleet/git-pushes";
import type { ReadError } from "@/lib/k8s/unread";
import type { Resource } from "@/lib/k8s/kinds";
import type { WriteMode, WriteResult } from "@/lib/write/types";
import { presetDeleteChange } from "./delete";
import { presetFixtures } from "./fixtures";
import { PresetListLive } from "./preset-list";
import type { PresetActions, PresetSources } from "./sources";

const failing = vi.hoisted(() => ({ kind: "" }));
const watch = vi.hoisted(() => ({ stale: [] as string[] }));
vi.mock("@/lib/k8s/hooks", () => ({
  useResources: (kind: string, _opts: unknown, initial: unknown[]) => ({
    items: initial,
    ...(kind === failing.kind && { error: `${kind} is forbidden` }),
  }),
  useWatchesFresh: (lists: { kind: string }[]) =>
    lists.map((l) => !watch.stale.includes(l.kind)),
  useGitSync: () => "waiting",
  useGitGone: () => "waiting",
}));

afterEach(() => {
  cleanup();
  failing.kind = "";
  watch.stale = [];
});

function setup(
  opts: {
    mode?: WriteMode;
    sources?: PresetSources;
    readErrors?: ReadError[];
    result?: WriteResult;
  } = {},
) {
  const {
    sources = presetFixtures(),
    readErrors = [],
    result = { ok: true, mode: "direct" },
  } = opts;
  const mode = "mode" in opts ? opts.mode : "direct";
  const actions = {
    previewChange: vi.fn<PresetActions["previewChange"]>(async () => ({
      objects: [],
      errors: [],
    })),
    applyChange: vi.fn<PresetActions["applyChange"]>(async () => result),
  };
  const view = (s: PresetSources) => (
    <GitPushesProvider>
      <PresetListLive
        initial={{ ...s, readErrors }}
        mode={mode}
        actions={actions}
      />
    </GitPushesProvider>
  );
  const { rerender } = render(view(sources));
  return { actions, rerender: (s: PresetSources) => rerender(view(s)) };
}

const rowOf = (name: RegExp) => screen.getByRole("row", { name });

describe("PresetList", () => {
  it("shows name, kind, key facts, Allowed in, Used by, and last changed of each preset", () => {
    setup();
    const row = rowOf(/medium/);
    expect(
      within(row).getByText(
        "For production workloads with three brokers and replication.",
      ),
    ).toBeTruthy();
    expect(within(row).getByText("Cluster")).toBeTruthy();
    expect(within(row).getByText("Brokers")).toBeTruthy();
    expect(within(row).getByText("32Gi")).toBeTruthy();
    expect(within(row).getByText("production")).toBeTruthy();
    expect(within(row).getByText("1 cluster, 1 claim")).toBeTruthy();
    const time = within(row).getByText("2026-11-14 09:30 UTC");
    expect(time.tagName).toBe("TIME");
    expect(time.getAttribute("datetime")).toBe("2026-11-14T09:30:00.000Z");
  });

  it("says Nowhere, Unused, and Unknown for a preset that nothing names", () => {
    setup();
    const row = rowOf(/spare/);
    expect(within(row).getByText("Nowhere")).toBeTruthy();
    expect(within(row).getByText("Unused")).toBeTruthy();
    expect(within(row).getByText("Unknown")).toBeTruthy();
  });

  it("links each preset to its page by kind and name", () => {
    setup();
    const hrefs = screen
      .getAllByRole("link")
      .map((l) => l.getAttribute("href"));
    expect(hrefs).toContain("/setup/presets/elasticsearch/standard");
    expect(hrefs).toContain("/setup/presets/postgresql/standard");
  });

  it("selects nothing at first and selects only the unused presets with Select all unused", () => {
    setup();
    expect(screen.queryByRole("button", { name: /^Delete/ })).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Select all unused" }));
    expect(
      screen.getByRole("button", { name: "Delete 3 presets" }),
    ).toHaveProperty("disabled", false);
    expect(within(rowOf(/medium/)).getByRole("checkbox")).toHaveProperty(
      "disabled",
      true,
    );
  });

  it("filters by kind", () => {
    setup();
    fireEvent.change(screen.getByLabelText("Kind"), {
      target: { value: "postgresql" },
    });
    const names = screen
      .getAllByRole("row")
      .slice(1)
      .map((r) => within(r).getAllByRole("cell")[1]?.textContent);
    expect(names.map((n) => n?.split(" ")[0])).toEqual(["small", "standard"]);
  });

  it("deletes the selected presets in one write and shows the result", async () => {
    const { actions } = setup();
    fireEvent.click(within(rowOf(/spare/)).getByRole("checkbox"));
    fireEvent.click(
      screen.getByRole("button", { name: "Delete the preset spare" }),
    );
    const review = screen.getByRole("region", { name: "Review the delete" });
    expect(
      within(review).getByText("The preset spare is deleted."),
    ).toBeTruthy();
    const confirm = within(review).getByRole("button", { name: "Delete" });
    await waitFor(() => expect(confirm).toHaveProperty("disabled", false));
    fireEvent.click(confirm);
    await waitFor(() => expect(actions.applyChange).toHaveBeenCalledTimes(1));
    expect(actions.applyChange).toHaveBeenCalledWith(
      presetDeleteChange([presetFixtures().clusterPresets[1]]),
    );
    expect(
      await screen.findByText("The preset spare is deleted."),
    ).toBeTruthy();
    await waitFor(() =>
      expect(
        screen.queryByRole("region", { name: "Review the delete" }),
      ).toBeNull(),
    );
    expect(
      within(rowOf(/spare/)).getByRole("checkbox").getAttribute("aria-checked"),
    ).toBe("false");
  });

  it("reads Push the delete to Git in Git mode", () => {
    setup({ mode: "git" });
    fireEvent.click(within(rowOf(/spare/)).getByRole("checkbox"));
    fireEvent.click(
      screen.getByRole("button", { name: "Delete the preset spare" }),
    );
    const review = screen.getByRole("region", { name: "Review the delete" });
    expect(
      within(review).getByRole("button", { name: "Push the delete to Git" }),
    ).toBeTruthy();
    expect(review.textContent).toContain(
      "GitOps must prune the presets to delete them from the cluster.",
    );
  });

  it("closes the delete review when a selected preset gets a user", () => {
    const { actions, rerender } = setup();
    fireEvent.click(within(rowOf(/spare/)).getByRole("checkbox"));
    fireEvent.click(
      screen.getByRole("button", { name: "Delete the preset spare" }),
    );
    expect(
      screen.getByRole("region", { name: "Review the delete" }),
    ).toBeTruthy();
    const sources = presetFixtures();
    sources.clusters = [
      ...sources.clusters,
      {
        ...sources.clusters[0],
        metadata: { namespace: "team-orders", name: "spare-user" },
        spec: { ...sources.clusters[0].spec, presetRef: "spare" },
      } as Resource<"CamundaCluster">,
    ];
    rerender(sources);
    expect(
      screen.queryByRole("region", { name: "Review the delete" }),
    ).toBeNull();
    expect(actions.applyChange).not.toHaveBeenCalled();
  });

  it("offers no delete while a list of users cannot be read, and says why", () => {
    failing.kind = "PluginRequest";
    setup();
    expect(
      screen.getByText(
        "Could not read PluginRequest: PluginRequest is forbidden",
      ),
    ).toBeTruthy();
    expect(
      within(rowOf(/standard Elasticsearch/)).getByRole("checkbox"),
    ).toHaveProperty("disabled", true);
    expect(within(rowOf(/spare/)).getByRole("checkbox")).toHaveProperty(
      "disabled",
      false,
    );
  });

  it("closes the delete review when a list of users cannot be read", () => {
    const { actions, rerender } = setup();
    fireEvent.click(within(rowOf(/spare/)).getByRole("checkbox"));
    fireEvent.click(
      screen.getByRole("button", { name: "Delete the preset spare" }),
    );
    failing.kind = "CamundaClusterClaim";
    rerender(presetFixtures());
    expect(
      screen.queryByRole("region", { name: "Review the delete" }),
    ).toBeNull();
    expect(actions.applyChange).not.toHaveBeenCalled();
  });

  it("keeps a failed read of the server until the watch of its kind sends a list", () => {
    watch.stale = ["PluginRequest"];
    const { rerender } = setup({
      readErrors: [{ kind: "PluginRequest", message: "forbidden" }],
    });
    expect(
      screen.getByText("Could not read PluginRequest: forbidden"),
    ).toBeTruthy();
    const row = () => rowOf(/standard Elasticsearch/);
    expect(within(row()).getByRole("checkbox")).toHaveProperty(
      "disabled",
      true,
    );
    expect(within(row()).queryByText("Unused")).toBeNull();
    expect(within(row()).getAllByText("Unknown")).toHaveLength(2);
    fireEvent.click(screen.getByRole("button", { name: "Select all unused" }));
    expect(
      screen.getByRole("button", { name: "Delete the preset spare" }),
    ).toBeTruthy();
    watch.stale = [];
    rerender(presetFixtures());
    expect(
      screen.queryByText("Could not read PluginRequest: forbidden"),
    ).toBeNull();
    expect(within(row()).getByRole("checkbox")).toHaveProperty(
      "disabled",
      false,
    );
  });

  it("offers no delete until the watch of a list of users sends a list", () => {
    watch.stale = ["CamundaClusterClaim"];
    const { rerender } = setup();
    expect(within(rowOf(/spare/)).getByRole("checkbox")).toHaveProperty(
      "disabled",
      true,
    );
    expect(within(rowOf(/spare/)).getByText("Unused")).toBeTruthy();
    expect(screen.queryByRole("alert")).toBeNull();
    watch.stale = [];
    rerender(presetFixtures());
    expect(within(rowOf(/spare/)).getByRole("checkbox")).toHaveProperty(
      "disabled",
      false,
    );
  });

  it("closes the delete review while the watch of a list of users reconnects", () => {
    const { actions, rerender } = setup();
    fireEvent.click(within(rowOf(/spare/)).getByRole("checkbox"));
    fireEvent.click(
      screen.getByRole("button", { name: "Delete the preset spare" }),
    );
    watch.stale = ["CamundaCluster"];
    rerender(presetFixtures());
    expect(
      screen.queryByRole("region", { name: "Review the delete" }),
    ).toBeNull();
    expect(within(rowOf(/spare/)).getByRole("checkbox")).toHaveProperty(
      "disabled",
      true,
    );
    expect(actions.applyChange).not.toHaveBeenCalled();
  });

  it("says Unknown for Allowed in while the environments cannot be read", () => {
    failing.kind = "Environment";
    setup();
    const cells = within(rowOf(/spare/)).getAllByRole("cell");
    expect(cells.map((c) => c.textContent)).toContain("Unknown");
    expect(within(rowOf(/spare/)).queryByText("Nowhere")).toBeNull();
  });

  it("offers no delete and says why without a write mode", () => {
    setup({ mode: undefined });
    expect(screen.getByRole("alert").textContent).toBe(CHANGES_OFF);
    fireEvent.click(within(rowOf(/spare/)).getByRole("checkbox"));
    expect(screen.queryByRole("button", { name: /^Delete/ })).toBeNull();
  });

  it("says that no preset exists yet", () => {
    setup({
      sources: {
        ...presetFixtures(),
        clusterPresets: [],
        elasticsearchPresets: [],
        databasePresets: [],
      },
    });
    expect(
      screen.getByText(
        "No preset yet. A cluster that fits no preset sets everything itself.",
      ),
    ).toBeTruthy();
  });
});
