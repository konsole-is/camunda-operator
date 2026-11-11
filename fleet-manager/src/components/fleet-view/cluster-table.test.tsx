// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { BackupRow } from "@/components/cluster/backups";
import { ClusterTable, WaitingTable } from "./cluster-table";
import type { Warning } from "@/lib/warnings";
import type { ClusterRow } from "./model";

afterEach(cleanup);

const NOW = "2026-11-10T12:00:00Z";
const NONE = new Map<string, Warning[]>();

const row = (name: string, over: Partial<ClusterRow> = {}): ClusterRow => ({
  id: `cluster/team-a/${name}`,
  namespace: "team-a",
  name,
  managed: true,
  components: [],
  sections: [],
  href: `/fleet/clusters/team-a/${name}`,
  ...over,
});

const rowOf = (name: string) =>
  screen.getByRole("row", { name: new RegExp(name) });

describe("ClusterTable", () => {
  it("has the six columns in order", () => {
    render(
      <ClusterTable
        now={NOW}
        warnings={NONE}
        label="Production"
        rows={[row("orders")]}
      />,
    );
    expect(
      screen.getAllByRole("columnheader").map((h) => h.textContent),
    ).toEqual([
      "Cluster",
      "Status",
      "Version",
      "Components",
      "Last backup",
      "Warnings",
    ]);
  });

  it("links a cluster to its page and shows Not managed without the claim label", () => {
    render(
      <ClusterTable
        now={NOW}
        warnings={NONE}
        label="Production"
        rows={[row("orders"), row("legacy", { managed: false })]}
      />,
    );
    expect(
      screen.getByRole("link", { name: "orders" }).getAttribute("href"),
    ).toBe("/fleet/clusters/team-a/orders");
    expect(within(rowOf("legacy")).getByText("Not managed")).toBeTruthy();
    expect(within(rowOf("orders")).queryByText("Not managed")).toBeNull();
    expect(within(rowOf("orders")).getByText("team-a")).toBeTruthy();
  });

  it("names a claim without a cluster without a link", () => {
    render(
      <ClusterTable
        now={NOW}
        warnings={NONE}
        label="Production"
        rows={[row("returns", { href: undefined })]}
      />,
    );
    expect(screen.queryByRole("link", { name: "returns" })).toBeNull();
    expect(within(rowOf("returns")).getByText("returns")).toBeTruthy();
  });

  it("shows the reason of a cluster that is not ready", () => {
    render(
      <ClusterTable
        now={NOW}
        warnings={NONE}
        label="Production"
        rows={[row("orders", { ready: { status: "False", reason: "Down" } })]}
      />,
    );
    expect(within(rowOf("orders")).getByText("Not ready · Down")).toBeTruthy();
  });

  it("shows the reason of a ready cluster that is not Healthy", () => {
    render(
      <ClusterTable
        now={NOW}
        warnings={NONE}
        label="Production"
        rows={[
          row("orders", { ready: { status: "True", reason: "Suspended" } }),
        ]}
      />,
    );
    expect(within(rowOf("orders")).getByText("Ready · Suspended")).toBeTruthy();
  });

  it("shows the version with the preset below it, and a dash without a version", () => {
    render(
      <ClusterTable
        now={NOW}
        warnings={NONE}
        label="Production"
        rows={[
          row("orders", { version: "8.9.23", preset: "medium" }),
          row("billing", { preset: "small" }),
        ]}
      />,
    );
    const cells = (name: string) =>
      within(rowOf(name)).getAllByRole("cell")[2].textContent;
    expect(cells("orders")).toBe("8.9.23medium");
    expect(cells("billing")).toBe("-small");
  });

  it("shows the component dots of a cluster", () => {
    render(
      <ClusterTable
        now={NOW}
        warnings={NONE}
        label="Production"
        rows={[
          row("orders", {
            components: [
              { id: "connectors", label: "Connectors", used: false },
            ],
          }),
        ]}
      />,
    );
    expect(
      within(rowOf("orders")).getByRole("img", {
        name: "Connectors: Not used",
      }),
    ).toBeTruthy();
  });

  it("shows the phase and the start of the newest backup", () => {
    render(
      <ClusterTable
        now={NOW}
        warnings={NONE}
        label="Production"
        rows={[
          row("orders", {
            lastBackup: {
              name: "orders-nightly",
              phase: "Failed",
              tone: "bad",
              started: "2026-11-10 02:00 UTC",
            } as BackupRow,
          }),
        ]}
      />,
    );
    const cell = within(rowOf("orders")).getAllByRole("cell")[4];
    expect(cell.textContent).toContain("Failed");
    expect(cell.textContent).toContain("2026-11-10 02:00 UTC");
  });

  it("shows No backups for a cluster without backups", () => {
    render(
      <ClusterTable
        now={NOW}
        warnings={NONE}
        label="Production"
        rows={[row("orders")]}
      />,
    );
    expect(within(rowOf("orders")).getByText("No backups")).toBeTruthy();
  });

  it("shows the warnings of a row and nothing for a row without warnings", () => {
    const warnings = new Map<string, Warning[]>([
      [
        "cluster/team-a/orders",
        [
          {
            kind: "backup",
            severity: "Warning",
            label: "Backup failed",
            detail: "The newest backup orders-nightly failed",
          },
        ],
      ],
    ]);
    render(
      <ClusterTable
        now={NOW}
        warnings={warnings}
        label="Production"
        rows={[row("orders"), row("billing")]}
      />,
    );
    expect(
      within(rowOf("orders")).getByRole("button", { name: "Backup failed" }),
    ).toBeTruthy();
    expect(within(rowOf("billing")).getAllByRole("cell")[5].textContent).toBe(
      "-",
    );
  });

  it("names the table by its label", () => {
    render(
      <ClusterTable
        now={NOW}
        warnings={NONE}
        label="Production"
        rows={[row("orders")]}
      />,
    );
    expect(screen.getByRole("region", { name: "Production" })).toBeTruthy();
  });
});

