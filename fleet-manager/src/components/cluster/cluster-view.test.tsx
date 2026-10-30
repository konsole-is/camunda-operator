// @vitest-environment jsdom
import { act, cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { KindName, Resource } from "@/lib/k8s/kinds";
import {
  CLAIM_LABEL,
  BACKUP_SCHEDULE_LABEL,
  COMPONENT_LABEL,
  ENVIRONMENT_LABEL,
  REVISION_ANNOTATION,
} from "@/lib/labels";
import { type ClusterPageData, ClusterView } from "./cluster-view";
import { ACTION_SLOTS, type ActionSlots } from "./slots";

const live = vi.hoisted(() => ({
  objects: {} as Record<string, unknown[]>,
  calls: [] as { kind: string; opts: unknown }[],
  errors: {} as Record<string, string>,
}));
// The App Router keeps useSearchParams in step with history.replaceState, so
// the fake reads the URL of the window.
vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams(window.location.search),
}));
vi.mock("@/lib/k8s/hooks", () => ({
  useResources: (kind: string, opts: unknown, initial: unknown[]) => {
    live.calls.push({ kind, opts });
    return { items: live.objects[kind] ?? initial, error: live.errors[kind] };
  },
  useResource: (kind: string, ref: { name: string }, initial?: unknown) => {
    live.calls.push({ kind, opts: ref });
    const items = live.objects[kind] as
      { metadata: { name: string } }[] | undefined;
    return {
      item: items ? items.find((o) => o.metadata.name === ref.name) : initial,
    };
  },
}));

beforeEach(() => {
  live.objects = {};
  live.calls = [];
  live.errors = {};
  window.history.replaceState(null, "", "/fleet/clusters/team-a/orders");
});
afterEach(cleanup);

const now = "2026-10-18T12:00:00Z";

function brokerPod(i: number): Resource<"Pod"> {
  return {
    kind: "Pod",
    metadata: {
      name: `orders-zeebe-${i}`,
      namespace: "team-a",
      labels: { [COMPONENT_LABEL]: "zeebe" },
    },
    status: {
      phase: "Running",
      conditions: [{ type: "Ready", status: "True" }],
    },
  } as Resource<"Pod">;
}

function cluster(
  over: { labels?: Record<string, string>; spec?: object } = {},
): Resource<"CamundaCluster"> {
  return {
    kind: "CamundaCluster",
    metadata: {
      namespace: "team-a",
      name: "orders",
      labels: over.labels ?? { [CLAIM_LABEL]: "orders" },
    },
    spec: {
      externalUrl: "https://orders.example.com",
      presetRef: "medium",
      ...over.spec,
    },
    status: {
      conditions: [
        {
          type: "Ready",
          status: "True",
          reason: "Healthy",
          message: "",
          lastTransitionTime: now,
        },
      ],
      management: {
        version: "8.9.23",
        endpoint: "http://orders:9600",
        partitions: 3,
        auth: { method: "none" },
      },
    },
  } as Resource<"CamundaCluster">;
}

function claim(revision?: string): Resource<"CamundaClusterClaim"> {
  return {
    kind: "CamundaClusterClaim",
    metadata: {
      namespace: "team-a",
      name: "orders",
      annotations: revision ? { [REVISION_ANNOTATION]: revision } : {},
    },
    spec: {},
  } as unknown as Resource<"CamundaClusterClaim">;
}

