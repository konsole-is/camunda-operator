import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import type { CamundaClusterSpec } from "@/generated/crds/camundacluster";
import type { Resource } from "@/lib/k8s/kinds";
import { COMPONENT_LABEL } from "@/lib/labels";
import { type Process, type TopologyBox, podDot, topologyOf } from "./topology";

// shape renders the boxes as "Brokers[Gateway,Operate]" strings.
const shape = (boxes: TopologyBox[]) =>
  boxes.map((b) =>
    b.embedded.length
      ? `${b.label}[${b.embedded.map((e) => e.label).join(",")}]`
      : b.label,
  );

type Mode = "Standalone" | "Embedded";
type Case = {
  gateway: Mode;
  operate: Mode;
  tasklist: Mode;
  admin: Mode;
  connectors: boolean;
};

// expected is the oracle of the issue, written apart from the code: Brokers
// always, then each standalone process, then Connectors when enabled. An
// embedded web app is a chip in the gateway when the gateway is standalone,
// else in Brokers.
function expected(c: Case): string[] {
  const host = c.gateway === "Standalone" ? "Gateway" : "Brokers";
  const chips: Record<string, string[]> = { Brokers: [], Gateway: [] };
  if (c.gateway === "Embedded") chips.Brokers.push("Gateway");
  const apps = [
    ["Operate", c.operate],
    ["Tasklist", c.tasklist],
    ["Admin", c.admin],
  ] as const;
  for (const [label, mode] of apps) {
    if (mode === "Embedded") chips[host].push(label);
  }
  const boxes = ["Brokers"];
  if (c.gateway === "Standalone") boxes.push("Gateway");
  for (const [label, mode] of apps)
    if (mode === "Standalone") boxes.push(label);
  if (c.connectors) boxes.push("Connectors");
  return boxes.map((b) =>
    chips[b]?.length ? `${b}[${chips[b].join(",")}]` : b,
  );
}

const modes = ["Standalone", "Embedded"] as const;
const cases: Case[] = modes.flatMap((gateway) =>
  modes.flatMap((operate) =>
    modes.flatMap((tasklist) =>
      modes.flatMap((admin) =>
        [true, false].map((connectors) => ({
          gateway,
          operate,
          tasklist,
          admin,
          connectors,
        })),
      ),
    ),
  ),
);

function pod(
  name: string,
  component: string,
  over: Partial<Resource<"Pod">> = {},
): Resource<"Pod"> {
  return {
    kind: "Pod",
    metadata: {
      name,
      namespace: "team-a",
      labels: { [COMPONENT_LABEL]: component },
    },
    status: {
      phase: "Running",
      conditions: [{ type: "Ready", status: "True" }],
    },
    ...over,
  } as Resource<"Pod">;
}

