import { beforeEach, describe, expect, it, vi } from "vitest";
import { loadDeletePlanePage } from "./load";

const k8s = vi.hoisted(() => ({
  objects: {} as Record<string, unknown>,
  calls: [] as { kind: string; opts: unknown }[],
  gets: [] as string[],
  failing: "",
}));
vi.mock("@/lib/k8s/read", () => ({
  get: async (kind: string, _namespace: string | undefined, name: string) => {
    k8s.gets.push(kind);
    if (kind === k8s.failing) throw new Error(`${kind} is forbidden`);
    return k8s.objects[`${kind}/${name}`];
  },
  list: async (kind: string, opts: unknown) => {
    k8s.calls.push({ kind, opts });
    if (kind === k8s.failing) throw new Error(`${kind} is forbidden`);
    return [];
  },
}));

const claim = {
  kind: "CamundaManagementClusterClaim",
  metadata: { namespace: "platform", name: "plane-a" },
  spec: {},
};
const plane = {
  kind: "CamundaManagementCluster",
  metadata: { namespace: "platform", name: "plane-a" },
  spec: {},
};

beforeEach(() => {
  k8s.objects = {};
  k8s.calls = [];
  k8s.gets = [];
  k8s.failing = "";
});

describe("loadDeletePlanePage", () => {
  it("returns undefined when the claim does not exist, even with the plane", async () => {
    k8s.objects["CamundaManagementCluster/plane-a"] = plane;
    expect(await loadDeletePlanePage("platform", "plane-a")).toBeUndefined();
  });

  it("returns the claim and its plane", async () => {
    k8s.objects["CamundaManagementClusterClaim/plane-a"] = claim;
    k8s.objects["CamundaManagementCluster/plane-a"] = plane;
    const data = await loadDeletePlanePage("platform", "plane-a");
    expect(data).toMatchObject({ claim, plane, errors: [] });
  });

  it("lists the requests of the namespace and every CamundaOptimize", async () => {
    k8s.objects["CamundaManagementClusterClaim/plane-a"] = claim;
    await loadDeletePlanePage("platform", "plane-a");
    const opts = (kind: string) =>
      k8s.calls.filter((c) => c.kind === kind).map((c) => c.opts);
    expect(opts("PluginRequest")).toEqual([{ namespace: "platform" }]);
    expect(opts("CamundaOptimize")).toEqual([{}]);
  });

  it("names a failed plane read and returns no plane", async () => {
    k8s.objects["CamundaManagementClusterClaim/plane-a"] = claim;
    k8s.objects["CamundaManagementCluster/plane-a"] = plane;
    k8s.failing = "CamundaManagementCluster";
    const data = await loadDeletePlanePage("platform", "plane-a");
    expect(data?.plane).toBeUndefined();
    expect(data?.errors).toEqual([
      {
        kind: "CamundaManagementCluster",
        message: "CamundaManagementCluster is forbidden",
      },
    ]);
  });

  it("names a failed list and gives none of its kind", async () => {
    k8s.objects["CamundaManagementClusterClaim/plane-a"] = claim;
    k8s.failing = "PluginRequest";
    const data = await loadDeletePlanePage("platform", "plane-a");
    expect(data?.requests).toEqual([]);
    expect(data?.errors.map((e) => e.kind)).toEqual(["PluginRequest"]);
  });

  it("reads no Secret", async () => {
    k8s.objects["CamundaManagementClusterClaim/plane-a"] = claim;
    await loadDeletePlanePage("platform", "plane-a");
    expect(k8s.gets).not.toContain("Secret");
    expect(k8s.calls.map((c) => c.kind)).not.toContain("Secret");
  });
});