function data(over: Partial<ClusterPageData> = {}): ClusterPageData {
  return {
    cluster: cluster(),
    namespace: {
      kind: "Namespace",
      metadata: { name: "team-a", labels: { [ENVIRONMENT_LABEL]: "prod" } },
    } as Resource<"Namespace">,
    environments: [
      {
        kind: "Environment",
        metadata: { name: "prod" },
        spec: { displayName: "Production" },
      } as unknown as Resource<"Environment">,
    ],
    claims: [claim()],
    presets: [
      {
        kind: "CamundaClusterPreset",
        metadata: { name: "medium" },
        spec: { cluster: {} },
      } as unknown as Resource<"CamundaClusterPreset">,
    ],
    releases: [],
    storage: [],
    accessRequests: [],
    pods: [brokerPod(0), brokerPod(1), brokerPod(2)],
    podSelector: "camunda.io/cluster=orders",
    backups: { elasticsearch: [], rdbms: [] },
    restores: { elasticsearch: [], rdbms: [], pointInTime: [] },
    backupStorage: {
      objectStorage: [],
      databaseConfigs: [],
      databaseServerConfigs: [],
      databaseServers: [],
      databaseServerPresets: [],
      schedules: [],
    },
    optimizes: [],
    sections: [],
    registrations: [],
    ...over,
  };
}

// waitingLine returns the text of the Git waiting line, or undefined.
const waitingLine = () =>
  screen
    .queryAllByRole("status")
    .map((el) => el.textContent)
    .find((text) => text?.includes("waiting for GitOps"));

const view = (props: Partial<Parameters<typeof ClusterView>[0]> = {}) => (
  <ClusterView initial={data()} now={now} {...props} />
);

const filled: ActionSlots = Object.fromEntries(
  ACTION_SLOTS.map((name) => [name, <button key={name}>Slot {name}</button>]),
);

