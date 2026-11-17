import { describe, expect, it, vi } from "vitest";
import { listEach } from "./list-each";

const failing = vi.hoisted(() => new Set<string>());
vi.mock("./read", () => ({
  list: async (kind: string) => {
    if (failing.has(kind)) throw new Error(`${kind} is forbidden`);
    return [{ kind, metadata: { name: "a" } }];
  },
}));

describe("listEach", () => {
  it("gives the list of each kind under its key", async () => {
    failing.clear();
    const { lists, readErrors } = await listEach({
      clusters: "CamundaCluster",
      environments: "Environment",
    });
    expect(lists).toEqual({
      clusters: [{ kind: "CamundaCluster", metadata: { name: "a" } }],
      environments: [{ kind: "Environment", metadata: { name: "a" } }],
    });
    expect(readErrors).toEqual([]);
  });

  it("gives no objects and an error for each failed kind, in the order of kinds", async () => {
    failing.clear();
    failing.add("Namespace").add("CamundaCluster");
    const { lists, readErrors } = await listEach({
      clusters: "CamundaCluster",
      environments: "Environment",
      namespaces: "Namespace",
    });
    expect(lists.clusters).toEqual([]);
    expect(lists.namespaces).toEqual([]);
    expect(lists.environments).toHaveLength(1);
    expect(readErrors).toEqual([
      { kind: "CamundaCluster", message: "CamundaCluster is forbidden" },
      { kind: "Namespace", message: "Namespace is forbidden" },
    ]);
  });
});
