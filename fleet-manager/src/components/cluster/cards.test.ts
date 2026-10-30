import { describe, expect, it } from "vitest";
import type { CamundaClusterSpec } from "@/generated/crds/camundacluster";
import type { Resource } from "@/lib/k8s/kinds";
import { COMPONENT_LABEL, OPTIMIZE_UID_LABEL } from "@/lib/labels";
import type { PodUsage } from "@/lib/observe/types";
import {
  type ComponentCardModel,
  componentCards,
  componentOf,
  highestUse,
  partitionsLed,
} from "./cards";

function pod(
  name: string,
  component: string,
  status: Resource<"Pod">["status"],
  labels: Record<string, string> = {},
): Resource<"Pod"> {
  return {
    kind: "Pod",
    metadata: {
      name,
      namespace: "team-a",
      labels: { [COMPONENT_LABEL]: component, ...labels },
    },
    status,
  } as Resource<"Pod">;
}

const readyPod = (
  name: string,
  component: string,
  labels: Record<string, string> = {},
) =>
  pod(
    name,
    component,
    { phase: "Running", conditions: [{ type: "Ready", status: "True" }] },
    labels,
  );
const ofOptimize = { [OPTIMIZE_UID_LABEL]: "uid-optimize" };
const crashLooping = {
  phase: "Running",
  containerStatuses: [
    {
      name: "zeebe",
      image: "camunda/zeebe",
      imageID: "",
      ready: false,
      restartCount: 4,
      state: { waiting: { reason: "CrashLoopBackOff" } },
    },
  ],
} as Resource<"Pod">["status"];
const pending = { phase: "Pending" } as Resource<"Pod">["status"];

// usage gives the CPU in cores and the memory in bytes as [used, limit].
function usage(
  name: string,
  { cpu, memory }: { cpu: [number, number?]; memory: [number, number?] },
): PodUsage {
  return {
    pod: name,
    cpu: { usedCores: cpu[0], limitCores: cpu[1] },
    memory: { usedBytes: memory[0], limitBytes: memory[1] },
  };
}

const optimizeWith = (
  conditions: { type: string; status: string; reason: string }[],
) =>
  ({
    kind: "CamundaOptimize",
    metadata: {
      name: "orders-optimize",
      namespace: "team-a",
      uid: "uid-optimize",
    },
    spec: { clusterRef: { name: "orders" } },
    status: { conditions },
  }) as unknown as Resource<"CamundaOptimize">;

const spec = (s: object) => s as CamundaClusterSpec;

describe("componentCards", () => {
  it("orders the cards and puts an embedded app in its host", () => {
    const cards = componentCards({
      spec: spec({
        gateway: { mode: "Standalone" },
        operate: { mode: "Embedded" },
        tasklist: { mode: "Standalone" },
        admin: { mode: "Embedded" },
        connectors: { enabled: true },
      }),
      pods: [],
    });
    expect(cards.map((c) => c.label)).toEqual([
      "Brokers",
      "Gateway",
      "Tasklist",
      "Connectors",
      "Optimize",
    ]);
    expect(cards[1].embedded.map((e) => e.label)).toEqual(["Operate", "Admin"]);
  });

  it("shows Connectors and Optimize as not used, without numbers", () => {
    const cards = componentCards({ spec: spec({}), pods: [] });
    const connectors = cards.find((c) => c.id === "connectors")!;
    expect(connectors).toMatchObject({ used: false, numbers: [] });
    expect(connectors.condition).toBeUndefined();
    expect(cards.find((c) => c.id === "optimize")).toMatchObject({
      used: false,
      numbers: [],
    });
  });

  it("takes the mode of an embedded app from the spec, not from its Disabled condition", () => {
    const cards = componentCards({
      spec: spec({}),
      pods: [],
      conditions: [
        {
          type: "OperateReady",
          status: "True",
          reason: "Disabled",
          message: "Component is disabled.",
        },
      ],
    });
    expect(cards.map((c) => c.id)).not.toContain("operate");
    expect(
      cards.find((c) => c.id === "gateway")!.embedded.map((e) => e.process),
    ).toContain("operate");
  });

  it("gives each process card the condition that the map places there", () => {
    const cards = componentCards({
      spec: spec({}),
      pods: [],
      conditions: [
        { type: "ZeebeReady", status: "False", reason: "Degraded" },
        { type: "AdminSecretReady", status: "False", reason: "MissingSecret" },
      ],
    });
    expect(cards[0].condition).toMatchObject({
      type: "ZeebeReady",
      status: "False",
    });
    expect(
      cards.flatMap((c) => (c.condition ? [c.condition.type] : [])),
    ).not.toContain("AdminSecretReady");
  });

  it("a degraded broker shows 2/3 pods ready and 2/3 partitions with a leader", () => {
    const card = componentCards({
      spec: spec({}),
      pods: [
        readyPod("orders-zeebe-0", "zeebe"),
        readyPod("orders-zeebe-1", "zeebe"),
        pod("orders-zeebe-2", "zeebe", crashLooping),
      ],
      partitions: [
        {
          partition: 1,
          leader: "orders-zeebe-0",
          followers: ["orders-zeebe-1"],
          health: "unknown",
        },
        {
          partition: 2,
          leader: "orders-zeebe-1",
          followers: [],
          health: "unknown",
        },
        { partition: 3, followers: ["orders-zeebe-0"], health: "unknown" },
      ],
    })[0];
    expect(card.numbers).toEqual([
      { label: "Pods ready", value: "2/3" },
      { label: "Partitions with a leader", value: "2/3" },
    ]);
  });

  it("the Gateway follows the default rule: pods ready and highest use", () => {
    const gateway = componentCards({
      spec: spec({}),
      pods: [readyPod("orders-gateway-a", "gateway")],
      usage: [
        usage("orders-gateway-a", { cpu: [0.25, 1], memory: [900, 1000] }),
      ],
    }).find((c) => c.id === "gateway")!;
    expect(gateway.numbers).toEqual([
      { label: "Pods ready", value: "1/1" },
      { label: "Highest use", value: "90 % memory" },
    ]);
  });

  it("counts the Optimize pods by their two components and uses the Optimize Ready", () => {
    const card = componentCards({
      spec: spec({}),
      optimize: optimizeWith([
        { type: "Ready", status: "True", reason: "Healthy" },
      ]),
      pods: [
        readyPod("o-web", "optimize-webapp", ofOptimize),
        pod("o-imp", "optimize-importer", pending, ofOptimize),
        readyPod("orders-zeebe-0", "zeebe"),
      ],
      usage: "unavailable",
    }).find((c) => c.id === "optimize")!;
    expect(card).toMatchObject({
      used: true,
      pods: ["o-imp", "o-web"],
      condition: { type: "Ready", status: "True" },
    });
    expect(card.numbers).toEqual([
      { label: "Pods ready", value: "1/2" },
      { label: "Highest use", value: "Not available" },
    ]);
  });

  it("the Optimize card leaves out the pods of a deleted Optimize of the cluster", () => {
    const card = componentCards({
      spec: spec({}),
      optimize: optimizeWith([]),
      pods: [
        readyPod("o-web", "optimize-webapp", ofOptimize),
        pod("old-imp", "optimize-importer", pending, {
          [OPTIMIZE_UID_LABEL]: "uid-deleted",
        }),
      ],
    }).find((c) => c.id === "optimize")!;
    expect(card.pods).toEqual(["o-web"]);
    expect(card.numbers[0]).toEqual({ label: "Pods ready", value: "1/1" });
  });

  it("a card counts only the pods of its own process", () => {
    const cards = componentCards({
      spec: spec({}),
      pods: [
        readyPod("orders-zeebe-0", "zeebe"),
        readyPod("orders-gateway-a", "gateway"),
      ],
    });
    expect(cards.map((c) => c.pods)).toEqual([
      ["orders-zeebe-0"],
      ["orders-gateway-a"],
      [],
      [],
    ]);
  });
});

