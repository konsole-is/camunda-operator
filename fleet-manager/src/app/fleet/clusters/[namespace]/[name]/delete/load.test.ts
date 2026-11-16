import { beforeEach, describe, expect, it, vi } from "vitest";
import { loadDeleteClusterPage } from "./load";

const k8s = vi.hoisted(() => ({
  objects: {} as Record<string, unknown>,
  lists: {} as Record<string, unknown[]>,
  calls: [] as { kind: string; opts: unknown }[],
  failing: "",
}));
vi.mock("@/lib/k8s/read", () => ({
  get: async (kind: string, _namespace: string | undefined, name: string) => {
    if (kind === k8s.failing) throw new Error(`${kind} is forbidden`);
    return k8s.objects[`${kind}/${name}`];
  },
  list: async (kind: string, opts: unknown) => {
    k8s.calls.push({ kind, opts });
    if (kind === k8s.failing) throw new Error(`${kind} is forbidden`);
    return k8s.lists[kind] ?? [];
  },
}));

const claim = {
  kind: "CamundaClusterClaim",
  metadata: { namespace: "team-a", name: "orders" },
  spec: {},
};
const cluster = (presetRef?: string) => ({
  kind: "CamundaCluster",
  metadata: { namespace: "team-a", name: "orders" },
  spec: { ...(presetRef && { presetRef }) },
});

beforeEach(() => {
  k8s.objects = {};
  k8s.lists = {};
  k8s.calls = [];
  k8s.failing = "";
});

describe("loadDeleteClusterPage", () => {
  it("returns undefined when the claim does not exist, even with the cluster", async () => {
    k8s.objects["CamundaCluster/orders"] = cluster();
    expect(await loadDeleteClusterPage("team-a", "orders")).toBeUndefined();
  });

  it("returns the claim without the cluster while the cluster is gone", async () => {
    k8s.objects["CamundaClusterClaim/orders"] = claim;
    const data = await loadDeleteClusterPage("team-a", "orders");
    expect(data?.claim).toEqual(claim);
    expect(data?.cluster).toBeUndefined();
  });

  it("lists the requests of the namespace", async () => {
    k8s.objects["CamundaClusterClaim/orders"] = claim;
    await loadDeleteClusterPage("team-a", "orders");
    expect(
      k8s.calls.filter((c) => c.kind === "PluginRequest").map((c) => c.opts),
    ).toEqual([{ namespace: "team-a" }]);
  });

  it("returns the namespace of the claim and the Environments", async () => {
    k8s.objects["CamundaClusterClaim/orders"] = claim;
    const namespace = {
      metadata: {
        name: "team-a",
        labels: { "fleet.camunda.io/environment": "prod" },
      },
    };
    k8s.objects["Namespace/team-a"] = namespace;
    const environments = [
      { metadata: { name: "dev" } },
      { metadata: { name: "prod" } },
    ];
    k8s.lists.Environment = environments;
    const data = await loadDeleteClusterPage("team-a", "orders");
    expect(data?.namespace).toBe(namespace);
    expect(data?.environments).toEqual(environments);
  });

  it("returns the presets and the releases for the merged spec of the cluster", async () => {
    k8s.objects["CamundaClusterClaim/orders"] = claim;
    k8s.objects["CamundaCluster/orders"] = cluster("retaining");
    const preset = { metadata: { name: "retaining" } };
    const release = { metadata: { name: "camunda-8-9" } };
    k8s.lists.CamundaClusterPreset = [preset];
    k8s.lists.CamundaRelease = [release];
    const data = await loadDeleteClusterPage("team-a", "orders");
    expect(data?.presets).toEqual([preset]);
    expect(data?.releases).toEqual([release]);
  });

  it("a failed list gives no objects of that kind, and its error", async () => {
    k8s.objects["CamundaClusterClaim/orders"] = claim;
    k8s.lists.PluginRequest = [{ metadata: { name: "orders-backup" } }];
    k8s.failing = "PluginRequest";
    const data = await loadDeleteClusterPage("team-a", "orders");
    expect(data?.requests).toEqual([]);
    expect(data?.errors).toEqual([
      { kind: "PluginRequest", message: "PluginRequest is forbidden" },
    ]);
  });

  it("a failed read of the cluster gives its error", async () => {
    k8s.objects["CamundaClusterClaim/orders"] = claim;
    k8s.objects["CamundaCluster/orders"] = cluster();
    k8s.failing = "CamundaCluster";
    const data = await loadDeleteClusterPage("team-a", "orders");
    expect(data?.cluster).toBeUndefined();
    expect(data?.errors).toEqual([
      { kind: "CamundaCluster", message: "CamundaCluster is forbidden" },
    ]);
  });

  it("gives no errors when every read succeeds", async () => {
    k8s.objects["CamundaClusterClaim/orders"] = claim;
    expect((await loadDeleteClusterPage("team-a", "orders"))?.errors).toEqual(
      [],
    );
  });
});
