// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import { useState } from "react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { PendingPush } from "@/components/fleet/git-pushes";
import { type Plane, planesOf } from "@/components/planes/plane";
import type { Resource } from "@/lib/k8s/kinds";
import { CLAIM_LABEL, ENVIRONMENT_LABEL } from "@/lib/labels";
import { type FleetPageData, FleetView, FleetViewLive } from "./fleet-view";
import type { PodWarnings } from "@/lib/warnings";
import type { FleetSources } from "./model";

const live = vi.hoisted(() => ({
  objects: {} as Record<string, unknown[]>,
  calls: [] as string[],
  pushes: [] as unknown[],
  settled: [] as string[],
  fresh: [] as string[],
  warnings: undefined as unknown,
}));
vi.mock("@/lib/observe/use-observe", () => ({
  useWarnings: () => live.warnings,
}));
vi.mock("@/lib/k8s/hooks", () => ({
  useResources: (kind: string, _opts: unknown, initial: unknown[]) => {
    live.calls.push(kind);
    return { items: live.objects[kind] ?? initial };
  },
  useWatchesFresh: (lists: { kind: string }[]) =>
    lists.map(({ kind }) => live.fresh.includes(kind)),
}));
vi.mock("@/components/fleet/git-pushes", () => ({
  useGitPushes: () => ({
    pushes: live.pushes,
    settle: (key: string) => live.settled.push(key),
  }),
}));

beforeEach(() => {
  live.objects = {};
  live.calls = [];
  live.pushes = [];
  live.settled = [];
  live.fresh = [];
  live.warnings = undefined;
});
afterEach(cleanup);

const NOW = "2026-11-10T12:00:00Z";

function cluster(
  namespace: string,
  name: string,
  {
    managed = true,
    ready = "True",
  }: { managed?: boolean; ready?: string } = {},
): Resource<"CamundaCluster"> {
  return {
    apiVersion: "core.camunda.io/v1",
    kind: "CamundaCluster",
    metadata: {
      namespace,
      name,
      ...(managed ? { labels: { [CLAIM_LABEL]: name } } : {}),
    },
    spec: {},
    status: {
      conditions: [
        {
          type: "Ready",
          status: ready,
          reason: ready === "True" ? "Healthy" : "Down",
        },
      ],
    },
  } as Resource<"CamundaCluster">;
}

function claim(namespace: string, name: string) {
  return {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "CamundaClusterClaim",
    metadata: { namespace, name },
    spec: { presetRef: "small" },
  } as Resource<"CamundaClusterClaim">;
}

function environment(name: string, displayName: string, color?: string) {
  return {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "Environment",
    metadata: { name },
    spec: { displayName, color },
  } as Resource<"Environment">;
}

function namespace(name: string, env?: string): Resource<"Namespace"> {
  return {
    apiVersion: "v1",
    kind: "Namespace",
    metadata: {
      name,
      ...(env ? { labels: { [ENVIRONMENT_LABEL]: env } } : {}),
    },
  };
}

function managementClaim(name: string, environments: string[]) {
  return {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "CamundaManagementClusterClaim",
    metadata: { namespace: "platform", name },
    spec: { environments, management: { identityProvider: { keycloak: {} } } },
  } as never;
}

function managementCluster(name: string, ready: "True" | "False") {
  return {
    apiVersion: "core.camunda.io/v1",
    kind: "CamundaManagementCluster",
    metadata: { namespace: "platform", name, labels: { [CLAIM_LABEL]: name } },
    spec: { identityProvider: { keycloak: {} } },
    status: { conditions: [{ type: "Ready", status: ready, reason: "Down" }] },
  } as never;
}

function sources(over: Partial<FleetSources> = {}): FleetSources {
  return {
    clusters: [],
    claims: [],
    environments: [],
    namespaces: [],
    presets: [],
    releases: [],
    sections: [],
    registrations: [],
    requests: [],
    storage: [],
    optimizes: [],
    backups: [],
    ...over,
  };
}

