// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { PodTable, USAGE_FOOTNOTE, USAGE_UNAVAILABLE } from "./pod-table";

afterEach(cleanup);

const GiB = 2 ** 30;

describe("PodTable", () => {
  const pods = [
    {
      metadata: { name: "orders-zeebe-0" },
      spec: { nodeName: "node-2" },
      status: {
        phase: "Running",
        conditions: [{ type: "Ready", status: "True" }],
        containerStatuses: [
          { ready: true, restartCount: 2 },
          { ready: true, restartCount: 1 },
        ],
      },
    },
    {
      metadata: { name: "orders-zeebe-1" },
      status: {
        phase: "Running",
        containerStatuses: [
          {
            ready: false,
            restartCount: 7,
            state: { waiting: { reason: "CrashLoopBackOff" } },
          },
        ],
      },
    },
  ];
  const usage = [
    {
      pod: "orders-zeebe-0",
      cpu: { usedCores: 0.25, limitCores: 1 },
      memory: { usedBytes: GiB, limitBytes: 4 * GiB },
    },
  ];
  const row = (pod: string) =>
    screen
      .getByRole("rowheader", { name: new RegExp(`^${pod}`) })
      .closest("tr")!;

  it("names each column after what its cells hold", () => {
    render(<PodTable pods={pods} usage={usage} />);
    expect(
      screen.getAllByRole("columnheader").map((h) => h.textContent),
    ).toEqual(["Pod", "State", "Restarts", "CPU", "Memory"]);
  });

  it("shows the node of a pod under its name, and - for a pod without one", () => {
    render(<PodTable pods={pods} usage={usage} />);
    expect(
      screen.getByRole("rowheader", { name: /orders-zeebe-0/ }).textContent,
    ).toBe("orders-zeebe-0node-2");
    expect(
      screen.getByRole("rowheader", { name: /orders-zeebe-1/ }).textContent,
    ).toBe("orders-zeebe-1-");
  });

  it("shows the state of a pod in the words of the topology", () => {
    render(<PodTable pods={pods} usage={usage} />);
    expect(row("orders-zeebe-0").textContent).toContain("Ready");
    expect(row("orders-zeebe-1").textContent).toContain("CrashLoopBackOff");
  });

  it("counts the restarts of every container of a pod", () => {
    render(<PodTable pods={pods} usage={usage} />);
    expect(row("orders-zeebe-0").querySelectorAll("td")[1]?.textContent).toBe(
      "3",
    );
  });

  it("counts the restarts of an init container too", () => {
    render(
      <PodTable
        pods={[
          {
            metadata: { name: "orders-zeebe-0" },
            status: {
              initContainerStatuses: [{ ready: true, restartCount: 5 }],
              containerStatuses: [{ ready: true, restartCount: 1 }],
            },
          },
        ]}
        usage={usage}
      />,
    );
    expect(row("orders-zeebe-0").querySelectorAll("td")[1]?.textContent).toBe(
      "6",
    );
  });

  it("shows the CPU and the memory of a pod against its limit, and where they come from", () => {
    render(<PodTable pods={pods} usage={usage} />);
    const cells = within(row("orders-zeebe-0"));
    expect(
      cells.getByRole("meter", { name: "CPU" }).getAttribute("aria-valuetext"),
    ).toBe("250m of 1 cores");
    expect(
      cells
        .getByRole("meter", { name: "Memory" })
        .getAttribute("aria-valuetext"),
    ).toBe("1 GiB of 4 GiB");
    expect(screen.getByText(USAGE_FOOTNOTE)).toBeTruthy();
  });

  it("shows - for the CPU and the memory of a pod without usage data", () => {
    render(<PodTable pods={pods} usage={usage} />);
    const tds = row("orders-zeebe-1").querySelectorAll("td");
    expect([tds[2]?.textContent, tds[3]?.textContent]).toEqual(["-", "-"]);
  });

  it("hides the CPU and memory columns when usage is unavailable, and says why", () => {
    render(<PodTable pods={pods} usage="unavailable" />);
    expect(screen.queryByRole("columnheader", { name: "CPU" })).toBeNull();
    expect(screen.queryByRole("columnheader", { name: "Memory" })).toBeNull();
    expect(screen.getByText(USAGE_UNAVAILABLE)).toBeTruthy();
    expect(screen.queryByText(USAGE_FOOTNOTE)).toBeNull();
  });

  it("says that usage is not available without claiming why, as a failed read looks the same", () =>
    expect(USAGE_UNAVAILABLE).toBe(
      "Usage is not available. The cluster has no metrics server, or the app could not read the usage.",
    ));
});
