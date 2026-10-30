import { beforeEach, describe, expect, it, vi } from "vitest";
import { loadCreateCluster } from "./load";

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

const listOf = (kind: string) => k8s.lists.find((l) => l.kind === kind)?.opts;

describe("loadCreateCluster", () => {
  it("lists the access requests of every namespace by their section", async () => {
    await loadCreateCluster();
    expect(listOf("PluginRequest")).toEqual({
      labelSelector: "plugins.camunda.io/section=access",
    });
  });

  it("lists the other kinds in every namespace", async () => {
    await loadCreateCluster();
    for (const kind of [
      "Environment",
      "Namespace",
      "CamundaClusterClaim",
      "CamundaCluster",
      "CamundaClusterPreset",
      "PluginSection",
      "PluginRegistration",
    ]) {
      expect(listOf(kind)).toEqual({});
    }
  });

  it("gives no objects of a kind whose list fails", async () => {
    k8s.failing = "CamundaCluster";
    const data = await loadCreateCluster();
    expect(data.clusters).toEqual([]);
    expect(data.claims).toHaveLength(1);
  });
});
