import http from "node:http";
import type { AddressInfo } from "node:net";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { Resource } from "@/lib/k8s/kinds";
import { list } from "@/lib/k8s/read";
import { promRange } from "@/lib/observe/prometheus";
import { readSecretValues } from "@/lib/secrets/values";
import {
  brokerPartitions,
  mergePartitions,
  partitionHealth,
  withHealth,
} from "./partitions";

vi.mock("@/lib/k8s/read", () => ({ list: vi.fn() }));
vi.mock("@/lib/secrets/values", () => ({ readSecretValues: vi.fn() }));
vi.mock("@/lib/observe/prometheus", () => ({ promRange: vi.fn() }));

describe("mergePartitions", () => {
  it("merges the leader and followers of each partition", () =>
    expect(
      mergePartitions(
        [
          {
            pod: "o-zeebe-0",
            partitions: {
              "1": { role: "LEADER", streamProcessorPhase: "PROCESSING" },
              "2": { role: "FOLLOWER", streamProcessorPhase: null },
            },
          },
          {
            pod: "o-zeebe-1",
            partitions: {
              "1": { role: "FOLLOWER" },
              "2": { role: "LEADER", streamProcessorPhase: "PAUSED" },
            },
          },
        ],
        2,
      ),
    ).toEqual([
      {
        partition: 1,
        leader: "o-zeebe-0",
        followers: ["o-zeebe-1"],
        processorPhase: "PROCESSING",
        health: "unknown",
      },
      {
        partition: 2,
        leader: "o-zeebe-1",
        followers: ["o-zeebe-0"],
        processorPhase: "PAUSED",
        health: "unknown",
      },
    ]));

  it("lists a partition that no answering broker leads, without a leader", () =>
    expect(
      mergePartitions(
        [{ pod: "o-zeebe-0", partitions: { "1": { role: "FOLLOWER" } } }],
        3,
      ),
    ).toEqual(
      [1, 2, 3].map((p) => ({
        partition: p,
        followers: p === 1 ? ["o-zeebe-0"] : [],
        health: "unknown",
      })),
    ));

  it("a role other than LEADER or FOLLOWER counts as neither", () =>
    expect(
      mergePartitions(
        [{ pod: "o-zeebe-0", partitions: { "1": { role: "INACTIVE" } } }],
        1,
      ),
    ).toEqual([{ partition: 1, followers: [], health: "unknown" }]));

  it("leaves out a partition beyond the count", () =>
    expect(
      mergePartitions(
        [{ pod: "o-zeebe-0", partitions: { "4": { role: "LEADER" } } }],
        1,
      ),
    ).toEqual([{ partition: 1, followers: [], health: "unknown" }]));
});

describe("withHealth", () => {
  const partitions = mergePartitions([], 3);

  it.each([
    [1, "healthy"],
    [0, "unhealthy"],
    [-1, "dead"],
    [7, "unknown"],
  ] as const)("reads %i as %s", (value, state) =>
    expect(
      withHealth(partitions, [
        {
          name: 'partition="2"',
          points: [
            [1, 1],
            [2, value],
          ],
        },
      ])[1].health,
    ).toBe(state),
  );

  it("keeps unknown for a partition without a series", () =>
    expect(withHealth(partitions, []).map((p) => p.health)).toEqual([
      "unknown",
      "unknown",
      "unknown",
    ]));

  it("keeps unknown for a series without points", () =>
    expect(
      withHealth(partitions, [{ name: 'partition="1"', points: [] }])[0].health,
    ).toBe("unknown"));
});

type Handler = (req: http.IncomingMessage, res: http.ServerResponse) => void;
let server: http.Server | undefined;

async function serve(handler: Handler): Promise<number> {
  server = http.createServer(handler);
  await new Promise<void>((resolve) => server!.listen(0, "127.0.0.1", resolve));
  return (server.address() as AddressInfo).port;
}

const answer = {
  "1": { role: "LEADER", streamProcessorPhase: "PROCESSING" },
  "2": { role: "FOLLOWER", streamProcessorPhase: null },
};

function broker(seen: http.IncomingMessage[] = []): Handler {
  return (req, res) => {
    seen.push(req);
    res.writeHead(200, { "Content-Type": "application/json" });
    res.end(JSON.stringify(answer));
  };
}

function cluster(
  port: number,
  auth: object = { method: "none" },
): Resource<"CamundaCluster"> {
  return {
    metadata: { name: "orders", namespace: "team-a" },
    status: {
      management: {
        endpoint: `http://orders-zeebe.team-a.svc:${port}/base`,
        partitions: 2,
        version: "8.9.9",
        auth,
      },
    },
  } as unknown as Resource<"CamundaCluster">;
}

