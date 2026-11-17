import { beforeEach, describe, expect, it, vi } from "vitest";
import { CLAIM_LABEL } from "@/lib/labels";
import { ownerName } from "@/lib/labels/owner-name";
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
  gets: [] as string[],
  failing: [] as string[],
}));
vi.mock("@/lib/k8s/read", () => ({
  get: async (kind: string, namespace: string, name: string) => {
    k8s.gets.push(kind);
    if (k8s.failing.includes(kind)) throw new Error(`${kind} is forbidden`);
    return k8s.objects.find(
      (o) =>
        o.kind === kind &&
        o.metadata.namespace === namespace &&
        o.metadata.name === name,
    );
  },
  list: async (kind: string, opts: unknown) => {
    k8s.lists.push({ kind, opts });
    if (k8s.failing.includes(kind)) throw new Error(`${kind} is forbidden`);
    return [];
  },
}));

beforeEach(() => {
  k8s.objects = [];
  k8s.lists = [];
  k8s.gets = [];
  k8s.failing = [];
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

  it("returns a plane whose claim read fails, without a claim and with the read error", async () => {
    k8s.objects = [plane("plane-a", { [CLAIM_LABEL]: "plane-a" })];
    k8s.failing = ["CamundaManagementClusterClaim"];
    const data = await loadPlanePage("platform", "plane-a");
    expect(data?.plane).toBeDefined();
    expect(data?.claim).toBeUndefined();
    expect(data?.readErrors).toEqual([
      {
        kind: "CamundaManagementClusterClaim",
        message: "CamundaManagementClusterClaim is forbidden",
      },
    ]);
  });

  it("returns the claim and the read error when the plane read fails", async () => {
    k8s.objects = [claim("plane-a")];
    k8s.failing = ["CamundaManagementCluster"];
    const data = await loadPlanePage("platform", "plane-a");
    expect(data?.plane).toBeUndefined();
    expect(data?.claim?.metadata.name).toBe("plane-a");
    expect(data?.readErrors).toEqual([
      {
        kind: "CamundaManagementCluster",
        message: "CamundaManagementCluster is forbidden",
      },
    ]);
  });

  it("returns the page with the read error, not undefined, when the plane read fails and no claim exists", async () => {
    k8s.failing = ["CamundaManagementCluster"];
    const data = await loadPlanePage("platform", "plane-a");
    expect(data).toBeDefined();
    expect(data?.readErrors?.map((e) => e.kind)).toEqual([
      "CamundaManagementCluster",
    ]);
  });

  it("returns the page with both read errors when both object reads fail", async () => {
    k8s.failing = ["CamundaManagementCluster", "CamundaManagementClusterClaim"];
    const data = await loadPlanePage("platform", "plane-a");
    expect(data).toBeDefined();
    expect(data?.readErrors?.map((e) => e.kind).sort()).toEqual([
      "CamundaManagementCluster",
      "CamundaManagementClusterClaim",
    ]);
  });

  it("returns undefined when neither object exists, also when a list fails", async () => {
    k8s.failing = ["Environment"];
    expect(await loadPlanePage("platform", "plane-a")).toBeUndefined();
  });

  it("lists the namespaces and the environments everywhere, and a failed list gives none and a read error", async () => {
    k8s.objects = [claim("plane-a")];
    k8s.failing = ["Environment"];
    const data = await loadPlanePage("platform", "plane-a");
    expect(listOf("Namespace")).toEqual({});
    expect(listOf("Environment")).toEqual({});
    expect(data?.environments).toEqual([]);
    expect(data?.readErrors).toEqual([
      { kind: "Environment", message: "Environment is forbidden" },
    ]);
  });

  it("lists every CamundaOptimize and ManagementAuthConfig, and a failed list gives none and a read error", async () => {
    k8s.objects = [claim("plane-a")];
    k8s.failing = ["ManagementAuthConfig"];
    const data = await loadPlanePage("platform", "plane-a");
    expect(listOf("CamundaOptimize")).toEqual({});
    expect(listOf("ManagementAuthConfig")).toEqual({});
    expect(data?.optimizes).toEqual([]);
    expect(data?.managementAuthConfigs).toEqual([]);
    expect(data?.readErrors).toEqual([
      {
        kind: "ManagementAuthConfig",
        message: "ManagementAuthConfig is forbidden",
      },
    ]);
  });

  it("lists the pods of the plane in its namespace by its owner label", async () => {
    k8s.objects = [claim("plane-a")];
    await loadPlanePage("platform", "plane-a");
    expect(listOf("Pod")).toEqual({
      namespace: "platform",
      labelSelector: "camunda.io/management-cluster=plane-a",
    });
  });

  it("selects the pods of a plane with a long name by the owner value of ownerName", async () => {
    const name = `plane-${"a".repeat(70)}`;
    k8s.objects = [claim(name)];
    await loadPlanePage("platform", name);
    expect(listOf("Pod")).toEqual({
      namespace: "platform",
      labelSelector: `camunda.io/management-cluster=${ownerName(name)}`,
    });
    expect(ownerName(name)).not.toBe(name);
  });

  it("lists the platform configs and the registrations everywhere, and a failed list gives none and a read error", async () => {
    k8s.objects = [claim("plane-a")];
    k8s.failing = ["Pod", "PluginRegistration"];
    const data = await loadPlanePage("platform", "plane-a");
    expect(listOf("CamundaPlatformConfig")).toEqual({});
    expect(listOf("PluginRegistration")).toEqual({});
    expect(data).toMatchObject({
      pods: [],
      platformConfigs: [],
      registrations: [],
    });
    expect(data?.readErrors).toEqual([
      { kind: "Pod", message: "Pod is forbidden" },
      {
        kind: "PluginRegistration",
        message: "PluginRegistration is forbidden",
      },
    ]);
  });

  it("reads the lists of the Claim tab, the requests of the claim only", async () => {
    k8s.objects = [claim("plane-a")];
    const data = await loadPlanePage("platform", "plane-a");
    for (const kind of [
      "CamundaCluster",
      "CamundaPlatformConfig",
      "PluginSection",
      "PluginRegistration",
    ])
      expect(listOf(kind)).toEqual({});
    expect(listOf("PluginRequest")).toEqual({
      namespace: "platform",
      labelSelector: `${CLAIM_LABEL}=plane-a`,
    });
    expect(data?.claimTab).toEqual({
      clusters: [],
      platformConfigs: [],
      sections: [],
      registrations: [],
      requests: [],
      readErrors: [],
    });
  });

  it("keeps a failed read of a Claim tab list apart from the read errors of the page", async () => {
    k8s.objects = [claim("plane-a")];
    k8s.failing = ["PluginSection"];
    const data = await loadPlanePage("platform", "plane-a");
    expect(data?.readErrors).toEqual([]);
    expect(data?.claimTab?.readErrors).toEqual([
      { kind: "PluginSection", message: "PluginSection is forbidden" },
    ]);
    expect(data?.claimTab?.sections).toEqual([]);
  });

  it("reads no Secret", async () => {
    k8s.objects = [claim("plane-a")];
    await loadPlanePage("platform", "plane-a");
    expect(k8s.gets).toContain("CamundaManagementCluster");
    expect(k8s.gets).not.toContain("Secret");
    expect(k8s.lists.map((l) => l.kind)).not.toContain("Secret");
  });
});
