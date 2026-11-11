// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import type { TopologyBox } from "./topology";
import { TopologyBoxes, TopologyView } from "./topology-view";

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
  it("shows a box for each process with its embedded chips and the name and state of each pod", () => {
    render(<TopologyView boxes={boxes} />);
    const topology = screen.getByRole("region", { name: "Topology" });
    expect(within(topology).getByText("Brokers")).toBeTruthy();
    expect(within(topology).getByText("StatefulSet")).toBeTruthy();
    expect(within(topology).getByText("Embedded: Gateway")).toBeTruthy();
    const pods = within(topology).getByRole("list", {
      name: "Pods of Brokers",
    });
    expect(
      within(pods)
        .getAllByRole("listitem")
        .map((pod) => pod.textContent),
    ).toEqual([
      "orders-zeebe-0 Ready",
      "orders-zeebe-1 CrashLoopBackOff",
      "orders-zeebe-2 Pending",
      "orders-zeebe-3 Terminating",
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

  it("a component condition drives the dot of its box", () => {
    render(
      <TopologyView
        boxes={boxes}
        conditions={{ zeebe: { status: "False", reason: "Degraded" } }}
      />,
    );
    expect(
      screen.getByRole("img", { name: "Brokers: Not ready (Degraded)" }),
    ).toBeTruthy();
    expect(screen.queryByRole("img", { name: /^Operate:/ })).toBeNull();
  });
});

describe("TopologyBoxes", () => {
  it("marks a new box and a removed box", () => {
    render(
      <TopologyBoxes
        boxes={boxes}
        marks={{ zeebe: "new", operate: "removed" }}
        label="After the change"
      />,
    );
    const list = screen.getByRole("list", { name: "After the change" });
    const [brokers, operate] = within(list)
      .getAllByRole("listitem")
      .filter((li) => li.parentElement === list);
    expect(within(brokers).getByText("New")).toBeTruthy();
    expect(within(operate).getByText("Removed")).toBeTruthy();
  });

  it("leaves out the pods and the empty note when pods is false", () => {
    render(<TopologyBoxes boxes={boxes} pods={false} label="Now" />);
    expect(screen.queryByRole("list", { name: "Pods of Brokers" })).toBeNull();
    expect(screen.queryByText("No pods")).toBeNull();
    expect(screen.getByText("Embedded: Gateway")).toBeTruthy();
  });
});