const pod = (name: string, podIP?: string) => ({
  metadata: { name },
  status: podIP ? { podIP } : {},
});

beforeEach(() => vi.resetAllMocks());
afterEach(async () => {
  if (server) await new Promise((resolve) => server!.close(resolve));
  server = undefined;
});

describe("partitionHealth", () => {
  it("asks /actuator/partitions on the IP of each broker pod", async () => {
    const seen: http.IncomingMessage[] = [];
    const port = await serve(broker(seen));
    vi.mocked(list).mockResolvedValue([
      pod("orders-zeebe-0", "127.0.0.1"),
    ] as never);

    expect(await partitionHealth(cluster(port))).toEqual([
      {
        partition: 1,
        leader: "orders-zeebe-0",
        followers: [],
        processorPhase: "PROCESSING",
        health: "unknown",
      },
      { partition: 2, followers: ["orders-zeebe-0"], health: "unknown" },
    ]);
    expect(seen.map((r) => r.url)).toEqual(["/base/actuator/partitions"]);
    expect(seen[0].headers.authorization).toBeUndefined();
    expect(list).toHaveBeenCalledWith("Pod", {
      namespace: "team-a",
      labelSelector: "camunda.io/cluster=orders,camunda.io/component=zeebe",
    });
  });

  it("keeps the partitions of the brokers that answer", async () => {
    const port = await serve(broker());
    vi.mocked(list).mockResolvedValue([
      pod("orders-zeebe-0", "127.0.0.1"),
      // Nothing listens on 127.0.0.2, so the connection is refused.
      pod("orders-zeebe-1", "127.0.0.2"),
    ] as never);

    const result = await partitionHealth(cluster(port));

    expect(result).toEqual([
      expect.objectContaining({
        partition: 1,
        leader: "orders-zeebe-0",
        followers: [],
      }),
      expect.objectContaining({ partition: 2, followers: ["orders-zeebe-0"] }),
    ]);
  });

  it("counts an answer that is not an object of partitions as no answer", async () => {
    const port = await serve((_req, res) => res.end("[1,2]"));
    vi.mocked(list).mockResolvedValue([
      pod("orders-zeebe-0", "127.0.0.1"),
    ] as never);

    expect(await partitionHealth(cluster(port))).toBe("unavailable");
  });

  it("returns unavailable when the cluster has no status.management", async () => {
    const noManagement = {
      ...cluster(1),
      status: {},
    } as Resource<"CamundaCluster">;

    expect(await partitionHealth(noManagement)).toBe("unavailable");
    expect(list).not.toHaveBeenCalled();
  });

  it("returns unavailable when no broker pod has an IP", async () => {
    vi.mocked(list).mockResolvedValue([pod("orders-zeebe-0")] as never);

    expect(await partitionHealth(cluster(1))).toBe("unavailable");
  });

  it("returns unavailable when no broker answers", async () => {
    const port = await serve((_req, res) => res.writeHead(503).end());
    vi.mocked(list).mockResolvedValue([
      pod("orders-zeebe-0", "127.0.0.1"),
    ] as never);

    expect(await partitionHealth(cluster(port))).toBe("unavailable");
  });

  it("does not follow a redirect", async () => {
    const port = await serve((_req, res) =>
      res.writeHead(302, { Location: "http://127.0.0.1:1/x" }).end(),
    );
    vi.mocked(list).mockResolvedValue([
      pod("orders-zeebe-0", "127.0.0.1"),
    ] as never);

    expect(await partitionHealth(cluster(port))).toBe("unavailable");
  });

  it("sends basic auth from the Secret of the binding", async () => {
    const seen: http.IncomingMessage[] = [];
    const port = await serve(broker(seen));
    vi.mocked(list).mockResolvedValue([
      pod("orders-zeebe-0", "127.0.0.1"),
    ] as never);
    vi.mocked(readSecretValues).mockResolvedValue({
      username: "u",
      password: "p",
    });

    const result = await partitionHealth(
      cluster(port, {
        method: "basic",
        credentialsSecretRef: { name: "mgmt" },
      }),
    );

    expect(result).not.toBe("unavailable");
    expect(seen[0].headers.authorization).toBe(
      "Basic " + Buffer.from("u:p").toString("base64"),
    );
    expect(readSecretValues).toHaveBeenCalledWith("team-a", "mgmt");
  });

  it("reads the keys that the binding names", async () => {
    const seen: http.IncomingMessage[] = [];
    const port = await serve(broker(seen));
    vi.mocked(list).mockResolvedValue([
      pod("orders-zeebe-0", "127.0.0.1"),
    ] as never);
    vi.mocked(readSecretValues).mockResolvedValue({ user: "u", pass: "p" });

    await partitionHealth(
      cluster(port, {
        method: "basic",
        credentialsSecretRef: {
          name: "mgmt",
          usernameKey: "user",
          passwordKey: "pass",
        },
      }),
    );

    expect(seen[0].headers.authorization).toBe(
      "Basic " + Buffer.from("u:p").toString("base64"),
    );
  });

  it.each([
    ["is missing", undefined],
    ["has no password", { username: "u" }],
  ])("returns unavailable when the basic auth Secret %s", async (_, values) => {
    const seen: http.IncomingMessage[] = [];
    const port = await serve(broker(seen));
    vi.mocked(list).mockResolvedValue([
      pod("orders-zeebe-0", "127.0.0.1"),
    ] as never);
    vi.mocked(readSecretValues).mockResolvedValue(values);

    expect(
      await partitionHealth(
        cluster(port, {
          method: "basic",
          credentialsSecretRef: { name: "mgmt" },
        }),
      ),
    ).toBe("unavailable");
    expect(seen).toEqual([]);
  });

  it("writes an IPv6 pod address in brackets", async () => {
    vi.mocked(list).mockResolvedValue([pod("orders-zeebe-0", "::1")] as never);
    const fetchSpy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(new Response(JSON.stringify(answer)));

    await partitionHealth(cluster(9600));

    expect(String(fetchSpy.mock.calls[0][0])).toBe(
      "http://[::1]:9600/base/actuator/partitions",
    );
    fetchSpy.mockRestore();
  });
});

