import { readFileSync } from "node:fs";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { loadClusterPage } from "./load";

const k8s = vi.hoisted(() => ({
  cluster: undefined as unknown,
  lists: [] as { kind: string; opts: unknown }[],
  failing: "",
}));
vi.mock("@/lib/k8s/read", () => ({
  get: async (kind: string, _namespace: string, name: string) =>
    kind === "CamundaCluster"
      ? (k8s.cluster as { metadata: { name: string } } | undefined)?.metadata
          .name === name
        ? k8s.cluster
        : undefined
      : undefined,
  list: async (kind: string, opts: unknown) => {
    k8s.lists.push({ kind, opts });
    if (kind === k8s.failing) throw new Error(`${kind} is forbidden`);
    return [];
  },
}));

beforeEach(() => {
  k8s.lists = [];
  k8s.failing = "";
});

const named = (name: string) => {
  k8s.cluster = {
    kind: "CamundaCluster",
    metadata: { namespace: "team-a", name },
  };
};
const listOf = (kind: string) => k8s.lists.find((l) => l.kind === kind)?.opts;

describe("loadClusterPage", () => {
  it("returns undefined when the cluster does not exist", async () => {
    named("orders");
    expect(await loadClusterPage("team-a", "payments")).toBeUndefined();
  });

  it("lists the pods of the cluster by its label and the access requests by their section", async () => {
    named("orders");
    const data = await loadClusterPage("team-a", "orders");
    expect(data?.podSelector).toBe("camunda.io/cluster=orders");
    expect(listOf("Pod")).toEqual({
      namespace: "team-a",
      labelSelector: "camunda.io/cluster=orders",
    });
    expect(listOf("PluginRequest")).toEqual({
      namespace: "team-a",
      labelSelector: "plugins.camunda.io/section=access",
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
    ]) {
      expect(listOf(kind)).toEqual({ namespace: "team-a" });
    }
    for (const kind of [
      "Environment",
      "CamundaClusterPreset",
      "CamundaRelease",
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
    expect(data?.cluster.metadata?.name).toBe("orders");
    expect(data?.backups.rdbms).toEqual([]);
  });

  it("gives an empty list of schedules when their read fails", async () => {
    named("orders");
    k8s.failing = "BackupSchedule";
    const data = await loadClusterPage("team-a", "orders");
    expect(data?.cluster.metadata?.name).toBe("orders");
    expect(data?.backupStorage.schedules).toEqual([]);
  });
});
