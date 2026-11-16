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
import type { ReadError } from "@/components/planes/planes-table";
import type { Resource } from "@/lib/k8s/kinds";
import type {
  ApiError,
  Change,
  KubeObject,
  WriteMode,
} from "@/lib/write/types";
import type { ReleaseActions } from "./release-detail";
import { PARTIAL_MOVE_CAUTION, type RolloutData } from "./rollout";
import {
  cluster,
  clusterClaim,
  rolloutData,
  targetRelease,
} from "./rollout-fixtures";
import { RolloutTab } from "./rollout-tab";

vi.mock("@/lib/k8s/hooks", () => ({
  useGitSync: () => "waiting",
  useGitGone: () => "waiting",
}));

afterEach(cleanup);

type Data = RolloutData & {
  errors?: readonly ReadError[];
  stale?: readonly string[];
};

const changed = (o: KubeObject) => ({
  ref: {
    apiVersion: o.apiVersion,
    kind: o.kind,
    namespace: o.metadata.namespace,
    name: o.metadata.name,
  },
  diff: [{ op: "added" as const, text: "releaseRef: camunda-8-9-1" }],
});

const staleErrorOf = (namespace: string, name: string): ApiError => ({
  code: "Stale",
  message: `The CamundaClusterClaim ${name} changed since you opened it.`,
  ref: {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "CamundaClusterClaim",
    namespace,
    name,
  },
});

function renderTab(opts: { data?: Data; mode?: WriteMode } = {}) {
  const mode = "mode" in opts ? opts.mode : "direct";
  const actions = {
    previewMove: vi.fn<ReleaseActions["previewMove"]>(async (c: Change) => ({
      objects: c.apply.map(changed),
      errors: [],
    })),
    applyMove: vi.fn<ReleaseActions["applyMove"]>(async () => ({
      ok: true as const,
      mode: "direct" as const,
    })),
  };
  const view = (data: Data, release = targetRelease) => (
    <GitPushesProvider>
      <RolloutTab release={release} data={data} mode={mode} actions={actions} />
    </GitPushesProvider>
  );
  const { rerender } = render(view(opts.data ?? rolloutData()));
  return {
    actions,
    rerender: (data: Data, release?: typeof targetRelease) =>
      rerender(view(data, release)),
  };
}

const rowOf = (name: string) =>
  screen.getByRole("link", { name }).closest("tr") as HTMLElement;

const withBackupOf = (name: string, phase = "Running"): RolloutData =>
  rolloutData({
    rdbmsBackups: [
      {
        apiVersion: "core.camunda.io/v1",
        kind: "LogicalBackupRDBMS",
        metadata: { namespace: "team-orders", name: `${name}-now` },
        spec: { clusterRef: { name } },
        status: { phase },
      } as Resource<"LogicalBackupRDBMS">,
    ],
  });

// freshCanMove gives the cluster fresh the version 8.9.0, so that two
// clusters can move.
function freshCanMove(): RolloutData {
  const data = rolloutData();
  return {
    ...data,
    clusters: data.clusters.map((c) =>
      c.metadata?.name === "fresh"
        ? cluster("team-pay", "fresh", { version: "8.9.0" })
        : c,
    ),
  };
}

