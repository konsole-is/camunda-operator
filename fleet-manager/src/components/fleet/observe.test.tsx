// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it } from "vitest";
import type { LogLine } from "@/lib/observe/types";
import { EventList } from "./event-list";
import { LogViewer } from "./log-viewer";
import { MetricChart } from "./metric-chart";
import { PodTable } from "./pod-table";
import { ProgressRows } from "./progress-rows";
import { UsageBars } from "./usage-bars";

afterEach(cleanup);

const now = "2026-10-15T12:00:00Z";

describe("MetricChart", () => {
  it('renders nothing for "unavailable"', () => {
    const { container } = render(
      <MetricChart title="Exported records" series="unavailable" />,
    );
    expect(container.innerHTML).toBe("");
  });

  it("says so when the range has no data", () => {
    render(<MetricChart title="Exported records" series={[]} />);
    expect(screen.getByText("No data for this range")).toBeTruthy();
  });
});

describe("UsageBars", () => {
  it.each([
    ["a request of 0", { usedCores: 0.5, requestCores: 0 }, "0.5", "0.5"],
    ["no use and no limit", { usedCores: 0 }, "0", "1"],
  ])("keeps a valid meter range with %s", (_, cpu, now, max) => {
    render(<UsageBars usage={{ pod: "x", cpu, memory: { usedBytes: 0 } }} />);
    const meter = screen.getByRole("meter", { name: "CPU" });
    expect(meter.getAttribute("aria-valuenow")).toBe(now);
    expect(meter.getAttribute("aria-valuemax")).toBe(max);
  });

  it("keeps the meter value in range when the use passes the request", () => {
    render(
      <UsageBars
        usage={{
          pod: "x",
          cpu: { usedCores: 0.5, requestCores: 0.25 },
          memory: { usedBytes: 2 ** 30 },
        }}
      />,
    );
    const cpu = screen.getByRole("meter", { name: "CPU" });
    expect(cpu.getAttribute("aria-valuenow")).toBe("0.25");
    expect(cpu.getAttribute("aria-valuemax")).toBe("0.25");
    expect(cpu.getAttribute("aria-valuetext")).toBe("500m of 250m cores");
  });
});

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
});

describe("LogViewer", () => {
  const line = (message: string, level = "info"): LogLine => ({
    raw: message,
    message,
    level,
    time: "2026-10-15T11:59:00Z",
    fields: {},
  });

  it("keeps the shown lines while paused and counts the new ones", async () => {
    const { rerender } = render(<LogViewer lines={[line("a"), line("b")]} />);
    await userEvent.click(screen.getByRole("button", { name: "Pause" }));
    rerender(
      <LogViewer lines={[line("a"), line("b"), line("c"), line("d")]} />,
    );
    expect(screen.queryByText("c")).toBeNull();
    expect(screen.getByText("2 new lines")).toBeTruthy();
    await userEvent.click(screen.getByRole("button", { name: "Follow" }));
    expect(screen.getByText("d")).toBeTruthy();
  });

  it("keeps the shown lines while paused when the buffer drops old lines", async () => {
    const { rerender } = render(<LogViewer lines={[line("a"), line("b")]} />);
    await userEvent.click(screen.getByRole("button", { name: "Pause" }));
    rerender(<LogViewer lines={[line("b"), line("c")]} />);
    expect(screen.getByText("a")).toBeTruthy();
    expect(screen.queryByText("c")).toBeNull();
    expect(screen.getByText("1 new line")).toBeTruthy();
  });

  it("counts a new line that repeats a shown line", async () => {
    const shown = line("retrying");
    const { rerender } = render(<LogViewer lines={[shown]} />);
    await userEvent.click(screen.getByRole("button", { name: "Pause" }));
    rerender(<LogViewer lines={[shown, line("retrying")]} />);
    expect(screen.getByText("1 new line")).toBeTruthy();
  });

  it("keeps the level of the paused lines in the level list", async () => {
    const warn = line("slow disk", "warn");
    const { rerender } = render(<LogViewer lines={[warn]} />);
    await userEvent.click(screen.getByRole("button", { name: "Pause" }));
    rerender(<LogViewer lines={[line("started")]} />);
    expect(screen.getByRole("option", { name: "warn" })).toBeTruthy();
  });

  it("filters by level and by text", async () => {
    render(
      <LogViewer
        lines={[
          line("started"),
          line("disk full", "error"),
          line("retry", "warn"),
        ]}
      />,
    );
    await userEvent.selectOptions(
      screen.getByRole("combobox", { name: "Level" }),
      "error",
    );
    expect(screen.queryByText("started")).toBeNull();
    expect(screen.getByText("disk full")).toBeTruthy();
    await userEvent.selectOptions(
      screen.getByRole("combobox", { name: "Level" }),
      "",
    );
    await userEvent.type(
      screen.getByRole("searchbox", { name: "Filter" }),
      "retr",
    );
    expect(screen.getByText("retry")).toBeTruthy();
    expect(screen.queryByText("disk full")).toBeNull();
  });
});

