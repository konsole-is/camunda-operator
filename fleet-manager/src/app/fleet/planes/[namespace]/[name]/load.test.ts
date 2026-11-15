import { beforeEach, describe, expect, it, vi } from "vitest";
import { CLAIM_LABEL } from "@/lib/labels";
import { loadPlanePage } from "./load";

type Fixture = {
  kind: string;
  metadata: {
    namespace: string;
    name: string;
    labels?: Record<string, string>;
  };
};

const k8s = vi.hoisted(() => ({
  objects: [] as Fixture[],
  lists: [] as { kind: string; opts: unknown }[],
  failing: "",
}));
vi.mock("@/lib/k8s/read", () => ({
  get: async (kind: string, namespace: string, name: string) => {
    if (kind === k8s.failing) throw new Error(`${kind} is forbidden`);
    return k8s.objects.find(
      (o) =>
        o.kind === kind &&
        o.metadata.namespace === namespace &&
        o.metadata.name === name,
    );
  },
  list: async (kind: string, opts: unknown) => {
    k8s.lists.push({ kind, opts });
    if (kind === k8s.failing) throw new Error(`${kind} is forbidden`);
    return [];
  },
}));

beforeEach(() => {
  k8s.objects = [];
  k8s.lists = [];
  k8s.failing = "";
});

const plane = (name: string, labels: Record<string, string>): Fixture => ({
  kind: "CamundaManagementCluster",
  metadata: { namespace: "platform", name, labels },
});
const claim = (name: string): Fixture => ({
  kind: "CamundaManagementClusterClaim",
  metadata: { namespace: "platform", name },
});
const listOf = (kind: string) => k8s.lists.find((l) => l.kind === kind)?.opts;

describe("loadPlanePage", () => {
  it("returns undefined when neither the plane nor its claim exists", async () => {
    expect(await loadPlanePage("platform", "plane-a")).toBeUndefined();
  });

  it("returns a claim-only plane with its claim", async () => {
    k8s.objects = [claim("plane-a")];
    const data = await loadPlanePage("platform", "plane-a");
    expect(data?.plane).toBeUndefined();
    expect(data?.claim?.metadata.name).toBe("plane-a");
    expect(data).toMatchObject({ namespace: "platform", name: "plane-a" });
  });

  it("joins the claim that the plane names in its claim label", async () => {
    k8s.objects = [
      plane("plane-a", { [CLAIM_LABEL]: "plane-a" }),
      claim("plane-a"),
    ];
    const data = await loadPlanePage("platform", "plane-a");
    expect(data?.plane?.metadata?.name).toBe("plane-a");
    expect(data?.claim?.metadata.name).toBe("plane-a");
  });

  it("ignores a claim whose name the plane does not carry in its claim label", async () => {
    const unnamed: Record<string, string>[] = [{}, { [CLAIM_LABEL]: "other" }];
    for (const labels of unnamed) {
      k8s.objects = [plane("plane-a", labels), claim("plane-a")];
      const data = await loadPlanePage("platform", "plane-a");
      expect(data?.plane).toBeDefined();
      expect(data?.claim).toBeUndefined();
    }
  });

  it("returns a plane whose claim read fails, without a claim", async () => {
    k8s.objects = [plane("plane-a", { [CLAIM_LABEL]: "plane-a" })];
    k8s.failing = "CamundaManagementClusterClaim";
    const data = await loadPlanePage("platform", "plane-a");
    expect(data?.plane).toBeDefined();
    expect(data?.claim).toBeUndefined();
  });

  it("lists the namespaces and the environments everywhere, and a failed list gives none", async () => {
    k8s.objects = [claim("plane-a")];
    k8s.failing = "Environment";
    const data = await loadPlanePage("platform", "plane-a");
    expect(listOf("Namespace")).toEqual({});
    expect(listOf("Environment")).toEqual({});
    expect(data?.environments).toEqual([]);
  });
});
