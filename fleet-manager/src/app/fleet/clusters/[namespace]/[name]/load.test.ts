import { readFileSync } from "node:fs";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { loadClusterPage } from "./load";

const k8s = vi.hoisted(() => ({
  cluster: undefined as unknown,
  lists: [] as { kind: string; opts: unknown }[],
  claims: [] as unknown[],
  failing: "",
}));
vi.mock("@/lib/k8s/read", () => ({
  get: async (kind: string, _namespace: string, name: string) => {
    if (kind !== "CamundaCluster") return undefined;
    if (kind === k8s.failing) throw new Error(`${kind} is forbidden`);
    return (k8s.cluster as { metadata: { name: string } } | undefined)?.metadata
      .name === name
      ? k8s.cluster
      : undefined;
  },
  list: async (kind: string, opts: unknown) => {
    k8s.lists.push({ kind, opts });
    if (kind === k8s.failing) throw new Error(`${kind} is forbidden`);
    return kind === "CamundaClusterClaim" ? k8s.claims : [];
  },
}));

beforeEach(() => {
  k8s.lists = [];
  k8s.claims = [];
  k8s.failing = "";
});

const named = (name: string) => {
  k8s.cluster = {
    kind: "CamundaCluster",
    metadata: { namespace: "team-a", name },
  };
};
const listOf = (kind: string) => k8s.lists.find((l) => l.kind === kind)?.opts;
const claimNamed = (name: string, spec: object = {}) => ({
  kind: "CamundaClusterClaim",
  metadata: { namespace: "team-a", name },
  spec,
});