describe("partitionsLed", () => {
  it("the Brokers card says when the management API does not answer", () => {
    expect(partitionsLed("unavailable")).toEqual({
      label: "Partitions with a leader",
      value: "Not available",
    });
    expect(partitionsLed(undefined).value).toBe("Loading");
  });
});

describe("highestUse", () => {
  it.each([
    ["not loaded", undefined, "Loading"],
    ["no metrics server", "unavailable", "Not available"],
    [
      "no pod with a limit",
      [usage("p-0", { cpu: [0.5, undefined], memory: [10, undefined] })],
      "No limit",
    ],
    [
      "usage of a pod that is not on the card only",
      [usage("other", { cpu: [1, 1], memory: [1, 1] })],
      "Not available",
    ],
    ["no usage of any pod", [], "Not available"],
    [
      "usage of one pod of the card, the other without a sample yet",
      [usage("p-1", { cpu: [0.3, 1], memory: [1, 4] })],
      "30 % cpu",
    ],
    [
      "CPU higher than memory",
      [usage("p-0", { cpu: [0.75, 1], memory: [1, 4] })],
      "75 % cpu",
    ],
    [
      "the highest of two pods",
      [
        usage("p-0", { cpu: [0.1, 1], memory: [1, 4] }),
        usage("p-1", { cpu: [0.2, 1], memory: [3, 5] }),
      ],
      "60 % memory",
    ],
  ] as const)("highest use with %s", (_, given, value) => {
    expect(highestUse(given as never, ["p-0", "p-1"])).toEqual({
      label: "Highest use",
      value,
    });
  });
});

describe("componentOf", () => {
  const card = (
    id: ComponentCardModel["id"],
    used: boolean,
    embedded: ComponentCardModel["embedded"] = [],
  ): ComponentCardModel => ({
    id,
    label: id,
    used,
    embedded,
    pods: [],
    numbers: [],
  });
  const cards = [
    card("zeebe", true),
    card("gateway", true, [{ process: "operate", label: "Operate" }]),
    card("connectors", false),
    card("optimize", true),
  ];

  it.each(["zeebe", "optimize"])("gives the used card %s", (id) =>
    expect(componentOf(id, cards)).toBe(id),
  );

  it("gives an app that is embedded in a used card", () =>
    expect(componentOf("operate", cards)).toBe("operate"));

  it.each([
    ["a card that is not used", "connectors"],
    ["an unknown value", "keycloak"],
    ["no value", null],
  ])("gives undefined for %s", (_, value) =>
    expect(componentOf(value, cards)).toBeUndefined(),
  );
});
