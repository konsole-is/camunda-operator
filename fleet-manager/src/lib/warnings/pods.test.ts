import { describe, expect, it } from "vitest";
import type { Resource } from "@/lib/k8s/kinds";
import type { PodUsage } from "@/lib/observe/types";
import { backpressureWarning, containerWarnings, limitWarnings } from "./pods";

const NOW = "2026-11-10T12:00:00Z";

const minutesAgo = (minutes: number) =>
  new Date(Date.parse(NOW) - minutes * 60_000).toISOString();

const usage = ({
  memoryShare,
  cpuShare,
}: {
  memoryShare?: number;
  cpuShare?: number;
}): PodUsage => ({
  pod: "orders-zeebe-0",
  cpu:
    cpuShare === undefined
      ? { usedCores: 0.1 }
      : { usedCores: cpuShare * 2, limitCores: 2 },
  memory:
    memoryShare === undefined
      ? { usedBytes: 100 }
      : { usedBytes: memoryShare * 1000, limitBytes: 1000 },
});

type ContainerState = {
  name: string;
  restartCount: number;
  reason?: string;
  finishedAt?: string;
};

const pod = (
  containers: ContainerState[],
  initContainers: ContainerState[] = [],
): Resource<"Pod"> =>
  ({
    apiVersion: "v1",
    kind: "Pod",
    metadata: { namespace: "team-a", name: "orders-zeebe-0" },
    status: {
      initContainerStatuses: initContainers.map(containerStatus),
      containerStatuses: containers.map(containerStatus),
    },
  }) as Resource<"Pod">;

const containerStatus = (c: ContainerState) => ({
  name: c.name,
  image: "camunda/camunda",
  imageID: "",
  ready: true,
  restartCount: c.restartCount,
  ...(c.finishedAt
    ? {
        lastState: {
          terminated: {
            exitCode: 137,
            reason: c.reason ?? "Error",
            finishedAt: c.finishedAt,
          },
        },
      }
    : {}),
});

const kinds = (warnings: { kind: string; severity: string }[]) =>
  warnings.map((w) => [w.kind, w.severity]);

describe("limitWarnings", () => {
  it.each([
    [0.74, []],
    [0.75, [["memory", "Warning"]]],
    [0.89, [["memory", "Warning"]]],
    [0.9, [["memory", "Error"]]],
  ])("a memory use of %s of the limit gives %j", (share, expected) => {
    expect(kinds(limitWarnings(usage({ memoryShare: share })))).toEqual(
      expected,
    );
  });

  it.each([
    [0.74, []],
    [0.75, [["cpu", "Warning"]]],
    [0.89, [["cpu", "Warning"]]],
    [0.9, [["cpu", "Error"]]],
  ])("a CPU use of %s of the limit gives %j", (share, expected) => {
    expect(kinds(limitWarnings(usage({ cpuShare: share })))).toEqual(expected);
  });

  it("gives no limit warning without a limit", () => {
    expect(limitWarnings(usage({}))).toEqual([]);
  });

  it("names the pod and the share rounded down in the detail", () => {
    expect(limitWarnings(usage({ memoryShare: 0.899 }))).toEqual([
      {
        kind: "memory",
        severity: "Warning",
        label: "Near memory limit",
        detail: "Pod orders-zeebe-0 uses 89 % of its memory limit",
      },
    ]);
    expect(limitWarnings(usage({ cpuShare: 0.95 }))[0]).toMatchObject({
      label: "Near CPU limit",
      detail: "Pod orders-zeebe-0 uses 95 % of its CPU limit",
    });
  });
});

describe("containerWarnings", () => {
  it("gives an out of memory error for an OOMKilled container of the last hour", () => {
    const finishedAt = minutesAgo(12);
    expect(
      containerWarnings(
        pod([
          { name: "zeebe", restartCount: 1, reason: "OOMKilled", finishedAt },
        ]),
        NOW,
      ),
    ).toEqual([
      {
        kind: "oom",
        severity: "Error",
        label: "Out of memory",
        detail: "Container zeebe of pod orders-zeebe-0 ran out of memory",
        since: finishedAt,
      },
    ]);
  });

  it("gives no out of memory error for an OOMKilled container of more than one hour ago", () => {
    expect(
      containerWarnings(
        pod([
          {
            name: "zeebe",
            restartCount: 1,
            reason: "OOMKilled",
            finishedAt: minutesAgo(61),
          },
        ]),
        NOW,
      ),
    ).toEqual([]);
  });

  it.each([
    [3, false],
    [4, true],
  ])(
    "a container with %s restarts in the last hour warns: %s",
    (restartCount, warns) => {
      const finishedAt = minutesAgo(5);
      const warnings = containerWarnings(
        pod([{ name: "zeebe", restartCount, finishedAt }]),
        NOW,
      );
      expect(warnings).toEqual(
        warns
          ? [
              {
                kind: "restarts",
                severity: "Warning",
                label: "Restarts",
                detail: `Container zeebe of pod orders-zeebe-0 restarted ${restartCount} times`,
                since: finishedAt,
              },
            ]
          : [],
      );
    },
  );

  it("gives no restarts warning when the last restart is more than one hour ago", () => {
    expect(
      containerWarnings(
        pod([{ name: "zeebe", restartCount: 10, finishedAt: minutesAgo(61) }]),
        NOW,
      ),
    ).toEqual([]);
  });

  it("gives no restarts warning for a container that never terminated", () => {
    expect(
      containerWarnings(pod([{ name: "zeebe", restartCount: 10 }]), NOW),
    ).toEqual([]);
  });

  it("reads the init containers too, such as a native sidecar", () => {
    const finishedAt = minutesAgo(5);
    expect(
      kinds(
        containerWarnings(
          pod(
            [{ name: "zeebe", restartCount: 0 }],
            [
              {
                name: "proxy",
                restartCount: 5,
                reason: "OOMKilled",
                finishedAt,
              },
            ],
          ),
          NOW,
        ),
      ),
    ).toEqual([
      ["oom", "Error"],
      ["restarts", "Warning"],
    ]);
  });

  it("gives a warning for each container", () => {
    const finishedAt = minutesAgo(5);
    expect(
      kinds(
        containerWarnings(
          pod([
            { name: "zeebe", restartCount: 1, reason: "OOMKilled", finishedAt },
            { name: "sidecar", restartCount: 7, finishedAt },
          ]),
          NOW,
        ),
      ),
    ).toEqual([
      ["oom", "Error"],
      ["restarts", "Warning"],
    ]);
  });
});

describe("backpressureWarning", () => {
  it.each([
    [0.01, undefined],
    [0.0101, "Warning"],
  ])("a backpressure share of %s gives %s", (share, severity) => {
    expect(backpressureWarning(share)?.severity).toBe(severity);
  });

  it("gives the share of dropped requests in the detail", () => {
    expect(backpressureWarning(0.025)).toEqual({
      kind: "backpressure",
      severity: "Warning",
      label: "Backpressure",
      detail:
        "2 % of the requests were dropped by backpressure in the last 5 minutes",
    });
  });

  it("gives no warning for a share that is not a finite number", () => {
    expect(backpressureWarning(Number.NaN)).toBeUndefined();
    expect(backpressureWarning(Number.POSITIVE_INFINITY)).toBeUndefined();
  });
});
