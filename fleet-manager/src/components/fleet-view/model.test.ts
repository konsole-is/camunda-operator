import { describe, expect, it } from "vitest";
import type { PendingPush } from "@/components/fleet/git-pushes";
import type { Resource } from "@/lib/k8s/kinds";
import { CLAIM_LABEL, ENVIRONMENT_LABEL } from "@/lib/labels";
import {
  ALL_TAB,
  type ClusterRow,
  type FleetSources,
  NO_ENVIRONMENT_TAB,
  clusterRows,
  componentDots,
  environmentTabValue,
  environmentTabs,
  filterRows,
  groupRows,
  rowNamespaces,
  waitingRows,
} from "./model";

const NOW = "2026-11-10T12:00:00Z";

type Condition = { type: string; status: string; reason?: string };

function cluster(
  namespace: string,
  name: string,
  {
    claim,
    spec = {},
    status,
  }: {
    claim?: string;
    spec?: Record<string, unknown>;
    status?: Record<string, unknown>;
  } = {},
): Resource<"CamundaCluster"> {
  return {
    apiVersion: "core.camunda.io/v1",
    kind: "CamundaCluster",
    metadata: {
      namespace,
      name,
      ...(claim ? { labels: { [CLAIM_LABEL]: claim } } : {}),
    },
    spec,
    status,
  } as Resource<"CamundaCluster">;
}

function claim(
  namespace: string,
  name: string,
  {
    plugins = [],
    requests,
    conditions,
  }: {
    plugins?: { section: string; registration: string }[];
    requests?: { section: string; name: string; registration: string }[];
    conditions?: Condition[];
  } = {},
): Resource<"CamundaClusterClaim"> {
  return {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "CamundaClusterClaim",
    metadata: { namespace, name },
    spec: { presetRef: "small", plugins },
    status: { requests, conditions },
  } as Resource<"CamundaClusterClaim">;
}

