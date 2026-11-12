import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Resource } from "@/lib/k8s/kinds";
import { list } from "@/lib/k8s/read";
import { CLUSTER_LABEL, COMPONENT_LABEL } from "@/lib/labels";
import { ownerName } from "@/lib/labels/server";
import { promRange } from "./prometheus";
import { podUsage } from "./readers";
import type { PodUsage } from "./types";
import { fleetPodWarnings } from "./warnings";

vi.mock("@/lib/k8s/read", () => ({ list: vi.fn() }));
vi.mock("./readers", () => ({ podUsage: vi.fn() }));
vi.mock("./prometheus", () => ({ promRange: vi.fn() }));

const NOW = new Date("2026-11-10T12:00:00Z");
const VERSION = "8.9.4";

const cluster = (namespace: string, name: string, version?: string) =>
  ({
    metadata: { namespace, name },
    spec: {},
    status: version ? { management: { version } } : {},
  }) as unknown as Resource<"CamundaCluster">;

const pod = (
  namespace: string,
  cluster: string,
  name: string,
  {
    component = "zeebe",
    oomMinutesAgo,
  }: { component?: string; oomMinutesAgo?: number } = {},
) =>
  ({
    metadata: {
      namespace,
      name,
      labels: {
        [CLUSTER_LABEL]: ownerName(cluster),
        [COMPONENT_LABEL]: component,
      },
    },
    status: {
      containerStatuses: [
        {
          name: "zeebe",
          restartCount: 1,
          ...(oomMinutesAgo === undefined
            ? {}
            : {
                lastState: {
                  terminated: {
                    exitCode: 137,
                    reason: "OOMKilled",
                    finishedAt: new Date(
                      NOW.getTime() - oomMinutesAgo * 60_000,
                    ).toISOString(),
                  },
                },
              }),
        },
      ],
    },
  }) as unknown as Resource<"Pod">;

const usage = (name: string, memoryShare: number): PodUsage => ({
  pod: name,
  cpu: { usedCores: 0.1 },
  memory: { usedBytes: memoryShare * 1000, limitBytes: 1000 },
});

function fleet(
  clusters: Resource<"CamundaCluster">[],
  pods: Resource<"Pod">[],
) {
  vi.mocked(list).mockImplementation((async (kind: string) =>
    kind === "CamundaCluster" ? clusters : pods) as typeof list);
}

const NOW_MS = NOW.getTime();
const AGO_60S = NOW_MS - 60_000;

const rates = (
  kind: "received" | "dropped",
  namespace: string,
  pod: string,
  points: [number, number][],
) => ({
  name: `kind="${kind}", namespace="${namespace}", pod="${pod}"`,
  points,
});

const kinds = (warnings: { kind: string }[] | undefined) =>
  (warnings ?? []).map((w) => w.kind);

beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(podUsage).mockResolvedValue([]);
  vi.mocked(promRange).mockResolvedValue([]);
});

