// @vitest-environment jsdom
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { CHANGES_OFF } from "@/components/cluster/edit/cluster-edit";
import { GitPushesProvider } from "@/components/fleet/git-pushes";
import type { ReadError } from "@/components/planes/planes-table";
import type { Resource } from "@/lib/k8s/kinds";
import type { PresetKind } from "@/lib/usage";
import type { WriteMode } from "@/lib/write/types";
import { presetDeleteChange } from "./delete";
import { presetFixtures } from "./fixtures";
import { PresetDetail } from "./preset-detail";
import type { PresetActions, PresetSources } from "./sources";

const router = vi.hoisted(() => ({ push: vi.fn() }));
vi.mock("next/navigation", () => ({ useRouter: () => router }));
vi.mock("@/lib/k8s/hooks", () => ({
  useGitSync: () => "waiting",
  useGitGone: () => "waiting",
}));

beforeEach(() => router.push.mockClear());
afterEach(cleanup);

function setup(
  opts: {
    kind?: PresetKind;
    name?: string;
    mode?: WriteMode;
    sources?: PresetSources;
    errors?: ReadError[];
    pending?: string[];
  } = {},
) {
  const {
    kind = "CamundaClusterPreset",
    name = "medium",
    sources = presetFixtures(),
    errors = [],
    pending = [],
  } = opts;
  const mode = "mode" in opts ? opts.mode : "direct";
  const actions = {
    previewChange: vi.fn<PresetActions["previewChange"]>(async () => ({
      objects: [],
      errors: [],
    })),
    applyChange: vi.fn<PresetActions["applyChange"]>(async () => ({
      ok: true as const,
      mode: "direct" as const,
    })),
  };
  const view = (s: PresetSources, e: ReadError[]) => (
    <GitPushesProvider>
      <PresetDetail
        sources={{ ...s, errors: e, pending }}
        kind={kind}
        name={name}
        mode={mode}
        actions={actions}
      />
    </GitPushesProvider>
  );
  const { rerender } = render(view(sources, errors));
  return {
    actions,
    rerender: (s: PresetSources, e: ReadError[] = errors) =>
      rerender(view(s, e)),
  };
}

const openDelete = (name: string) => {
  fireEvent.change(screen.getByLabelText(`Type ${name} to confirm`), {
    target: { value: name },
  });
  fireEvent.click(screen.getByRole("button", { name: "Review the delete" }));
};