function environment(
  name: string,
  displayName?: string,
  color?: string,
): Resource<"Environment"> {
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

function section(name: string, label: string, order: number) {
  return {
    kind: "PluginSection",
    metadata: { name },
    spec: {
      appliesTo: ["CamundaClusterClaim"],
      label,
      order,
      selection: "One",
    },
  } as Resource<"PluginSection">;
}

function registration(name: string, label: string) {
  return {
    kind: "PluginRegistration",
    metadata: { name },
    spec: { label },
  } as unknown as Resource<"PluginRegistration">;
}

function request(namespace: string, name: string, ready: Condition) {
  return {
    kind: "PluginRequest",
    metadata: { namespace, name },
    spec: {},
    status: { conditions: [ready] },
  } as unknown as Resource<"PluginRequest">;
}

function backup(
  namespace: string,
  name: string,
  clusterName: string,
  created: string,
) {
  return {
    kind: "LogicalBackupElasticsearch",
    metadata: { namespace, name, creationTimestamp: created },
    spec: { clusterRef: { name: clusterName } },
    status: { phase: "Completed" },
  } as unknown as FleetSources["backups"][number];
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

const ready = (reason = "Healthy"): Condition => ({
  type: "Ready",
  status: "True",
  reason,
});

const labels = (dots: { label: string }[]) => dots.map((d) => d.label);

describe("componentDots", () => {
  const sections = [
    section("backup-storage", "Backup storage", 40),
    section("secondary-storage", "Secondary storage", 50),
  ];

  it("gives an Elasticsearch cluster of a claim the processes, Optimize, and its sections", () => {
    const orders = cluster("team-a", "orders", {
      claim: "orders",
      spec: {
        gateway: { mode: "Standalone" },
        operate: { mode: "Embedded" },
        connectors: { enabled: true },
      },
    });
    const dots = componentDots(
      orders,
      sources({
        claims: [
          claim("team-a", "orders", {
            plugins: [
              { section: "secondary-storage", registration: "elasticsearch" },
              {
                section: "backup-storage",
                registration: "existing-object-storage",
              },
            ],
          }),
        ],
        sections,
        registrations: [
          registration("elasticsearch", "New Elasticsearch cluster"),
          registration("existing-object-storage", "Existing bucket"),
        ],
        optimizes: [
          {
            kind: "CamundaOptimize",
            metadata: { namespace: "team-a", name: "orders-optimize" },
            spec: { clusterRef: { name: "orders" } },
          } as unknown as Resource<"CamundaOptimize">,
        ],
      }),
    );
    expect(labels(dots)).toEqual([
      "Brokers",
      "Gateway",
      "Connectors",
      "Optimize",
      "Backup storage (Existing bucket)",
      "Secondary storage (New Elasticsearch cluster)",
    ]);
    expect(dots.every((d) => d.used)).toBe(true);
    expect(dots.slice(-2).map((d) => d.id)).toEqual([
      "section:backup-storage",
      "section:secondary-storage",
    ]);
  });

  it("gives a PostgreSQL cluster of a claim the dot of its database server", () => {
    const orders = cluster("team-a", "orders", { claim: "orders" });
    const dots = componentDots(
      orders,
      sources({
        claims: [
          claim("team-a", "orders", {
            plugins: [
              {
                section: "secondary-storage",
                registration: "database-server",
              },
            ],
            requests: [
              {
                section: "secondary-storage",
                name: "orders-db",
                registration: "database-server",
              },
            ],
          }),
        ],
        sections,
        registrations: [
          registration("database-server", "New PostgreSQL server"),
        ],
        requests: [
          request("team-a", "orders-db", {
            type: "Ready",
            status: "False",
            reason: "Creating",
          }),
        ],
      }),
    );
    const last = dots.at(-1);
    expect(last?.label).toBe("Secondary storage (New PostgreSQL server)");
    expect(last?.condition).toMatchObject({
      status: "False",
      reason: "Creating",
    });
  });

  it("names a section dot by the registration name when the registration is not found", () => {
    const orders = cluster("team-a", "orders", { claim: "orders" });
    const dots = componentDots(
      orders,
      sources({
        claims: [
          claim("team-a", "orders", {
            plugins: [{ section: "secondary-storage", registration: "gone" }],
          }),
        ],
        sections,
      }),
    );
    expect(dots.at(-1)?.label).toBe("Secondary storage (gone)");
  });

  it("gives an unmanaged cluster its processes and its secondary storage from storageRef", () => {
    const legacy = cluster("team-a", "legacy", {
      spec: { storageRef: "legacy-db" },
    });
    const dots = componentDots(
      legacy,
      sources({
        storage: [
          {
            kind: "SecondaryStorageConfig",
            metadata: { namespace: "team-a", name: "legacy-db" },
            spec: { type: "rdbms" },
            status: { conditions: [ready("Ready")] },
          } as unknown as Resource<"SecondaryStorageConfig">,
        ],
      }),
    );
    expect(dots.at(-1)).toMatchObject({
      label: "Secondary storage (Relational database)",
      used: true,
      condition: { status: "True" },
    });
  });

  it("gives an unmanaged cluster a storage dot without a state when its SecondaryStorageConfig is missing", () => {
    const legacy = cluster("team-a", "legacy", {
      spec: { storageRef: "legacy-db" },
    });
    const dot = componentDots(legacy, sources()).at(-1);
    expect(dot?.label).toBe("Secondary storage");
    expect(dot?.condition).toBeUndefined();
  });

  it("ignores a SecondaryStorageConfig of the same name in another namespace", () => {
    const legacy = cluster("team-a", "legacy", {
      spec: { storageRef: "legacy-db" },
    });
    const dot = componentDots(
      legacy,
      sources({
        storage: [
          {
            kind: "SecondaryStorageConfig",
            metadata: { namespace: "team-b", name: "legacy-db" },
            spec: { type: "rdbms" },
          } as unknown as Resource<"SecondaryStorageConfig">,
        ],
      }),
    ).at(-1);
    expect(dot?.label).toBe("Secondary storage");
  });

  it("gives a cluster without storageRef and without a claim no storage dot", () => {
    const dots = componentDots(cluster("team-a", "legacy"), sources());
    expect(labels(dots)).not.toContain("Secondary storage");
  });

  it("greys out disabled Connectors and a cluster without Optimize", () => {
    const dots = componentDots(
      cluster("team-a", "orders", { spec: { connectors: { enabled: false } } }),
      sources(),
    );
    expect(dots.filter((d) => !d.used).map((d) => d.label)).toEqual([
      "Connectors",
      "Optimize",
    ]);
  });

  it("gives an embedded process no dot of its own", () => {
    const dots = componentDots(
      cluster("team-a", "orders", { spec: { gateway: { mode: "Embedded" } } }),
      sources(),
    );
    expect(labels(dots)).not.toContain("Gateway");
  });

  it("takes the modes from the merged spec of preset and release", () => {
    const dots = componentDots(
      cluster("team-a", "orders", { spec: { presetRef: "big" } }),
      sources({
        presets: [
          {
            kind: "CamundaClusterPreset",
            metadata: { name: "big" },
            spec: { cluster: { operate: { mode: "Standalone" } } },
          } as unknown as Resource<"CamundaClusterPreset">,
        ],
      }),
    );
    expect(labels(dots)).toContain("Operate");
  });
});

describe("clusterRows", () => {
  it("marks a cluster without the claim label as not managed", () => {
    const [row] = clusterRows(
      sources({ clusters: [cluster("team-a", "legacy")] }),
      NOW,
    );
    expect(row).toMatchObject({
      id: "cluster/team-a/legacy",
      managed: false,
      href: "/fleet/clusters/team-a/legacy",
      sections: [],
    });
  });

  it("gives a claim without a cluster a row with the Ready of the claim", () => {
    const rows = clusterRows(
      sources({
        claims: [
          claim("team-a", "orders", {
            conditions: [
              { type: "Ready", status: "False", reason: "NotAllowed" },
            ],
          }),
        ],
      }),
      NOW,
    );
    expect(rows).toHaveLength(1);
    expect(rows[0]).toMatchObject({
      id: "claim/team-a/orders",
      managed: true,
      ready: { reason: "NotAllowed" },
      preset: "small",
      components: [],
    });
    expect(rows[0].href).toBeUndefined();
  });

  it("gives a claim with a cluster one row of the cluster", () => {
    const rows = clusterRows(
      sources({
        clusters: [cluster("team-a", "orders", { claim: "orders" })],
        claims: [claim("team-a", "orders")],
      }),
      NOW,
    );
    expect(rows.map((r) => r.id)).toEqual(["cluster/team-a/orders"]);
  });

  it("keeps a claim whose cluster of the same name is in another namespace", () => {
    const rows = clusterRows(
      sources({
        clusters: [cluster("team-b", "orders", { claim: "orders" })],
        claims: [claim("team-a", "orders")],
      }),
      NOW,
    );
    expect(rows.map((r) => r.id)).toEqual([
      "claim/team-a/orders",
      "cluster/team-b/orders",
    ]);
  });

  it("shows the version of the status and the preset of the spec", () => {
    const [row] = clusterRows(
      sources({
        clusters: [
          cluster("team-a", "orders", {
            spec: { presetRef: "medium" },
            status: {
              management: { version: "8.9.23" },
              conditions: [ready()],
            },
          }),
        ],
      }),
      NOW,
    );
    expect(row).toMatchObject({
      version: "8.9.23",
      preset: "medium",
      ready: { status: "True", reason: "Healthy" },
    });
  });

  it("shows no version for a suspended cluster", () => {
    const [row] = clusterRows(
      sources({
        clusters: [
          cluster("team-a", "orders", {
            spec: { suspend: true },
            status: { management: { version: "8.9.23" } },
          }),
        ],
      }),
      NOW,
    );
    expect(row.version).toBeUndefined();
  });

  it("takes the newest backup of the cluster", () => {
    const [row] = clusterRows(
      sources({
        clusters: [cluster("team-a", "orders")],
        backups: [
          backup("team-a", "old", "orders", "2026-11-08T10:00:00Z"),
          backup("team-a", "new", "orders", "2026-11-09T10:00:00Z"),
          backup("team-a", "other", "billing", "2026-11-10T10:00:00Z"),
        ],
      }),
      NOW,
    );
    expect(row.lastBackup?.name).toBe("new");
  });

  it("has no last backup without backups", () => {
    const [row] = clusterRows(
      sources({ clusters: [cluster("team-a", "orders")] }),
      NOW,
    );
    expect(row.lastBackup).toBeUndefined();
  });

  it("gives a row the environment of its namespace", () => {
    const [row] = clusterRows(
      sources({
        clusters: [cluster("team-a", "orders")],
        environments: [environment("production")],
        namespaces: [namespace("team-a", "production")],
      }),
      NOW,
    );
    expect(row.environment).toBe("production");
  });

  it("puts a cluster of an unknown environment into No environment", () => {
    const [row] = clusterRows(
      sources({
        clusters: [cluster("team-a", "orders")],
        environments: [environment("production")],
        namespaces: [namespace("team-a", "ghost")],
      }),
      NOW,
    );
    expect(row.environment).toBeUndefined();
  });

  it("puts a cluster of a namespace that is not listed into No environment", () => {
    const [row] = clusterRows(
      sources({
        clusters: [cluster("team-a", "orders")],
        environments: [environment("production")],
      }),
      NOW,
    );
    expect(row.environment).toBeUndefined();
  });

  it("gives the row of a managed cluster the section components of its claim", () => {
    const [row] = clusterRows(
      sources({
        clusters: [cluster("team-a", "orders", { claim: "orders" })],
        claims: [
          claim("team-a", "orders", {
            plugins: [{ section: "access", registration: "gateway-api" }],
          }),
        ],
      }),
      NOW,
    );
    expect(row.sections.map((s) => s.id)).toEqual(["section:access"]);
  });

  it("sorts the rows by namespace, then name", () => {
    const rows = clusterRows(
      sources({
        clusters: [
          cluster("team-b", "a"),
          cluster("team-a", "b"),
          cluster("team-a", "a"),
        ],
      }),
      NOW,
    );
    expect(rows.map((r) => `${r.namespace}/${r.name}`)).toEqual([
      "team-a/a",
      "team-a/b",
      "team-b/a",
    ]);
  });
});

describe("clusterRows on a large fleet", () => {
  // counted wraps a list and counts the reads of its items, so a test can
  // tell one pass over a list from a pass for each cluster.
  function counted<T>(items: T[]): { list: T[]; reads: () => number } {
    let reads = 0;
    const list = new Proxy(items, {
      get(target, key, receiver) {
        if (typeof key === "string" && /^\d+$/.test(key)) reads++;
        return Reflect.get(target, key, receiver);
      },
    });
    return { list, reads: () => reads };
  }

  it("reads each list a fixed number of times, not once for each cluster", () => {
    const clusters = Array.from({ length: 200 }, (_, i) =>
      cluster(`team-${i % 20}`, `c${i}`, { claim: `c${i}` }),
    );
    const claims = counted(
      clusters.map((c) =>
        claim(c.metadata!.namespace!, c.metadata!.name!, {
          plugins: [{ section: "access", registration: "gateway-api" }],
          requests: [
            {
              section: "access",
              registration: "gateway-api",
              name: `${c.metadata!.name}-access`,
            },
          ],
        }),
      ),
    );
    const requests = counted(
      clusters.map((c) =>
        request(c.metadata!.namespace!, `${c.metadata!.name}-access`, {
          type: "Ready",
          status: "True",
        }),
      ),
    );
    const backups = counted(
      Array.from({ length: 2000 }, (_, i) =>
        backup(
          `team-${i % 20}`,
          `b${i}`,
          `c${i % 200}`,
          new Date(Date.UTC(2026, 10, 1) + i * 60_000).toISOString(),
        ),
      ),
    );
    const rows = clusterRows(
      sources({
        clusters,
        claims: claims.list,
        requests: requests.list,
        backups: backups.list,
      }),
      NOW,
    );
    expect(rows).toHaveLength(200);
    expect(rows.find((r) => r.name === "c7")?.lastBackup?.name).toBe("b1807");
    expect(
      rows.find((r) => r.name === "c7")?.sections[0].request,
    ).toBeDefined();
    expect(claims.reads()).toBeLessThanOrEqual(5 * 200);
    expect(requests.reads()).toBeLessThanOrEqual(5 * 200);
    expect(backups.reads()).toBeLessThanOrEqual(5 * 2000);
  });
});

describe("waitingRows", () => {
  const push = (
    kind: string,
    namespace: string,
    name: string,
    deleted = false,
  ): PendingPush =>
    ({
      key: `${kind}/${namespace}/${name}`,
      ref: { kind, namespace, name },
      path: `clusters/${namespace}/${name}.yaml`,
      pushedAt: "2026-11-10T11:58:00Z",
      ...(deleted ? { deleted: true } : { revision: "abc" }),
    }) as PendingPush;

  it("gives a row to a pushed claim that is not live", () => {
    expect(
      waitingRows([push("CamundaClusterClaim", "team-x", "returns")], []),
    ).toEqual([
      {
        id: "push/CamundaClusterClaim/team-x/returns",
        namespace: "team-x",
        name: "returns",
        path: "clusters/team-x/returns.yaml",
        pushedAt: "2026-11-10T11:58:00Z",
      },
    ]);
  });

  it("gives no row to a pushed change of a live claim, a delete, or another kind", () => {
    expect(
      waitingRows(
        [
          push("CamundaClusterClaim", "team-a", "orders"),
          push("CamundaClusterClaim", "team-x", "gone", true),
          push("CamundaManagementClusterClaim", "platform", "plane"),
        ],
        [claim("team-a", "orders")],
      ),
    ).toEqual([]);
  });
});

const row = (
  namespace: string,
  name: string,
  over: Partial<ClusterRow> = {},
): ClusterRow => ({
  id: `cluster/${namespace}/${name}`,
  namespace,
  name,
  managed: true,
  components: [],
  sections: [],
  ...over,
});

const environments = [
  environment("staging", "Staging"),
  environment("production", "Production", "#2E7D32"),
  environment("development", "Development", "#abc"),
];
const rows = [
  row("dev", "a", { environment: "development" }),
  row("prod", "b", { environment: "production" }),
  row("prod", "c", { environment: "production" }),
  row("stage", "d", { environment: "staging" }),
  row("loose", "e"),
];

describe("environmentTabs", () => {
  it("has no tabs without an Environment", () => {
    expect(environmentTabs([], rows)).toEqual([]);
  });

  it("lists All, each Environment by name with its count, then No environment", () => {
    const tabs = environmentTabs(environments, rows);
    expect(tabs.map((t) => [t.value, t.label, t.count])).toEqual([
      [ALL_TAB, "All", 5],
      [environmentTabValue("development"), "Development", 1],
      [environmentTabValue("production"), "Production", 2],
      [environmentTabValue("staging"), "Staging", 1],
      [NO_ENVIRONMENT_TAB, "No environment", 1],
    ]);
  });

  it("gives a tab the color of its Environment", () => {
    const tabs = environmentTabs(environments, rows);
    expect(tabs.map((t) => t.color)).toEqual([
      undefined,
      "#abc",
      "#2E7D32",
      undefined,
      undefined,
    ]);
  });

  it("names an Environment without a display name by its name", () => {
    const [, tab] = environmentTabs([environment("qa")], []);
    expect(tab).toMatchObject({ label: "qa", count: 0 });
  });

  it("has no No environment tab while every row has an environment", () => {
    const tabs = environmentTabs(environments, rows.slice(0, 4));
    expect(tabs.map((t) => t.value)).not.toContain(NO_ENVIRONMENT_TAB);
  });

  it("drops a color that is not a hex color", () => {
    const [, tab] = environmentTabs(
      [environment("qa", "QA", "red; background:url(x)")],
      [],
    );
    expect(tab.color).toBeUndefined();
  });

  it("keeps the environment tab values apart from All and No environment", () => {
    const values = environmentTabs(
      [environment("all"), environment("none")],
      [row("x", "y")],
    ).map((t) => t.value);
    expect(new Set(values).size).toBe(values.length);
  });
});

describe("groupRows", () => {
  it("gives one table for each environment with rows on All, then No environment", () => {
    const groups = groupRows(
      rows,
      [...environments, environment("empty")],
      ALL_TAB,
    );
    expect(
      groups.map((g) => [g.environment, g.label, g.color, g.rows.length]),
    ).toEqual([
      ["development", "Development", "#abc", 1],
      ["production", "Production", "#2E7D32", 2],
      ["staging", "Staging", undefined, 1],
      [undefined, "No environment", undefined, 1],
    ]);
  });

  it("gives the one table of the tab, also when it is empty", () => {
    const production = groupRows(
      rows,
      environments,
      environmentTabValue("production"),
    );
    expect(production.map((g) => [g.label, g.rows.map((r) => r.name)])).toEqual(
      [["Production", ["b", "c"]]],
    );
    const empty = groupRows(rows.slice(0, 4), environments, NO_ENVIRONMENT_TAB);
    expect(empty.map((g) => [g.label, g.rows.length])).toEqual([
      ["No environment", 0],
    ]);
  });

  it("gives one table without a label when no Environment exists", () => {
    const groups = groupRows(rows, [], ALL_TAB);
    expect(groups).toHaveLength(1);
    expect(groups[0].label).toBeUndefined();
    expect(groups[0].rows).toHaveLength(5);
  });
});

describe("filterRows", () => {
  const isReady = (r: ClusterRow) => r.ready?.status === "True";
  const all = { search: "", namespace: "", status: "all" as const };

  it("keeps every row without a filter", () => {
    expect(filterRows(rows, all, isReady)).toHaveLength(5);
  });

  it("searches the name without case", () => {
    const named = [row("a", "Orders"), row("a", "billing")];
    expect(
      filterRows(named, { ...all, search: " ORD " }, isReady).map(
        (r) => r.name,
      ),
    ).toEqual(["Orders"]);
  });

  it("keeps the rows of the namespace", () => {
    expect(
      filterRows(rows, { ...all, namespace: "prod" }, isReady).map(
        (r) => r.name,
      ),
    ).toEqual(["b", "c"]);
  });

  it("keeps the rows that are not ready", () => {
    const states = [
      row("a", "false", { ready: { status: "False", reason: "Down" } }),
      row("a", "unknown", { ready: { status: "Unknown" } }),
      row("a", "none"),
      row("a", "suspended", { ready: { status: "True", reason: "Suspended" } }),
    ];
    expect(
      filterRows(states, { ...all, status: "not-ready" }, isReady).map(
        (r) => r.name,
      ),
    ).toEqual(["false", "unknown", "none"]);
  });
});

describe("rowNamespaces", () => {
  it("lists each namespace once, sorted", () => {
    expect(rowNamespaces(rows)).toEqual(["dev", "loose", "prod", "stage"]);
  });
});