describe("ClusterView", () => {
  it("a managed cluster shows the header, the four tabs, and empty action slots", async () => {
    const { container } = render(view());
    expect(
      screen.getByRole("heading", { level: 1, name: "orders" }),
    ).toBeTruthy();
    for (const text of ["Production", "team-a", "8.9.23", "medium"]) {
      expect(screen.getByText(text)).toBeTruthy();
    }
    expect(screen.getAllByRole("tab").map((t) => t.textContent)).toEqual([
      "Overview",
      "Backups",
      "Endpoints",
      "Claim",
    ]);

    const slot = (name: string) =>
      container.querySelector(`[data-action-slot="${name}"]`);
    for (const name of ["cluster.header.actions", "cluster.topology.actions"]) {
      expect(slot(name)?.childNodes.length).toBe(0);
    }
    await userEvent.click(screen.getByRole("tab", { name: "Backups" }));
    expect(slot("cluster.backups.actions")?.childNodes.length).toBe(0);
    await userEvent.click(screen.getByRole("tab", { name: "Claim" }));
    expect(slot("cluster.claim.editor")?.childNodes.length).toBe(0);
  });

  it("an unmanaged cluster shows the banner and no actions", async () => {
    const pushedClaim = {
      path: "clusters/team-a/camundaclusterclaims/orders.yaml",
      revision: "bbb",
      pushedAt: now,
    };
    const { container } = render(
      view({
        initial: data({ cluster: cluster({ labels: {} }) }),
        slots: filled,
        pushedClaim,
      }),
    );
    expect(
      screen.getByText(
        "Not managed by the fleet manager. You can read this cluster but not change it here.",
      ),
    ).toBeTruthy();
    expect(waitingLine()).toBeUndefined();
    for (const tab of ["Overview", "Backups", "Endpoints", "Claim"]) {
      await userEvent.click(screen.getByRole("tab", { name: tab }));
      expect(container.querySelector("[data-action-slot]")).toBeNull();
      expect(screen.queryByRole("button", { name: /^Slot / })).toBeNull();
    }
  });

  it("a managed cluster mounts the nodes of the slots", () => {
    render(view({ slots: filled }));
    expect(
      screen.getByRole("button", { name: "Slot cluster.header.actions" }),
    ).toBeTruthy();
  });

  it("a running LogicalBackupRDBMS of the cluster shows the banner with a link to the Backups tab", async () => {
    const backup = {
      kind: "LogicalBackupRDBMS",
      metadata: { namespace: "team-a", name: "nightly-1" },
      spec: { clusterRef: { name: "orders" } },
      status: { phase: "Running" },
    } as unknown as Resource<"LogicalBackupRDBMS">;
    render(
      view({
        initial: data({ backups: { elasticsearch: [], rdbms: [backup] } }),
      }),
    );
    expect(screen.getByText(/Backup nightly-1 is running\./)).toBeTruthy();
    const link = screen.getByRole("link", { name: "Open the Backups tab" });
    expect(link.getAttribute("href")).toBe("?tab=backups");
    await userEvent.click(link);
    expect(
      screen
        .getByRole("tab", { name: "Backups" })
        .getAttribute("aria-selected"),
    ).toBe("true");
    expect(window.location.search).toBe("?tab=backups");
  });

  it("in Git mode, a claim whose live revision differs from the pushed one shows the waiting line", () => {
    const pushedClaim = {
      path: "clusters/team-a/camundaclusterclaims/orders.yaml",
      revision: "bbb",
      pushedAt: "2026-10-18T11:58:00Z",
    };
    const { rerender } = render(
      view({ initial: data({ claims: [claim("aaa")] }), pushedClaim }),
    );
    expect(waitingLine()).toBe(
      "1 change waiting for GitOps: clusters/team-a/camundaclusterclaims/orders.yaml, pushed 2m ago.",
    );

    live.objects.CamundaClusterClaim = [claim("bbb")];
    rerender(view({ initial: data({ claims: [claim("aaa")] }), pushedClaim }));
    expect(waitingLine()).toBeUndefined();
  });

  it("opens on the tab of the URL", () => {
    window.history.replaceState(null, "", "?tab=claim");
    render(view());
    expect(
      screen.getByRole("tab", { name: "Claim" }).getAttribute("aria-selected"),
    ).toBe("true");
  });

  it("keeps the selected tab in the URL", async () => {
    render(view());
    await userEvent.click(screen.getByRole("tab", { name: "Endpoints" }));
    expect(window.location.search).toBe("?tab=endpoints");
  });

  it("the topology follows the merged spec of the preset", () => {
    const preset = {
      kind: "CamundaClusterPreset",
      metadata: { name: "medium" },
      spec: { cluster: { gateway: { mode: "Embedded" } } },
    } as unknown as Resource<"CamundaClusterPreset">;
    render(view({ initial: data({ presets: [preset] }) }));
    const topology = screen.getByRole("region", { name: "Topology" });
    expect(within(topology).getByText("Brokers")).toBeTruthy();
    expect(within(topology).queryByText("Gateway")).toBeNull();
    expect(within(topology).getByText("Embedded: Gateway")).toBeTruthy();
  });

  it("shows the launch links of the access request of the cluster claim without externalUrl", () => {
    const request = (
      kind: string,
      claimName: string,
      name: string,
      url: string,
    ) =>
      ({
        kind: "PluginRequest",
        metadata: {
          namespace: "team-a",
          name: `${kind}-${claimName}-access`,
          labels: { [CLAIM_LABEL]: claimName },
        },
        spec: { claimRef: { kind, name: claimName } },
        status: { links: [{ name, url }] },
      }) as unknown as Resource<"PluginRequest">;
    render(
      view({
        initial: data({
          cluster: cluster({ spec: { externalUrl: undefined } }),
          accessRequests: [
            request(
              "CamundaManagementClusterClaim",
              "orders",
              "admin",
              "https://plane.example.com",
            ),
            request(
              "CamundaClusterClaim",
              "payments",
              "tasklist",
              "https://t.example.com",
            ),
            request(
              "CamundaClusterClaim",
              "orders",
              "operate",
              "https://o.example.com",
            ),
          ],
        }),
      }),
    );
    expect(
      screen
        .getAllByRole("link", { name: /Operate|Tasklist|Admin/ })
        .map((l) => [l.textContent, l.getAttribute("href")]),
    ).toEqual([["Operate", "https://o.example.com"]]);
  });

  it("follows a navigation to another tab without a remount", () => {
    const { rerender } = render(view());
    window.history.replaceState(null, "", "?tab=backups");
    rerender(view());
    expect(
      screen
        .getByRole("tab", { name: "Backups" })
        .getAttribute("aria-selected"),
    ).toBe("true");
  });

  it("follows a navigation back to the first tab after a click", async () => {
    const { rerender } = render(view());
    await userEvent.click(screen.getByRole("tab", { name: "Backups" }));
    window.history.replaceState(null, "", "?tab=overview");
    rerender(view());
    expect(
      screen
        .getByRole("tab", { name: "Overview" })
        .getAttribute("aria-selected"),
    ).toBe("true");
  });

  it("shows a failed read with its kind and keeps the page", () => {
    live.errors.LogicalBackupRDBMS = "forbidden";
    render(view());
    expect(screen.getByRole("alert").textContent).toBe(
      "Could not read LogicalBackupRDBMS: forbidden",
    );
    expect(
      screen.getByRole("heading", { level: 1, name: "orders" }),
    ).toBeTruthy();
  });

  it("says the cluster no longer exists when it goes away", () => {
    live.objects.CamundaCluster = [];
    render(view());
    expect(screen.getByText("This cluster no longer exists.")).toBeTruthy();
    expect(screen.queryByRole("heading", { level: 1 })).toBeNull();
    expect(screen.queryAllByRole("tab")).toEqual([]);
  });

  it("watches the pods with the selector of the page", () => {
    render(
      view({
        initial: data({ podSelector: "camunda.io/cluster=long-0123456789" }),
      }),
    );
    expect(live.calls.find((c) => c.kind === "Pod")?.opts).toEqual({
      namespace: "team-a",
      labelSelector: "camunda.io/cluster=long-0123456789",
    });
  });

  it("shows the pods of the cluster", () => {
    render(view());
    expect(
      within(screen.getByRole("list", { name: "Pods of Brokers" }))
        .getAllByRole("listitem")
        .map((pod) => pod.textContent),
    ).toEqual([
      "orders-zeebe-0 Ready",
      "orders-zeebe-1 Ready",
      "orders-zeebe-2 Ready",
    ]);
  });
});