describe("topologyOf", () => {
  it.each(cases)("boxes for %o", (c) => {
    const spec = {
      gateway: { mode: c.gateway },
      operate: { mode: c.operate },
      tasklist: { mode: c.tasklist },
      admin: { mode: c.admin },
      connectors: { enabled: c.connectors },
    } as CamundaClusterSpec;
    expect(shape(topologyOf(spec, []))).toEqual(expected(c));
  });

  it("an empty spec runs Brokers and a standalone Gateway that hosts the three web apps", () => {
    expect(shape(topologyOf({} as CamundaClusterSpec, []))).toEqual([
      "Brokers",
      "Gateway[Operate,Tasklist,Admin]",
    ]);
  });

  it("an embedded gateway puts the gateway and the web apps in Brokers", () => {
    expect(
      shape(
        topologyOf({ gateway: { mode: "Embedded" } } as CamundaClusterSpec, []),
      ),
    ).toEqual(["Brokers[Gateway,Operate,Tasklist,Admin]"]);
  });

  it("every process standalone with Connectors", () => {
    const spec = {
      gateway: { mode: "Standalone" },
      operate: { mode: "Standalone" },
      tasklist: { mode: "Standalone" },
      admin: { mode: "Standalone" },
      connectors: { enabled: true },
    } as CamundaClusterSpec;
    expect(shape(topologyOf(spec, []))).toEqual([
      "Brokers",
      "Gateway",
      "Operate",
      "Tasklist",
      "Admin",
      "Connectors",
    ]);
  });

  it.each([
    ["disabled", { connectors: { enabled: false } }],
    ["unset", {}],
  ])("%s Connectors have no box", (_, spec) => {
    expect(shape(topologyOf(spec as CamundaClusterSpec, []))).not.toContain(
      "Connectors",
    );
  });

  it("names the workload of each box", () => {
    const boxes = topologyOf({} as CamundaClusterSpec, []);
    expect(boxes.map((b) => [b.process, b.workload])).toEqual([
      ["zeebe", "StatefulSet"],
      ["gateway", "Deployment"],
    ]);
  });

  it("puts each pod in the box of its component, sorted by name", () => {
    const boxes = topologyOf({} as CamundaClusterSpec, [
      pod("orders-zeebe-1", "zeebe"),
      pod("orders-zeebe-0", "zeebe"),
      pod("orders-gateway-x", "gateway"),
    ]);
    expect(boxes.map((b) => b.pods.map((p) => p.name))).toEqual([
      ["orders-zeebe-0", "orders-zeebe-1"],
      ["orders-gateway-x"],
    ]);
  });

  it("leaves out the pods of Optimize", () => {
    const boxes = topologyOf({} as CamundaClusterSpec, [
      pod("orders-optimize-webapp-0", "optimize-webapp"),
      pod("orders-optimize-importer-0", "optimize-importer"),
    ]);
    expect(boxes.flatMap((b) => b.pods)).toEqual([]);
  });

  it("leaves out the pods of a process without a box", () => {
    const boxes = topologyOf({} as CamundaClusterSpec, [
      pod("orders-operate-0", "operate"),
    ]);
    expect(boxes.flatMap((b) => b.pods)).toEqual([]);
  });

  it("PROCESS values match pkg/components/camundacluster/names.go", () => {
    const source = readFileSync(
      new URL(
        "../../../../pkg/components/camundacluster/names.go",
        import.meta.url,
      ),
      "utf8",
    );
    const go = (name: string) =>
      new RegExp(`\\b${name}\\s*=\\s*"([^"]*)"`).exec(source)?.[1];
    const processes: Process[] = [
      "zeebe",
      "gateway",
      "operate",
      "tasklist",
      "admin",
      "connectors",
    ];
    expect(
      [
        "ComponentZeebe",
        "ComponentGateway",
        "ComponentOperate",
        "ComponentTasklist",
        "ComponentAdmin",
        "ComponentConnectors",
      ].map(go),
    ).toEqual(processes);
  });
});

describe("podDot", () => {
  it("is ok and Ready for a ready pod", () => {
    expect(podDot(pod("p", "zeebe"))).toEqual({
      name: "p",
      tone: "ok",
      state: "Ready",
    });
  });

  it("is off for a terminating pod, also when it is ready", () => {
    const p = pod("p", "zeebe");
    p.metadata!.deletionTimestamp = new Date("2026-10-18T10:00:00Z");
    expect(podDot(p)).toMatchObject({ tone: "off", state: "Terminating" });
  });

  it.each([
    ["CrashLoopBackOff", "bad"],
    ["ImagePullBackOff", "bad"],
    ["ErrImagePull", "bad"],
    ["CreateContainerConfigError", "bad"],
    ["CreateContainerError", "bad"],
    ["ContainerCreating", "warn"],
  ])("shows the waiting reason %s with the tone %s", (reason, tone) => {
    const p = pod("p", "zeebe", {
      status: {
        phase: "Running",
        containerStatuses: [
          {
            name: "zeebe",
            image: "camunda/zeebe",
            imageID: "",
            ready: false,
            restartCount: 3,
            state: { waiting: { reason } },
          },
        ],
      },
    });
    expect(podDot(p)).toMatchObject({ tone, state: reason });
  });

  it("is bad for a Failed pod", () => {
    expect(
      podDot(pod("p", "zeebe", { status: { phase: "Failed" } })),
    ).toMatchObject({ tone: "bad", state: "Failed" });
  });

  it("is warn with the phase for a pod that is not ready", () => {
    expect(
      podDot(pod("p", "zeebe", { status: { phase: "Pending" } })),
    ).toMatchObject({ tone: "warn", state: "Pending" });
    expect(
      podDot(
        pod("p", "zeebe", {
          status: {
            phase: "Running",
            conditions: [{ type: "Ready", status: "False" }],
          },
        }),
      ),
    ).toMatchObject({ tone: "warn", state: "Running" });
  });

  it("is warn and Unknown for a pod without status", () => {
    expect(podDot(pod("p", "zeebe", { status: undefined }))).toMatchObject({
      tone: "warn",
      state: "Unknown",
    });
  });
});
