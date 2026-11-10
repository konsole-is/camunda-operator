import { beforeEach, describe, expect, it, vi } from "vitest";
import { loadFleetPage } from "./load";

const k8s = vi.hoisted(() => ({
  lists: [] as { kind: string; opts: unknown }[],
  failing: "",
}));
vi.mock("@/lib/k8s/read", () => ({
  list: async (kind: string, opts?: unknown) => {
    k8s.lists.push({ kind, opts });
    if (kind === k8s.failing) throw new Error(`${kind} is forbidden`);
    return [{ kind, metadata: { name: `${kind}-a` } }];
  },
}));

beforeEach(() => {
  k8s.lists = [];
  k8s.failing = "";
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

describe("loadFleetPage", () => {
  it("lists every kind of the fleet view across all namespaces", async () => {
    await loadFleetPage();
    expect(new Set(k8s.lists.map((l) => l.kind))).toEqual(new Set(KINDS));
    for (const { opts } of k8s.lists) {
      expect(opts === undefined || JSON.stringify(opts) === "{}").toBe(true);
    }
  });

  it("never lists pods or events", async () => {
    await loadFleetPage();
    const kinds = k8s.lists.map((l) => l.kind);
    expect(kinds).not.toContain("Pod");
    expect(kinds).not.toContain("Event");
  });

  it("gives no objects of a kind whose list fails", async () => {
    k8s.failing = "PluginRequest";
    const data = await loadFleetPage();
    expect(data.requests).toEqual([]);
    expect(data.clusters.map((c) => c.metadata?.name)).toEqual([
      "CamundaCluster-a",
    ]);
  });

  it("joins both backup kinds", async () => {
    const data = await loadFleetPage();
    expect(data.backups.map((b) => b.kind).sort()).toEqual([
      "LogicalBackupElasticsearch",
      "LogicalBackupRDBMS",
    ]);
  });

  it("gives the planes the management lists and the namespaces", async () => {
    const data = await loadFleetPage();
    expect(data.planes.clusters.map((c) => c.kind)).toEqual([
      "CamundaManagementCluster",
    ]);
    expect(data.planes.claims.map((c) => c.kind)).toEqual([
      "CamundaManagementClusterClaim",
    ]);
    expect(data.planes.namespaces).toBe(data.namespaces);
  });
});
