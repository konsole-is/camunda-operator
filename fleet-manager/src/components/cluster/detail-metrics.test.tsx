// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { MetricChartData, MetricsAnswer } from "@/lib/observe/types";
import { DetailMetrics, DetailMetricsView } from "./detail-metrics";

const live = vi.hoisted(() => ({
  calls: [] as unknown[][],
  answer: undefined as unknown,
}));
vi.mock("@/lib/observe/use-observe", () => ({
  useMetrics: (...args: unknown[]) => {
    live.calls.push(args);
    return live.answer;
  },
}));

beforeEach(() => {
  live.calls = [];
  live.answer = undefined;
});
afterEach(cleanup);

const pod = (name: string) => ({
  metadata: { name, creationTimestamp: "2026-11-10T08:00:00Z" },
  status: { phase: "Running" },
});
const usageOf = (name: string, cores: number) => ({
  pod: name,
  cpu: { usedCores: cores, limitCores: 1 },
  memory: { usedBytes: 2 ** 30, limitBytes: 4 * 2 ** 30 },
});
const chart = (label: string, declaredBy?: string): MetricChartData => ({
  label,
  source: "s",
  ...(declaredBy && { declaredBy }),
  series: [{ name: "orders-zeebe-0", points: [[1, 2]] }],
});
const answered = (
  charts: MetricChartData[],
  errors: Extract<MetricsAnswer, { prometheus: "answered" }>["errors"] = [],
): MetricsAnswer => ({ prometheus: "answered", charts, errors });

const pods = [pod("orders-zeebe-0"), pod("orders-zeebe-1")];
const usage = [usageOf("orders-zeebe-0", 0.5), usageOf("orders-zeebe-1", 0.7)];

describe("DetailMetricsView", () => {
  it("without Prometheus shows the live rows and the not configured line, and no charts", () => {
    render(
      <DetailMetricsView
        pods={pods}
        usage={usage}
        charts={{ prometheus: "notConfigured" }}
        range="1h"
      />,
    );
    const rows = screen.getByRole("region", { name: "Live usage" });
    expect(
      within(rows).getByRole("rowheader", { name: "orders-zeebe-0" }),
    ).toBeTruthy();
    expect(within(rows).getAllByRole("meter", { name: "CPU" })).toHaveLength(2);
    expect(
      screen.getByText(
        "Prometheus is not configured, so the charts do not show.",
      ),
    ).toBeTruthy();
    expect(
      screen
        .getByRole("link", { name: "Set the Prometheus URL in Settings" })
        .getAttribute("href"),
    ).toBe("/setup/settings");
    expect(screen.queryByRole("figure")).toBeNull();
  });

  it("without a metrics server hides the live rows and keeps the charts", () => {
    render(
      <DetailMetricsView
        pods={pods}
        usage="unavailable"
        charts={answered([chart("CPU")])}
        range="1h"
      />,
    );
    expect(screen.queryByRole("region", { name: "Live usage" })).toBeNull();
    expect(
      screen.getByText(
        "Usage is not available. The cluster has no metrics server.",
      ),
    ).toBeTruthy();
    expect(screen.getByRole("figure", { name: /CPU/ })).toBeTruthy();
  });

  it("shows a dash for a pod without usage", () => {
    render(
      <DetailMetricsView pods={pods} usage={usage.slice(0, 1)} range="1h" />,
    );
    const row = screen
      .getByRole("rowheader", { name: "orders-zeebe-1" })
      .closest("tr")!;
    expect(within(row).getByText("-")).toBeTruthy();
  });

  it("shows no history without charts", () => {
    render(<DetailMetricsView pods={pods} usage={usage} range="1h" />);
    expect(screen.queryByRole("heading", { name: "History" })).toBeNull();
    expect(screen.queryByRole("combobox", { name: "Range" })).toBeNull();
  });

  it("shows one chart for each answered query and names the registration that declared it", () => {
    render(
      <DetailMetricsView
        pods={pods}
        usage={usage}
        charts={answered([chart("CPU"), chart("Requests", "Bucket on RustFS")])}
        range="1h"
      />,
    );
    expect(screen.getAllByRole("figure")).toHaveLength(2);
    expect(screen.getByText("Declared by Bucket on RustFS")).toBeTruthy();
  });

  it("says so when no metric has data for the range", () => {
    render(
      <DetailMetricsView
        pods={pods}
        usage={usage}
        charts={answered([])}
        range="1h"
      />,
    );
    expect(screen.getByText("No metric has data for this range.")).toBeTruthy();
    expect(
      screen.queryByText(/Prometheus is not configured/),
    ).toBeNull();
  });

  it("says so when Prometheus did not answer", () => {
    render(
      <DetailMetricsView
        pods={pods}
        usage={usage}
        charts={{ prometheus: "unavailable" }}
        range="1h"
      />,
    );
    expect(
      screen.getByText(
        "Prometheus did not answer. The charts show again when it answers.",
      ),
    ).toBeTruthy();
    expect(screen.queryByRole("figure")).toBeNull();
  });

  it.each([
    ["loading", "Loading the charts"],
    ["failed", "Could not load the charts."],
  ] as const)("says so while the charts are %s", (charts, text) => {
    render(
      <DetailMetricsView pods={pods} usage={usage} charts={charts} range="1h" />,
    );
    expect(screen.getByText(text)).toBeTruthy();
  });

  it("lists a registration query that does not render, with its reason", () => {
    render(
      <DetailMetricsView
        pods={pods}
        usage={usage}
        charts={answered(
          [],
          [
            {
              label: "Bad",
              declaredBy: "Bucket on RustFS",
              error: "the if construct is not allowed",
            },
          ],
        )}
        range="1h"
      />,
    );
    expect(
      screen.getByRole("heading", { name: "Queries that do not render" }),
    ).toBeTruthy();
    expect(
      screen.getByText(
        "Bad, declared by Bucket on RustFS: the if construct is not allowed",
      ),
    ).toBeTruthy();
  });

  it("shows the range it is given in the Range select", () => {
    render(
      <DetailMetricsView
        pods={pods}
        usage={usage}
        charts={answered([])}
        range="24h"
      />,
    );
    expect(
      (screen.getByRole("combobox", { name: "Range" }) as HTMLSelectElement)
        .value,
    ).toBe("24h");
  });
});

describe("DetailMetrics", () => {
  it("asks the charts of the last hour first", () => {
    render(
      <DetailMetrics
        namespace="team-a"
        source={{ cluster: "orders", component: "zeebe" }}
        pods={pods}
        usage={usage}
      />,
    );
    expect(live.calls.at(-1)).toEqual([
      "team-a",
      { cluster: "orders", component: "zeebe" },
      "1h",
    ]);
    expect(screen.getByText("Loading the charts")).toBeTruthy();
  });

  it("asks for the range that the user picks", async () => {
    render(
      <DetailMetrics
        namespace="team-a"
        source={{ cluster: "orders", component: "zeebe" }}
        pods={pods}
        usage={usage}
      />,
    );
    await userEvent.selectOptions(
      screen.getByRole("combobox", { name: "Range" }),
      "Last 6 hours",
    );
    expect(live.calls.at(-1)).toEqual([
      "team-a",
      { cluster: "orders", component: "zeebe" },
      "6h",
    ]);
  });

  it("asks no charts without a source and shows no history", () => {
    render(<DetailMetrics namespace="team-a" pods={pods} usage={usage} />);
    expect(live.calls.at(-1)).toEqual(["team-a", undefined, "1h"]);
    expect(screen.queryByRole("heading", { name: "History" })).toBeNull();
  });
});
