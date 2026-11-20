import { beforeEach, describe, expect, it, vi } from "vitest";
import { loadSnapshot } from "./snapshot";

const k8s = vi.hoisted(() => ({ failing: [] as string[] }));
vi.mock("./read", () => ({
  list: async (kind: string) => {
    if (k8s.failing.includes(kind)) throw new Error(`${kind} is forbidden`);
    return [{ kind, metadata: { name: `${kind}-a` } }];
  },
}));

beforeEach(() => {
  k8s.failing = [];
});

const KINDS = {
  environments: "Environment",
  namespaces: "Namespace",
  releases: "CamundaRelease",
} as const;

describe("loadSnapshot", () => {
  it("gives the list of each kind under its key and no read error", async () => {
    const snapshot = await loadSnapshot(KINDS);
    expect(snapshot.readErrors).toEqual([]);
    expect(snapshot.namespaces).toEqual([
      { kind: "Namespace", metadata: { name: "Namespace-a" } },
    ]);
  });

  it("gives no objects and a read error for each kind whose read fails, in the order of the kinds", async () => {
    k8s.failing = ["CamundaRelease", "Environment"];
    const snapshot = await loadSnapshot(KINDS);
    expect(snapshot.environments).toEqual([]);
    expect(snapshot.releases).toEqual([]);
    expect(snapshot.namespaces).toHaveLength(1);
    expect(snapshot.readErrors).toEqual([
      { kind: "Environment", message: "Environment is forbidden" },
      { kind: "CamundaRelease", message: "CamundaRelease is forbidden" },
    ]);
  });
});