describe("RolloutTab", () => {
  it("shows each cluster with its environment, release, version, and eligibility", () => {
    renderTab();
    const orders = within(rowOf("orders"));
    expect(orders.getByRole("link", { name: "Production" })).toHaveProperty(
      "pathname",
      "/setup/environments/production",
    );
    for (const text of ["team-orders", "camunda-8-9-0", "8.9.0", "Can move."])
      expect(orders.getByText(text)).toBeTruthy();
    expect(within(rowOf("fresh")).getByText("Not reported")).toBeTruthy();
    expect(rowOf("orders").querySelector("a")?.getAttribute("href")).toBe(
      "/fleet/clusters/team-orders/orders",
    );
  });

  it("selects nothing at first", () => {
    renderTab();
    expect(
      screen
        .getAllByRole("checkbox")
        .filter((c) => c.getAttribute("aria-checked") === "true"),
    ).toEqual([]);
    expect(
      screen.getByRole("button", { name: "Move selected" }),
    ).toHaveProperty("disabled", true);
  });

  it("selects only the clusters that can move, then moves them in one write", async () => {
    const { actions } = renderTab();
    await userEvent.click(
      screen.getByRole("button", { name: "Select all that can move" }),
    );
    await userEvent.click(
      screen.getByRole("button", { name: "Move selected" }),
    );
    expect(
      await screen.findByText(
        /team-orders\/orders: .*The brokers restart one at a time/,
      ),
    ).toBeTruthy();
    const confirm = await screen.findByRole("button", { name: "Move" });
    await waitFor(() => expect(confirm).toHaveProperty("disabled", false));
    await userEvent.click(confirm);
    await waitFor(() => expect(actions.applyMove).toHaveBeenCalledTimes(1));
    expect(
      actions.applyMove.mock.calls[0][0].apply.map((o) => o.metadata.name),
    ).toEqual(["orders"]);
    expect(
      await screen.findByText("Moved team-orders/orders to camunda-8-9-1."),
    ).toBeTruthy();
  });

  it("confirms a downgrade with allowVersionDowngrade on the claim", async () => {
    const { actions } = renderTab();
    await userEvent.click(
      within(rowOf("payments")).getByRole("button", {
        name: "Confirm downgrade",
      }),
    );
    const confirm = await screen.findByRole("button", {
      name: "Confirm downgrade",
    });
    await waitFor(() => expect(confirm).toHaveProperty("disabled", false));
    await userEvent.click(confirm);
    await waitFor(() => expect(actions.applyMove).toHaveBeenCalledTimes(1));
    expect(actions.applyMove.mock.calls[0][0].apply[0].spec).toMatchObject({
      releaseRef: "camunda-8-9-1",
      allowVersionDowngrade: "8.9.1",
    });
  });

  it("offers no move for a skip, a running backup, or an unmanaged cluster", () => {
    renderTab();
    for (const name of ["ledger", "billing", "legacy"])
      expect(
        within(rowOf(name)).queryByRole("button", { name: /Move|Confirm/ }),
      ).toBeNull();
  });

  it("links a cluster that the environment does not allow to its Environment", () => {
    renderTab();
    expect(
      within(rowOf("stage"))
        .getByRole("link", { name: "Open Environments" })
        .getAttribute("href"),
    ).toBe("/setup/environments/staging");
  });

  it("names each claim that failed after a partial write", async () => {
    const { actions } = renderTab({ data: freshCanMove() });
    actions.applyMove.mockResolvedValueOnce({
      ok: false,
      errors: [staleErrorOf("team-pay", "fresh")],
    });
    await userEvent.click(
      screen.getByRole("checkbox", { name: "Select team-orders/orders" }),
    );
    await userEvent.click(
      screen.getByRole("checkbox", { name: "Select team-pay/fresh" }),
    );
    await userEvent.click(
      screen.getByRole("button", { name: "Move selected" }),
    );
    expect(await screen.findByText(PARTIAL_MOVE_CAUTION)).toBeTruthy();
    const confirm = await screen.findByRole("button", { name: "Move" });
    await waitFor(() => expect(confirm).toHaveProperty("disabled", false));
    await userEvent.click(confirm);
    expect(
      await screen.findByText(
        "team-pay/fresh: The CamundaClusterClaim fresh changed since you opened it.",
      ),
    ).toBeTruthy();
    expect(
      actions.applyMove.mock.calls[0][0].apply.map((o) => o.metadata.name),
    ).toEqual(["orders", "fresh"]);
  });

  it("keeps the result of a partial write after the written claim closes the review", async () => {
    const data = freshCanMove();
    const { actions, rerender } = renderTab({ data });
    actions.applyMove.mockResolvedValueOnce({
      ok: false,
      errors: [staleErrorOf("team-pay", "fresh")],
    });
    await userEvent.click(
      screen.getByRole("checkbox", { name: "Select team-orders/orders" }),
    );
    await userEvent.click(
      screen.getByRole("checkbox", { name: "Select team-pay/fresh" }),
    );
    await userEvent.click(
      screen.getByRole("button", { name: "Move selected" }),
    );
    const confirm = await screen.findByRole("button", { name: "Move" });
    await waitFor(() => expect(confirm).toHaveProperty("disabled", false));
    await userEvent.click(confirm);
    await screen.findByText(
      "team-pay/fresh: The CamundaClusterClaim fresh changed since you opened it.",
    );
    rerender({
      ...data,
      claims: data.claims.map((c) =>
        c.metadata.name === "orders"
          ? { ...c, spec: { ...c.spec, releaseRef: "camunda-8-9-1" } }
          : c,
      ),
    });
    expect(
      screen.queryByRole("region", { name: "Review the move" }),
    ).toBeNull();
    const result = screen.getByRole("alert", {
      name: "The move did not finish",
    });
    expect(
      within(result).getByText("team-orders/orders: moved to camunda-8-9-1."),
    ).toBeTruthy();
    expect(
      within(result).getByText(
        "team-pay/fresh: The CamundaClusterClaim fresh changed since you opened it.",
      ),
    ).toBeTruthy();
    await userEvent.click(
      within(result).getByRole("button", { name: "Dismiss" }),
    );
    expect(
      screen.queryByRole("alert", { name: "The move did not finish" }),
    ).toBeNull();
  });

  it("clears the failed move when a retry writes", async () => {
    const { actions } = renderTab({ data: freshCanMove() });
    actions.applyMove.mockResolvedValueOnce({
      ok: false,
      errors: [
        { code: "Unavailable", message: "The API server did not answer." },
      ],
    });
    await userEvent.click(
      screen.getByRole("checkbox", { name: "Select team-orders/orders" }),
    );
    await userEvent.click(
      screen.getByRole("button", { name: "Move selected" }),
    );
    const confirm = await screen.findByRole("button", { name: "Move" });
    await waitFor(() => expect(confirm).toHaveProperty("disabled", false));
    await userEvent.click(confirm);
    await waitFor(() => expect(actions.applyMove).toHaveBeenCalledTimes(1));
    await userEvent.click(await screen.findByRole("button", { name: "Move" }));
    await waitFor(() => expect(actions.applyMove).toHaveBeenCalledTimes(2));
    await waitFor(() =>
      expect(
        screen.queryByRole("alert", { name: "The move did not finish" }),
      ).toBeNull(),
    );
  });

  it("keeps the failed move when the write rejects", async () => {
    const { actions } = renderTab({ data: freshCanMove() });
    actions.applyMove.mockRejectedValueOnce(new Error("connection lost"));
    await userEvent.click(
      screen.getByRole("checkbox", { name: "Select team-orders/orders" }),
    );
    await userEvent.click(
      screen.getByRole("button", { name: "Move selected" }),
    );
    const confirm = await screen.findByRole("button", { name: "Move" });
    await waitFor(() => expect(confirm).toHaveProperty("disabled", false));
    await userEvent.click(confirm);
    await waitFor(() => expect(actions.applyMove).toHaveBeenCalledTimes(1));
    await userEvent.click(await screen.findByRole("button", { name: "Back" }));
    const result = await screen.findByRole("alert", {
      name: "The move did not finish",
    });
    expect(
      within(result).getByText(/team-orders\/orders: not moved/),
    ).toBeTruthy();
  });

  it("keeps the boxes and Move selected in step after Back", async () => {
    renderTab({ data: freshCanMove() });
    await userEvent.click(
      screen.getByRole("checkbox", { name: "Select team-orders/orders" }),
    );
    await userEvent.click(
      screen.getByRole("checkbox", { name: "Select team-pay/fresh" }),
    );
    await userEvent.click(
      screen.getByRole("button", { name: "Move selected" }),
    );
    await userEvent.click(await screen.findByRole("button", { name: "Back" }));
    for (const name of ["Select team-orders/orders", "Select team-pay/fresh"])
      expect(
        screen.getByRole("checkbox", { name }).getAttribute("aria-checked"),
      ).toBe("true");
    await userEvent.click(
      screen.getByRole("checkbox", { name: "Select team-pay/fresh" }),
    );
    await userEvent.click(
      screen.getByRole("button", { name: "Move selected" }),
    );
    expect(
      await screen.findByText(
        /team-orders\/orders: .*The brokers restart one at a time/,
      ),
    ).toBeTruthy();
    expect(screen.queryByText(/team-pay\/fresh: /)).toBeNull();
  });

  it("names a claim whose releaseRef another field manager owns, and writes nothing", async () => {
    const { actions } = renderTab();
    actions.previewMove.mockResolvedValueOnce({
      objects: [],
      errors: [
        {
          code: "Conflict",
          message: 'conflict with "kubectl-client-side-apply"',
          path: "spec.releaseRef",
          manager: "kubectl-client-side-apply",
          ref: staleErrorOf("team-orders", "orders").ref,
        },
      ],
    });
    await userEvent.click(
      within(rowOf("orders")).getByRole("button", { name: "Move" }),
    );
    expect(
      await screen.findByText(
        'team-orders/orders: spec.releaseRef belongs to the field manager "kubectl-client-side-apply". Change it with that tool, or leave this cluster out of the move.',
      ),
    ).toBeTruthy();
    expect(
      within(screen.getByRole("region", { name: "Review the move" })).getByRole(
        "button",
        { name: "Move" },
      ),
    ).toHaveProperty("disabled", true);
    expect(actions.applyMove).not.toHaveBeenCalled();
  });

  it("drops a selected row that a backup now blocks", async () => {
    const { rerender } = renderTab();
    await userEvent.click(
      screen.getByRole("checkbox", { name: "Select team-orders/orders" }),
    );
    expect(
      screen.getByRole("button", { name: "Move selected" }),
    ).toHaveProperty("disabled", false);
    rerender(withBackupOf("orders"));
    expect(
      screen.getByRole("button", { name: "Move selected" }),
    ).toHaveProperty("disabled", true);
  });

  it("closes an open review when a backup of its cluster starts", async () => {
    const { rerender } = renderTab();
    await userEvent.click(
      within(rowOf("orders")).getByRole("button", { name: "Move" }),
    );
    expect(
      screen.getByRole("region", { name: "Review the move" }),
    ).toBeTruthy();
    rerender(withBackupOf("orders", "Pending"));
    expect(
      screen.queryByRole("region", { name: "Review the move" }),
    ).toBeNull();
    expect(
      within(rowOf("orders")).getByText(
        "Backup orders-now is pending. The move waits until it ends.",
      ),
    ).toBeTruthy();
  });

  it("closes an open review when the version of the release changes", async () => {
    const data = rolloutData();
    const { rerender } = renderTab({ data });
    await userEvent.click(
      within(rowOf("orders")).getByRole("button", { name: "Move" }),
    );
    expect(
      screen.getByRole("region", { name: "Review the move" }),
    ).toBeTruthy();
    rerender(data, {
      ...targetRelease,
      spec: { ...targetRelease.spec, version: "8.9.99" },
    });
    expect(
      screen.queryByRole("region", { name: "Review the move" }),
    ).toBeNull();
  });

  it("closes the review of a refused downgrade when a backup of its cluster starts", async () => {
    const base = rolloutData();
    const refused: RolloutData = {
      ...base,
      clusters: base.clusters.map((c) =>
        c.metadata?.name === "payments"
          ? cluster("team-pay", "payments", {
              version: "8.9.4",
              releaseRef: "camunda-8-9-1",
              conditions: [
                {
                  type: "Ready",
                  status: "False",
                  reason: "VersionDowngradeRefused",
                  message: "the effective version is refused",
                },
              ],
            })
          : c,
      ),
      claims: base.claims.map((c) =>
        c.metadata.name === "payments"
          ? clusterClaim("team-pay", "payments", {
              releaseRef: "camunda-8-9-1",
            })
          : c,
      ),
    };
    const { rerender } = renderTab({ data: refused });
    await userEvent.click(
      within(rowOf("payments")).getByRole("button", {
        name: "Confirm downgrade",
      }),
    );
    expect(
      screen.getByRole("region", { name: "Review the move" }),
    ).toBeTruthy();
    rerender({
      ...refused,
      rdbmsBackups: [
        {
          apiVersion: "core.camunda.io/v1",
          kind: "LogicalBackupRDBMS",
          metadata: { namespace: "team-pay", name: "payments-now" },
          spec: { clusterRef: { name: "payments" } },
          status: { phase: "Running" },
        } as Resource<"LogicalBackupRDBMS">,
      ],
    });
    expect(
      screen.queryByRole("region", { name: "Review the move" }),
    ).toBeNull();
    expect(
      within(rowOf("payments")).getByText(
        "Backup payments-now is running. The move waits until it ends.",
      ),
    ).toBeTruthy();
  });

  it("offers no move, names the list, and closes an open review while a list that the move checks is not read", async () => {
    const { rerender } = renderTab();
    await userEvent.click(
      within(rowOf("orders")).getByRole("button", { name: "Move" }),
    );
    rerender({
      ...rolloutData(),
      errors: [{ kind: "LogicalBackupRDBMS", message: "forbidden" }],
      stale: ["PluginRequest"],
    });
    expect(
      screen.queryByRole("region", { name: "Review the move" }),
    ).toBeNull();
    expect(
      screen.getByText("Could not read LogicalBackupRDBMS: forbidden"),
    ).toBeTruthy();
    expect(
      screen.getByText(
        "A move is possible when the fleet manager has read LogicalBackupRDBMS, PluginRequest.",
      ),
    ).toBeTruthy();
    expect(within(rowOf("orders")).queryByText("Can move.")).toBeNull();
    expect(
      screen.queryByRole("button", { name: /^(Move|Confirm downgrade)$/ }),
    ).toBeNull();
    expect(
      screen.getByRole("button", { name: "Select all that can move" }),
    ).toHaveProperty("disabled", true);
  });

  it("shows the errors of a claim that the checks refuse, and opens no review", async () => {
    const data = rolloutData();
    renderTab({
      data: {
        ...data,
        claims: data.claims.map((c) =>
          c.metadata.name === "fresh"
            ? ({
                ...clusterClaim("team-pay", "fresh"),
                spec: {
                  presetRef: 7,
                  releaseRef: null,
                  cluster: { version: 42 },
                },
              } as unknown as Resource<"CamundaClusterClaim">)
            : c,
        ),
      },
    });
    await userEvent.click(
      within(rowOf("fresh")).getByRole("button", { name: "Move" }),
    );
    const alert = screen.getByRole("alert");
    expect(
      within(alert).getAllByText(/^team-pay\/fresh: /).length,
    ).toBeGreaterThan(0);
    expect(
      screen.queryByRole("region", { name: "Review the move" }),
    ).toBeNull();
    expect(screen.getByRole("region", { name: "Clusters" })).toBeTruthy();
  });

  it("offers no move without a write mode", () => {
    renderTab({ mode: undefined });
    expect(screen.queryAllByRole("checkbox")).toEqual([]);
    expect(screen.queryByRole("button", { name: "Move" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Move selected" })).toBeNull();
    expect(
      screen.getByRole("link", { name: /settings/i }).getAttribute("href"),
    ).toBe("/setup/settings");
  });
});
