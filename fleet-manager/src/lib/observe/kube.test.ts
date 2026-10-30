import { beforeEach, describe, expect, it, vi } from "vitest";
import { KubernetesRequestError, getJSON } from "@/lib/k8s/client";
import type { Resource } from "@/lib/k8s/kinds";
import { list } from "@/lib/k8s/read";
import { events, metricsServerFound, podUsage, requestPods } from "./kube";

vi.mock("@/lib/k8s/client", async (real) => ({
  ...(await real<typeof import("@/lib/k8s/client")>()),
  getJSON: vi.fn(),
  kubeConfig: vi.fn(),
}));
vi.mock("@/lib/k8s/read", () => ({ list: vi.fn() }));

const MI = 2 ** 20;

beforeEach(() => vi.resetAllMocks());

describe("podUsage path", () => {
  it("keeps a namespace with a slash in its own segment", async () => {
    vi.mocked(getJSON).mockResolvedValue({ items: [] });
    vi.mocked(list).mockResolvedValue([]);

    await podUsage("../../../../api/v1/namespaces/kube-system/secrets?", "a=b");

    const path = String(vi.mocked(getJSON).mock.calls[0][1]);
    const url = new URL(`https://api.example${path}`);
    expect(url.pathname).toBe(
      "/apis/metrics.k8s.io/v1beta1/namespaces/..%2F..%2F..%2F..%2Fapi%2Fv1%2Fnamespaces%2Fkube-system%2Fsecrets%3F/pods",
    );
  });

  it.each(["..", "."])(
    "refuses the namespace %j and sends nothing",
    async (ns) => {
      await expect(podUsage(ns, "a=b")).rejects.toThrow();
      expect(getJSON).not.toHaveBeenCalled();
    },
  );
});

