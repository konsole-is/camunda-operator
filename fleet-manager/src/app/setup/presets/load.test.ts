import { beforeEach, describe, expect, it, vi } from "vitest";
import { loadPresets } from "./load";

const k8s = vi.hoisted(() => ({ failing: "" }));
vi.mock("@/lib/k8s/read", () => ({
  list: async (kind: string) => {
    if (kind === k8s.failing) throw new Error(`${kind} is forbidden`);
    return [{ kind, metadata: { name: `${kind}-a` } }];
  },
}));

beforeEach(() => {
  k8s.failing = "";
});

describe("loadPresets", () => {
  it("gives each list and no read error when every read succeeds", async () => {
    const loaded = await loadPresets();
    expect(loaded.readErrors).toEqual([]);
    expect(loaded.requests).toEqual([
      { kind: "PluginRequest", metadata: { name: "PluginRequest-a" } },
    ]);
  });

  it("gives no objects and a read error for a kind whose read fails", async () => {
    k8s.failing = "PluginRequest";
    const loaded = await loadPresets();
    expect(loaded.requests).toEqual([]);
    expect(loaded.readErrors).toEqual([
      { kind: "PluginRequest", message: "PluginRequest is forbidden" },
    ]);
    expect(loaded.clusters).toHaveLength(1);
  });
});