function nsObject<K extends KindName>(
  kind: K,
  name: string,
  spec: object,
  status: object = {},
  meta: object = {},
): Resource<K> {
  return {
    kind,
    metadata: { namespace: "team-a", name, ...meta },
    spec,
    status,
  } as unknown as Resource<K>;
}

function esBackup(
  name: string,
  phase: string,
  created: string,
  labels?: Record<string, string>,
) {
  return nsObject(
    "LogicalBackupElasticsearch",
    name,
    { clusterRef: { name: "orders" } },
    { phase },
    { creationTimestamp: created, labels },
  );
}

const noBackupStorage = data().backupStorage;

function pgPage(
  server: object = {
    databaseServerConfig: "orders-pg",
    archive: { objectStorageRef: "wal", retentionPeriodDays: 30 },
  },
  status?: object,
): ClusterPageData {
  return data({
    cluster: cluster({
      spec: {
        storageRef: "orders-db-storage",
        backupStorageRef: "backups",
      },
    }),
    storage: [
      nsObject("SecondaryStorageConfig", "orders-db-storage", {
        type: "rdbms",
        rdbms: { databaseConfigRef: "orders-db" },
      }),
    ],
    backupStorage: {
      objectStorage: [
        nsObject("ObjectStorageConfig", "backups", {
          type: "S3",
          s3: { bucketName: "acme-backups" },
        }),
        nsObject("ObjectStorageConfig", "wal", {
          type: "GCS",
          gcs: { bucketName: "acme-wal" },
        }),
      ],
      databaseConfigs: [
        nsObject("DatabaseConfig", "orders-db", {
          serverRef: "orders-pg",
        }),
      ],
      databaseServerConfigs: [
        nsObject(
          "DatabaseServerConfig",
          "orders-pg",
          {
            host: "orders-pg-rw.team-a.svc",
            port: 5432,
            pitr: {
              enabled: true,
              recovery: "operator",
              retentionPeriodDays: 30,
            },
          },
          {},
          {
            ownerReferences: [
              {
                apiVersion: "core.camunda.io/v1",
                kind: "DatabaseServer",
                name: "orders-pg",
                uid: "uid-orders-pg",
                controller: true,
              },
            ],
          },
        ),
      ],
      databaseServers: [
        nsObject(
          "DatabaseServer",
          "orders-pg",
          server,
          status ?? {
            conditions: [
              {
                type: "ArchiveReady",
                status: "True",
                reason: "Archiving",
              },
            ],
            archive: {
              history: [
                {
                  serverName: "orders-pg",
                  from: "2026-09-10T00:00:00Z",
                  to: "2026-09-20T00:00:00Z",
                },
                { serverName: "orders-pg-r1", from: "2026-10-11T00:00:00Z" },
              ],
            },
          },
          { uid: "uid-orders-pg" },
        ),
      ],
      databaseServerPresets: [],
      schedules: [],
    },
  });
}

