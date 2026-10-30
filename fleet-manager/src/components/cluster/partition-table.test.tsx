// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import type { PartitionHealth } from "@/lib/observe/types";
import { PartitionTable } from "./partition-table";

afterEach(cleanup);

const degraded: PartitionHealth[] = [
  {
    partition: 1,
    leader: "orders-zeebe-0",
    followers: ["orders-zeebe-1", "orders-zeebe-2"],
    processorPhase: "PROCESSING",
    health: "healthy",
  },
  {
    partition: 2,
    leader: "orders-zeebe-1",
    followers: ["orders-zeebe-2"],
    processorPhase: "PROCESSING",
    health: "unhealthy",
  },
];

const cells = (row: HTMLElement) =>
  [...row.querySelectorAll("th, td")].map((c) => c.textContent);

describe("PartitionTable", () => {
  it("shows a partition with a follower down", () => {
    render(<PartitionTable partitions={degraded} />);
    const table = screen.getByRole("region", { name: "Partitions" });
    const row = within(table)
      .getByRole("rowheader", { name: "2" })
      .closest("tr")!;
    expect(cells(row)).toEqual([
      "2",
      "orders-zeebe-1",
      "orders-zeebe-2",
      "PROCESSING",
      "Unhealthy",
    ]);
  });

  it("shows - for a partition without a leader, followers, or phase", () => {
    render(
      <PartitionTable
        partitions={[{ partition: 3, followers: [], health: "dead" }]}
      />,
    );
    const row = screen.getByRole("rowheader", { name: "3" }).closest("tr")!;
    expect(cells(row)).toEqual(["3", "-", "-", "-", "Dead"]);
  });

  it("says when the management API does not answer", () => {
    render(<PartitionTable partitions="unavailable" />);
    expect(
      screen.getByText(
        "Partitions are not available. The management API of the brokers did not answer.",
      ),
    ).toBeTruthy();
    expect(screen.queryByRole("table")).toBeNull();
  });

  it("says that health needs Prometheus when every health is unknown", () => {
    render(
      <PartitionTable
        partitions={degraded.map((p) => ({ ...p, health: "unknown" }))}
      />,
    );
    expect(screen.getAllByText("Unknown")).toHaveLength(2);
    expect(
      screen.getByText("Health needs Prometheus. Set it in the settings."),
    ).toBeTruthy();
  });

  it("gives no Prometheus note when one health is known", () => {
    render(<PartitionTable partitions={degraded} />);
    expect(screen.queryByText(/Health needs Prometheus/)).toBeNull();
    expect(screen.getByText("Healthy")).toBeTruthy();
  });

  it("says Loading before the first answer", () => {
    render(<PartitionTable partitions={undefined} />);
    expect(screen.getByText("Loading")).toBeTruthy();
  });
});
