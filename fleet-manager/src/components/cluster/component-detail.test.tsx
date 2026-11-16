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
  podsError: undefined as string | undefined,
  usage: undefined as unknown,
  calls: [] as { kind: string; opts: unknown }[],
  usageCalls: [] as string[][],
  metrics: undefined as unknown,
  metricsCalls: [] as unknown[][],
  events: [] as unknown[],
  logSources: [] as unknown[][],
  registrations: [] as unknown[],
}));
vi.mock("@/lib/k8s/hooks", () => ({
  useResources: (kind: string, opts: unknown, initial: unknown[]) => {
    live.calls.push({ kind, opts });
    if (kind === "Event") return { items: live.events };
    if (kind === "Pod" && live.podsError) {
      return { items: live.pods, error: live.podsError };
    }
    return { items: kind === "Pod" && live.pods.length ? live.pods : initial };
  },
}));
vi.mock("@/lib/observe/use-log-stream", () => ({
  useLogStreams: (sources: unknown[]) => {
    live.logSources.push(sources);
    return { lines: [], errors: [], ended: false };
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
  useControllerPods: (registration: unknown) => {
    live.registrations.push(registration);
    return undefined;
  },
}));

beforeEach(() => {
  live.pods = [];
  live.podsError = undefined;
  live.usage = undefined;
  live.calls = [];
  live.usageCalls = [];
  live.metrics = undefined;
  live.metricsCalls = [];
  live.events = [];
  live.logSources = [];
  live.registrations = [];
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

const keycloakReady = {
  type: "KeycloakReady",
  status: "False" as const,
  reason: "Degraded",
  message: "1 of 2 pods ready",
};

const keycloak = (
  props: Partial<Parameters<typeof ComponentDetail>[0]> = {},
) => (
  <ComponentDetail
    title="Keycloak"
    namespace="mgmt"
    podSelector={SELECTOR}
    conditions={[keycloakReady]}
    ready={keycloakReady}
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
      within(region).getByRole("rowheader", { name: /^plane-a-keycloak-0/ })
        .textContent,
    ).toBe("plane-a-keycloak-0node-2");
    expect(
      within(
        within(region).getByRole("list", { name: "Keycloak conditions" }),
      ).getByText("KeycloakReady"),
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

  it("opens the logs of its pods on the Logs tab only", async () => {
    live.pods = [pod("plane-a-keycloak-0")];
    render(keycloak());
    expect(live.logSources).toEqual([]);

    await userEvent.click(screen.getByRole("tab", { name: "Logs" }));

    expect(live.logSources.at(-1)).toEqual([
      { namespace: "mgmt", pod: "plane-a-keycloak-0" },
    ]);
    expect(live.registrations.every((r) => r === undefined)).toBe(true);
  });

  it("names a failed read of its pods on the Logs and Events tabs", async () => {
    live.podsError = "The API server refused the list of Pod";
    render(keycloak());

    await userEvent.click(screen.getByRole("tab", { name: "Logs" }));
    expect(
      screen.getByText(
        "Could not read Pod: The API server refused the list of Pod",
      ),
    ).toBeTruthy();
    expect(
      screen.queryByText("This component has no pods, so it has no logs."),
    ).toBeNull();

    await userEvent.click(screen.getByRole("tab", { name: "Events" }));
    expect(
      screen.getByText(
        "Could not read Pod: The API server refused the list of Pod",
      ),
    ).toBeTruthy();
  });

  it("asks the controller pods of its controller logs on the Logs tab", async () => {
    render(
      keycloak({
        controllerLogs: {
          registration: "rustfs-bucket",
          request: { namespace: "mgmt", name: "orders-backup-storage" },
        },
      }),
    );

    await userEvent.click(screen.getByRole("tab", { name: "Logs" }));

    expect(live.registrations).toContain("rustfs-bucket");
  });

  it("lists the events of its refs on the Events tab only", async () => {
    live.events = [
      {
        metadata: { name: "e", namespace: "mgmt", uid: "e" },
        reason: "Reconciled",
        eventTime: "2026-10-30T11:00:00Z",
        regarding: {
          kind: "PluginRequest",
          name: "orders-backup-storage",
          namespace: "mgmt",
          uid: "u-r",
        },
      },
    ];
    render(
      keycloak({
        eventRefs: [
          {
            kind: "PluginRequest",
            name: "orders-backup-storage",
            namespace: "mgmt",
            uid: "u-r",
          },
        ],
      }),
    );
    expect(live.calls.some((c) => c.kind === "Event")).toBe(false);

    await userEvent.click(screen.getByRole("tab", { name: "Events" }));

    expect(live.calls).toContainEqual({
      kind: "Event",
      opts: { namespace: "mgmt" },
    });
    expect(
      screen.getByText("PluginRequest/orders-backup-storage"),
    ).toBeTruthy();
  });

  it("watches the Services and PVCs of its related selector on the Events tab", async () => {
    render(keycloak({ relatedSelector: "camunda.io/component=keycloak" }));

    await userEvent.click(screen.getByRole("tab", { name: "Events" }));

    expect(live.calls).toContainEqual({
      kind: "Service",
      opts: {
        namespace: "mgmt",
        labelSelector: "camunda.io/component=keycloak",
      },
    });
    expect(live.calls).toContainEqual({
      kind: "Pod",
      opts: { namespace: "mgmt", labelSelector: SELECTOR },
    });
  });

  it("watches the Services and PVCs of its pod selector without a related selector", async () => {
    render(keycloak());

    await userEvent.click(screen.getByRole("tab", { name: "Events" }));

    expect(live.calls).toContainEqual({
      kind: "PersistentVolumeClaim",
      opts: { namespace: "mgmt", labelSelector: SELECTOR },
    });
  });

  it("shows the children on the Overview", () => {
    render(keycloak({ children: <p>Partitions here</p> }));
    expect(screen.getByText("Partitions here")).toBeTruthy();
  });

  it("watches no pods and polls no usage without a pod selector", () => {
    render(keycloak({ podSelector: undefined }));
    expect(live.calls.filter((c) => c.kind === "Pod")).toEqual([]);
    expect(live.usageCalls).toEqual([]);
    expect(screen.getByText("This component runs no pods.")).toBeTruthy();
    expect(screen.queryByText("Pods ready")).toBeNull();
  });

  it("treats an empty pod selector as none", () => {
    render(keycloak({ podSelector: "" }));
    expect(live.calls.filter((c) => c.kind === "Pod")).toEqual([]);
    expect(live.usageCalls).toEqual([]);
  });

  it("starts to watch the pods when a pod selector arrives", () => {
    const { rerender } = render(keycloak({ podSelector: "" }));
    rerender(keycloak({ podSelector: "app=echo" }));
    expect(live.calls).toContainEqual({
      kind: "Pod",
      opts: { namespace: "mgmt", labelSelector: "app=echo" },
    });
    expect(live.usageCalls).toContainEqual(["mgmt", "app=echo"]);
  });

  it("keeps the open tab when a pod selector arrives", async () => {
    const { rerender } = render(keycloak({ podSelector: "" }));
    await userEvent.click(screen.getByRole("tab", { name: "Logs" }));
    rerender(keycloak({ podSelector: "app=echo" }));
    expect(
      screen.getByRole("tab", { name: "Logs" }).getAttribute("aria-selected"),
    ).toBe("true");
  });

  it("says on the Metrics tab that a component without a pod selector runs no pods", async () => {
    render(keycloak({ podSelector: "" }));
    await userEvent.click(screen.getByRole("tab", { name: "Metrics" }));
    const panel = screen.getByRole("tabpanel", { name: "Metrics" });
    expect(
      within(panel).getByText("This component runs no pods."),
    ).toBeTruthy();
    expect(within(panel).queryByRole("table")).toBeNull();
  });

  it("lists only the events of its refs without a pod selector", async () => {
    live.events = [
      {
        metadata: { name: "r", namespace: "mgmt", uid: "r" },
        reason: "Applied",
        eventTime: "2026-10-30T11:00:00Z",
        regarding: {
          kind: "PluginRequest",
          name: "orders-access",
          namespace: "mgmt",
          uid: "u-r",
        },
      },
      {
        metadata: { name: "p", namespace: "mgmt", uid: "p" },
        reason: "Pulled",
        eventTime: "2026-10-30T11:00:00Z",
        regarding: { kind: "Pod", name: "orders-zeebe-0", namespace: "mgmt" },
      },
    ];
    render(
      keycloak({
        podSelector: "",
        eventRefs: [
          {
            kind: "PluginRequest",
            name: "orders-access",
            namespace: "mgmt",
            uid: "u-r",
          },
        ],
      }),
    );

    await userEvent.click(screen.getByRole("tab", { name: "Events" }));

    expect(screen.getByText("PluginRequest/orders-access")).toBeTruthy();
    expect(screen.queryByText("Pod/orders-zeebe-0")).toBeNull();
    expect(
      live.calls.filter(
        (c) => c.kind === "Service" || c.kind === "PersistentVolumeClaim",
      ),
    ).toEqual([]);
  });
});

describe("ComponentDetailView", () => {
  const view = (over: Partial<Parameters<typeof ComponentDetailView>[0]>) => (
    <ComponentDetailView
      title="Operate"
      conditions={[]}
      ready={undefined}
      pods={[pod("orders-gateway-0")]}
      usage={undefined}
      now={NOW}
      {...over}
    />
  );

  it("names the conditions list after the title and shows the subtitle", () => {
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
      screen.getByRole("list", { name: "Operate conditions" }),
    ).toBeTruthy();
  });

  it("shows the state as Not known, not as the old condition, with readyUnknown", () => {
    render(
      view({
        ready: { type: "OperateReady", status: "True", reason: "Healthy" },
        readyUnknown: true,
      }),
    );
    const header = screen.getByRole("heading", { name: "Operate" })
      .parentElement as HTMLElement;
    expect(within(header).getByText("Not known")).toBeTruthy();
    expect(within(header).queryByText("Ready")).toBeNull();
  });

  it("shows the state of the component beside the title on every tab", async () => {
    render(
      view({
        ready: {
          type: "OperateReady",
          status: "False",
          reason: "Degraded",
          message: "2 of 3 pods ready",
          lastTransitionTime: "2026-10-30T11:52:00Z",
        },
      }),
    );
    const header = screen.getByRole("heading", {
      name: "Operate",
    }).parentElement!;
    expect(within(header).getByText("Not ready · Degraded")).toBeTruthy();
    await userEvent.click(screen.getByRole("tab", { name: "Settings" }));
    expect(within(header).getByText("Not ready · Degraded")).toBeTruthy();
  });

  it("says No status beside the title when no condition decides the state", () => {
    render(view({ ready: undefined }));
    const header = screen.getByRole("heading", {
      name: "Operate",
    }).parentElement!;
    expect(within(header).getByText("No status")).toBeTruthy();
  });

  it("starts the Overview with the numbers of the pods", () => {
    render(view({}));
    const overview = screen.getByRole("tabpanel", { name: "Overview" });
    expect(
      within(overview)
        .getAllByRole("term")
        .map((t) => t.textContent),
    ).toEqual(["Pods ready", "Highest CPU", "Highest memory", "Restarts"]);
  });

  it("names the sections Pods and Conditions, then shows the children", () => {
    render(view({ children: <h3>Partitions</h3> }));
    const overview = screen.getByRole("tabpanel", { name: "Overview" });
    expect(
      within(overview)
        .getAllByRole("heading", { level: 3 })
        .map((h) => h.textContent),
    ).toEqual(["Pods", "Conditions", "Partitions"]);
  });

  it("lists the pods with no usage column while the usage is loading", () => {
    render(view({}));
    expect(
      screen.getByRole("rowheader", { name: /^orders-gateway-0/ }),
    ).toBeTruthy();
    expect(screen.queryByText(/Usage is not available/)).toBeNull();
  });

  it("says that a component without pods runs none and shows no pod numbers", () => {
    render(view({ pods: [], usage: [], podless: true }));
    const overview = screen.getByRole("tabpanel", { name: "Overview" });
    expect(
      within(overview).getByText("This component runs no pods."),
    ).toBeTruthy();
    expect(within(overview).queryAllByRole("term")).toEqual([]);
    expect(within(overview).queryByRole("table")).toBeNull();
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

  it("says that a component without a logs slot has no logs", async () => {
    render(view({}));
    await userEvent.click(screen.getByRole("tab", { name: "Logs" }));
    expect(screen.getByText("No logs.")).toBeTruthy();
  });

  it("says that a component without an events slot has no events", async () => {
    render(view({}));
    await userEvent.click(screen.getByRole("tab", { name: "Events" }));
    expect(screen.getByText("No events")).toBeTruthy();
  });

  it("shows the logs and events slots on their tabs", async () => {
    render(view({ logsTab: <p>Logs here</p>, eventsTab: <p>Events here</p> }));
    await userEvent.click(screen.getByRole("tab", { name: "Logs" }));
    expect(screen.getByText("Logs here")).toBeTruthy();
    await userEvent.click(screen.getByRole("tab", { name: "Events" }));
    expect(screen.getByText("Events here")).toBeTruthy();
  });

  it("has the tabs Overview, Metrics, Logs, Events, and Settings", () =>
    expect(DETAIL_TABS.map((t) => t.label)).toEqual([
      "Overview",
      "Metrics",
      "Logs",
      "Events",
      "Settings",
    ]));
});
