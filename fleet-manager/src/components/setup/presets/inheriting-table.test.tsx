// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { presetFixtures } from "./fixtures";
import { inheritingRows } from "./inheriting";
import { InheritingTable } from "./inheriting-table";

afterEach(cleanup);

const sources = presetFixtures();
const rows = inheritingRows(sources.clusterPresets[0], sources);

describe("InheritingTable", () => {
  it("shows each resource with its environment, namespace, own fields, and Ready", () => {
    render(<InheritingTable rows={rows} unread={[]} />);
    const cells = within(screen.getByRole("row", { name: /orders/ }))
      .getAllByRole("cell")
      .map((c) => c.textContent);
    expect(cells[0]).toContain("orders");
    expect(cells.slice(1, 3)).toEqual(["Production", "team-orders"]);
    expect(cells[3]).toContain("zeebe.replicas");
    expect(cells[4]).toBe("Ready");
  });

  it("shows a slot-filled field as Filled by its section", () => {
    render(<InheritingTable rows={rows} unread={[]} />);
    expect(
      screen.getByText("zeebe.storageClassName").closest("li")?.textContent,
    ).toContain("Filled by Volume encryption");
  });

  it("links a cluster to its cluster page", () => {
    render(<InheritingTable rows={rows} unread={[]} />);
    expect(
      screen.getByRole("link", { name: /orders/ }).getAttribute("href"),
    ).toBe("/fleet/clusters/team-orders/orders");
  });

  it("says that no resource inherits the preset", () => {
    render(<InheritingTable rows={[]} unread={[]} />);
    expect(screen.getByText("No resource inherits this preset.")).toBeTruthy();
  });

  it("names the kind it cannot read and never says that no resource inherits", () => {
    render(<InheritingTable rows={[]} unread={["CamundaCluster"]} />);
    expect(screen.getByRole("alert").textContent).toBe(
      "The inheriting resources are not known while the fleet manager cannot read CamundaCluster.",
    );
    expect(screen.queryByText("No resource inherits this preset.")).toBeNull();
  });
});