describe("fleetPodWarnings", () => {
  it("gives the warnings of the pods of each cluster", async () => {
    fleet(
      [cluster("team-a", "orders", VERSION), cluster("team-a", "billing")],
      [
        pod("team-a", "orders", "orders-zeebe-0", { oomMinutesAgo: 12 }),
        pod("team-a", "billing", "billing-zeebe-0"),
      ],
    );

    const answer = await fleetPodWarnings(NOW);

    expect(kinds(answer.clusters["team-a/orders"])).toEqual(["oom"]);
    expect(kinds(answer.clusters["team-a/billing"])).toEqual([]);
    expect(answer.missing).toEqual([]);
    expect(list).toHaveBeenCalledWith("Pod", { labelSelector: CLUSTER_LABEL });
  });

  it("finds the pods of a cluster with a long name by its owner label", async () => {
    const name = "o".repeat(70);
    fleet(
      [cluster("team-a", name)],
      [pod("team-a", name, "long-zeebe-0", { oomMinutesAgo: 1 })],
    );

    const answer = await fleetPodWarnings(NOW);

    expect(kinds(answer.clusters[`team-a/${name}`])).toEqual(["oom"]);
  });

  it("does not give a pod to a cluster of the same name in another namespace", async () => {
    fleet(
      [cluster("team-a", "orders"), cluster("team-b", "orders")],
      [pod("team-b", "orders", "orders-zeebe-0", { oomMinutesAgo: 1 })],
    );

    const answer = await fleetPodWarnings(NOW);

    expect(kinds(answer.clusters["team-a/orders"])).toEqual([]);
    expect(kinds(answer.clusters["team-b/orders"])).toEqual(["oom"]);
  });

  it("gives a near limit warning from the usage of the namespace", async () => {
    fleet(
      [cluster("team-a", "orders")],
      [pod("team-a", "orders", "orders-zeebe-0")],
    );
    vi.mocked(podUsage).mockResolvedValue([usage("orders-zeebe-0", 0.8)]);

    const answer = await fleetPodWarnings(NOW);

    expect(answer.clusters["team-a/orders"]).toEqual([
      expect.objectContaining({ kind: "memory", severity: "Warning" }),
    ]);
    expect(podUsage).toHaveBeenCalledTimes(1);
    expect(podUsage).toHaveBeenCalledWith("team-a", CLUSTER_LABEL);
  });

  it("does not read the usage of a namespace without cluster pods", async () => {
    fleet([cluster("team-a", "orders")], []);

    await fleetPodWarnings(NOW);

    expect(podUsage).not.toHaveBeenCalled();
  });

  it("gives a backpressure warning above 1 %", async () => {
    fleet(
      [cluster("team-a", "orders", VERSION)],
      [
        pod("team-a", "orders", "orders-zeebe-0"),
        pod("team-a", "orders", "orders-gateway-0", { component: "gateway" }),
      ],
    );
    vi.mocked(promRange).mockResolvedValue([
      rates("received", "team-a", "orders-zeebe-0", [
        [AGO_60S, 100],
        [NOW_MS, 100],
      ]),
      rates("dropped", "team-a", "orders-zeebe-0", [
        [AGO_60S, 0.5],
        [NOW_MS, 2],
      ]),
    ]);

    const answer = await fleetPodWarnings(NOW);

    expect(answer.clusters["team-a/orders"]).toEqual([
      expect.objectContaining({ kind: "backpressure", severity: "Warning" }),
    ]);
    expect(promRange).toHaveBeenCalledWith(
      'label_replace(sum by (namespace, pod) (rate(zeebe_received_request_count_total[5m])), "kind", "received", "", "") or label_replace(sum by (namespace, pod) (rate(zeebe_dropped_request_count_total[5m])), "kind", "dropped", "", "")',
      { start: NOW_MS - 300_000, end: NOW_MS, stepSeconds: 60 },
    );
  });

  it("asks Prometheus once for the whole fleet and gives each cluster its own share", async () => {
    fleet(
      [
        cluster("team-a", "orders", VERSION),
        cluster("team-a", "billing", VERSION),
        cluster("team-b", "orders", VERSION),
      ],
      [
        pod("team-a", "orders", "orders-zeebe-0"),
        pod("team-a", "billing", "billing-zeebe-0"),
        pod("team-b", "orders", "orders-zeebe-0"),
      ],
    );
    vi.mocked(promRange).mockResolvedValue([
      rates("received", "team-a", "orders-zeebe-0", [[NOW_MS, 100]]),
      rates("dropped", "team-a", "orders-zeebe-0", [[NOW_MS, 5]]),
      rates("received", "team-a", "billing-zeebe-0", [[NOW_MS, 100]]),
      rates("dropped", "team-a", "billing-zeebe-0", [[NOW_MS, 0.5]]),
      rates("received", "team-b", "orders-zeebe-0", [[NOW_MS, 100]]),
      rates("dropped", "team-b", "orders-zeebe-0", [[NOW_MS, 2]]),
    ]);

    const answer = await fleetPodWarnings(NOW);

    expect(promRange).toHaveBeenCalledTimes(1);
    expect(answer.clusters["team-a/orders"]).toEqual([
      expect.objectContaining({
        kind: "backpressure",
        detail: expect.stringMatching(/^5 % /),
      }),
    ]);
    expect(kinds(answer.clusters["team-a/billing"])).toEqual([]);
    expect(answer.clusters["team-b/orders"]).toEqual([
      expect.objectContaining({
        kind: "backpressure",
        detail: expect.stringMatching(/^2 % /),
      }),
    ]);
  });

  it("divides the dropped by the received requests of all the brokers of a cluster", async () => {
    fleet(
      [cluster("team-a", "orders", VERSION)],
      [
        pod("team-a", "orders", "orders-zeebe-0"),
        pod("team-a", "orders", "orders-zeebe-1"),
      ],
    );
    // Each broker alone is above 1 %, the two together are not.
    vi.mocked(promRange).mockResolvedValue([
      rates("received", "team-a", "orders-zeebe-0", [[NOW_MS, 10]]),
      rates("dropped", "team-a", "orders-zeebe-0", [[NOW_MS, 0.2]]),
      rates("received", "team-a", "orders-zeebe-1", [[NOW_MS, 1000]]),
      rates("dropped", "team-a", "orders-zeebe-1", [[NOW_MS, 0]]),
    ]);

    const answer = await fleetPodWarnings(NOW);

    expect(kinds(answer.clusters["team-a/orders"])).toEqual([]);
  });

  it("counts only the series of the broker pods of a cluster", async () => {
    fleet(
      [cluster("team-a", "orders", VERSION)],
      [
        pod("team-a", "orders", "orders-zeebe-0"),
        pod("team-a", "orders", "orders-gateway-0", { component: "gateway" }),
      ],
    );
    vi.mocked(promRange).mockResolvedValue([
      rates("received", "team-a", "orders-zeebe-0", [[NOW_MS, 100]]),
      rates("dropped", "team-a", "orders-zeebe-0", [[NOW_MS, 0]]),
      rates("dropped", "team-a", "orders-gateway-0", [[NOW_MS, 50]]),
      rates("dropped", "team-b", "orders-zeebe-0", [[NOW_MS, 50]]),
      rates("dropped", "team-a", "other-zeebe-0", [[NOW_MS, 50]]),
      {
        name: 'namespace="team-a", pod="orders-zeebe-0"',
        points: [[NOW_MS, 50]],
      },
    ]);

    const answer = await fleetPodWarnings(NOW);

    expect(kinds(answer.clusters["team-a/orders"])).toEqual([]);
  });

  it("gives no backpressure warning to a cluster without series", async () => {
    fleet(
      [
        cluster("team-a", "orders", VERSION),
        cluster("team-a", "billing", VERSION),
      ],
      [
        pod("team-a", "orders", "orders-zeebe-0"),
        pod("team-a", "billing", "billing-zeebe-0"),
      ],
    );
    vi.mocked(promRange).mockResolvedValue([
      rates("received", "team-a", "orders-zeebe-0", [[NOW_MS, 100]]),
      rates("dropped", "team-a", "orders-zeebe-0", [[NOW_MS, 5]]),
    ]);

    const answer = await fleetPodWarnings(NOW);

    expect(kinds(answer.clusters["team-a/orders"])).toEqual(["backpressure"]);
    expect(kinds(answer.clusters["team-a/billing"])).toEqual([]);
    expect(answer.missing).toEqual([]);
  });

  it("gives no backpressure warning while the brokers drop or receive nothing", async () => {
    fleet(
      [cluster("team-a", "orders", VERSION)],
      [pod("team-a", "orders", "orders-zeebe-0")],
    );
    vi.mocked(promRange).mockResolvedValue([
      rates("received", "team-a", "orders-zeebe-0", [[NOW_MS, 0]]),
      rates("dropped", "team-a", "orders-zeebe-0", [[NOW_MS, 0]]),
    ]);

    const answer = await fleetPodWarnings(NOW);

    expect(kinds(answer.clusters["team-a/orders"])).toEqual([]);
  });

  it("gives no backpressure warning without a dropped series", async () => {
    fleet(
      [cluster("team-a", "orders", VERSION)],
      [pod("team-a", "orders", "orders-zeebe-0")],
    );
    vi.mocked(promRange).mockResolvedValue([
      rates("received", "team-a", "orders-zeebe-0", [[NOW_MS, 100]]),
    ]);

    const answer = await fleetPodWarnings(NOW);

    expect(kinds(answer.clusters["team-a/orders"])).toEqual([]);
  });

  it("gives no backpressure warning when Prometheus gives no series", async () => {
    fleet(
      [cluster("team-a", "orders", VERSION)],
      [pod("team-a", "orders", "orders-zeebe-0")],
    );
    vi.mocked(promRange).mockResolvedValue([]);

    const answer = await fleetPodWarnings(NOW);

    expect(kinds(answer.clusters["team-a/orders"])).toEqual([]);
    expect(answer.missing).toEqual([]);
  });

  it("gives no backpressure warning from a point older than the last step", async () => {
    fleet(
      [cluster("team-a", "orders", VERSION)],
      [pod("team-a", "orders", "orders-zeebe-0")],
    );
    // The brokers got no request in the last two minutes, so the newest
    // points have no value and Prometheus leaves them out.
    vi.mocked(promRange).mockResolvedValue([
      rates("received", "team-a", "orders-zeebe-0", [[NOW_MS - 120_000, 10]]),
      rates("dropped", "team-a", "orders-zeebe-0", [[NOW_MS - 120_000, 5]]),
    ]);

    const answer = await fleetPodWarnings(NOW);

    expect(kinds(answer.clusters["team-a/orders"])).toEqual([]);
  });

  it("takes the share of the newest point that has both rates", async () => {
    fleet(
      [cluster("team-a", "orders", VERSION)],
      [pod("team-a", "orders", "orders-zeebe-0")],
    );
    vi.mocked(promRange).mockResolvedValue([
      rates("received", "team-a", "orders-zeebe-0", [
        [AGO_60S, 100],
        [NOW_MS, 100],
      ]),
      rates("dropped", "team-a", "orders-zeebe-0", [[AGO_60S, 5]]),
    ]);

    const answer = await fleetPodWarnings(NOW);

    expect(kinds(answer.clusters["team-a/orders"])).toEqual(["backpressure"]);
  });

  it("names metrics-server when the usage is unavailable and gives no near limit warning", async () => {
    fleet(
      [cluster("team-a", "orders")],
      [pod("team-a", "orders", "orders-zeebe-0")],
    );
    vi.mocked(podUsage).mockResolvedValue("unavailable");

    const answer = await fleetPodWarnings(NOW);

    expect(answer.missing).toEqual(["metrics-server"]);
    expect(kinds(answer.clusters["team-a/orders"])).toEqual([]);
  });

  it("names metrics-server when the usage read rejects", async () => {
    fleet(
      [cluster("team-a", "orders")],
      [pod("team-a", "orders", "orders-zeebe-0")],
    );
    vi.mocked(podUsage).mockRejectedValue(new Error("forbidden"));

    const answer = await fleetPodWarnings(NOW);

    expect(answer.missing).toEqual(["metrics-server"]);
  });

  it("names prometheus when the query is unavailable and gives no cluster a backpressure warning", async () => {
    fleet(
      [
        cluster("team-a", "orders", VERSION),
        cluster("team-b", "billing", VERSION),
      ],
      [
        pod("team-a", "orders", "orders-zeebe-0"),
        pod("team-b", "billing", "billing-zeebe-0"),
      ],
    );
    vi.mocked(promRange).mockResolvedValue("unavailable");

    const answer = await fleetPodWarnings(NOW);

    expect(answer.missing).toEqual(["prometheus"]);
    expect(kinds(answer.clusters["team-a/orders"])).toEqual([]);
    expect(kinds(answer.clusters["team-b/billing"])).toEqual([]);
    expect(promRange).toHaveBeenCalledTimes(1);
  });

  it("names prometheus when the query rejects", async () => {
    fleet(
      [cluster("team-a", "orders", VERSION)],
      [pod("team-a", "orders", "orders-zeebe-0")],
    );
    vi.mocked(promRange).mockRejectedValue(new Error("no secret"));

    const answer = await fleetPodWarnings(NOW);

    expect(answer.missing).toEqual(["prometheus"]);
  });

  it("runs no query for a cluster without a version", async () => {
    fleet(
      [cluster("team-a", "orders")],
      [pod("team-a", "orders", "orders-zeebe-0")],
    );

    await fleetPodWarnings(NOW);

    expect(promRange).not.toHaveBeenCalled();
  });

  it("runs no query for a version outside the range of the backpressure query", async () => {
    fleet(
      [cluster("team-a", "orders", "8.10.0")],
      [pod("team-a", "orders", "orders-zeebe-0")],
    );

    await fleetPodWarnings(NOW);

    expect(promRange).not.toHaveBeenCalled();
  });

  it("counts no broker of a cluster outside the range of the backpressure query", async () => {
    fleet(
      [
        cluster("team-a", "orders", VERSION),
        cluster("team-a", "billing", "8.10.0"),
      ],
      [
        pod("team-a", "orders", "orders-zeebe-0"),
        pod("team-a", "billing", "billing-zeebe-0"),
      ],
    );
    vi.mocked(promRange).mockResolvedValue([
      rates("received", "team-a", "billing-zeebe-0", [[NOW_MS, 100]]),
      rates("dropped", "team-a", "billing-zeebe-0", [[NOW_MS, 5]]),
    ]);

    const answer = await fleetPodWarnings(NOW);

    expect(promRange).toHaveBeenCalledTimes(1);
    expect(kinds(answer.clusters["team-a/billing"])).toEqual([]);
  });

  it("runs no query for a cluster without broker pods", async () => {
    fleet(
      [cluster("team-a", "orders", VERSION)],
      [pod("team-a", "orders", "orders-operate-0", { component: "operate" })],
    );

    await fleetPodWarnings(NOW);

    expect(promRange).not.toHaveBeenCalled();
  });

  it("keeps the restart and out of memory warnings when both sources are missing", async () => {
    fleet(
      [cluster("team-a", "orders", VERSION)],
      [pod("team-a", "orders", "orders-zeebe-0", { oomMinutesAgo: 3 })],
    );
    vi.mocked(podUsage).mockResolvedValue("unavailable");
    vi.mocked(promRange).mockResolvedValue("unavailable");

    const answer = await fleetPodWarnings(NOW);

    expect(answer.missing).toEqual(["metrics-server", "prometheus"]);
    expect(kinds(answer.clusters["team-a/orders"])).toEqual(["oom"]);
  });
});
