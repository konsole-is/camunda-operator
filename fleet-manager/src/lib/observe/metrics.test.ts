import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Resource } from "@/lib/k8s/kinds";
import { get, list } from "@/lib/k8s/read";
import { requestPods } from "@/lib/observe/kube";
import { promRange } from "@/lib/observe/prometheus";
import { type Settings, getSettings } from "@/lib/settings";
import {
  chartsOf,
  componentMetrics,
  podVolumes,
  requestMetrics,
} from "./metrics";
import type { Series } from "./types";

vi.mock("@/lib/k8s/read", () => ({ get: vi.fn(), list: vi.fn() }));
vi.mock("@/lib/observe/prometheus", () => ({ promRange: vi.fn() }));
vi.mock("@/lib/observe/kube", () => ({ requestPods: vi.fn() }));
vi.mock("@/lib/settings", () => ({ getSettings: vi.fn() }));

const NOW = 1_700_000_000_000;
const PROMETHEUS = {
  "prometheus.url": "http://prometheus:9090",
} as unknown as Settings;

const cluster = (version?: string) =>
  ({
    metadata: { name: "orders", namespace: "team-a" },
    status: version ? { management: { version } } : {},
  }) as unknown as Resource<"CamundaCluster">;

const broker = (n: number) =>
  ({
    metadata: { name: `orders-zeebe-${n}` },
    spec: {
      volumes: [
        {
          name: "data",
          persistentVolumeClaim: { claimName: `data-orders-zeebe-${n}` },
        },
        { name: "config", configMap: { name: "orders-zeebe" } },
      ],
    },
  }) as unknown as Resource<"Pod">;

const request = {
  metadata: {
    name: "orders-backup-storage",
    namespace: "team-a",
    generation: 1,
  },
  spec: {
    registration: "rustfs-bucket",
    claimRef: { name: "orders" },
    target: {},
  },
  status: {
    outputs: { bucket: "b-1" },
    conditions: [
      { type: "OutputsReady", status: "True", observedGeneration: 1 },
    ],
  },
} as unknown as Resource<"PluginRequest">;

const registration = {
  metadata: { name: "rustfs-bucket" },
  spec: {
    label: "Bucket on RustFS",
    metricQueries: [
      {
        label: "Requests",
        query:
          'sum(rate(rustfs_requests_total{namespace="{{ .namespace }}",pod=~"{{ .pods }}",bucket="{{ .outputs.bucket }}"}[5m]))',
        unit: "req/s",
      },
      { label: "Bad", query: "{{ if .namespace }}x{{ end }}" },
    ],
  },
} as unknown as Resource<"PluginRegistration">;

const one: Series[] = [{ name: 'pod="orders-zeebe-0"', points: [[1, 2]] }];

function objects(found: Partial<Record<string, unknown>>) {
  vi.mocked(get).mockImplementation(
    async (kind: string) => found[kind] as never,
  );
}

function lists(found: Partial<Record<string, unknown[]>>) {
  vi.mocked(list).mockImplementation(
    async (kind: string) => (found[kind] ?? []) as never,
  );
}

const queries = () => vi.mocked(promRange).mock.calls.map(([q]) => q);

beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(getSettings).mockResolvedValue(PROMETHEUS);
  vi.mocked(promRange).mockResolvedValue(one);
});

describe("chartsOf", () => {
  const def = (label: string, declaredBy?: string) => ({
    label,
    query: "q",
    source: "s",
    ...(declaredBy && { declaredBy }),
  });

  it("gives one chart for each query with a series, named by the pod", () =>
    expect(chartsOf([def("CPU")], [one], [])).toEqual({
      prometheus: "answered",
      charts: [
        {
          label: "CPU",
          source: "s",
          series: [{ name: "orders-zeebe-0", points: [[1, 2]] }],
        },
      ],
      errors: [],
    }));

  it("keeps the unit and the registration that declared the query", () =>
    expect(
      chartsOf(
        [{ ...def("Requests", "Bucket on RustFS"), unit: "req/s" }],
        [one],
        [],
      ),
    ).toMatchObject({
      charts: [
        { label: "Requests", unit: "req/s", declaredBy: "Bucket on RustFS" },
      ],
    }));

  it("hides the chart of a query without a series", () =>
    expect(
      chartsOf([def("CPU"), def("Disk used")], [one, []], []),
    ).toMatchObject({
      charts: [{ label: "CPU" }],
    }));

  it("hides the chart of one query that Prometheus refused", () =>
    expect(
      chartsOf([def("CPU"), def("Memory")], [one, "unavailable"], []),
    ).toMatchObject({ prometheus: "answered", charts: [{ label: "CPU" }] }));

  it("is unavailable when Prometheus answered no query", () =>
    expect(chartsOf([def("CPU")], ["unavailable"], [])).toEqual({
      prometheus: "unavailable",
    }));

  it("answers with no charts when there is no query", () =>
    expect(chartsOf([], [], [])).toEqual({
      prometheus: "answered",
      charts: [],
      errors: [],
    }));

  it("keeps a series name with more than one label as it is", () =>
    expect(
      chartsOf(
        [def("x")],
        [[{ name: 'partition="1", pod="p"', points: [[1, 1]] }]],
        [],
      ),
    ).toMatchObject({
      charts: [{ series: [{ name: 'partition="1", pod="p"' }] }],
    }));

  it.each([
    ["no points", []],
    [
      "only values that are not numbers",
      [
        [1, Number.NaN],
        [2, Infinity],
      ],
    ],
  ] as const)("hides the chart of a query whose series have %s", (_, points) =>
    expect(
      chartsOf(
        [def("CPU")],
        [[{ name: 'pod="p"', points: [...points] as [number, number][] }]],
        [],
      ),
    ).toMatchObject({ prometheus: "answered", charts: [] }),
  );

  it("leaves out a series without a point and keeps the others", () =>
    expect(
      chartsOf(
        [def("CPU")],
        [
          [
            { name: 'pod="a"', points: [] },
            { name: 'pod="b"', points: [[1, 2]] },
          ],
        ],
        [],
      ),
    ).toMatchObject({ charts: [{ series: [{ name: "b" }] }] }));

  it("keeps the errors", () => {
    const errors = [{ label: "Bad", declaredBy: "R", error: "e" }];
    expect(chartsOf([], [], errors)).toEqual({
      prometheus: "answered",
      charts: [],
      errors,
    });
  });
});

