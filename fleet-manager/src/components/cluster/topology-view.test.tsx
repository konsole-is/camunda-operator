// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import type { TopologyBox } from "./topology";
import { TopologyView } from "./topology-view";

afterEach(cleanup);

const boxes: TopologyBox[] = [
  {
    process: "zeebe",
    label: "Brokers",
    workload: "StatefulSet",
    embedded: [{ process: "gateway", label: "Gateway" }],
    pods: [
      { name: "orders-zeebe-0", tone: "ok", state: "Ready" },
      { name: "orders-zeebe-1", tone: "bad", state: "CrashLoopBackOff" },
      { name: "orders-zeebe-2", tone: "warn", state: "Pending" },
      { name: "orders-zeebe-3", tone: "off", state: "Terminating" },
    ],
  },
  {
    process: "operate",
    label: "Operate",
    workload: "Deployment",
    embedded: [],
    pods: [],
  },
];

describe("TopologyView", () => {
  it("shows a box for each process with its embedded chips and pod dots", () => {
    render(<TopologyView boxes={boxes} />);
    const topology = screen.getByRole("region", { name: "Topology" });
    expect(within(topology).getByText("Brokers")).toBeTruthy();
    expect(within(topology).getByText("StatefulSet")).toBeTruthy();
    expect(within(topology).getByText("Embedded: Gateway")).toBeTruthy();
    expect(
      within(topology)
        .getAllByRole("img")
        .map((dot) => dot.getAttribute("aria-label")),
    ).toEqual([
      "orders-zeebe-0: Ready",
      "orders-zeebe-1: Not ready (CrashLoopBackOff)",
      "orders-zeebe-2: Unknown (Pending)",
      "orders-zeebe-3: Not used",
    ]);
  });

  it("says No pods for a box without pods", () => {
    render(<TopologyView boxes={boxes} />);
    expect(screen.getByText("No pods")).toBeTruthy();
  });

  it("shows the actions", () => {
    render(<TopologyView boxes={boxes} actions={<button>Scale</button>} />);
    expect(screen.getByRole("button", { name: "Scale" })).toBeTruthy();
  });
});
