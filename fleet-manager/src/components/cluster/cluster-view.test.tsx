// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { cloneElement, type ReactElement } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { GitPushesProvider } from "@/components/fleet/git-pushes";
import type { Step } from "@/lib/fields";
import type { KindName, Resource } from "@/lib/k8s/kinds";
import { claimAfter, valueAt } from "@/lib/session";
import {
  CLAIM_LABEL,
  BACKUP_SCHEDULE_LABEL,
  COMPONENT_LABEL,
  ENVIRONMENT_LABEL,
  OPTIMIZE_UID_LABEL,
  REVISION_ANNOTATION,
  SECTION_LABEL,
  ACCESS_SECTION,
} from "@/lib/labels";
import { type ClusterPageData, ClusterView } from "./cluster-view";
import { useClusterEdit } from "./edit/cluster-edit";
import {
  coreSections,
  fixtureClaim,
  fixtureContract,
  fixtureGrant,
  fixtureRegistrations,
  fixtureRequest,
  fixtureRequests,
  fixtureSelections,
} from "./request-fixtures";
import {
  ACTION_SLOTS,
  type ActionSlots,
  type ClusterSlotContext,
  type ClusterSlots,
} from "./slots";

const live = vi.hoisted(() => ({
  objects: {} as Record<string, unknown[]>,
  calls: [] as { kind: string; opts: unknown }[],
  errors: {} as Record<string, string>,
  usage: undefined as unknown,
  partitions: undefined as unknown,
  observed: [] as { hook: string; args: string[] }[],
  stale: [] as string[],
  overdue: [] as string[],
}));
// The App Router keeps useSearchParams in step with history.replaceState, so
// the fake reads the URL of the window.
vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams(window.location.search),
}));
vi.mock("@/lib/observe/use-observe", () => ({
  usePodUsage: (...args: string[]) => {
    live.observed.push({ hook: "usePodUsage", args });
    return live.usage;
  },
  usePartitions: (...args: string[]) => {
    live.observed.push({ hook: "usePartitions", args });
    return live.partitions;
  },
}));
vi.mock("@/lib/k8s/hooks", () => ({
  useResources: (kind: string, opts: unknown, initial: unknown[]) => {
    live.calls.push({ kind, opts });
    return { items: live.objects[kind] ?? initial, error: live.errors[kind] };
  },
  useWatchesFresh: (lists: readonly { kind: string }[]) =>
    lists.map((l) => !live.stale.includes(l.kind)),
  useWatchesOverdue: (lists: readonly { kind: string }[]) =>
    lists.map((l) => live.overdue.includes(l.kind)),
  useGitSync: () => "waiting",
  useGitGone: () => "waiting",
  useResource: (kind: string, ref: { name: string }, initial?: unknown) => {
    live.calls.push({ kind, opts: ref });
    const items = live.objects[kind] as
      { metadata: { name: string } }[] | undefined;
    return {
      item: items ? items.find((o) => o.metadata.name === ref.name) : initial,
      error: live.errors[kind],
    };
  },
}));