describe("PresetDetail", () => {
  it("shows the kind, the description, the facts, Allowed in, and Used by in the header", () => {
    setup();
    const header = screen.getByRole("banner");
    expect(
      within(header).getByRole("heading", { level: 1, name: "medium" }),
    ).toBeTruthy();
    const text = header.textContent ?? "";
    for (const part of [
      "Cluster",
      "For production workloads with three brokers and replication.",
      "Brokers",
      "32Gi",
      "production",
      "1 cluster, 1 claim",
      "team-orders/orders",
      "2026-11-14 09:30 UTC",
    ])
      expect(text).toContain(part);
  });

  it("shows the settings, the fields it cannot set, and the inheriting resources", () => {
    setup();
    expect(screen.getByRole("region", { name: "Brokers" })).toBeTruthy();
    expect(
      screen.getByRole("list", { name: "Fields a preset cannot set" }),
    ).toBeTruthy();
    expect(
      screen.getByRole("region", { name: "Inheriting resources" }).textContent,
    ).toContain("team-orders");
  });

  it("offers no delete for a used preset and says why", () => {
    setup();
    expect(
      screen.getByText(
        "Delete is possible when no resource, claim, or request uses the preset.",
      ),
    ).toBeTruthy();
    expect(screen.queryByLabelText("Type medium to confirm")).toBeNull();
  });

  it("deletes an unused preset after the typed confirm and the review", async () => {
    const { actions } = setup({ name: "spare" });
    openDelete("spare");
    const review = screen.getByRole("region", { name: "Review the delete" });
    expect(review.textContent).toContain("The preset spare is deleted.");
    const confirm = within(review).getByRole("button", {
      name: "Delete the preset",
    });
    await waitFor(() => expect(confirm).toHaveProperty("disabled", false));
    fireEvent.click(confirm);
    await waitFor(() =>
      expect(actions.applyChange).toHaveBeenCalledWith(
        presetDeleteChange([presetFixtures().clusterPresets[1]]),
      ),
    );
    await waitFor(() =>
      expect(router.push).toHaveBeenCalledWith("/setup/presets"),
    );
  });

  it("reads Push the delete to Git in Git mode", () => {
    setup({ name: "spare", mode: "git" });
    openDelete("spare");
    const review = screen.getByRole("region", { name: "Review the delete" });
    expect(
      within(review).getByRole("button", { name: "Push the delete to Git" }),
    ).toBeTruthy();
    expect(review.textContent).toContain("GitOps must prune the presets");
  });

  it("keeps two presets of one name and two kinds apart", async () => {
    const { actions } = setup({
      kind: "DatabaseServerPreset",
      name: "standard",
    });
    expect(screen.getByRole("banner").textContent).toContain("PostgreSQL");
    openDelete("standard");
    const confirm = within(
      screen.getByRole("region", { name: "Review the delete" }),
    ).getByRole("button", { name: "Delete the preset" });
    await waitFor(() => expect(confirm).toHaveProperty("disabled", false));
    fireEvent.click(confirm);
    await waitFor(() =>
      expect(actions.applyChange).toHaveBeenCalledWith(
        presetDeleteChange([presetFixtures().databasePresets[1]]),
      ),
    );
  });

  it("closes the delete review when an environment allows the preset", () => {
    const { actions, rerender } = setup({ name: "spare" });
    openDelete("spare");
    expect(
      screen.getByRole("region", { name: "Review the delete" }),
    ).toBeTruthy();
    const sources = presetFixtures();
    sources.environments[0].spec.presets = { camundaCluster: ["spare"] };
    rerender(sources);
    expect(
      screen.queryByRole("region", { name: "Review the delete" }),
    ).toBeNull();
    expect(
      screen.getByText(
        "Delete is possible when no environment allows the preset.",
      ),
    ).toBeTruthy();
    expect(actions.applyChange).not.toHaveBeenCalled();
  });

  it("offers no delete while a list of users cannot be read", () => {
    setup({
      name: "spare",
      errors: [{ kind: "CamundaClusterClaim", message: "forbidden" }],
    });
    expect(
      screen.getByText(
        "Delete is possible when the fleet manager can read CamundaClusterClaim.",
      ),
    ).toBeTruthy();
    expect(
      screen.getByText("Could not read CamundaClusterClaim: forbidden"),
    ).toBeTruthy();
  });

  it("closes the delete review when a list of users cannot be read", () => {
    const { actions, rerender } = setup({ name: "spare" });
    openDelete("spare");
    rerender(presetFixtures(), [
      { kind: "CamundaCluster", message: "forbidden" },
    ]);
    expect(
      screen.queryByRole("region", { name: "Review the delete" }),
    ).toBeNull();
    expect(
      screen.getByText(
        "Delete is possible when the fleet manager can read CamundaCluster.",
      ),
    ).toBeTruthy();
    expect(actions.applyChange).not.toHaveBeenCalled();
  });

  it("offers no delete until the lists of users are current", () => {
    setup({ name: "spare", pending: ["CamundaCluster"] });
    expect(
      screen.getByText(
        "Delete is possible when the fleet manager has the current lists.",
      ),
    ).toBeTruthy();
    expect(screen.queryByLabelText("Type spare to confirm")).toBeNull();
  });

  it("says Unknown for Used by and Allowed in while their lists cannot be read", () => {
    setup({
      errors: [
        { kind: "Environment", message: "forbidden" },
        { kind: "CamundaClusterClaim", message: "forbidden" },
      ],
    });
    const header = within(screen.getByRole("banner"));
    expect(header.getAllByText("Unknown")).toHaveLength(2);
    expect(header.queryByText("1 cluster, 1 claim")).toBeNull();
    expect(header.queryByText("production")).toBeNull();
  });

  it("pages through the inheriting resources without a write mode", () => {
    const sources = presetFixtures();
    sources.clusters = Array.from(
      { length: 30 },
      (_, i) =>
        ({
          ...sources.clusters[0],
          metadata: { namespace: "team-orders", name: `orders-${i}` },
        }) as Resource<"CamundaCluster">,
    );
    setup({ sources, mode: undefined });
    const next = screen.getByRole("button", { name: "Next page" });
    expect(next.matches(":disabled")).toBe(false);
  });

  it("says that the preset was deleted when it leaves the live list", () => {
    const { rerender } = setup({ name: "spare" });
    const sources = presetFixtures();
    sources.clusterPresets = sources.clusterPresets.slice(0, 1);
    rerender(sources);
    expect(screen.getByText("The preset spare was deleted.")).toBeTruthy();
    expect(screen.queryByLabelText("Type spare to confirm")).toBeNull();
  });

  it("disables the delete without a write mode", () => {
    setup({ name: "spare", mode: undefined });
    expect(screen.getByText(CHANGES_OFF)).toBeTruthy();
    expect(
      screen.getByRole("button", { name: "Review the delete" }),
    ).toHaveProperty("disabled", true);
  });
});