describe("podVolumes", () => {
  it("lists the claims of the volumes of the pods once, sorted", () =>
    expect(podVolumes([broker(1), broker(0), broker(1)])).toEqual([
      "data-orders-zeebe-0",
      "data-orders-zeebe-1",
    ]));

  it("is empty for pods without a claim", () =>
    expect(
      podVolumes([{ metadata: { name: "p" } } as Resource<"Pod">]),
    ).toEqual([]));
});

describe("componentMetrics", () => {
  it("says Prometheus is not configured and asks it nothing without prometheus.url", async () => {
    objects({ CamundaCluster: cluster("8.9.4") });
    vi.mocked(getSettings).mockResolvedValue({} as Settings);

    expect(
      await componentMetrics("team-a", "orders", "zeebe", "1h", NOW),
    ).toEqual({ prometheus: "notConfigured" });
    expect(promRange).not.toHaveBeenCalled();
    expect(list).not.toHaveBeenCalled();
  });

  it("is undefined for a cluster that does not exist", async () => {
    objects({});

    expect(
      await componentMetrics("team-a", "gone", "zeebe", "1h", NOW),
    ).toBeUndefined();
    expect(get).toHaveBeenCalledWith("CamundaCluster", "team-a", "gone");
  });

  it("renders the broker queries of the cluster version for its pods and volumes over the range", async () => {
    objects({ CamundaCluster: cluster("8.9.4") });
    lists({ Pod: [broker(0), broker(1)] });

    const answer = await componentMetrics(
      "team-a",
      "orders",
      "zeebe",
      "6h",
      NOW,
    );

    expect(list).toHaveBeenCalledWith("Pod", {
      namespace: "team-a",
      labelSelector: "camunda.io/cluster=orders,camunda.io/component=zeebe",
    });
    expect(queries()).toHaveLength(8);
    expect(queries()).toContainEqual(
      expect.stringContaining('pod=~"orders-zeebe-0|orders-zeebe-1"'),
    );
    expect(queries()).toContainEqual(
      expect.stringContaining(
        'persistentvolumeclaim=~"data-orders-zeebe-0|data-orders-zeebe-1"',
      ),
    );
    expect(vi.mocked(promRange).mock.calls[0][1]).toEqual({
      start: NOW - 21_600_000,
      end: NOW,
      stepSeconds: 120,
    });
    expect(answer).toMatchObject({ prometheus: "answered", errors: [] });
  });

  it.each([
    ["1h", 3_600_000, 30],
    ["24h", 86_400_000, 300],
  ] as const)("reads the range %s", async (range, ms, step) => {
    objects({ CamundaCluster: cluster("8.9.4") });
    lists({ Pod: [broker(0)] });

    await componentMetrics("team-a", "orders", "gateway", range, NOW);

    expect(vi.mocked(promRange).mock.calls[0][1]).toEqual({
      start: NOW - ms,
      end: NOW,
      stepSeconds: step,
    });
  });

  it("charts only the pod queries of a cluster without a version, and says so", async () => {
    objects({ CamundaCluster: cluster() });
    lists({ Pod: [broker(0)] });

    expect(
      await componentMetrics("team-a", "orders", "zeebe", "1h", NOW),
    ).toMatchObject({ hiddenForVersion: {} });
    expect(vi.mocked(promRange).mock.calls).toHaveLength(3);
  });

  it("names the version for which the Camunda queries are left out", async () => {
    objects({ CamundaCluster: cluster("8.10.0") });
    lists({ Pod: [broker(0)] });

    expect(
      await componentMetrics("team-a", "orders", "zeebe", "1h", NOW),
    ).toMatchObject({ hiddenForVersion: { version: "8.10.0" } });
  });

  it.each([
    ["the brokers of a version in range", "zeebe", "8.9.4"],
    ["a component without Camunda queries", "gateway", "8.10.0"],
  ] as const)(
    "leaves no version note for %s",
    async (_, component, version) => {
      objects({ CamundaCluster: cluster(version) });
      lists({ Pod: [broker(0)] });

      expect(
        await componentMetrics("team-a", "orders", component, "1h", NOW),
      ).not.toHaveProperty("hiddenForVersion");
    },
  );

  it("answers no charts for a component without pods", async () => {
    objects({ CamundaCluster: cluster("8.9.4") });
    lists({});

    expect(
      await componentMetrics("team-a", "orders", "zeebe", "1h", NOW),
    ).toEqual({ prometheus: "answered", charts: [], errors: [] });
    expect(promRange).not.toHaveBeenCalled();
  });

  it("selects the pods of the CamundaOptimize of the cluster", async () => {
    objects({ CamundaCluster: cluster("8.9.4") });
    lists({
      CamundaOptimize: [
        {
          metadata: { name: "o", uid: "u-1" },
          spec: { clusterRef: { name: "orders" } },
        },
      ],
    });

    await componentMetrics("team-a", "orders", "optimize", "1h", NOW);

    expect(list).toHaveBeenCalledWith("CamundaOptimize", {
      namespace: "team-a",
    });
    expect(list).toHaveBeenCalledWith("Pod", {
      namespace: "team-a",
      labelSelector:
        "camunda.io/cluster=orders,camunda.io/component in (optimize-webapp,optimize-importer),camunda.io/optimize-uid=u-1",
    });
  });

  it("answers no charts for Optimize when no CamundaOptimize names the cluster", async () => {
    objects({ CamundaCluster: cluster("8.9.4") });
    lists({
      CamundaOptimize: [
        {
          metadata: { name: "o", uid: "u-1" },
          spec: { clusterRef: { name: "other" } },
        },
      ],
      Pod: [broker(0)],
    });

    expect(
      await componentMetrics("team-a", "orders", "optimize", "1h", NOW),
    ).toEqual({ prometheus: "answered", charts: [], errors: [] });
    expect(promRange).not.toHaveBeenCalled();
  });
});