describe("podUsage", () => {
  const usage = {
    items: [
      {
        metadata: { name: "p-0" },
        containers: [
          { name: "a", usage: { cpu: "250m", memory: "64Mi" } },
          { name: "b", usage: { cpu: "50m", memory: "16Mi" } },
        ],
      },
    ],
  };

  it("joins usage with the requests and limits of the pod spec", async () => {
    vi.mocked(getJSON).mockResolvedValue(usage);
    vi.mocked(list).mockResolvedValue([
      {
        metadata: { name: "p-0" },
        spec: {
          containers: [
            {
              name: "a",
              resources: {
                requests: { cpu: "500m", memory: "128Mi" },
                limits: { memory: "256Mi" },
              },
            },
            { name: "b", resources: { requests: { cpu: "100m" } } },
          ],
        },
      },
    ] as never);

    expect(await podUsage("team-a", "camunda.io/cluster=orders")).toEqual([
      {
        pod: "p-0",
        cpu: { usedCores: 0.3, requestCores: 0.6 },
        memory: { usedBytes: 80 * MI, requestBytes: 128 * MI },
      },
    ]);
    expect(getJSON).toHaveBeenCalledWith(
      undefined,
      "/apis/metrics.k8s.io/v1beta1/namespaces/team-a/pods?labelSelector=camunda.io%2Fcluster%3Dorders",
      expect.any(Number),
    );
    expect(list).toHaveBeenCalledWith("Pod", {
      namespace: "team-a",
      labelSelector: "camunda.io/cluster=orders",
    });
  });

  it("sets a limit when every container has one", async () => {
    vi.mocked(getJSON).mockResolvedValue(usage);
    vi.mocked(list).mockResolvedValue([
      {
        metadata: { name: "p-0" },
        spec: {
          containers: [
            { name: "a", resources: { limits: { cpu: "1", memory: "1Gi" } } },
            { name: "b", resources: { limits: { cpu: "500m" } } },
          ],
        },
      },
    ] as never);

    const [pod] = (await podUsage("team-a", "")) as {
      cpu: object;
      memory: object;
    }[];
    expect(pod.cpu).toEqual({ usedCores: 0.3, limitCores: 1.5 });
    expect(pod.memory).toEqual({ usedBytes: 80 * MI });
  });

  it("counts a native sidecar and leaves out an init container that ends", async () => {
    vi.mocked(getJSON).mockResolvedValue(usage);
    vi.mocked(list).mockResolvedValue([
      {
        metadata: { name: "p-0" },
        spec: {
          initContainers: [
            {
              name: "setup",
              resources: { requests: { cpu: "4" }, limits: { cpu: "4" } },
            },
            {
              name: "b",
              restartPolicy: "Always",
              resources: { requests: { cpu: "100m" }, limits: { cpu: "500m" } },
            },
          ],
          containers: [
            {
              name: "a",
              resources: { requests: { cpu: "500m" }, limits: { cpu: "1" } },
            },
          ],
        },
      },
    ] as never);

    const [usageOf] = (await podUsage("team-a", "")) as { cpu: object }[];
    expect(usageOf.cpu).toEqual({
      usedCores: 0.3,
      requestCores: 0.6,
      limitCores: 1.5,
    });
  });

  it("gives no limit when a native sidecar has none", async () => {
    vi.mocked(getJSON).mockResolvedValue(usage);
    vi.mocked(list).mockResolvedValue([
      {
        metadata: { name: "p-0" },
        spec: {
          initContainers: [{ name: "b", restartPolicy: "Always" }],
          containers: [{ name: "a", resources: { limits: { cpu: "1" } } }],
        },
      },
    ] as never);

    const [usageOf] = (await podUsage("team-a", "")) as { cpu: object }[];
    expect(usageOf.cpu).toEqual({ usedCores: 0.3 });
  });

  it("gives no limit when a container has a limit of zero, which means none", async () => {
    vi.mocked(getJSON).mockResolvedValue(usage);
    vi.mocked(list).mockResolvedValue([
      {
        metadata: { name: "p-0" },
        spec: {
          containers: [
            { name: "a", resources: { limits: { cpu: "1", memory: "0" } } },
            { name: "b", resources: { limits: { cpu: "0", memory: "1Gi" } } },
          ],
        },
      },
    ] as never);

    const [pod] = (await podUsage("team-a", "")) as {
      cpu: object;
      memory: object;
    }[];
    expect(pod.cpu).toEqual({ usedCores: 0.3 });
    expect(pod.memory).toEqual({ usedBytes: 80 * MI });
  });

  it("gives a pod that is not in the cache no requests and no limits", async () => {
    vi.mocked(getJSON).mockResolvedValue(usage);
    vi.mocked(list).mockResolvedValue([]);

    expect(await podUsage("team-a", "")).toEqual([
      { pod: "p-0", cpu: { usedCores: 0.3 }, memory: { usedBytes: 80 * MI } },
    ]);
  });

  it("asks without labelSelector for the empty selector", async () => {
    vi.mocked(getJSON).mockResolvedValue({ items: [] });
    vi.mocked(list).mockResolvedValue([]);

    expect(await podUsage("team-a", "")).toEqual([]);
    expect(getJSON).toHaveBeenCalledWith(
      undefined,
      "/apis/metrics.k8s.io/v1beta1/namespaces/team-a/pods",
      expect.any(Number),
    );
  });

  it.each([404, 503])(
    "returns unavailable when metrics.k8s.io answers %i",
    async (code) => {
      vi.mocked(getJSON).mockRejectedValue(
        new KubernetesRequestError(code, "no metrics"),
      );
      vi.mocked(list).mockResolvedValue([]);

      expect(await podUsage("team-a", "")).toBe("unavailable");
    },
  );

  it("rejects when metrics.k8s.io answers 403", async () => {
    vi.mocked(getJSON).mockRejectedValue(
      new KubernetesRequestError(403, "forbidden"),
    );
    vi.mocked(list).mockResolvedValue([]);

    await expect(podUsage("team-a", "")).rejects.toThrow("forbidden");
  });
});