beforeEach(() => {
  live.objects = {};
  live.calls = [];
  live.errors = {};
  live.usage = undefined;
  live.partitions = undefined;
  live.observed = [];
  live.stale = [];
  live.overdue = [];
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
    ref: { namespace: "team-a", name: "orders" },
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
    requests: [],
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
      databases: [],
      schedules: [],
    },
    optimizes: [],
    sections: [],
    registrations: [],
    platformConfigs: [],
    managementAuthConfigs: [],
    grants: [],
    elasticsearchClusters: [],
    fieldDocs: {},
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

  it("watches the platform configs and the management auth configs of every namespace", () => {
    render(view());
    expect(live.calls).toContainEqual({
      kind: "CamundaPlatformConfig",
      opts: {},
    });
    expect(live.calls).toContainEqual({
      kind: "ManagementAuthConfig",
      opts: {},
    });
  });

  it("an unmanaged cluster gets no edit host, also with claim actions", () => {
    const claimActions = { preview: vi.fn(), apply: vi.fn() };
    render(
      view({
        initial: data({ cluster: cluster({ labels: {} }) }),
        claimActions,
      }),
    );
    expect(
      screen.queryByRole("region", { name: "Unsaved changes" }),
    ).toBeNull();
    expect(screen.queryByText(/^Changes are off/)).toBeNull();
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
          labels: { [CLAIM_LABEL]: claimName, [SECTION_LABEL]: ACCESS_SECTION },
        },
        spec: { claimRef: { kind, name: claimName } },
        status: { links: [{ name, url }] },
      }) as unknown as Resource<"PluginRequest">;
    render(
      view({
        initial: data({
          cluster: cluster({ spec: { externalUrl: undefined } }),
          requests: [
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
      within(screen.getByRole("banner"))
        .getAllByRole("link", { name: /Operate|Tasklist|Admin/ })
        .map((l) => [l.textContent, l.getAttribute("href")]),
    ).toEqual([["Operate", "https://o.example.com"]]);
  });

  it("reads no access link from a request of another section", () => {
    const backup = nsObject(
      "PluginRequest",
      "orders-backup-storage",
      { claimRef: { kind: "CamundaClusterClaim", name: "orders" } },
      { links: [{ name: "operate", url: "https://o.example.com" }] },
      {
        labels: { [CLAIM_LABEL]: "orders", [SECTION_LABEL]: "backup-storage" },
      },
    );
    render(
      view({
        initial: data({
          cluster: cluster({ spec: { externalUrl: undefined } }),
          requests: [backup],
        }),
      }),
    );
    expect(
      within(screen.getByRole("banner")).queryAllByRole("link", {
        name: /Operate/,
      }),
    ).toEqual([]);
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

  it("follows the Grants and ElasticsearchClusters of the namespace of the cluster", () => {
    render(view());
    expect(live.calls).toContainEqual({
      kind: "Grant",
      opts: { namespace: "team-a" },
    });
    expect(live.calls).toContainEqual({
      kind: "ElasticsearchCluster",
      opts: { namespace: "team-a" },
    });
  });

  it("names a failed read of the Grants", () => {
    live.errors.Grant = "forbidden";
    render(view());
    expect(screen.getByRole("alert").textContent).toBe(
      "Could not read Grant: forbidden",
    );
  });

  it("names the kind in the Backups and Restores boxes, never that there are none, while their reads failed", async () => {
    live.errors.LogicalBackupRDBMS = "forbidden";
    live.errors.PointInTimeRestore = "forbidden";
    render(view());
    await userEvent.click(screen.getByRole("tab", { name: "Backups" }));
    const backups = screen.getByRole("region", { name: "Backups" });
    expect(
      within(backups).getByText(
        "Not known: the fleet manager has no current list of LogicalBackupRDBMS.",
      ),
    ).toBeTruthy();
    expect(within(backups).queryByText("No backups yet.")).toBeNull();
    const restores = screen.getByRole("region", { name: "Restores" });
    expect(
      within(restores).getByText(
        "Not known: the fleet manager has no current list of PointInTimeRestore.",
      ),
    ).toBeTruthy();
    expect(within(restores).queryByText("No restores.")).toBeNull();
  });

  it("names the claim kind, never that the claim is gone, while the read of the claims failed", async () => {
    live.errors.CamundaClusterClaim = "forbidden";
    render(view({ initial: data({ claims: [] }) }));
    expect(
      screen.getByText(
        "Not known: the fleet manager has no current list of CamundaClusterClaim.",
      ),
    ).toBeTruthy();
    await userEvent.click(screen.getByRole("tab", { name: "Claim" }));
    expect(screen.queryByText(/is not in this namespace/)).toBeNull();
    expect(
      screen.getByText(
        "Not known: the fleet manager has no current list of CamundaClusterClaim.",
      ),
    ).toBeTruthy();
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

  it("says the cluster no longer exists when it and its claim go away", () => {
    live.objects.CamundaCluster = [];
    live.objects.CamundaClusterClaim = [];
    render(view());
    expect(screen.getByText("This cluster no longer exists.")).toBeTruthy();
    expect(screen.queryByRole("heading", { level: 1 })).toBeNull();
    expect(screen.queryAllByRole("tab")).toEqual([]);
  });

  it("a claim that the precheck refused shows the refusal while its cluster is Ready", () => {
    const refused = {
      ...claim(),
      metadata: { ...claim().metadata, generation: 2 },
      status: {
        conditions: [
          {
            type: "Ready",
            status: "False",
            reason: "NotAllowed",
            observedGeneration: 2,
            message:
              "The Environment prod does not allow the release camunda-8-8.",
            lastTransitionTime: now,
          },
        ],
      },
    } as unknown as Resource<"CamundaClusterClaim">;
    render(view({ initial: data({ claims: [refused] }) }));
    expect(
      screen.getByText(
        "The Environment prod does not allow the release camunda-8-8.",
      ),
    ).toBeTruthy();
    expect(
      screen
        .getByRole("link", { name: "Open the Claim tab" })
        .getAttribute("href"),
    ).toBe("?tab=claim");
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
            adminCredentialsSecretRef: {
              name: "pg-admin",
              usernameKey: "username",
              passwordKey: "password",
            },
            pitr: {
              enabled: true,
              recovery: "operator",
              retentionPeriodDays: 30,
            },
          },
          {
            systemIdentifier: "7300",
            probedAt: "2026-10-11T00:00:00Z",
            probedEndpoint: "orders-pg-rw.team-a.svc:5432",
            probedSecretName: "pg-admin",
            probedSecretKeys: "username/password",
          },
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
      databases: [],
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

  // serverUseSeen renders the page of initial on the Backups tab and gives
  // the server use that the backup actions slot gets.
  const serverUseSeen = (initial: ClusterPageData) => {
    let seen: ClusterSlotContext | undefined;
    window.history.replaceState(null, "", "?tab=backups");
    const ui = () =>
      view({
        initial,
        slots: {
          "cluster.backups.actions": (c) => {
            seen = c;
            return null;
          },
        },
      });
    const page = render(ui());
    const use = () =>
      seen?.backupStorage.kind === "rdbms"
        ? seen.backupStorage.database?.serverUse
        : undefined;
    return { rerender: () => page.rerender(ui()), use };
  };

  it("the server use is unread while the first read of the Database resources failed, and follows the watch once it answers", () => {
    live.stale = ["Database"];
    const { rerender, use } = serverUseSeen({
      ...pgPage(),
      readErrors: [{ kind: "Database", message: "forbidden" }],
    });
    expect(use()).toBe("unread");
    live.stale = [];
    rerender();
    expect(use()).toBe("unpublished");
  });

  it("the server use is unread while the watch of the Database resources lost its connection", () => {
    const { rerender, use } = serverUseSeen(pgPage());
    expect(use()).toBe("unpublished");
    live.stale = ["Database"];
    rerender();
    expect(use()).toBe("unread");
  });

  it.each(["DatabaseConfig", "DatabaseServerConfig"])(
    "the server use is unread while the watch of the %s resources lost its connection",
    (kind) => {
      const { rerender, use } = serverUseSeen(pgPage());
      expect(use()).toBe("unpublished");
      live.stale = [kind];
      rerender();
      expect(use()).toBe("unread");
    },
  );

  it("the server use is unread while the first read of the DatabaseServerConfig resources failed", () => {
    live.stale = ["DatabaseServerConfig"];
    const { use } = serverUseSeen({
      ...pgPage(),
      readErrors: [{ kind: "DatabaseServerConfig", message: "forbidden" }],
    });
    expect(use()).toBe("unread");
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

  const twoBackups = () =>
    data({
      backups: {
        elasticsearch: [
          esBackup("nightly-1", "Completed", "2026-10-18T02:00:00Z"),
          esBackup("manual-1", "Completed", "2026-10-18T10:00:00Z"),
        ],
        rdbms: [],
      },
    });
  const rowSlots: ClusterSlots = {
    "cluster.backups.actions": <button>Back up now</button>,
    "cluster.backups.row": (_ctx, row) => <button>Delete {row.name}</button>,
  };

  it("a managed cluster mounts the row slot on each backup row and keeps the header slot", () => {
    const { container } = render(
      view({ initial: twoBackups(), slots: rowSlots }),
    );
    for (const name of ["nightly-1", "manual-1"]) {
      const row = screen
        .getByRole("button", { name: `Details of ${name}` })
        .closest("tr")!;
      const mount = row.querySelector(
        '[data-action-slot="cluster.backups.row"]',
      );
      expect(mount?.textContent).toBe(`Delete ${name}`);
    }
    const header = container.querySelector(
      '[data-action-slot="cluster.backups.actions"]',
    );
    expect(header?.textContent).toBe("Back up now");
  });

  it("an unmanaged cluster mounts no row slot", () => {
    const initial = twoBackups();
    render(
      view({
        initial: { ...initial, cluster: cluster({ labels: {} }) },
        slots: rowSlots,
      }),
    );
    expect(screen.queryByRole("button", { name: /^Delete/ })).toBeNull();
    expect(screen.queryByRole("columnheader", { name: "Actions" })).toBeNull();
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
      window.history.replaceState(null, "", "?tab=endpoints");
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

  it("watches the PluginRequests of the claim of the cluster by the claim label", () => {
    render(view());
    expect(live.calls).toContainEqual({
      kind: "PluginRequest",
      opts: {
        namespace: "team-a",
        labelSelector: `${CLAIM_LABEL}=orders`,
      },
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

function podOf(
  name: string,
  component: string,
  labels: Record<string, string> = {},
): Resource<"Pod"> {
  return {
    kind: "Pod",
    metadata: {
      name,
      namespace: "team-a",
      labels: { [COMPONENT_LABEL]: component, ...labels },
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
      { creationTimestamp: now, uid: "uid-optimize" },
    );
    const ofOptimize = { [OPTIMIZE_UID_LABEL]: "uid-optimize" };
    render(
      view({
        initial: data({
          optimizes: [optimize],
          pods: [
            brokerPod(0),
            podOf("orders-optimize-web", "optimize-webapp", ofOptimize),
            podOf("orders-optimize-imp", "optimize-importer", ofOptimize),
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

  it("counts the condition ages with the clock on the Overview", () => {
    vi.useFakeTimers({ now: Date.parse(now) });
    try {
      render(
        view({
          initial: data({
            cluster: withConditions(
              condition("AdminSecretReady", "False", "MissingSecret"),
            ),
          }),
        }),
      );
      const row = () =>
        within(screen.getByRole("region", { name: "Conditions" }))
          .getByRole("rowheader", { name: "AdminSecretReady" })
          .closest("tr")!;
      expect(row().textContent).toContain("0s");
      act(() => {
        vi.advanceTimersByTime(65_000);
      });
      expect(row().textContent).toContain("1m");
    } finally {
      vi.useRealTimers();
    }
  });

  it("the Brokers card shows the partitions with a leader and the Gateway its highest use", () => {
    live.partitions = [
      {
        partition: 1,
        leader: "orders-zeebe-0",
        followers: [],
        health: "unknown",
      },
      { partition: 2, followers: [], health: "unknown" },
    ];
    live.usage = [
      {
        pod: "orders-gateway-0",
        cpu: { usedCores: 0.25, limitCores: 1 },
        memory: { usedBytes: 600, limitBytes: 1000 },
      },
    ];
    render(
      view({
        initial: data({
          pods: [brokerPod(0), podOf("orders-gateway-0", "gateway")],
        }),
      }),
    );
    const number = (card: string, label: string) =>
      within(
        within(components()).getByRole("article", { name: card }),
      ).getByText(label, { selector: "dt" }).nextElementSibling?.textContent;
    expect(number("Brokers", "Partitions with a leader")).toBe("1/2");
    expect(number("Gateway", "Highest use")).toBe("60 % memory");
    expect(live.observed).toContainEqual({
      hook: "usePodUsage",
      args: ["team-a", "camunda.io/cluster=orders"],
    });
    expect(live.observed).toContainEqual({
      hook: "usePartitions",
      args: ["team-a", "orders"],
    });
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

type ClusterEndpoints = NonNullable<
  Resource<"CamundaCluster">["status"]
>["endpoints"];

describe("Endpoints tab", () => {
  const httpRoute = (name: string, pathPrefix: string) => ({
    name,
    pathPrefix,
    port: 8080,
    protocol: "HTTP",
    service: "orders-gateway",
  });
  const optimizeOf = (clusterRef: string, externalUrl: string) =>
    nsObject(
      "CamundaOptimize",
      `${clusterRef}-optimize`,
      { clusterRef: { name: clusterRef }, externalUrl },
      {
        endpoints: [
          {
            name: "optimize",
            pathPrefix: "/",
            port: 8090,
            protocol: "HTTP",
            service: `${clusterRef}-optimize`,
          },
        ],
      },
    );

  it("lists the routes of a cluster with an embedded Operate and an Optimize attachment", () => {
    window.history.replaceState(null, "", "?tab=endpoints");
    const c = cluster();
    c.status!.endpoints = [
      httpRoute("default", "/"),
      httpRoute("operate", "/operate"),
    ] as ClusterEndpoints;
    render(
      view({
        initial: data({
          cluster: c,
          optimizes: [
            optimizeOf("orders", "https://optimize.example.com"),
            optimizeOf("payments", "https://payments-optimize.example.com"),
          ],
        }),
      }),
    );
    const table = screen.getByRole("table", { name: "Endpoints" });
    const operate = within(table).getByRole("row", { name: /^operate/ });
    expect(within(operate).getByText("Gateway")).toBeTruthy();
    expect(within(operate).getByText("(embedded)")).toBeTruthy();
    expect(
      within(operate).getByRole("link", {
        name: "https://orders.example.com/operate",
      }),
    ).toBeTruthy();
    expect(
      within(table).getByRole("link", {
        name: "https://optimize.example.com/",
      }),
    ).toBeTruthy();
    expect(
      within(table).getAllByRole("row", { name: /^optimize/ }),
    ).toHaveLength(1);
  });

  it("gives the gRPC route the host of the access request of the claim", () => {
    window.history.replaceState(null, "", "?tab=endpoints");
    const c = cluster();
    c.status!.endpoints = [
      {
        name: "grpc",
        port: 26500,
        protocol: "GRPC",
        service: "orders-gateway",
      },
    ] as ClusterEndpoints;
    const access = nsObject(
      "PluginRequest",
      "orders-access",
      { claimRef: { kind: "CamundaClusterClaim", name: "orders" } },
      { outputs: { grpcHost: "grpc.orders.example.com:443" } },
      { labels: { [CLAIM_LABEL]: "orders", [SECTION_LABEL]: ACCESS_SECTION } },
    );
    render(view({ initial: data({ cluster: c, requests: [access] }) }));
    const table = screen.getByRole("table", { name: "Endpoints" });
    expect(within(table).getByText("grpc.orders.example.com:443")).toBeTruthy();
  });
});

describe("slot context", () => {
  it("gives a render function the live claim, the merged spec, the write mode, and the open component", () => {
    let seen: ClusterSlotContext | undefined;
    window.history.replaceState(null, "", "?tab=overview&component=zeebe");
    render(
      view({
        writeMode: "git",
        slots: {
          "cluster.header.actions": (c) => {
            seen = c;
            return null;
          },
        },
      }),
    );
    expect(seen?.cluster.metadata?.name).toBe("orders");
    expect(seen?.claim?.metadata?.name).toBe("orders");
    expect(seen?.merged.spec.presetRef).toBe("medium");
    expect(seen?.writeMode).toBe("git");
    expect(seen?.openId).toBe("zeebe");
    expect(seen?.now).toBe(now);
  });

  it("gives the requests of the claim and no request of another claim", () => {
    let seen: ClusterSlotContext | undefined;
    const request = (kind: string, claimName: string) =>
      nsObject("PluginRequest", `${claimName}-secondary-storage`, {
        claimRef: { kind, name: claimName },
        section: "secondary-storage",
        registration: "elasticsearch",
      });
    render(
      view({
        initial: data({
          requests: [
            request("CamundaClusterClaim", "orders"),
            request("CamundaManagementClusterClaim", "orders"),
          ],
        }),
        slots: {
          "cluster.header.actions": (c) => {
            seen = c;
            return null;
          },
        },
      }),
    );
    expect(seen?.requests.map((r) => r.spec.claimRef)).toEqual([
      { kind: "CamundaClusterClaim", name: "orders" },
    ]);
  });

  it("gives the environment of the namespace and the storage of spec.storageRef", () => {
    let seen: ClusterSlotContext | undefined;
    const storage = (name: string) =>
      ({
        kind: "SecondaryStorageConfig",
        metadata: { namespace: "team-a", name },
        spec: { type: "elasticsearch" },
      }) as unknown as Resource<"SecondaryStorageConfig">;
    render(
      view({
        initial: data({
          cluster: cluster({ spec: { storageRef: "search" } }),
          storage: [storage("other"), storage("search")],
        }),
        slots: {
          "cluster.header.actions": (c) => {
            seen = c;
            return null;
          },
        },
      }),
    );
    expect(seen?.environment?.metadata?.name).toBe("prod");
    expect(seen?.storage?.metadata?.name).toBe("search");
    expect(seen?.writeMode).toBeUndefined();
    expect(seen?.openId).toBeUndefined();
  });
});

describe("Claim tab", () => {
  const editor: ActionSlots = { "cluster.claim.editor": <p>editor</p> };

  it("a filled claim editor slot shows in place of the read view", async () => {
    render(
      view({ slots: { "cluster.claim.editor": () => <p>Claim editor</p> } }),
    );
    await userEvent.click(screen.getByRole("tab", { name: "Claim" }));
    expect(screen.getByText("Claim editor")).toBeTruthy();
    expect(screen.queryByRole("region", { name: "Basics" })).toBeNull();
  });

  it("an empty claim editor slot keeps the read view", () => {
    window.history.replaceState(null, "", "?tab=claim");
    const { container } = render(view());
    expect(screen.getByRole("region", { name: "Basics" })).toBeTruthy();
    expect(
      container.querySelector('[data-action-slot="cluster.claim.editor"]'),
    ).not.toBeNull();
  });

  it("says that the claim is missing and keeps the editor slot", () => {
    window.history.replaceState(null, "", "?tab=claim");
    const { container } = render(
      view({ initial: data({ claims: [] }), slots: editor }),
    );
    expect(
      screen.getByText("The claim orders is not in this namespace."),
    ).toBeTruthy();
    expect(
      container.querySelector('[data-action-slot="cluster.claim.editor"]'),
    ).not.toBeNull();
  });

  it("says that an unmanaged cluster has no claim", () => {
    window.history.replaceState(null, "", "?tab=claim");
    const { container } = render(
      view({
        initial: data({ cluster: cluster({ labels: {} }) }),
        slots: editor,
      }),
    );
    expect(
      screen.getByText("This cluster was not made from a claim."),
    ).toBeTruthy();
    expect(
      container.querySelector('[data-action-slot="cluster.claim.editor"]'),
    ).toBeNull();
  });

  it("counts the ages of the claim conditions on with the clock", () => {
    vi.useFakeTimers({ now: Date.parse(now) });
    try {
      window.history.replaceState(null, "", "?tab=claim");
      const waiting = claim();
      (waiting as { status?: object }).status = {
        conditions: [condition("Ready", "False", "PrerequisiteNotMet")],
      };
      render(view({ initial: data({ claims: [waiting] }) }));
      const row = () =>
        within(screen.getByRole("region", { name: "Claim conditions" }))
          .getByRole("rowheader", { name: "Ready" })
          .closest("tr")!;
      expect(row().textContent).toContain("0s");
      act(() => {
        vi.advanceTimersByTime(65_000);
      });
      expect(row().textContent).toContain("1m");
    } finally {
      vi.useRealTimers();
    }
  });

  it("labels the sections of the claim with the PluginSections of the page", () => {
    window.history.replaceState(null, "", "?tab=claim");
    const withSection = claim();
    withSection.spec = {
      presetRef: "medium",
      plugins: [{ section: "access", registration: "gateway-access" }],
    };
    render(
      view({
        initial: data({
          claims: [withSection],
          sections: [
            {
              kind: "PluginSection",
              metadata: { name: "access" },
              spec: { label: "Access", appliesTo: ["CamundaClusterClaim"] },
            } as unknown as Resource<"PluginSection">,
          ],
          registrations: [
            {
              kind: "PluginRegistration",
              metadata: { name: "gateway-access" },
              spec: { label: "Gateway", deletionPolicies: ["Delete"] },
            } as unknown as Resource<"PluginRegistration">,
          ],
        }),
      }),
    );
    const access = screen.getByRole("region", { name: "Access: Gateway" });
    expect(within(access).getByText("Gateway")).toBeTruthy();
  });
});

describe("Component detail", () => {
  const components = () => screen.getByRole("region", { name: "Components" });
  const open = async (card: string) =>
    userEvent.click(
      within(
        within(components()).getByRole("article", { name: card }),
      ).getByRole("link", { name: card }),
    );
  const detail = (title: string) => screen.getByRole("region", { name: title });
  const podWatches = () =>
    live.calls
      .filter((c) => c.kind === "Pod")
      .map((c) => (c.opts as { labelSelector: string }).labelSelector);
  const notReady = (i: number) => {
    const pod = brokerPod(i);
    pod.status!.conditions = [{ type: "Ready", status: "False" }];
    return pod;
  };

  it("a degraded broker shows the right card, conditions, and partitions", async () => {
    live.objects.Pod = [brokerPod(0), brokerPod(1), notReady(2)];
    live.partitions = [
      {
        partition: 1,
        leader: "orders-zeebe-0",
        followers: ["orders-zeebe-1", "orders-zeebe-2"],
        health: "healthy",
      },
      {
        partition: 2,
        leader: "orders-zeebe-1",
        followers: ["orders-zeebe-0"],
        processorPhase: "PROCESSING",
        health: "unhealthy",
      },
      {
        partition: 3,
        leader: "orders-zeebe-2",
        followers: ["orders-zeebe-0", "orders-zeebe-1"],
        health: "healthy",
      },
    ];
    render(
      view({
        initial: data({
          cluster: withConditions(condition("ZeebeReady", "False", "Degraded")),
        }),
      }),
    );
    const card = within(components()).getByRole("article", { name: "Brokers" });
    const value = (label: string) =>
      within(card).getByText(label, { selector: "dt" }).nextElementSibling
        ?.textContent;
    expect(value("Pods ready")).toBe("2/3");
    expect(value("Partitions with a leader")).toBe("3/3");

    await open("Brokers");

    const brokers = detail("Brokers");
    expect(
      within(
        within(brokers).getByRole("list", { name: "Brokers conditions" }),
      ).getByText("ZeebeReady"),
    ).toBeTruthy();
    for (const i of [0, 1, 2]) {
      expect(
        within(brokers).getByRole("rowheader", {
          name: new RegExp(`^orders-zeebe-${i}`),
        }),
      ).toBeTruthy();
    }
    const partition2 = within(
      within(brokers).getByRole("region", { name: "Partitions" }),
    )
      .getByRole("rowheader", { name: "2" })
      .closest("tr")!;
    expect(partition2.textContent).toContain("orders-zeebe-0");
    expect(partition2.textContent).not.toContain("orders-zeebe-2");
    expect(window.location.search).toBe("?tab=overview&component=zeebe");
    expect(screen.queryByRole("region", { name: "Components" })).toBeNull();
  });

  it("opening a card moves the focus to the detail heading, and All components moves it back to the card", async () => {
    render(view());
    await open("Brokers");
    expect(document.activeElement).toBe(
      within(detail("Brokers")).getByRole("heading", { name: "Brokers" }),
    );
    await userEvent.click(screen.getByRole("link", { name: "All components" }));
    expect(document.activeElement).toBe(
      within(components()).getByRole("link", { name: "Brokers" }),
    );
  });

  it("closing the detail of an embedded app moves the focus back to its chip", async () => {
    render(view());
    await userEvent.click(
      within(components()).getByRole("link", { name: "Embedded: Operate" }),
    );
    await userEvent.click(screen.getByRole("link", { name: "All components" }));
    expect(document.activeElement).toBe(
      within(components()).getByRole("link", { name: "Embedded: Operate" }),
    );
  });

  it("ZeebeReady=False shows in the Brokers detail only", async () => {
    const optimize = nsObject(
      "CamundaOptimize",
      "orders-optimize",
      { clusterRef: { name: "orders" } },
      { conditions: [condition("Ready", "True", "Healthy")] },
      { creationTimestamp: now, uid: "uid-optimize" },
    );
    render(
      view({
        initial: data({
          cluster: withConditions(condition("ZeebeReady", "False", "Degraded")),
          optimizes: [optimize],
        }),
      }),
    );
    for (const name of ["Gateway", "Optimize"]) {
      await open(name);
      expect(within(detail(name)).queryByText("ZeebeReady")).toBeNull();
      await userEvent.click(
        screen.getByRole("link", { name: "All components" }),
      );
    }
  });

  it("AdminSecretReady=False shows in no component detail", async () => {
    render(
      view({
        initial: data({
          cluster: withConditions(
            condition("AdminSecretReady", "False", "MissingSecret"),
          ),
        }),
      }),
    );
    const links = within(components())
      .getAllByRole("link")
      .map((l) => l.textContent ?? "");
    expect(links).toEqual([
      "Brokers",
      "Gateway",
      "Embedded: Operate",
      "Embedded: Tasklist",
      "Embedded: Admin",
    ]);
    for (const link of links) {
      await userEvent.click(
        within(components()).getByRole("link", { name: link }),
      );
      const name = link.replace("Embedded: ", "");
      expect(within(detail(name)).queryByText("AdminSecretReady")).toBeNull();
      await userEvent.click(
        screen.getByRole("link", { name: "All components" }),
      );
    }
    expect(window.location.search).toBe("?tab=overview");
  });

  it("an embedded Operate opens with the gateway pods", async () => {
    render(
      view({
        initial: data({
          cluster: withConditions(
            condition("OperateReady", "True", "Disabled"),
          ),
        }),
      }),
    );
    await userEvent.click(
      within(components()).getByRole("link", { name: "Embedded: Operate" }),
    );
    const operate = detail("Operate");
    expect(within(operate).getByText("Embedded in Gateway")).toBeTruthy();
    expect(
      within(
        within(operate).getByRole("list", { name: "Operate conditions" }),
      ).getByText("OperateReady"),
    ).toBeTruthy();
    expect(podWatches()).toContain(
      "camunda.io/cluster=orders,camunda.io/component=gateway",
    );
  });

  it("the Optimize detail watches the pods of the Optimize that holds the cluster", async () => {
    const optimize = nsObject(
      "CamundaOptimize",
      "orders-optimize",
      { clusterRef: { name: "orders" } },
      { conditions: [condition("WebappReady", "False", "Degraded")] },
      { creationTimestamp: now, uid: "uid-optimize" },
    );
    render(view({ initial: data({ optimizes: [optimize] }) }));
    await open("Optimize");
    expect(podWatches()).toContain(
      "camunda.io/cluster=orders,camunda.io/component in (optimize-webapp,optimize-importer),camunda.io/optimize-uid=uid-optimize",
    );
    expect(
      within(
        within(detail("Optimize")).getByRole("list", {
          name: "Optimize conditions",
        }),
      ).getByText("WebappReady"),
    ).toBeTruthy();
  });

  it("the detail selector starts with the selector of the page", async () => {
    render(
      view({
        initial: data({ podSelector: "camunda.io/cluster=orders-3f9a1c2b" }),
      }),
    );
    await open("Brokers");
    expect(podWatches()).toContain(
      "camunda.io/cluster=orders-3f9a1c2b,camunda.io/component=zeebe",
    );
    expect(podWatches().some((s) => s.includes("=orders,"))).toBe(false);
  });

  it("the Settings tab mounts the settings of the component and the editor slot", async () => {
    render(
      view({
        initial: data({
          cluster: cluster({
            labels: { [CLAIM_LABEL]: "orders" },
            spec: { zeebe: { replicas: 3 } },
          }),
        }),
        slots: { "component.settings.editor": <p>Editor</p> },
      }),
    );
    await open("Brokers");
    await userEvent.click(
      within(detail("Brokers")).getByRole("tab", { name: "Settings" }),
    );
    expect(
      screen.getByRole("table", { name: "Brokers settings" }),
    ).toBeTruthy();
    expect(screen.getByText("Editor")).toBeTruthy();
  });

  it.each([
    ["a used card", "zeebe", "Brokers"],
    ["an embedded Operate", "operate", "Operate"],
  ])("the URL opens %s", (_, component, title) => {
    window.history.replaceState(
      null,
      "",
      `/fleet/clusters/team-a/orders?tab=overview&component=${component}`,
    );
    render(view());
    expect(detail(title)).toBeTruthy();
    expect(screen.queryByRole("region", { name: "Components" })).toBeNull();
  });

  it("an unknown component in the URL shows the cards", () => {
    window.history.replaceState(
      null,
      "",
      "/fleet/clusters/team-a/orders?tab=overview&component=keycloak",
    );
    render(view());
    expect(components()).toBeTruthy();
  });
});

describe("Section components", () => {
  const components = () => screen.getByRole("region", { name: "Components" });
  const cardNames = () =>
    within(components())
      .getAllByRole("article")
      .map((a) => a.getAttribute("aria-label"));

  const pluginSection = (name: string, label: string, order: number) =>
    ({
      kind: "PluginSection",
      metadata: { name },
      spec: { label, order, selection: "One" },
    }) as unknown as Resource<"PluginSection">;

  const sectionClaim = {
    ...claim(),
    spec: {
      plugins: [
        { section: "access", registration: "gateway-access" },
        { section: "backup-storage", registration: "aws-s3" },
      ],
    },
    status: {
      requests: [
        {
          section: "backup-storage",
          registration: "aws-s3",
          name: "orders-backup-storage",
        },
      ],
    },
  } as unknown as Resource<"CamundaClusterClaim">;

  const backupRequest = nsObject(
    "PluginRequest",
    "orders-backup-storage",
    {
      registration: "aws-s3",
      section: "backup-storage",
      claimRef: { kind: "CamundaClusterClaim", name: "orders" },
    },
    {
      summary: [{ label: "Bucket", value: "orders-backups" }],
      resources: [
        { kind: "S3Bucket", name: "orders-backups", onOrphan: "Kept" },
      ],
    },
    { labels: { [CLAIM_LABEL]: "orders", [SECTION_LABEL]: "backup-storage" } },
  );

  const withSections = (over: Partial<ClusterPageData> = {}) =>
    data({
      claims: [sectionClaim],
      sections: [
        pluginSection("access", "Access", 5),
        pluginSection("backup-storage", "Backup storage", 2),
      ],
      requests: [backupRequest],
      ...over,
    });

  it("shows a card for each selection of the claim after the process cards", () => {
    render(view({ initial: withSections() }));
    expect(cardNames().slice(-2)).toEqual(["Backup storage", "Access"]);
    const card = within(components()).getByRole("article", {
      name: "Backup storage",
    });
    expect(
      within(card).getByText("Bucket", { selector: "dt" }).nextElementSibling
        ?.textContent,
    ).toBe("orders-backups");
  });

  it("says that the request could not be read, never that it is not made yet, when the requests watch fails", async () => {
    live.errors.PluginRequest = "forbidden";
    render(view({ initial: withSections({ requests: [] }) }));
    const card = within(components()).getByRole("article", {
      name: "Backup storage",
    });
    expect(
      within(card).getByText("Request", { selector: "dt" }).nextElementSibling
        ?.textContent,
    ).toBe("Warning: Could not read");
    await userEvent.click(
      within(card).getByRole("link", { name: "Backup storage" }),
    );
    expect(
      within(screen.getByRole("region", { name: "Backup storage" })).getByText(
        "Could not read PluginRequest: forbidden",
      ),
    ).toBeTruthy();
    expect(
      screen.queryByText(
        "The claim controller has not made the request of this section yet.",
      ),
    ).toBeNull();
  });

  it("shows no section card for a cluster without a claim", () => {
    render(
      view({
        initial: withSections({ cluster: cluster({ labels: {} }) }),
      }),
    );
    expect(cardNames()).not.toContain("Backup storage");
    expect(cardNames()).toContain("Brokers");
  });

  it("opens a section from an encoded URL", () => {
    window.history.replaceState(
      null,
      "",
      "?tab=overview&component=section%3Aaccess",
    );
    render(view({ initial: withSections() }));
    expect(screen.getByRole("heading", { name: "Access" })).toBeTruthy();
    expect(
      screen.getByText(
        "The claim controller has not made the request of this section yet.",
      ),
    ).toBeTruthy();
    expect(screen.getByRole("link", { name: "All components" })).toBeTruthy();
  });

  it("opens a section card in place and writes its encoded id to the URL", async () => {
    render(view({ initial: withSections() }));
    await userEvent.click(
      within(
        within(components()).getByRole("article", { name: "Backup storage" }),
      ).getByRole("link", { name: "Backup storage" }),
    );
    expect(window.location.search).toBe(
      "?tab=overview&component=section%3Abackup-storage",
    );
    expect(
      screen.getByRole("heading", { name: "Backup storage" }),
    ).toBeTruthy();
    expect(screen.queryByRole("region", { name: "Components" })).toBeNull();
  });
});

describe("The detail of a section", () => {
  const open = (component: string) =>
    window.history.replaceState(
      null,
      "",
      `/fleet/clusters/team-a/orders?tab=overview&component=${encodeURIComponent(component)}`,
    );
  const detail = (name: string) => screen.getByRole("region", { name });
  const withClaim = (over: Partial<ClusterPageData> = {}) =>
    data({
      claims: [fixtureClaim()],
      sections: coreSections,
      registrations: fixtureRegistrations,
      requests: fixtureRequests,
      ...over,
    });
  // renderOutOfDate renders ui with every watch answered, then again while
  // the watches of kinds have lost their connection.
  const renderOutOfDate = (ui: ReactElement, kinds: string[]) => {
    const page = render(ui);
    live.stale = kinds;
    page.rerender(cloneElement(ui));
  };

  it("shows the Overview blocks of the request of an open section", () => {
    open("section:backup-storage");
    render(view({ initial: withClaim() }));
    const backup = detail("Backup storage");
    expect(
      within(within(backup).getByRole("region", { name: "Summary" })).getByText(
        "orders-backups",
      ),
    ).toBeTruthy();
    expect(
      within(backup).getByRole("region", { name: "Resources" }),
    ).toBeTruthy();
  });

  it("shows the grants of the request of an open section", () => {
    open("section:backup-storage");
    render(
      view({
        initial: withClaim({
          grants: [fixtureGrant("orders-read", "backup-storage")],
        }),
      }),
    );
    expect(
      within(detail("Grants")).getByLabelText("Statement of orders-read"),
    ).toBeTruthy();
  });

  it("shows an existing secondary storage as connected with the Ready of its contract", () => {
    open("section:secondary-storage");
    render(
      view({
        initial: withClaim({ storage: [fixtureContract("orders-storage")] }),
      }),
    );
    expect(
      within(detail("Connected")).getByRole("row", {
        name: "SecondaryStorageConfig orders-storage Ready",
      }),
    ).toBeTruthy();
  });

  it("shows the ElasticsearchCluster of a built-in secondary storage", () => {
    const selections = fixtureSelections.map((s) =>
      s.section === "secondary-storage"
        ? { section: s.section, registration: "elasticsearch" }
        : s,
    );
    const request = fixtureRequest("secondary-storage", {
      resources: [
        {
          kind: "ElasticsearchCluster",
          name: "orders-es",
          onOrphan: "Deleted",
        },
      ],
    });
    request.spec.registration = "elasticsearch";
    const es = {
      kind: "ElasticsearchCluster",
      metadata: { namespace: "team-a", name: "orders-es" },
      spec: {},
      status: { version: "8.19.2", snapshotRepository: "camunda" },
    } as unknown as Resource<"ElasticsearchCluster">;
    open("section:secondary-storage");
    render(
      view({
        initial: withClaim({
          claims: [fixtureClaim(selections)],
          requests: fixtureRequests.map((r) =>
            r.spec.section === "secondary-storage" ? request : r,
          ),
          elasticsearchClusters: [es],
        }),
      }),
    );
    const box = detail("ElasticsearchCluster");
    expect(within(box).getByText("8.19.2")).toBeTruthy();
    expect(within(box).getByText("camunda")).toBeTruthy();
  });

  it("shows the parameters of a section in its Settings tab", async () => {
    open("section:secondary-storage");
    render(view({ initial: withClaim() }));
    await userEvent.click(screen.getByRole("tab", { name: "Settings" }));
    const table = screen.getByRole("table", {
      name: "Secondary storage: Existing secondary storage settings",
    });
    expect(
      within(table)
        .getAllByRole("rowheader")
        .map((h) => h.textContent),
    ).toEqual(["name"]);
  });

  it("names the Grant kind and shows no grant state while the Grants are not read", () => {
    open("section:cloud-identity");
    renderOutOfDate(view({ initial: withClaim() }), ["Grant"]);
    expect(
      within(detail("Grants")).getByText(
        "Not known: the fleet manager has no current list of Grant.",
      ),
    ).toBeTruthy();
  });

  it("shows no fate while the claims are not read", () => {
    live.errors.CamundaClusterClaim = "forbidden";
    open("section:backup-storage");
    render(view({ initial: withClaim() }));
    const resources = detail("Resources");
    expect(within(resources).queryByText("Kept")).toBeNull();
    expect(within(resources).getAllByText("Not known").length).toBe(2);
  });

  it("shows nothing of a request that is out of date, and names its kind", () => {
    open("section:backup-storage");
    renderOutOfDate(view({ initial: withClaim() }), ["PluginRequest"]);
    const backup = detail("Backup storage");
    expect(within(backup).getByText("Ready: Not known")).toBeTruthy();
    expect(
      within(backup).getAllByText(
        "Not known: the fleet manager has no current list of PluginRequest.",
      ).length,
    ).toBeGreaterThan(0);
    expect(
      within(backup).queryByRole("region", { name: "Summary" }),
    ).toBeNull();
    expect(within(backup).queryByText("Healthy")).toBeNull();
  });

  it("shows no state and no headline on a section card while the requests are out of date", () => {
    renderOutOfDate(view({ initial: withClaim() }), ["PluginRequest"]);
    const card = within(
      screen.getByRole("region", { name: "Components" }),
    ).getByRole("article", { name: "Backup storage" });
    expect(
      within(card).getByText("Request", { selector: "dt" }).nextElementSibling
        ?.textContent,
    ).toBe("Warning: Not known");
    expect(within(card).queryByText("orders-backups")).toBeNull();
  });

  it("shows the read of the server on a section card until the watch of the requests answers", () => {
    live.stale = ["PluginRequest"];
    render(view({ initial: withClaim() }));
    const card = within(
      screen.getByRole("region", { name: "Components" }),
    ).getByRole("article", { name: "Backup storage" });
    expect(within(card).getByText("orders-backups")).toBeTruthy();
    expect(within(card).queryByText(/Not known/)).toBeNull();
  });

  it("says that the first read of the requests failed until their watch answers", () => {
    live.stale = ["PluginRequest"];
    const ui = view({
      initial: withClaim({
        requests: [],
        readErrors: [{ kind: "PluginRequest", message: "forbidden" }],
      }),
    });
    const page = render(ui);
    const card = () =>
      within(screen.getByRole("region", { name: "Components" })).getByRole(
        "article",
        { name: "Backup storage" },
      );
    expect(
      screen.getByText("Could not read PluginRequest: forbidden"),
    ).toBeTruthy();
    expect(
      within(card()).getByText("Request", { selector: "dt" }).nextElementSibling
        ?.textContent,
    ).toBe("Warning: Could not read");

    live.stale = [];
    page.rerender(cloneElement(ui));
    expect(
      screen.queryByText("Could not read PluginRequest: forbidden"),
    ).toBeNull();
  });

  it("shows no parameter in the Settings tab while the claim is out of date, and names its kind", async () => {
    open("section:secondary-storage");
    renderOutOfDate(view({ initial: withClaim() }), ["CamundaClusterClaim"]);
    await userEvent.click(screen.getByRole("tab", { name: "Settings" }));
    expect(
      screen.getByText(
        "Not known: the fleet manager has no current list of CamundaClusterClaim.",
      ),
    ).toBeTruthy();
    expect(screen.queryByRole("table", { name: /settings$/ })).toBeNull();
  });

  it("hides the state and headline of a section card once the requests watch is overdue without an answer", () => {
    live.stale = ["PluginRequest"];
    live.overdue = ["PluginRequest"];
    render(view({ initial: withClaim() }));
    const card = within(
      screen.getByRole("region", { name: "Components" }),
    ).getByRole("article", { name: "Backup storage" });
    expect(
      within(card).getByText("Request", { selector: "dt" }).nextElementSibling
        ?.textContent,
    ).toBe("Warning: Not known");
  });

  it("offers no editor in the Settings tab once the claims watch is overdue without an answer", async () => {
    live.stale = ["CamundaClusterClaim"];
    live.overdue = ["CamundaClusterClaim"];
    open("section:secondary-storage");
    render(
      view({
        initial: withClaim(),
        slots: { "component.settings.editor": <p>Editor</p> },
      }),
    );
    await userEvent.click(screen.getByRole("tab", { name: "Settings" }));
    expect(screen.queryByText("Editor")).toBeNull();
  });

  it("names the backup kind, never that there are no backups, while a backup list is out of date", async () => {
    renderOutOfDate(view({ initial: withClaim() }), ["LogicalBackupRDBMS"]);
    await userEvent.click(screen.getByRole("tab", { name: "Backups" }));
    const backups = screen.getByRole("region", { name: "Backups" });
    expect(
      within(backups).getByText(
        "Not known: the fleet manager has no current list of LogicalBackupRDBMS.",
      ),
    ).toBeTruthy();
    expect(within(backups).queryByText("No backups yet.")).toBeNull();
  });

  it("shows no claim content and no claim editor while the claim that the page holds is out of date", async () => {
    renderOutOfDate(
      view({
        initial: withClaim(),
        slots: { "cluster.claim.editor": <p>Claim editor</p> },
      }),
      ["CamundaClusterClaim"],
    );
    expect(
      screen.getByText(
        "Not known: the fleet manager has no current list of CamundaClusterClaim.",
      ),
    ).toBeTruthy();
    const card = within(
      screen.getByRole("region", { name: "Components" }),
    ).getByRole("article", { name: "Backup storage" });
    expect(within(card).queryByText("orders-backups")).toBeNull();
    await userEvent.click(screen.getByRole("tab", { name: "Claim" }));
    expect(
      screen.getByText(
        "Not known: the fleet manager has no current list of CamundaClusterClaim.",
      ),
    ).toBeTruthy();
    expect(screen.queryByText("Claim editor")).toBeNull();
  });

  it("offers no editor in the Settings tab while the claim is out of date", async () => {
    open("section:secondary-storage");
    renderOutOfDate(
      view({
        initial: withClaim(),
        slots: { "component.settings.editor": <p>Editor</p> },
      }),
      ["CamundaClusterClaim"],
    );
    await userEvent.click(screen.getByRole("tab", { name: "Settings" }));
    expect(screen.queryByText("Editor")).toBeNull();
  });

  it("shows the Not known line of the Settings tab, not No settings, when the registrations could not be read", async () => {
    live.errors.PluginRegistration = "forbidden";
    open("section:secondary-storage");
    render(view({ initial: withClaim({ registrations: [] }) }));
    await userEvent.click(screen.getByRole("tab", { name: "Settings" }));
    expect(screen.queryByText("No settings.")).toBeNull();
    expect(
      screen.getByText(
        "Not known: the fleet manager has no current list of PluginRegistration.",
      ),
    ).toBeTruthy();
  });
});

describe("Before the cluster exists", () => {
  const condition = (type: string, reason: string) => ({
    type,
    status: "False",
    reason,
    message: "",
    observedGeneration: 1,
    lastTransitionTime: now,
  });
  const waitingClaim = {
    ...claim(),
    metadata: { ...claim().metadata, generation: 1 },
    spec: {
      plugins: [
        {
          section: "secondary-storage",
          registration: "existing-secondary-storage",
        },
      ],
    },
    status: {
      requests: [
        {
          section: "secondary-storage",
          registration: "existing-secondary-storage",
          name: "orders-secondary-storage",
        },
      ],
      conditions: [
        condition("Ready", "PrerequisiteNotMet"),
        condition("RequestsReady", "OperationPending"),
      ],
    },
  } as unknown as Resource<"CamundaClusterClaim">;
  const section = {
    kind: "PluginSection",
    metadata: { name: "secondary-storage" },
    spec: {
      label: "Secondary storage",
      selection: "One",
      slot: "secondaryStorage",
      appliesTo: ["CamundaClusterClaim"],
    },
  } as unknown as Resource<"PluginSection">;
  const request = {
    kind: "PluginRequest",
    metadata: {
      namespace: "team-a",
      name: "orders-secondary-storage",
      generation: 1,
      labels: { [CLAIM_LABEL]: "orders", [SECTION_LABEL]: "secondary-storage" },
    },
    spec: {
      claimRef: { kind: "CamundaClusterClaim", name: "orders" },
      section: "secondary-storage",
      registration: "existing-secondary-storage",
    },
    status: { conditions: [] },
  } as unknown as Resource<"PluginRequest">;
  const waiting = data({
    cluster: undefined,
    claims: [waitingClaim],
    sections: [section],
    requests: [request],
    pods: [],
  });

  it("shows the claim, its section cards, its conditions, and the requests it waits for", () => {
    render(view({ initial: waiting }));
    expect(
      screen.getByRole("heading", { level: 1, name: "orders" }),
    ).toBeTruthy();
    const block = screen.getByRole("region", {
      name: "The cluster does not exist yet",
    });
    expect(
      within(block)
        .getAllByRole("listitem")
        .map((li) => li.textContent),
    ).toEqual(["Secondary storage: orders-secondary-storage"]);
    expect(screen.queryByRole("region", { name: "Topology" })).toBeNull();
    expect(
      within(screen.getByRole("region", { name: "Components" }))
        .getAllByRole("article")
        .map((a) => a.getAttribute("aria-label")),
    ).toEqual(["Secondary storage"]);
    expect(
      within(screen.getByRole("region", { name: "Conditions" }))
        .getAllByRole("rowheader")
        .map((h) => h.textContent),
    ).toEqual(["Claim", "RequestsReady"]);
    expect(screen.getAllByRole("tab").map((t) => t.textContent)).toEqual([
      "Overview",
      "Backups",
      "Endpoints",
      "Claim",
    ]);
  });

  it("watches the requests of the claim by the page name", () => {
    render(view({ initial: waiting }));
    expect(live.calls.find((c) => c.kind === "PluginRequest")?.opts).toEqual({
      namespace: "team-a",
      labelSelector: `${CLAIM_LABEL}=orders`,
    });
  });

  it("the Backups and Endpoints tabs say that the cluster does not exist yet", async () => {
    const { container } = render(view({ initial: waiting, slots: filled }));
    await userEvent.click(screen.getByRole("tab", { name: "Backups" }));
    expect(
      screen.getByText("The cluster does not exist yet, so it has no backups."),
    ).toBeTruthy();
    expect(
      container.querySelector('[data-action-slot="cluster.backups.actions"]'),
    ).toBeNull();
    await userEvent.click(screen.getByRole("tab", { name: "Endpoints" }));
    expect(
      screen.getByText(
        "The cluster does not exist yet, so it has no endpoints.",
      ),
    ).toBeTruthy();
  });

  it("the Claim tab shows the claim and mounts the claim editor slot", () => {
    window.history.replaceState(null, "", "?tab=claim");
    const { container } = render(view({ initial: waiting }));
    expect(screen.getByRole("region", { name: "Basics" })).toBeTruthy();
    expect(
      container.querySelector('[data-action-slot="cluster.claim.editor"]'),
    ).not.toBeNull();
  });

  it("gives the slots the cluster that the claim would make, marked as drafted", () => {
    let seen: ClusterSlotContext | undefined;
    render(
      view({
        initial: waiting,
        slots: {
          "cluster.header.actions": (c) => {
            seen = c;
            return null;
          },
        },
      }),
    );
    expect(seen?.drafted).toBe(true);
    expect(seen?.cluster.metadata).toEqual({
      name: "orders",
      namespace: "team-a",
      labels: { [CLAIM_LABEL]: "orders" },
    });
    expect(seen?.claim?.metadata?.name).toBe("orders");
    expect(seen?.backups).toEqual([]);
    expect(seen?.restores).toEqual([]);
  });

  it("gives the slots no drafted mark while the cluster exists", () => {
    let seen: ClusterSlotContext | undefined;
    render(
      view({
        slots: {
          "cluster.header.actions": (c) => {
            seen = c;
            return null;
          },
        },
      }),
    );
    expect(seen?.drafted).toBeUndefined();
  });

  it("shows the cluster once its watch gives it, without a new page", () => {
    const { rerender } = render(view({ initial: waiting }));
    expect(
      screen.getByRole("region", { name: "The cluster does not exist yet" }),
    ).toBeTruthy();
    live.objects.CamundaCluster = [cluster()];
    rerender(view({ initial: waiting }));
    expect(screen.getByRole("region", { name: "Topology" })).toBeTruthy();
    expect(
      screen.queryByRole("region", { name: "The cluster does not exist yet" }),
    ).toBeNull();
  });

  it("a failed read of the cluster is an error, never a claim page that says the cluster does not exist yet", () => {
    live.errors.CamundaCluster = "forbidden";
    render(view({ initial: waiting }));
    expect(screen.getByRole("alert").textContent).toBe(
      "Could not read CamundaCluster: forbidden",
    );
    expect(
      screen.getByText(
        "Not known: the fleet manager has no current list of CamundaCluster.",
      ),
    ).toBeTruthy();
    expect(screen.queryByText("The cluster does not exist yet")).toBeNull();
    expect(screen.queryByText("This cluster no longer exists.")).toBeNull();
  });

  it("names the cluster kind, never that the cluster does not exist yet, while the watch of the cluster is not current", () => {
    live.stale = ["CamundaCluster"];
    live.overdue = ["CamundaCluster"];
    render(view({ initial: waiting }));
    expect(
      screen.getByText(
        "Not known: the fleet manager has no current list of CamundaCluster.",
      ),
    ).toBeTruthy();
    expect(screen.queryByText("The cluster does not exist yet")).toBeNull();
  });

  it("the Backups and Endpoints tabs name the cluster kind, never that the cluster does not exist yet, while the cluster read is not current", async () => {
    live.stale = ["CamundaCluster"];
    live.overdue = ["CamundaCluster"];
    render(view({ initial: waiting }));
    const notKnown =
      "Not known: the fleet manager has no current list of CamundaCluster.";
    await userEvent.click(screen.getByRole("tab", { name: "Backups" }));
    expect(screen.getByText(notKnown)).toBeTruthy();
    expect(
      screen.queryByText(
        "The cluster does not exist yet, so it has no backups.",
      ),
    ).toBeNull();
    await userEvent.click(screen.getByRole("tab", { name: "Endpoints" }));
    expect(screen.getByText(notKnown)).toBeTruthy();
    expect(
      screen.queryByText(
        "The cluster does not exist yet, so it has no endpoints.",
      ),
    ).toBeNull();
  });

  it("shows the cluster of the server read while the watch has only an older answer that has none", () => {
    live.objects.CamundaCluster = [];
    live.stale = ["CamundaCluster"];
    render(view({ initial: data() }));
    expect(screen.getByRole("region", { name: "Topology" })).toBeTruthy();
    expect(
      screen.queryByRole("region", { name: "The cluster does not exist yet" }),
    ).toBeNull();
  });

  it("does not show a deleted cluster again when its watch loses its connection later", () => {
    const loaded = { ...waiting, cluster: cluster() };
    live.objects.CamundaCluster = [cluster()];
    const { rerender } = render(view({ initial: loaded }));
    live.objects.CamundaCluster = [];
    rerender(view({ initial: loaded }));
    expect(
      screen.getByRole("region", { name: "The cluster does not exist yet" }),
    ).toBeTruthy();
    live.stale = ["CamundaCluster"];
    rerender(view({ initial: loaded }));
    expect(screen.queryByRole("region", { name: "Topology" })).toBeNull();
    expect(
      within(screen.getByRole("region", { name: "Cluster" })).getByText(
        "Not known: the fleet manager has no current list of CamundaCluster.",
      ),
    ).toBeTruthy();
  });

  it("does not show a cluster of an earlier visit that the server read no longer has, before the watch answers", () => {
    live.objects.CamundaCluster = [cluster()];
    live.stale = ["CamundaCluster"];
    render(view({ initial: waiting }));
    expect(screen.queryByRole("region", { name: "Topology" })).toBeNull();
    expect(
      screen.getByRole("region", { name: "The cluster does not exist yet" }),
    ).toBeTruthy();
  });

  it("a failed first read of the cluster is an error until its watch answers", () => {
    live.stale = ["CamundaCluster"];
    render(
      view({
        initial: {
          ...waiting,
          readErrors: [{ kind: "CamundaCluster", message: "forbidden" }],
        },
      }),
    );
    expect(screen.getByRole("alert").textContent).toBe(
      "Could not read CamundaCluster: forbidden",
    );
    expect(screen.queryByText("The cluster does not exist yet")).toBeNull();
  });

  it("names the claim kind, never that the cluster is gone, while the claims are not read and no cluster exists", () => {
    live.errors.CamundaClusterClaim = "forbidden";
    render(view({ initial: data({ cluster: undefined, claims: [] }) }));
    expect(
      screen.getByText(
        "Not known: the fleet manager has no current list of CamundaClusterClaim.",
      ),
    ).toBeTruthy();
    expect(screen.queryByText("This cluster no longer exists.")).toBeNull();
  });

  it("names the claim kind, never that the cluster is gone, while the claims are stale and no cluster exists", () => {
    live.stale = ["CamundaClusterClaim"];
    live.overdue = ["CamundaClusterClaim"];
    render(view({ initial: data({ cluster: undefined, claims: [] }) }));
    expect(
      screen.getByText(
        "Not known: the fleet manager has no current list of CamundaClusterClaim.",
      ),
    ).toBeTruthy();
    expect(screen.queryByText("This cluster no longer exists.")).toBeNull();
  });

  it("says once that the claim is not current while no cluster exists", () => {
    live.stale = ["CamundaClusterClaim"];
    live.overdue = ["CamundaClusterClaim"];
    render(view({ initial: waiting }));
    expect(
      screen.getAllByText(
        "Not known: the fleet manager has no current list of CamundaClusterClaim.",
      ),
    ).toHaveLength(1);
  });

  it("says that the claim is being deleted after its cluster went", () => {
    const deleting = {
      ...waitingClaim,
      metadata: { ...waitingClaim.metadata, deletionTimestamp: now },
    } as Resource<"CamundaClusterClaim">;
    render(view({ initial: data({ cluster: undefined, claims: [deleting] }) }));
    expect(
      screen.getByRole("region", { name: "The cluster no longer exists" }),
    ).toBeTruthy();
    expect(screen.getByText("The claim is being deleted.")).toBeTruthy();
    expect(screen.queryByText("The cluster does not exist yet")).toBeNull();
  });

  it("a process in the URL shows the claim state while no cluster exists", () => {
    window.history.replaceState(null, "", "?component=zeebe");
    render(view({ initial: waiting }));
    expect(
      screen.getByRole("region", { name: "The cluster does not exist yet" }),
    ).toBeTruthy();
  });

  it("names the request kind in the block, never a list, while the requests are not read", () => {
    live.errors.PluginRequest = "forbidden";
    render(view({ initial: waiting }));
    const block = screen.getByRole("region", {
      name: "The cluster does not exist yet",
    });
    expect(within(block).queryByRole("list")).toBeNull();
    expect(
      within(block).getByText(
        "Not known: the fleet manager has no current list of PluginRequest.",
      ),
    ).toBeTruthy();
  });
});

describe("A pending claim edit while the cluster changes", () => {
  const presetRef: Step[] = [{ field: "spec" }, { field: "presetRef" }];
  // PresetProbe is an editor of the Claim tab that changes the claim only
  // through useClusterEdit.
  function PresetProbe() {
    const edit = useClusterEdit();
    return (
      <input
        aria-label="Preset"
        value={String(
          (edit.session && valueAt(claimAfter(edit.session), presetRef)) ?? "",
        )}
        onChange={(e) => edit.set(presetRef, e.target.value)}
      />
    );
  }
  const actions = { preview: vi.fn(), apply: vi.fn() };
  const client = new QueryClient();
  const page = (initial: ClusterPageData) => (
    <QueryClientProvider client={client}>
      <GitPushesProvider>
        <ClusterView
          initial={initial}
          now={now}
          writeMode="direct"
          claimActions={actions}
          slots={{ "cluster.claim.editor": <PresetProbe /> }}
        />
      </GitPushesProvider>
    </QueryClientProvider>
  );
  const editable = () =>
    ({
      ...claim(),
      metadata: { ...claim().metadata, resourceVersion: "41" },
    }) as Resource<"CamundaClusterClaim">;
  const claimOnly = () =>
    data({ cluster: undefined, pods: [], claims: [editable()] });
  const preset = () =>
    screen.getByRole<HTMLInputElement>("textbox", { name: "Preset" });
  const box = (name: string) => screen.queryByRole("region", { name });

  it("keeps the edit and its session while the cluster watch is not current, and says that the cluster is not known", async () => {
    window.history.replaceState(null, "", "?tab=claim");
    const { rerender } = render(page(claimOnly()));
    await userEvent.type(preset(), "large");
    expect(preset().value).toBe("large");

    live.stale = ["CamundaCluster"];
    live.overdue = ["CamundaCluster"];
    rerender(page(claimOnly()));
    expect(preset().value).toBe("large");
    await userEvent.click(screen.getByRole("tab", { name: "Overview" }));
    expect(
      within(box("Cluster")!).getByText(
        "Not known: the fleet manager has no current list of CamundaCluster.",
      ),
    ).toBeTruthy();
    expect(box("The cluster does not exist yet")).toBeNull();
    await userEvent.click(screen.getByRole("tab", { name: "Claim" }));
    expect(preset().value).toBe("large");
  });

  it("says that the cluster does not exist yet again once its watch is current, with the edit kept", async () => {
    window.history.replaceState(null, "", "?tab=claim");
    live.stale = ["CamundaCluster"];
    live.overdue = ["CamundaCluster"];
    const { rerender } = render(page(claimOnly()));
    await userEvent.type(preset(), "large");

    live.stale = [];
    live.overdue = [];
    rerender(page(claimOnly()));
    expect(preset().value).toBe("large");
    await userEvent.click(screen.getByRole("tab", { name: "Overview" }));
    expect(box("The cluster does not exist yet")).toBeTruthy();
    expect(box("Cluster")).toBeNull();
  });

  it("shows the claim again, with the edit kept, when its cluster is deleted while the page is open", async () => {
    window.history.replaceState(null, "", "?tab=claim");
    live.objects.CamundaCluster = [cluster()];
    const withCluster = data({ claims: [editable()] });
    const { rerender } = render(page(withCluster));
    await userEvent.type(preset(), "large");

    live.objects.CamundaCluster = [];
    rerender(page(withCluster));
    expect(preset().value).toBe("large");
    expect(screen.queryByText("This cluster no longer exists.")).toBeNull();
    await userEvent.click(screen.getByRole("tab", { name: "Overview" }));
    expect(box("The cluster does not exist yet")).toBeTruthy();
    expect(box("Topology")).toBeNull();
  });
});