const fleet = sources({
  clusters: [
    cluster("dev", "sandbox"),
    cluster("prod", "orders"),
    cluster("prod", "legacy", { managed: false, ready: "False" }),
    cluster("stage", "billing"),
    cluster("loose", "scratch"),
  ],
  environments: [
    environment("staging", "Staging", "#ffc14d"),
    environment("production", "Production", "#3dd68c"),
    environment("development", "Development"),
  ],
  namespaces: [
    namespace("dev", "development"),
    namespace("prod", "production"),
    namespace("stage", "staging"),
    namespace("loose"),
  ],
});

const planeSources = {
  clusters: [managementCluster("plane-a", "True")],
  claims: [
    managementClaim("plane-a", ["production"]),
    managementClaim("plane-b", ["staging"]),
  ],
  namespaces: fleet.namespaces,
  environments: fleet.environments,
};
const planes: Plane[] = planesOf(planeSources);

const push = (namespace: string, name: string): PendingPush => ({
  key: `CamundaClusterClaim/${namespace}/${name}`,
  ref: { kind: "CamundaClusterClaim", namespace, name },
  path: `clusters/${namespace}/${name}.yaml`,
  revision: "abc",
  pushedAt: "2026-11-10T11:58:00Z",
});

function view(over: Partial<Parameters<typeof FleetView>[0]> = {}) {
  return (
    <FleetView
      sources={fleet}
      planes={planes}
      pushes={[]}
      now={NOW}
      {...over}
    />
  );
}

// The links of the cluster tables, not those of Needs attention.
const clusterNames = () => {
  const attention = screen.queryByRole("region", { name: "Needs attention" });
  return screen
    .queryAllByRole("link")
    .filter((l) => !attention?.contains(l))
    .map((l) => l.getAttribute("href") ?? "")
    .filter(
      (href) =>
        href.startsWith("/fleet/clusters/") && href !== "/fleet/clusters/new",
    )
    .map((href) => href.split("/").slice(3).join("/"));
};

const podWarnings: PodWarnings = {
  clusters: {
    "prod/orders": [
      {
        kind: "oom",
        severity: "Error",
        label: "Out of memory",
        detail: "Container zeebe of pod orders-zeebe-0 ran out of memory",
      },
    ],
  },
  missing: ["prometheus"],
};

const regions = () =>
  screen.getAllByRole("region").map((r) => r.getAttribute("aria-label"));

