// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { PodTable } from "./pod-table";

afterEach(cleanup);

const now = "2026-10-15T12:00:00Z";

describe("PodTable", () => {
  const pods = [
    {
      metadata: {
        name: "orders-zeebe-0",
        creationTimestamp: "2026-10-15T09:00:00Z",
      },
      status: {
        phase: "Running",
        containerStatuses: [
          { ready: true, restartCount: 2 },
          { ready: false, restartCount: 0 },
        ],
      },
    },
  ];

  it("hides the usage column when usage is unavailable", () => {
    render(<PodTable pods={pods} usage="unavailable" now={now} />);
    expect(screen.queryByRole("columnheader", { name: "Usage" })).toBeNull();
    expect(screen.getByText(/Usage is not available/)).toBeTruthy();
    const row = screen.getByRole("row", { name: /orders-zeebe-0/ });
    expect(row.textContent).toContain("1/2");
    expect(row.textContent).toContain("2");
    expect(row.textContent).toContain("3h");
  });

  it("counts the declared containers of a pod without container status", () => {
    render(
      <PodTable
        pods={[
          {
            metadata: { name: "orders-zeebe-1" },
            spec: { containers: [{}, {}] },
            status: { phase: "Pending" },
          },
        ]}
        usage="unavailable"
        now={now}
      />,
    );
    const row = screen.getByRole("row", { name: /orders-zeebe-1/ });
    expect(row.textContent).toContain("0/2");
  });

  it("shows CPU and memory for each pod", () => {
    render(
      <PodTable
        pods={pods}
        usage={[
          {
            pod: "orders-zeebe-0",
            cpu: { usedCores: 0.25, limitCores: 1 },
            memory: { usedBytes: 2 ** 30, limitBytes: 4 * 2 ** 30 },
          },
        ]}
        now={now}
      />,
    );
    expect(
      screen.getByRole("meter", { name: "CPU" }).getAttribute("aria-valuetext"),
    ).toBe("250m of 1 cores");
    expect(
      screen
        .getByRole("meter", { name: "Memory" })
        .getAttribute("aria-valuetext"),
    ).toBe("1 GiB of 4 GiB");
  });

  it("shows the node of each pod", () => {
    render(
      <PodTable
        pods={[
          {
            metadata: { name: "orders-zeebe-0" },
            spec: { nodeName: "node-2" },
          },
          { metadata: { name: "orders-zeebe-1" } },
        ]}
        usage="unavailable"
        now={now}
      />,
    );
    expect(screen.getByRole("columnheader", { name: "Node" })).toBeTruthy();
    const node = (pod: string) =>
      screen
        .getByRole("row", { name: new RegExp(pod) })
        .querySelectorAll("td")[2]?.textContent;
    expect(node("orders-zeebe-0")).toBe("node-2");
    expect(node("orders-zeebe-1")).toBe("-");
  });
});
