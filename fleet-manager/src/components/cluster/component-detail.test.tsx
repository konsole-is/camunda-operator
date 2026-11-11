// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  ComponentDetail,
  ComponentDetailView,
  DETAIL_TABS,
} from "./component-detail";

const live = vi.hoisted(() => ({
  pods: [] as unknown[],
  usage: undefined as unknown,
  calls: [] as { kind: string; opts: unknown }[],
  usageCalls: [] as string[][],
  metrics: undefined as unknown,
  metricsCalls: [] as unknown[][],
}));
vi.mock("@/lib/k8s/hooks", () => ({
  useResources: (kind: string, opts: unknown, initial: unknown[]) => {
    live.calls.push({ kind, opts });
    return { items: live.pods.length ? live.pods : initial };
  },
}));
vi.mock("@/lib/observe/use-observe", () => ({
  usePodUsage: (...args: string[]) => {
    live.usageCalls.push(args);
    return live.usage;
  },
  useMetrics: (...args: unknown[]) => {
    live.metricsCalls.push(args);
    return live.metrics;
  },
}));

beforeEach(() => {
  live.pods = [];
  live.usage = undefined;
  live.calls = [];
  live.usageCalls = [];
  live.metrics = undefined;
  live.metricsCalls = [];
});
afterEach(cleanup);

const NOW = "2026-10-30T12:00:00Z";
const SELECTOR =
  "camunda.io/management-cluster=plane-a,camunda.io/component=keycloak";

function pod(name: string, spec: { nodeName?: string } = {}) {
  return {
    kind: "Pod",
    metadata: { name, creationTimestamp: "2026-10-30T11:00:00Z" },
    spec,
    status: { phase: "Running" },
  };
}

const keycloak = (
  props: {
    children?: React.ReactNode;
    metrics?: { cluster: string; component: "zeebe" };
  } = {},
) => (
  <ComponentDetail
    title="Keycloak"
    namespace="mgmt"
    podSelector={SELECTOR}
    conditions={[
      {
        type: "KeycloakReady",
        status: "False",
        reason: "Degraded",
        message: "1 of 2 pods ready",
      },
    ]}
    settings={<p>Plane settings</p>}
    now={NOW}
    {...props}
  />
);

describe("ComponentDetail", () => {
  it("shows the pods and conditions of a selector that does not come from a CamundaCluster", () => {
    live.pods = [pod("plane-a-keycloak-0", { nodeName: "node-2" })];
    render(keycloak());
    expect(live.calls).toContainEqual({
      kind: "Pod",
      opts: { namespace: "mgmt", labelSelector: SELECTOR },
    });
    expect(live.usageCalls).toContainEqual(["mgmt", SELECTOR]);
    const region = screen.getByRole("region", { name: "Keycloak" });
    expect(
      within(region).getByRole("rowheader", { name: "plane-a-keycloak-0" }),
    ).toBeTruthy();
    expect(within(region).getByRole("cell", { name: "node-2" })).toBeTruthy();
    expect(
      within(region).getByRole("rowheader", { name: "KeycloakReady" }),
    ).toBeTruthy();
  });

  it("opens the settings slot on the Settings tab", async () => {
    render(keycloak());
    expect(screen.queryByText("Plane settings")).toBeNull();
    await userEvent.click(screen.getByRole("tab", { name: "Settings" }));
    expect(screen.getByText("Plane settings")).toBeTruthy();
  });

  it("asks the charts of its metrics ref on the Metrics tab", async () => {
    live.pods = [pod("plane-a-keycloak-0")];
    render(keycloak({ metrics: { cluster: "orders", component: "zeebe" } }));
    await userEvent.click(screen.getByRole("tab", { name: "Metrics" }));
    expect(live.metricsCalls).toContainEqual([
      "mgmt",
      { cluster: "orders", component: "zeebe" },
      "1h",
    ]);
    expect(screen.getByRole("region", { name: "Live usage" })).toBeTruthy();
  });

  it("asks no charts without a metrics ref and still shows the live rows", async () => {
    live.pods = [pod("plane-a-keycloak-0")];
    render(keycloak());
    await userEvent.click(screen.getByRole("tab", { name: "Metrics" }));
    expect(live.metricsCalls.every(([, ref]) => ref === undefined)).toBe(true);
    expect(screen.getByRole("region", { name: "Live usage" })).toBeTruthy();
    expect(screen.queryByRole("heading", { name: "History" })).toBeNull();
  });

  it("shows the children on the Overview", () => {
    render(keycloak({ children: <p>Partitions here</p> }));
    expect(screen.getByText("Partitions here")).toBeTruthy();
  });
});

describe("ComponentDetailView", () => {
  const view = (over: Partial<Parameters<typeof ComponentDetailView>[0]>) => (
    <ComponentDetailView
      title="Operate"
      conditions={[]}
      pods={[pod("orders-gateway-0")]}
      usage={undefined}
      now={NOW}
      {...over}
    />
  );

  it("names the conditions table after the title and shows the subtitle", () => {
    render(
      view({
        subtitle: "Embedded in Gateway",
        conditions: [
          { type: "OperateReady", status: "True", reason: "Disabled" },
        ],
      }),
    );
    expect(screen.getByRole("heading", { name: "Operate" })).toBeTruthy();
    expect(screen.getByText("Embedded in Gateway")).toBeTruthy();
    expect(
      screen.getByRole("region", { name: "Operate conditions" }),
    ).toBeTruthy();
  });

  it("lists the pods with no usage column while the usage is loading", () => {
    render(view({}));
    expect(
      screen.getByRole("rowheader", { name: "orders-gateway-0" }),
    ).toBeTruthy();
    expect(screen.queryByText(/Usage is not available/)).toBeNull();
  });

  it("says that a component without a settings slot has no settings", async () => {
    render(view({}));
    await userEvent.click(screen.getByRole("tab", { name: "Settings" }));
    expect(screen.getByText("No settings.")).toBeTruthy();
  });

  it("the heading takes focus from a script but is not in the tab order", () => {
    render(view({}));
    expect(
      screen.getByRole("heading", { name: "Operate" }).getAttribute("tabindex"),
    ).toBe("-1");
  });

  it("shows the live usage of its pods on the Metrics tab without a metrics slot", async () => {
    render(view({}));
    await userEvent.click(screen.getByRole("tab", { name: "Metrics" }));
    expect(
      within(screen.getByRole("region", { name: "Live usage" })).getByRole(
        "rowheader",
        { name: "orders-gateway-0" },
      ),
    ).toBeTruthy();
  });

  it("shows the metrics slot on the Metrics tab", async () => {
    render(view({ metricsTab: <p>Charts here</p> }));
    expect(screen.queryByText("Charts here")).toBeNull();
    await userEvent.click(screen.getByRole("tab", { name: "Metrics" }));
    expect(screen.getByText("Charts here")).toBeTruthy();
  });

  it("has the tabs Overview, Metrics, and Settings", () =>
    expect(DETAIL_TABS.map((t) => t.label)).toEqual([
      "Overview",
      "Metrics",
      "Settings",
    ]));
});