describe("EventList", () => {
  it("lists the newest event first and marks a warning", () => {
    render(
      <EventList
        now={now}
        events={[
          {
            type: "Normal",
            reason: "Created",
            message: "Created the StatefulSet",
            lastTimestamp: "2026-10-15T10:00:00Z",
          },
          {
            type: "Warning",
            reason: "BackOff",
            message: "Back-off restarting the container",
            count: 3,
            lastTimestamp: "2026-10-15T11:59:00Z",
          },
        ]}
      />,
    );
    const items = screen
      .getAllByRole("listitem")
      .map((li) => li.textContent ?? "");
    expect(items[0]).toContain("BackOff");
    expect(items[0]).toContain("Warning");
    expect(items[0]).toContain("3 times");
    expect(items[1]).toContain("Created");
  });

  it("orders, ages, and counts a recurring event by its series", () => {
    render(
      <EventList
        now={now}
        events={[
          {
            type: "Normal",
            reason: "Scheduled",
            message: "Scheduled the pod",
            eventTime: "2026-10-15T11:00:00Z",
          },
          {
            type: "Warning",
            reason: "Unhealthy",
            message: "Readiness probe failed",
            eventTime: "2026-10-15T10:00:00Z",
            series: { count: 4, lastObservedTime: "2026-10-15T11:59:00Z" },
          },
        ]}
      />,
    );
    const items = screen
      .getAllByRole("listitem")
      .map((li) => li.textContent ?? "");
    expect(items[0]).toContain("Unhealthy");
    expect(items[0]).toContain("1m");
    expect(items[0]).toContain("4 times");
  });

  it("says so when there is no event", () => {
    render(<EventList now={now} events={[]} />);
    expect(screen.getByText("No events")).toBeTruthy();
  });
});

describe("ProgressRows", () => {
  it("shows the Ready message on top and the state of each check of each row", () => {
    render(
      <ProgressRows
        ready={{
          status: "False",
          reason: "WaitingForOutputs",
          message: "Waiting for Backup storage",
        }}
        rows={[
          {
            id: "backup",
            label: "Backup storage",
            detail: "New S3 bucket",
            checks: [
              {
                label: "Outputs",
                condition: { status: "False", reason: "Provisioning" },
              },
              {
                label: "Ready",
                condition: { status: "False", reason: "Provisioning" },
              },
            ],
          },
          {
            id: "cluster",
            label: "Cluster",
            checks: [{ label: "ClusterReady" }],
          },
        ]}
      />,
    );
    expect(screen.getByText("Waiting for Backup storage")).toBeTruthy();
    expect(
      screen.getByRole("img", { name: "Outputs: Not ready (Provisioning)" }),
    ).toBeTruthy();
    expect(
      screen.getByRole("img", { name: "ClusterReady: No status" }),
    ).toBeTruthy();
  });
});