describe("brokerPartitions", () => {
  const NOW = 1_700_000_000_000;

  it("gives the health of zeebe_health when Prometheus answers", async () => {
    const port = await serve(broker());
    vi.mocked(list).mockResolvedValue([
      pod("orders-zeebe-0", "127.0.0.1"),
    ] as never);
    vi.mocked(promRange).mockResolvedValue([
      { name: 'partition="1"', points: [[NOW, 1]] },
      { name: 'partition="2"', points: [[NOW, -1]] },
    ]);

    const result = await brokerPartitions(cluster(port), NOW);

    expect(result).toEqual([
      expect.objectContaining({ partition: 1, health: "healthy" }),
      expect.objectContaining({ partition: 2, health: "dead" }),
    ]);
    expect(promRange).toHaveBeenCalledWith(
      'min by (partition) (zeebe_health{namespace="team-a", pod=~"orders-zeebe-0"})',
      { start: NOW - 300_000, end: NOW, stepSeconds: 30 },
    );
  });

  it("keeps the health unknown when Prometheus is not set", async () => {
    const port = await serve(broker());
    vi.mocked(list).mockResolvedValue([
      pod("orders-zeebe-0", "127.0.0.1"),
    ] as never);
    vi.mocked(promRange).mockResolvedValue("unavailable");

    expect(await brokerPartitions(cluster(port), NOW)).toEqual([
      expect.objectContaining({
        partition: 1,
        leader: "orders-zeebe-0",
        health: "unknown",
      }),
      expect.objectContaining({ partition: 2, health: "unknown" }),
    ]);
  });

  it("reads the health of every broker pod, also one that does not answer", async () => {
    const port = await serve(broker());
    vi.mocked(list).mockResolvedValue([
      pod("orders-zeebe-0", "127.0.0.1"),
      // Nothing listens on 127.0.0.2, so the connection is refused.
      pod("orders-zeebe-1", "127.0.0.2"),
      pod("orders-zeebe-2"),
    ] as never);
    vi.mocked(promRange).mockResolvedValue([]);

    await brokerPartitions(cluster(port), NOW);

    expect(vi.mocked(promRange).mock.calls[0][0]).toContain(
      'pod=~"orders-zeebe-0|orders-zeebe-1|orders-zeebe-2"',
    );
  });

  it("keeps the partitions with unknown health when the Prometheus read rejects", async () => {
    const port = await serve(broker());
    vi.mocked(list).mockResolvedValue([
      pod("orders-zeebe-0", "127.0.0.1"),
    ] as never);
    vi.mocked(promRange).mockRejectedValue(
      new Error("settings are unreadable"),
    );

    expect(await brokerPartitions(cluster(port), NOW)).toEqual([
      expect.objectContaining({ partition: 1, health: "unknown" }),
      expect.objectContaining({ partition: 2, health: "unknown" }),
    ]);
  });

  it("is unavailable when the management API is", async () => {
    const noManagement = {
      ...cluster(1),
      status: {},
    } as Resource<"CamundaCluster">;

    expect(await brokerPartitions(noManagement, NOW)).toBe("unavailable");
    expect(promRange).not.toHaveBeenCalled();
  });
});