describe("WaitingTable", () => {
  it("lists a claim that waits for GitOps with its file and the age of the push", () => {
    render(
      <WaitingTable
        rows={[
          {
            id: "push/CamundaClusterClaim/team-x/returns",
            key: "CamundaClusterClaim/team-x/returns",
            namespace: "team-x",
            name: "returns",
            path: "clusters/team-x/returns.yaml",
            pushedAt: "2026-11-10T11:58:00Z",
          },
        ]}
        now={NOW}
      />,
    );
    expect(
      screen.getAllByRole("columnheader").map((h) => h.textContent),
    ).toEqual(["Cluster", "Status", "File", "Pushed", ""]);
    const cells = within(rowOf("returns"))
      .getAllByRole("cell")
      .map((c) => c.textContent);
    expect(cells).toEqual([
      "returnsteam-x",
      "Waiting for GitOps",
      "clusters/team-x/returns.yaml",
      "2m ago",
      "Stop waiting",
    ]);
  });

  it("gives the key of the push to onStopWaiting", async () => {
    const onStopWaiting = vi.fn();
    render(
      <WaitingTable
        rows={[
          {
            id: "push/CamundaClusterClaim/team-x/returns",
            key: "CamundaClusterClaim/team-x/returns",
            namespace: "team-x",
            name: "returns",
            path: "clusters/team-x/returns.yaml",
            pushedAt: "2026-11-10T11:58:00Z",
          },
        ]}
        now={NOW}
        onStopWaiting={onStopWaiting}
      />,
    );
    await userEvent.click(screen.getByRole("button", { name: "Stop waiting" }));
    expect(onStopWaiting).toHaveBeenCalledWith(
      "CamundaClusterClaim/team-x/returns",
    );
  });
});