describe("loadClusterPage", () => {
  it("returns undefined when neither the cluster nor a claim of the name exists", async () => {
    named("orders");
    k8s.claims = [claimNamed("orders")];
    expect(await loadClusterPage("team-a", "payments")).toBeUndefined();
  });

  it("returns the page of a claim that has no cluster yet", async () => {
    k8s.cluster = undefined;
    k8s.claims = [claimNamed("orders")];
    const data = await loadClusterPage("team-a", "orders");
    expect(data?.ref).toEqual({ namespace: "team-a", name: "orders" });
    expect(data?.cluster).toBeUndefined();
    expect(data?.claims.map((c) => c.metadata?.name)).toEqual(["orders"]);
    expect(listOf("PluginRequest")).toEqual({
      namespace: "team-a",
      labelSelector: "fleet.camunda.io/claim=orders",
    });
    expect(data?.readErrors).toEqual([]);
  });

  it("gives the docs of the fields of the cluster that the claim would make", async () => {
    k8s.cluster = undefined;
    k8s.claims = [
      claimNamed("orders", { cluster: { zeebe: { replicas: 3 } } }),
    ];
    const data = await loadClusterPage("team-a", "orders");
    expect(
      data?.fieldDocs["CamundaCluster:spec.zeebe.replicas"]?.description,
    ).toBeTruthy();
  });

  it("shows a failed read of the cluster as an error, never as a missing page", async () => {
    k8s.cluster = undefined;
    k8s.failing = "CamundaCluster";
    const data = await loadClusterPage("team-a", "orders");
    expect(data).toBeDefined();
    expect(data?.cluster).toBeUndefined();
    expect(data?.readErrors).toContainEqual({
      kind: "CamundaCluster",
      message: "CamundaCluster is forbidden",
    });
  });

  it("gives no docs of the cluster that a claim would make while the cluster read failed", async () => {
    k8s.cluster = undefined;
    k8s.failing = "CamundaCluster";
    k8s.claims = [
      claimNamed("orders", { cluster: { zeebe: { replicas: 3 } } }),
    ];
    const data = await loadClusterPage("team-a", "orders");
    expect(data?.fieldDocs).toEqual({});
  });

  it("shows a failed read of the claims as an error, never as a missing page", async () => {
    k8s.cluster = undefined;
    k8s.failing = "CamundaClusterClaim";
    const data = await loadClusterPage("team-a", "orders");
    expect(data).toBeDefined();
    expect(data?.readErrors).toContainEqual({
      kind: "CamundaClusterClaim",
      message: "CamundaClusterClaim is forbidden",
    });
  });

  it("lists the pods of the cluster by its label", async () => {
    named("orders");
    const data = await loadClusterPage("team-a", "orders");
    expect(data?.podSelector).toBe("camunda.io/cluster=orders");
    expect(listOf("Pod")).toEqual({
      namespace: "team-a",
      labelSelector: "camunda.io/cluster=orders",
    });
  });

  it("lists the PluginRequests once, by the claim label", async () => {
    named("orders");
    await loadClusterPage("team-a", "orders");
    const lists = k8s.lists.filter((l) => l.kind === "PluginRequest");
    expect(lists).toHaveLength(1);
    expect(lists[0].opts).toMatchObject({
      labelSelector: expect.stringMatching(/^fleet\.camunda\.io\/claim=/),
    });
  });

  it("lists the requests of the claim of the cluster by the claim label", async () => {
    k8s.cluster = {
      kind: "CamundaCluster",
      metadata: {
        namespace: "team-a",
        name: "orders",
        labels: { "fleet.camunda.io/claim": "orders-claim" },
      },
    };
    await loadClusterPage("team-a", "orders");
    expect(
      k8s.lists.filter((l) => l.kind === "PluginRequest").map((l) => l.opts),
    ).toContainEqual({
      namespace: "team-a",
      labelSelector: "fleet.camunda.io/claim=orders-claim",
    });
  });

  it("lists the namespaced kinds in the namespace and the cluster-scoped kinds everywhere", async () => {
    named("orders");
    await loadClusterPage("team-a", "orders");
    for (const kind of [
      "CamundaClusterClaim",
      "SecondaryStorageConfig",
      "LogicalBackupRDBMS",
      "PointInTimeRestore",
      "ObjectStorageConfig",
      "DatabaseConfig",
      "DatabaseServerConfig",
      "DatabaseServer",
      "BackupSchedule",
      "CamundaOptimize",
      "Grant",
      "ElasticsearchCluster",
    ]) {
      expect(listOf(kind)).toEqual({ namespace: "team-a" });
    }
    for (const kind of [
      "Environment",
      "CamundaClusterPreset",
      "CamundaRelease",
      "DatabaseServerPreset",
      "PluginSection",
      "PluginRegistration",
      "CamundaPlatformConfig",
      "ManagementAuthConfig",
    ]) {
      expect(listOf(kind)).toEqual({});
    }
  });

  it("selects the pods of a name longer than 63 characters by the hashed owner name", async () => {
    const fixture = JSON.parse(
      readFileSync(
        new URL(
          "../../../../../../../test/fixtures/labels/owner-names.json",
          import.meta.url,
        ),
        "utf8",
      ),
    ) as { names: { name: string; value: string }[] };
    const long = fixture.names.find((c) => c.name.length > 63)!;
    named(long.name);
    await loadClusterPage("team-a", long.name);
    expect(listOf("Pod")).toEqual({
      namespace: "team-a",
      labelSelector: `camunda.io/cluster=${long.value}`,
    });
  });

  it("gives an empty list for a read that fails, and keeps the cluster", async () => {
    named("orders");
    k8s.failing = "LogicalBackupRDBMS";
    const data = await loadClusterPage("team-a", "orders");
    expect(data?.cluster?.metadata?.name).toBe("orders");
    expect(data?.backups.rdbms).toEqual([]);
  });

  it("gives an empty list of schedules when their read fails", async () => {
    named("orders");
    k8s.failing = "BackupSchedule";
    const data = await loadClusterPage("team-a", "orders");
    expect(data?.cluster?.metadata?.name).toBe("orders");
    expect(data?.backupStorage.schedules).toEqual([]);
  });

  it("gives empty lists of grants and Elasticsearch clusters when their reads fail", async () => {
    named("orders");
    k8s.failing = "Grant";
    expect((await loadClusterPage("team-a", "orders"))?.grants).toEqual([]);
    k8s.failing = "ElasticsearchCluster";
    expect(
      (await loadClusterPage("team-a", "orders"))?.elasticsearchClusters,
    ).toEqual([]);
  });

  it("names the kind and the error of each read that fails, so the page never shows its empty list as read", async () => {
    named("orders");
    expect((await loadClusterPage("team-a", "orders"))?.readErrors).toEqual([]);
    k8s.failing = "PluginRequest";
    expect((await loadClusterPage("team-a", "orders"))?.readErrors).toEqual([
      { kind: "PluginRequest", message: "PluginRequest is forbidden" },
    ]);
  });

  it("gives the docs of the fields of the cluster", async () => {
    k8s.cluster = {
      kind: "CamundaCluster",
      metadata: { namespace: "team-a", name: "orders" },
      spec: { zeebe: { replicas: 3 } },
    };
    const data = await loadClusterPage("team-a", "orders");
    expect(
      data?.fieldDocs["CamundaCluster:spec.zeebe.replicas"]?.description,
    ).toBeTruthy();
  });

  it("gives no docs when the cluster has no fields to document", async () => {
    named("orders");
    const data = await loadClusterPage("team-a", "orders");
    expect(data?.fieldDocs).toEqual({});
  });

  it("gives no Optimize objects when their list fails", async () => {
    named("orders");
    k8s.failing = "CamundaOptimize";
    const data = await loadClusterPage("team-a", "orders");
    expect(data?.optimizes).toEqual([]);
  });

  it("gives no platform configs and no management auth configs when their lists fail", async () => {
    named("orders");
    k8s.failing = "CamundaPlatformConfig";
    expect(
      (await loadClusterPage("team-a", "orders"))?.platformConfigs,
    ).toEqual([]);
    k8s.failing = "ManagementAuthConfig";
    expect(
      (await loadClusterPage("team-a", "orders"))?.managementAuthConfigs,
    ).toEqual([]);
  });
});