describe("metricsServerFound", () => {
  it("is true when metrics.k8s.io answers", async () => {
    vi.mocked(getJSON).mockResolvedValue({ items: [] });

    expect(await metricsServerFound("fleet")).toBe(true);
    expect(vi.mocked(getJSON).mock.calls[0][1]).toBe(
      "/apis/metrics.k8s.io/v1beta1/namespaces/fleet/pods?limit=1",
    );
  });

  it.each([404, 503])("is false on %i", async (status) => {
    vi.mocked(getJSON).mockRejectedValue(
      new KubernetesRequestError(status, "no metrics"),
    );

    expect(await metricsServerFound("fleet")).toBe(false);
  });

  it("is true on 403, because the API is served", async () => {
    vi.mocked(getJSON).mockRejectedValue(
      new KubernetesRequestError(403, "forbidden"),
    );

    expect(await metricsServerFound("fleet")).toBe(true);
  });

  it("rejects when the API server does not answer", async () => {
    vi.mocked(getJSON).mockRejectedValue(new Error("connect ECONNREFUSED"));

    await expect(metricsServerFound("fleet")).rejects.toThrow("ECONNREFUSED");
  });
});

describe("requestPods", () => {
  const request = (podSelector?: object) =>
    ({
      metadata: { name: "r", namespace: "team-a" },
      spec: {},
      status: podSelector && { podSelector },
    }) as unknown as Resource<"PluginRequest">;

  it.each([
    ["no podSelector", undefined],
    ["an empty podSelector", {}],
  ])("returns no pods for %s", async (_, selector) => {
    expect(await requestPods(request(selector))).toEqual([]);
    expect(list).not.toHaveBeenCalled();
  });

  it("returns the names of the pods that the selector selects", async () => {
    vi.mocked(list).mockResolvedValue([
      { metadata: { name: "x-0" } },
      { metadata: { name: "x-1" } },
    ] as never);

    expect(await requestPods(request({ matchLabels: { app: "x" } }))).toEqual([
      "x-0",
      "x-1",
    ]);
    expect(list).toHaveBeenCalledWith("Pod", {
      namespace: "team-a",
      labelSelector: "app=x",
    });
  });
});

describe("events", () => {
  const event = (
    name: string,
    regarding: { kind: string; name: string; namespace?: string; uid?: string },
  ) => ({ metadata: { name, namespace: "team-a" }, regarding });

  it("keeps the events whose regarding object is the ref", async () => {
    vi.mocked(list).mockResolvedValue([
      event("hit", { kind: "PluginRequest", name: "r", namespace: "team-a" }),
      event("kind", { kind: "Pod", name: "r", namespace: "team-a" }),
      event("name", { kind: "PluginRequest", name: "s", namespace: "team-a" }),
      event("ns", { kind: "PluginRequest", name: "r", namespace: "team-b" }),
    ] as never);

    const found = await events({
      kind: "PluginRequest",
      name: "r",
      namespace: "team-a",
    });

    expect(found.map((e) => e.metadata?.name)).toEqual(["hit"]);
    expect(list).toHaveBeenCalledWith("Event", { namespace: "team-a" });
  });

  it("checks the uid only when the ref has one", async () => {
    vi.mocked(list).mockResolvedValue([
      event("old", { kind: "Pod", name: "p", namespace: "team-a", uid: "1" }),
      event("new", { kind: "Pod", name: "p", namespace: "team-a", uid: "2" }),
    ] as never);
    const ref = { kind: "Pod", name: "p", namespace: "team-a" };

    expect((await events(ref)).map((e) => e.metadata?.name)).toEqual([
      "old",
      "new",
    ]);
    expect(
      (await events({ ...ref, uid: "2" })).map((e) => e.metadata?.name),
    ).toEqual(["new"]);
  });

  it("lists every namespace for a cluster-scoped object", async () => {
    vi.mocked(list).mockResolvedValue([
      event("hit", { kind: "Environment", name: "prod" }),
    ] as never);

    expect(await events({ kind: "Environment", name: "prod" })).toHaveLength(1);
    expect(list).toHaveBeenCalledWith("Event", {});
  });
});