describe("FleetView", () => {
  it("has the heading and the button to create a cluster", () => {
    render(view());
    expect(
      screen.getByRole("heading", { level: 1, name: "Fleet" }),
    ).toBeTruthy();
    expect(
      screen.getByRole("link", { name: "New cluster" }).getAttribute("href"),
    ).toBe("/fleet/clusters/new");
  });

  it("shows All, the three environments with their counts, and No environment", () => {
    render(view());
    expect(screen.getAllByRole("tab").map((t) => t.textContent)).toEqual([
      "All5",
      "Development1",
      "Production2",
      "Staging1",
      "No environment1",
    ]);
    expect(
      screen.getByRole("tab", { name: /All/ }).getAttribute("aria-selected"),
    ).toBe("true");
  });

  it("shows one table for each environment group on All, then No environment", () => {
    render(view());
    expect(regions()).toEqual([
      "Needs attention",
      "Management planes",
      "Development",
      "Production",
      "Staging",
      "No environment",
    ]);
    expect(
      within(screen.getByRole("region", { name: "Production" }))
        .getAllByRole("link")
        .map((l) => l.textContent),
    ).toEqual(["legacy", "orders"]);
  });

  it("shows only the clusters of the selected tab", async () => {
    render(view());
    await userEvent.click(screen.getByRole("tab", { name: /Production/ }));
    expect(clusterNames()).toEqual(["prod/legacy", "prod/orders"]);
    expect(
      within(screen.getByRole("row", { name: /legacy/ })).getByText(
        "Not managed",
      ),
    ).toBeTruthy();
    expect(
      within(screen.getByRole("row", { name: /orders/ })).queryByText(
        "Not managed",
      ),
    ).toBeNull();
  });

  it("filters every group by name, namespace, and status", async () => {
    render(view());
    const search = screen.getByRole("searchbox", { name: "Search clusters" });
    await userEvent.type(search, "S");
    expect(clusterNames()).toEqual([
      "dev/sandbox",
      "prod/orders",
      "loose/scratch",
    ]);
    await userEvent.selectOptions(
      screen.getByRole("combobox", { name: "Namespace" }),
      "prod",
    );
    expect(clusterNames()).toEqual(["prod/orders"]);
    await userEvent.clear(search);
    expect(clusterNames()).toEqual(["prod/legacy", "prod/orders"]);
    await userEvent.selectOptions(
      screen.getByRole("combobox", { name: "Status" }),
      "not-ready",
    );
    expect(clusterNames()).toEqual(["prod/legacy"]);
  });

  it("shows every cluster again when the selected namespace is gone", async () => {
    const { rerender } = render(view());
    await userEvent.selectOptions(
      screen.getByRole("combobox", { name: "Namespace" }),
      "dev",
    );
    expect(clusterNames()).toEqual(["dev/sandbox"]);
    rerender(
      view({
        sources: {
          ...fleet,
          clusters: fleet.clusters.filter(
            (c) => c.metadata?.namespace !== "dev",
          ),
        },
      }),
    );
    expect(clusterNames()).toEqual([
      "prod/legacy",
      "prod/orders",
      "stage/billing",
      "loose/scratch",
    ]);
  });

  it("starts again at the first page when a filter changes", async () => {
    const many = sources({
      clusters: Array.from({ length: 30 }, (_, i) =>
        cluster("team-a", `c${String(i).padStart(2, "0")}`),
      ),
    });
    render(view({ sources: many, planes: [] }));
    await userEvent.click(screen.getByRole("button", { name: "Next page" }));
    expect(clusterNames()).toContain("team-a/c29");
    await userEvent.type(
      screen.getByRole("searchbox", { name: "Search clusters" }),
      "c",
    );
    expect(clusterNames()).toContain("team-a/c00");
  });

  it("starts the waiting table again at its first page when a filter changes", async () => {
    const pushes = Array.from({ length: 30 }, (_, i) =>
      push("team-x", `p${String(i).padStart(2, "0")}`),
    );
    render(view({ sources: sources(), planes: [], pushes }));
    await userEvent.click(screen.getByRole("button", { name: "Next page" }));
    const waiting = () =>
      within(screen.getByRole("region", { name: "Waiting for GitOps" }))
        .getAllByRole("row")
        .map((r) => r.textContent ?? "");
    expect(waiting().some((t) => t.includes("p29"))).toBe(true);
    await userEvent.type(
      screen.getByRole("searchbox", { name: "Search clusters" }),
      "p",
    );
    expect(waiting().some((t) => t.includes("p00"))).toBe(true);
  });

  it("shows the warning chips of each cluster", () => {
    render(view({ podWarnings }));
    expect(
      within(screen.getByRole("row", { name: /orders/ })).getByRole("button", {
        name: "Error: Out of memory",
      }),
    ).toBeTruthy();
    expect(
      within(screen.getByRole("row", { name: /billing/ })).queryByRole(
        "button",
        { name: "Error: Out of memory" },
      ),
    ).toBeNull();
  });

  it("keeps the clusters with warnings", async () => {
    render(view({ podWarnings }));
    await userEvent.selectOptions(
      screen.getByRole("combobox", { name: "Status" }),
      "with-warnings",
    );
    expect(clusterNames()).toEqual(["prod/orders"]);
  });

  it("says which source is missing above the clusters", () => {
    render(view({ podWarnings }));
    const note = screen.getByText(/Prometheus is not set or does not answer/);
    const firstGroup = screen.getByRole("region", { name: "Development" });
    expect(
      note.compareDocumentPosition(firstGroup) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    expect(
      screen.getAllByText(/Prometheus is not set or does not answer/),
    ).toHaveLength(1);
  });

  it("shows a management plane that is not ready in Needs attention", () => {
    const down = planesOf({
      ...planeSources,
      clusters: [managementCluster("plane-a", "False")],
      claims: [managementClaim("plane-a", ["production"])],
    });
    render(view({ planes: down }));
    expect(
      within(screen.getByRole("region", { name: "Needs attention" })).getByText(
        "Management plane platform/plane-a",
      ),
    ).toBeTruthy();
  });

  it("does not say that nothing needs attention while a list it reads failed", () => {
    render(
      view({
        sources: sources(),
        planes: [],
        errors: [{ kind: "PluginRegistration", message: "forbidden" }],
      }),
    );
    const panel = screen.getByRole("region", { name: "Needs attention" });
    expect(within(panel).queryByText("Nothing needs attention.")).toBeNull();
  });

  it("shows Needs attention on every tab", async () => {
    render(view());
    const panel = () => screen.getByRole("region", { name: "Needs attention" });
    expect(within(panel()).getByText("Cluster prod/legacy")).toBeTruthy();
    await userEvent.click(screen.getByRole("tab", { name: /Staging/ }));
    expect(within(panel()).getByText("Cluster prod/legacy")).toBeTruthy();
    expect(
      screen.getAllByRole("region", { name: "Needs attention" }),
    ).toHaveLength(1);
  });

  it("says that no cluster matches the filters", async () => {
    render(view());
    await userEvent.type(
      screen.getByRole("searchbox", { name: "Search clusters" }),
      "nothing",
    );
    expect(
      screen.getByText("No cluster matches the search and the filters."),
    ).toBeTruthy();
    expect(clusterNames()).toEqual([]);
  });

  it("shows one list without tabs when no Environment exists", () => {
    render(
      view({
        sources: sources({
          clusters: [cluster("team-a", "orders"), cluster("team-b", "billing")],
        }),
        planes: [],
      }),
    );
    expect(screen.queryAllByRole("tab")).toEqual([]);
    expect(regions()).toEqual(["Needs attention", "Clusters"]);
    expect(clusterNames()).toEqual(["team-a/orders", "team-b/billing"]);
    expect(screen.getByText("No management plane yet.")).toBeTruthy();
  });

  it("shows the management planes above the clusters and follows the tab", async () => {
    render(view());
    const planesRegion = screen.getByRole("region", {
      name: "Management planes",
    });
    const firstClusters = screen.getByRole("region", { name: "Development" });
    expect(
      planesRegion.compareDocumentPosition(firstClusters) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    expect(within(planesRegion).getByText("plane-a")).toBeTruthy();
    expect(within(planesRegion).getByText("plane-b")).toBeTruthy();

    await userEvent.click(screen.getByRole("tab", { name: /Staging/ }));
    const staging = screen.getByRole("region", { name: "Management planes" });
    expect(within(staging).getByText("plane-b")).toBeTruthy();
    expect(within(staging).queryByText("plane-a")).toBeNull();
  });

  it("says that no management plane serves an environment", async () => {
    render(view());
    await userEvent.click(screen.getByRole("tab", { name: /Development/ }));
    expect(
      screen.getByText("No management plane serves this environment."),
    ).toBeTruthy();
    expect(
      screen.queryByRole("region", { name: "Management planes" }),
    ).toBeNull();
  });

  it("shows a pushed claim that is not live as Waiting for GitOps", () => {
    // The pushes are state of this browser tab. Nothing on the server lists
    // the claims that exist only in Git, so only the tab that pushed the
    // claim shows its row.
    render(view({ pushes: [push("team-x", "returns")] }));
    const row = within(
      screen.getByRole("region", { name: "Waiting for GitOps" }),
    ).getByRole("row", { name: /returns/ });
    expect(within(row).getByText("Waiting for GitOps")).toBeTruthy();
    expect(within(row).getByText("clusters/team-x/returns.yaml")).toBeTruthy();
    expect(within(row).getByText("2m ago")).toBeTruthy();
  });

  it("shows the Git waiting rows on every tab", async () => {
    render(view({ pushes: [push("team-x", "returns")] }));
    for (const tab of [/Production/, /No environment/, /All/]) {
      await userEvent.click(screen.getByRole("tab", { name: tab }));
      expect(
        screen.getByRole("region", { name: "Waiting for GitOps" }),
      ).toBeTruthy();
    }
    expect(screen.getAllByRole("tab").map((t) => t.textContent)).toContain(
      "No environment1",
    );
  });

  it("does not make a No environment tab for a waiting row", () => {
    render(
      view({
        sources: { ...fleet, clusters: fleet.clusters.slice(0, 4) },
        pushes: [push("team-x", "returns")],
      }),
    );
    expect(screen.queryByRole("tab", { name: /No environment/ })).toBeNull();
  });

  it("drops a waiting row once the claim is live", () => {
    render(
      view({
        sources: { ...fleet, claims: [claim("team-x", "returns")] },
        pushes: [push("team-x", "returns")],
      }),
    );
    expect(
      screen.queryByRole("region", { name: "Waiting for GitOps" }),
    ).toBeNull();
  });

  it("removes a waiting row and its namespace on Stop waiting", async () => {
    function Harness() {
      const [pushes, setPushes] = useState([push("team-x", "returns")]);
      return view({
        pushes,
        onStopWaiting: (key) =>
          setPushes((old) => old.filter((p) => p.key !== key)),
      });
    }
    render(<Harness />);
    const namespaces = () =>
      within(screen.getByRole("combobox", { name: "Namespace" }))
        .getAllByRole("option")
        .map((o) => o.textContent);
    expect(namespaces()).toContain("team-x");
    await userEvent.click(screen.getByRole("button", { name: "Stop waiting" }));
    expect(
      screen.queryByRole("region", { name: "Waiting for GitOps" }),
    ).toBeNull();
    expect(namespaces()).not.toContain("team-x");
  });

  it("lists the namespaces of the selected tab and of the waiting rows", async () => {
    render(view({ pushes: [push("team-x", "returns")] }));
    await userEvent.click(screen.getByRole("tab", { name: /Production/ }));
    expect(
      within(screen.getByRole("combobox", { name: "Namespace" }))
        .getAllByRole("option")
        .map((o) => o.textContent),
    ).toEqual(["All", "prod", "team-x"]);
  });

  it("does not say that the fleet is empty when the clusters cannot be read", () => {
    render(
      view({
        sources: sources(),
        planes: [],
        errors: [{ kind: "CamundaCluster", message: "forbidden" }],
      }),
    );
    expect(screen.getByRole("alert").textContent).toContain("CamundaCluster");
    expect(screen.queryByText(/No Camunda cluster yet/)).toBeNull();
  });

  it("does not say that the fleet is empty when the claims cannot be read", () => {
    render(
      view({
        sources: sources(),
        planes: [],
        errors: [{ kind: "CamundaClusterClaim", message: "forbidden" }],
      }),
    );
    expect(screen.queryByText(/No Camunda cluster yet/)).toBeNull();
  });

  it("gives a tab its full label as a hover text", () => {
    const long =
      "Production in the European Union, west region, blue and green";
    render(
      view({
        sources: {
          ...fleet,
          environments: [environment("production", long)],
        },
      }),
    );
    expect(
      screen
        .getByRole("tab", { name: new RegExp(long) })
        .querySelector(`[title="${long}"]`),
    ).toBeTruthy();
  });

  it("adds no Git banner of its own", () => {
    render(view({ pushes: [push("team-x", "returns")] }));
    expect(screen.queryByText("Waiting for GitOps to apply")).toBeNull();
  });

  it("says what it could not read and keeps the rows", () => {
    render(view({ errors: [{ kind: "PluginRequest", message: "forbidden" }] }));
    expect(screen.getByRole("alert").textContent).toBe(
      "Could not read PluginRequest: forbidden",
    );
    expect(clusterNames()).toHaveLength(5);
  });

  it("says that the fleet has no cluster yet", () => {
    render(view({ sources: sources(), planes: [] }));
    expect(
      screen.getByText(
        "No Camunda cluster yet. Create one with New cluster, or apply a CamundaCluster with kubectl.",
      ),
    ).toBeTruthy();
    expect(screen.queryByRole("searchbox")).toBeNull();
  });

  it("shows the swatch of an Environment color in its tab", () => {
    render(view());
    const swatch = screen
      .getByRole("tab", { name: /Production/ })
      .querySelector("[data-swatch]") as HTMLElement | null;
    expect(swatch?.style.backgroundColor).toBe("rgb(61, 214, 140)");
    expect(
      screen
        .getByRole("tab", { name: /Development/ })
        .querySelector("[data-swatch]"),
    ).toBeNull();
  });
});

const KINDS = [
  "CamundaCluster",
  "CamundaClusterClaim",
  "CamundaManagementCluster",
  "CamundaManagementClusterClaim",
  "LogicalBackupElasticsearch",
  "LogicalBackupRDBMS",
  "PluginRequest",
  "PluginRegistration",
  "PluginSection",
  "Environment",
  "Namespace",
  "CamundaClusterPreset",
  "CamundaRelease",
  "CamundaOptimize",
  "SecondaryStorageConfig",
];

const initial: FleetPageData = { ...fleet, planes: planeSources };

describe("FleetViewLive", () => {
  it("shows a failed first read until the watch gives the list", () => {
    const failed: FleetPageData = {
      ...sources(),
      planes: { clusters: [], claims: [], namespaces: [], environments: [] },
      readErrors: [{ kind: "CamundaCluster", message: "forbidden" }],
    };
    const { rerender } = render(<FleetViewLive initial={failed} now={NOW} />);
    expect(screen.getByRole("alert").textContent).toBe(
      "Could not read CamundaCluster: forbidden",
    );
    expect(screen.queryByText(/No Camunda cluster yet/)).toBeNull();
    // An empty snapshot can keep the initial list, so only the freshness of
    // the watch tells that the read now works.
    live.fresh = ["CamundaCluster"];
    rerender(<FleetViewLive initial={failed} now={NOW} />);
    expect(screen.queryByRole("alert")).toBeNull();
    expect(screen.getByText(/No Camunda cluster yet/)).toBeTruthy();
  });

  it("stops waiting for a push of this browser tab", async () => {
    live.pushes = [push("team-x", "returns")];
    render(<FleetViewLive initial={initial} now={NOW} />);
    await userEvent.click(screen.getByRole("button", { name: "Stop waiting" }));
    expect(live.settled).toEqual(["CamundaClusterClaim/team-x/returns"]);
  });

  it("follows the lists of the fleet view and never pods, logs, or events", () => {
    render(<FleetViewLive initial={initial} now={NOW} />);
    expect(new Set(live.calls)).toEqual(new Set(KINDS));
  });

  it("follows the live clusters", () => {
    const { rerender } = render(<FleetViewLive initial={initial} now={NOW} />);
    expect(clusterNames()).not.toContain("loose/fresh");
    live.objects.CamundaCluster = [
      ...fleet.clusters,
      cluster("loose", "fresh"),
    ];
    rerender(<FleetViewLive initial={initial} now={NOW} />);
    expect(clusterNames()).toContain("loose/fresh");
  });

  it("shows the warnings of the route", () => {
    live.warnings = podWarnings;
    render(<FleetViewLive initial={initial} now={NOW} />);
    expect(
      screen.getByRole("button", { name: "Error: Out of memory" }),
    ).toBeTruthy();
  });

  it("shows a pushed claim of this browser tab", () => {
    live.pushes = [push("team-x", "returns")];
    render(<FleetViewLive initial={initial} now={NOW} />);
    expect(
      screen.getByRole("region", { name: "Waiting for GitOps" }),
    ).toBeTruthy();
  });
});
