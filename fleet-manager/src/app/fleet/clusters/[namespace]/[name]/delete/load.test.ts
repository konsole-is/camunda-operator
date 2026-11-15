import { beforeEach, describe, expect, it, vi } from "vitest";
import { loadDeleteClusterPage } from "./load";

const k8s = vi.hoisted(() => ({
  objects: {} as Record<string, unknown>,
  lists: {} as Record<string, unknown[]>,
  calls: [] as { kind: string; opts: unknown }[],
  failing: "",
}));
vi.mock("@/lib/k8s/read", () => ({
  get: async (kind: string, _namespace: string | undefined, name: string) =>
    k8s.objects[`${kind}/${name}`],
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
    expect(data?.volumes).toBe("Delete");
  });

  it("lists the requests of the claim by the claim label", async () => {
    k8s.objects["CamundaClusterClaim/orders"] = claim;
    await loadDeleteClusterPage("team-a", "orders");
    expect(
      k8s.calls.filter((c) => c.kind === "PluginRequest").map((c) => c.opts),
    ).toEqual([
      { namespace: "team-a", labelSelector: "fleet.camunda.io/claim=orders" },
    ]);
  });

  it("takes the Environment that the label of the namespace names", async () => {
    k8s.objects["CamundaClusterClaim/orders"] = claim;
    k8s.objects["Namespace/team-a"] = {
      metadata: {
        name: "team-a",
        labels: { "fleet.camunda.io/environment": "prod" },
      },
    };
    const prod = { metadata: { name: "prod" } };
    k8s.lists.Environment = [{ metadata: { name: "dev" } }, prod];
    expect((await loadDeleteClusterPage("team-a", "orders"))?.environment).toBe(
      prod,
    );
  });

  it("reads the retention of the broker volumes from the merged spec", async () => {
    k8s.objects["CamundaClusterClaim/orders"] = claim;
    k8s.objects["CamundaCluster/orders"] = cluster("retaining");
    k8s.lists.CamundaClusterPreset = [
      {
        metadata: { name: "retaining" },
        spec: {
          cluster: {
            zeebe: {
              persistentVolumeClaimRetentionPolicy: { whenDeleted: "Retain" },
            },
          },
        },
      },
    ];
    expect((await loadDeleteClusterPage("team-a", "orders"))?.volumes).toBe(
      "Retain",
    );
    k8s.objects["CamundaCluster/orders"] = cluster();
    expect((await loadDeleteClusterPage("team-a", "orders"))?.volumes).toBe(
      "Delete",
    );
  });

  it("a failed list gives no objects of that kind", async () => {
    k8s.objects["CamundaClusterClaim/orders"] = claim;
    k8s.lists.BackupSchedule = [{ metadata: { name: "orders-daily" } }];
    k8s.failing = "BackupSchedule";
    expect(
      (await loadDeleteClusterPage("team-a", "orders"))?.schedules,
    ).toEqual([]);
  });
});