describe("requestMetrics", () => {
  beforeEach(() => vi.mocked(requestPods).mockResolvedValue(["rustfs-0"]));

  it("charts the pod queries of the request pods and the queries of its registration", async () => {
    objects({ PluginRequest: request, PluginRegistration: registration });

    const answer = await requestMetrics(
      "team-a",
      "orders-backup-storage",
      "1h",
      NOW,
    );

    expect(get).toHaveBeenCalledWith(
      "PluginRequest",
      "team-a",
      "orders-backup-storage",
    );
    expect(get).toHaveBeenCalledWith(
      "PluginRegistration",
      undefined,
      "rustfs-bucket",
    );
    expect(answer).toMatchObject({
      prometheus: "answered",
      charts: [
        { label: "CPU" },
        { label: "Memory" },
        { label: "Requests", unit: "req/s", declaredBy: "Bucket on RustFS" },
      ],
    });
    expect(queries()).toContainEqual(
      expect.stringContaining('pod=~"rustfs-0",bucket="b-1"'),
    );
  });

  it("names a registration query that does not render, with its reason", async () => {
    objects({ PluginRequest: request, PluginRegistration: registration });

    expect(
      await requestMetrics("team-a", "orders-backup-storage", "1h", NOW),
    ).toMatchObject({
      errors: [
        {
          label: "Bad",
          declaredBy: "Bucket on RustFS",
          error: expect.stringContaining("if"),
        },
      ],
    });
  });

  it("names a registration without a label by its name", async () => {
    objects({
      PluginRequest: request,
      PluginRegistration: {
        ...registration,
        spec: { ...registration.spec, label: "" },
      },
    });

    expect(
      await requestMetrics("team-a", "orders-backup-storage", "1h", NOW),
    ).toMatchObject({ errors: [{ declaredBy: "rustfs-bucket" }] });
  });

  it("charts only the pod queries when the registration is gone", async () => {
    objects({ PluginRequest: request });

    expect(
      await requestMetrics("team-a", "orders-backup-storage", "1h", NOW),
    ).toEqual({
      prometheus: "answered",
      charts: [
        expect.objectContaining({ label: "CPU" }),
        expect.objectContaining({ label: "Memory" }),
      ],
      errors: [],
    });
  });

  it("says Prometheus is not configured and asks it nothing without prometheus.url", async () => {
    objects({ PluginRequest: request, PluginRegistration: registration });
    vi.mocked(getSettings).mockResolvedValue({} as Settings);

    expect(
      await requestMetrics("team-a", "orders-backup-storage", "1h", NOW),
    ).toEqual({ prometheus: "notConfigured" });
    expect(requestPods).not.toHaveBeenCalled();
    expect(promRange).not.toHaveBeenCalled();
  });

  it("is undefined for a request that does not exist", async () => {
    objects({ PluginRegistration: registration });

    expect(await requestMetrics("team-a", "gone", "1h", NOW)).toBeUndefined();
  });
});
