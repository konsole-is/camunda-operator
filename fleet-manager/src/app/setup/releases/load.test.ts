import { beforeEach, describe, expect, it, vi } from "vitest";
import { loadRolloutLists } from "./load";

const k8s = vi.hoisted(() => ({
  lists: [] as { kind: string; opts: unknown }[],
  failing: "",
}));
vi.mock("@/lib/k8s/read", () => ({
  list: async (kind: string, opts: unknown) => {
    k8s.lists.push({ kind, opts });
    if (kind === k8s.failing) throw new Error(`${kind} is forbidden`);
    return [{ kind, metadata: { name: "x" } }];
  },
}));

beforeEach(() => {
  k8s.lists = [];
  k8s.failing = "";
});

describe("loadRolloutLists", () => {
  it("lists each kind that the Rollout tab checks in every namespace", async () => {
    const data = await loadRolloutLists();
    expect(k8s.lists).toEqual(
      [
        "CamundaClusterPreset",
        "PluginSection",
        "PluginRegistration",
        "PluginRequest",
        "LogicalBackupElasticsearch",
        "LogicalBackupRDBMS",
      ].map((kind) => ({ kind, opts: {} })),
    );
    expect(data.rdbmsBackups).toEqual([
      { kind: "LogicalBackupRDBMS", metadata: { name: "x" } },
    ]);
  });

  it("gives no objects of a kind whose server read fails, and keeps the others", async () => {
    k8s.failing = "LogicalBackupRDBMS";
    const data = await loadRolloutLists();
    expect(data.rdbmsBackups).toEqual([]);
    expect(data.elasticsearchBackups).toHaveLength(1);
  });
});