describe("Backups tab", () => {
  beforeEach(() => {
    window.history.replaceState(null, "", "?tab=backups");
  });

  it("an Elasticsearch cluster shows its storage, a running, a completed, and a failed backup, and a running restore", () => {
    const live = cluster({
      spec: { storageRef: "orders-es", backupStorageRef: "backups" },
    });
    live.status!.management!.backupRepository = "camunda-backups";
    render(
      view({
        initial: data({
          cluster: live,
          storage: [
            nsObject("SecondaryStorageConfig", "orders-es", {
              type: "elasticsearch",
              elasticsearch: { endpoint: "http://orders-es:9200" },
            }),
          ],
          backupStorage: {
            ...noBackupStorage,
            objectStorage: [
              nsObject("ObjectStorageConfig", "backups", {
                type: "S3",
                s3: { bucketName: "acme-backups" },
              }),
            ],
            schedules: [
              nsObject("BackupSchedule", "nightly", {
                clusterRef: { name: "orders" },
                schedule: "0 2 * * *",
                retained: { completed: 7, failed: 3 },
              }),
            ],
          },
          backups: {
            elasticsearch: [
              esBackup("nightly-1", "Completed", "2026-10-18T02:00:00Z", {
                [BACKUP_SCHEDULE_LABEL]: "nightly",
              }),
              esBackup("manual-1", "Failed", "2026-10-18T10:00:00Z"),
              esBackup("running-1", "Running", "2026-10-18T11:30:00Z"),
            ],
            rdbms: [],
          },
          restores: {
            elasticsearch: [
              nsObject(
                "LogicalRestoreElasticsearch",
                "restore-1",
                {
                  targetClusterRef: { name: "orders" },
                  backupRef: { name: "nightly-1" },
                },
                { phase: "RestoringPrimaryStorage" },
              ),
            ],
            rdbms: [],
            pointInTime: [],
          },
        }),
      }),
    );
    const storage = screen.getByRole("region", { name: "Backup storage" });
    expect(within(storage).getByText("camunda-backups")).toBeTruthy();
    expect(within(storage).getByText("s3://acme-backups")).toBeTruthy();
    const schedule = screen.getByRole("region", { name: "Backup schedule" });
    expect(schedule.textContent).toContain(
      "nightly: 0 2 * * *, keeps 7 completed and 3 failed",
    );
    const backups = screen.getByRole("region", { name: "Backups" });
    const rows = within(backups).getAllByRole("row").slice(1);
    expect(
      rows.map((r) => [
        within(r).getByRole("button").textContent,
        within(r).getAllByRole("cell")[1].textContent,
        within(r).getAllByRole("cell")[5].textContent,
      ]),
    ).toEqual([
      ["running-1", "Manual", "Running"],
      ["manual-1", "Manual", "Failed"],
      ["nightly-1", "Scheduled", "Completed"],
    ]);
    const restores = screen.getByRole("region", { name: "Restores" });
    expect(within(restores).getByText("restore-1")).toBeTruthy();
    expect(within(restores).getByText("RestoringPrimaryStorage")).toBeTruthy();
  });

  it("a PostgreSQL cluster with point-in-time restore on shows its server, archive, and the intervals of its window", () => {
    render(
      view({
        initial: pgPage(),
      }),
    );
    const storage = screen.getByRole("region", { name: "Backup storage" });
    expect(
      within(storage).getByText("orders-pg-rw.team-a.svc:5432"),
    ).toBeTruthy();
    expect(
      within(storage).getByText("On, 30 days, recovery by the operator"),
    ).toBeTruthy();
    expect(within(storage).getByText("Archiving")).toBeTruthy();
    expect(within(storage).getByText("gs://acme-wal")).toBeTruthy();
    expect(
      within(storage)
        .getAllByRole("listitem")
        .map((li) => li.textContent),
    ).toEqual([
      "2026-09-18 12:00 UTC to 2026-09-20 00:00 UTC",
      "No restore point in this gap",
      "2026-10-11 00:00 UTC to now",
    ]);
  });

  it("a cluster without backup storage says where a bucket comes from", () => {
    render(view());
    expect(
      screen.getByText(
        "This cluster has no backup storage. Backups need a bucket from the Backup storage section.",
      ),
    ).toBeTruthy();
    expect(screen.queryByText(/^s3:\/\//)).toBeNull();
  });

  it("an unmanaged cluster shows its backups and no actions slot", () => {
    const { container } = render(
      view({
        initial: data({
          cluster: cluster({ labels: {} }),
          backups: {
            elasticsearch: [
              esBackup("nightly-1", "Completed", "2026-10-18T02:00:00Z"),
            ],
            rdbms: [],
          },
        }),
        slots: filled,
      }),
    );
    expect(
      screen.getByRole("button", { name: "Details of nightly-1" }),
    ).toBeTruthy();
    expect(container.querySelector("[data-action-slot]")).toBeNull();
    expect(
      screen.queryByRole("button", { name: "Slot cluster.backups.actions" }),
    ).toBeNull();
  });

  it("a managed cluster mounts the backups actions slot in the tab", () => {
    render(view({ slots: filled }));
    expect(
      screen.getByRole("button", { name: "Slot cluster.backups.actions" }),
    ).toBeTruthy();
  });

  it("follows a backup that appears after the first render", () => {
    const { rerender } = render(view());
    expect(screen.getByText("No backups yet.")).toBeTruthy();
    live.objects.LogicalBackupRDBMS = [
      nsObject(
        "LogicalBackupRDBMS",
        "manual-2",
        { clusterRef: { name: "orders" } },
        { phase: "Running" },
        { creationTimestamp: "2026-10-18T11:59:00Z" },
      ),
    ];
    rerender(view());
    expect(
      screen.getByRole("button", { name: "Details of manual-2" }),
    ).toBeTruthy();
  });

  it("a running backup counts its duration on with the clock", () => {
    vi.useFakeTimers({ now: Date.parse(now) });
    try {
      render(
        view({
          initial: data({
            backups: {
              elasticsearch: [
                esBackup("running-1", "Running", "2026-10-18T11:30:00Z"),
              ],
              rdbms: [],
            },
          }),
        }),
      );
      const row = () =>
        within(screen.getByRole("region", { name: "Backups" })).getAllByRole(
          "row",
        )[1];
      expect(row().textContent).toContain("30m");
      act(() => {
        vi.advanceTimersByTime(65_000);
      });
      expect(row().textContent).toContain("31m 5s");
    } finally {
      vi.useRealTimers();
    }
  });

  it("moves the start of the restore window on with the clock on the Backups tab", () => {
    vi.useFakeTimers({ now: Date.parse(now) });
    try {
      render(
        view({
          initial: pgPage(
            {
              databaseServerConfig: "orders-pg",
              archive: { objectStorageRef: "wal", retentionPeriodDays: 7 },
            },
            {
              archive: {
                history: [
                  { serverName: "orders-pg", from: "2026-01-01T00:00:00Z" },
                ],
              },
            },
          ),
        }),
      );
      expect(screen.getByText("2026-10-11 12:00 UTC to now")).toBeTruthy();
      act(() => {
        vi.advanceTimersByTime(60_000);
      });
      expect(screen.getByText("2026-10-11 12:01 UTC to now")).toBeTruthy();
    } finally {
      vi.useRealTimers();
    }
  });

  it("does not move the clock while another tab is open", () => {
    vi.useFakeTimers({ now: Date.parse(now) });
    try {
      const page = pgPage(
        {
          databaseServerConfig: "orders-pg",
          archive: { objectStorageRef: "wal", retentionPeriodDays: 7 },
        },
        {
          archive: {
            history: [
              { serverName: "orders-pg", from: "2026-01-01T00:00:00Z" },
            ],
          },
        },
      );
      window.history.replaceState(null, "", "?tab=overview");
      const { rerender } = render(view({ initial: page }));
      act(() => {
        vi.advanceTimersByTime(60_000);
      });
      window.history.replaceState(null, "", "?tab=backups");
      rerender(view({ initial: page }));
      expect(screen.getByText("2026-10-11 12:00 UTC to now")).toBeTruthy();
      act(() => {
        vi.advanceTimersByTime(1_000);
      });
      expect(screen.getByText("2026-10-11 12:01 UTC to now")).toBeTruthy();
    } finally {
      vi.useRealTimers();
    }
  });

  it("watches the DatabaseServerPresets of every namespace", () => {
    render(view());
    expect(
      live.calls.find((c) => c.kind === "DatabaseServerPreset")?.opts,
    ).toEqual({});
  });

  it("watches the backup kinds in the namespace of the cluster", () => {
    render(view());
    for (const kind of [
      "ObjectStorageConfig",
      "DatabaseConfig",
      "DatabaseServerConfig",
      "DatabaseServer",
      "BackupSchedule",
    ]) {
      expect(live.calls.find((c) => c.kind === kind)?.opts).toEqual({
        namespace: "team-a",
      });
    }
  });

  it("watches the CamundaOptimize objects in the namespace of the cluster", () => {
    render(view());
    expect(live.calls).toContainEqual({
      kind: "CamundaOptimize",
      opts: { namespace: "team-a" },
    });
  });

  it("watches the PluginSections and PluginRegistrations of the whole cluster", () => {
    render(view());
    expect(live.calls).toContainEqual({ kind: "PluginSection", opts: {} });
    expect(live.calls).toContainEqual({ kind: "PluginRegistration", opts: {} });
  });
});

// condition is a status condition of the API server, with its time.
const condition = (
  type: string,
  status: "True" | "False" | "Unknown",
  reason: string,
) => ({
  type,
  status,
  reason,
  message: "",
  lastTransitionTime: now,
});

// withConditions gives the cluster of data() with more conditions after Ready.
function withConditions(
  ...conditions: ReturnType<typeof condition>[]
): Resource<"CamundaCluster"> {
  const c = cluster();
  c.status!.conditions = [...c.status!.conditions!, ...conditions];
  return c;
}

function podOf(name: string, component: string): Resource<"Pod"> {
  return {
    kind: "Pod",
    metadata: {
      name,
      namespace: "team-a",
      labels: { [COMPONENT_LABEL]: component },
    },
    status: {
      phase: "Running",
      conditions: [{ type: "Ready", status: "True" }],
    },
  } as Resource<"Pod">;
}

describe("Overview components and conditions", () => {
  const components = () => screen.getByRole("region", { name: "Components" });

  it("an AdminSecretReady=False shows in the Conditions section of the Overview and in no card", () => {
    render(
      view({
        initial: data({
          cluster: withConditions(
            condition("AdminSecretReady", "False", "MissingSecret"),
          ),
        }),
      }),
    );
    expect(
      within(screen.getByRole("region", { name: "Conditions" })).getByRole(
        "rowheader",
        { name: "AdminSecretReady" },
      ),
    ).toBeTruthy();
    expect(within(components()).queryByText("AdminSecretReady")).toBeNull();
    expect(
      within(components()).queryByRole("img", { name: /MissingSecret/ }),
    ).toBeNull();
  });

  it("a ZeebeReady=False turns the Brokers dot red", () => {
    render(
      view({
        initial: data({
          cluster: withConditions(condition("ZeebeReady", "False", "Degraded")),
        }),
      }),
    );
    const name = "Brokers: Not ready (Degraded)";
    const topologyDot = within(
      screen.getByRole("region", { name: "Topology" }),
    ).getByRole("img", { name });
    const cardDot = within(
      within(components()).getByRole("article", { name: "Brokers" }),
    ).getByRole("img", { name });
    expect(topologyDot.className).toContain("bg-bad");
    expect(cardDot.className).toContain("bg-bad");
    expect(
      within(screen.getByRole("region", { name: "Conditions" })).queryByRole(
        "rowheader",
        { name: "ZeebeReady" },
      ),
    ).toBeNull();
  });

  it("an embedded Operate with a Disabled OperateReady shows inside the gateway", () => {
    render(
      view({
        initial: data({
          cluster: withConditions(
            condition("OperateReady", "True", "Disabled"),
          ),
        }),
      }),
    );
    expect(
      within(
        within(components()).getByRole("article", { name: "Gateway" }),
      ).getByText("Embedded: Operate"),
    ).toBeTruthy();
    expect(
      within(components()).queryByRole("article", { name: "Operate" }),
    ).toBeNull();
    expect(
      within(components()).queryByRole("img", { name: "Operate: Not used" }),
    ).toBeNull();
  });

  it("a cluster without Optimize shows the Optimize card as not used", () => {
    render(view());
    expect(
      within(components()).getByRole("img", { name: "Optimize: Not used" }),
    ).toBeTruthy();
  });

  it("a cluster with Optimize shows its pods on the Optimize card", () => {
    const optimize = nsObject(
      "CamundaOptimize",
      "orders-optimize",
      { clusterRef: { name: "orders" } },
      { conditions: [condition("Ready", "True", "Healthy")] },
      { creationTimestamp: now },
    );
    render(
      view({
        initial: data({
          optimizes: [optimize],
          pods: [
            brokerPod(0),
            podOf("orders-optimize-web", "optimize-webapp"),
            podOf("orders-optimize-imp", "optimize-importer"),
          ],
        }),
      }),
    );
    const card = within(components()).getByRole("article", {
      name: "Optimize",
    });
    expect(
      within(card).getByText("Pods ready", { selector: "dt" })
        .nextElementSibling?.textContent,
    ).toBe("2/2");
    expect(
      within(card).getByRole("img", { name: "Optimize: Ready" }),
    ).toBeTruthy();
  });

  it("the claim conditions show in the Claim group", () => {
    const blocked = claim();
    (blocked as { status?: object }).status = {
      conditions: [
        condition("Ready", "False", "Blocked"),
        condition("RequestsReady", "False", "Blocked"),
      ],
    };
    render(view({ initial: data({ claims: [blocked] }) }));
    const conditions = screen.getByRole("region", { name: "Conditions" });
    expect(
      within(conditions)
        .getAllByRole("rowheader")
        .map((h) => h.textContent),
    ).toEqual(["Claim", "RequestsReady"]);
  });
});
