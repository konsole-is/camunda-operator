// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { PendingPush } from "@/components/fleet/git-pushes";
import { type Plane, planesOf } from "@/components/planes/plane";
import type { Resource } from "@/lib/k8s/kinds";
import { CLAIM_LABEL, ENVIRONMENT_LABEL } from "@/lib/labels";
import { type FleetPageData, FleetView, FleetViewLive } from "./fleet-view";
import type { FleetSources } from "./model";

const live = vi.hoisted(() => ({
  objects: {} as Record<string, unknown[]>,
  calls: [] as string[],
  pushes: [] as unknown[],
}));
vi.mock("@/lib/k8s/hooks", () => ({
  useResources: (kind: string, _opts: unknown, initial: unknown[]) => {
    live.calls.push(kind);
    return { items: live.objects[kind] ?? initial };
  },
}));
vi.mock("@/components/fleet/git-pushes", () => ({
  useGitPushes: () => ({ pushes: live.pushes }),
}));

beforeEach(() => {
  live.objects = {};
  live.calls = [];
  live.pushes = [];
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

const clusterNames = () =>
  screen
    .queryAllByRole("link")
    .map((l) => l.getAttribute("href") ?? "")
    .filter(
      (href) =>
        href.startsWith("/fleet/clusters/") && href !== "/fleet/clusters/new",
    )
    .map((href) => href.split("/").slice(3).join("/"));

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
    expect(regions()).toEqual(["Clusters"]);
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

  it("shows a pushed claim of this browser tab", () => {
    live.pushes = [push("team-x", "returns")];
    render(<FleetViewLive initial={initial} now={NOW} />);
    expect(
      screen.getByRole("region", { name: "Waiting for GitOps" }),
    ).toBeTruthy();
  });
});
