import { describe, expect, it } from "vitest";
import type { Resource } from "@/lib/k8s/kinds";
import { CLAIM_LABEL } from "@/lib/labels";
import {
  openSession,
  setValue,
  type LockProvider,
  type Session,
} from "@/lib/session";
import { restartsBrokers, type ClusterChangeInput } from "./impacts";
import type { ComponentMode, TopologyBox } from "./topology";
import {
  MODE_PROCESSES,
  modeSteps,
  modeWrite,
  othersPending,
  topologyChange,
  type ModeProcess,
} from "./topology-change";

type Claim = Resource<"CamundaClusterClaim">;
type Pick = [ModeProcess, ComponentMode];

// shape gives the boxes as "Brokers[Gateway,Operate]" strings.
const shape = (boxes: TopologyBox[]) =>
  boxes.map((b) =>
    b.embedded.length
      ? `${b.label}[${b.embedded.map((e) => e.label).join(",")}]`
      : b.label,
  );

function claim(cluster: Record<string, unknown> = {}): Claim {
  return {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "CamundaClusterClaim",
    metadata: { name: "orders", namespace: "team-a", resourceVersion: "41" },
    spec: { cluster },
  } as unknown as Claim;
}

function live(spec: Record<string, unknown> = {}): Resource<"CamundaCluster"> {
  return {
    apiVersion: "core.camunda.io/v1",
    kind: "CamundaCluster",
    metadata: {
      name: "orders",
      namespace: "team-a",
      labels: { [CLAIM_LABEL]: "orders" },
    },
    spec,
  } as unknown as Resource<"CamundaCluster">;
}

// input opens a session over a claim and a live cluster that both hold
// spec, and picks each mode with setValue.
function input(
  picks: Pick[],
  spec: Record<string, unknown> = {},
  locks: LockProvider = () => undefined,
): ClusterChangeInput {
  let s: Session<Claim> = openSession({
    claim: claim(spec),
    resource: live(spec),
    locks,
  })!;
  for (const [p, mode] of picks) {
    const r = setValue(s, modeSteps(p), mode);
    if (!("session" in r)) throw new Error("refused");
    s = r.session;
  }
  return {
    session: s,
    live: live(spec),
    presets: [],
    releases: [],
    sections: [],
  };
}

describe("topologyChange", () => {
  it.each([
    {
      name: "the gateway becomes embedded and takes the web apps into the brokers",
      spec: {},
      picks: [["gateway", "Embedded"]] as Pick[],
      before: ["Brokers", "Gateway[Operate,Tasklist,Admin]"],
      after: ["Brokers[Gateway,Operate,Tasklist,Admin]"],
      marks: { before: { gateway: "removed" }, after: {} },
    },
    {
      name: "the gateway becomes standalone and the web apps move from the brokers to the gateway",
      spec: { gateway: { mode: "Embedded" } },
      picks: [["gateway", "Standalone"]] as Pick[],
      before: ["Brokers[Gateway,Operate,Tasklist,Admin]"],
      after: ["Brokers", "Gateway[Operate,Tasklist,Admin]"],
      marks: { before: {}, after: { gateway: "new" } },
    },
    {
      name: "Operate becomes standalone and leaves the gateway",
      spec: {},
      picks: [["operate", "Standalone"]] as Pick[],
      before: ["Brokers", "Gateway[Operate,Tasklist,Admin]"],
      after: ["Brokers", "Gateway[Tasklist,Admin]", "Operate"],
      marks: { before: {}, after: { operate: "new" } },
    },
    {
      name: "Admin becomes embedded in the brokers of an embedded gateway",
      spec: { gateway: { mode: "Embedded" }, admin: { mode: "Standalone" } },
      picks: [["admin", "Embedded"]] as Pick[],
      before: ["Brokers[Gateway,Operate,Tasklist]", "Admin"],
      after: ["Brokers[Gateway,Operate,Tasklist,Admin]"],
      marks: { before: { admin: "removed" }, after: {} },
    },
    {
      name: "the gateway becomes embedded, Operate stays standalone",
      spec: { operate: { mode: "Standalone" } },
      picks: [["gateway", "Embedded"]] as Pick[],
      before: ["Brokers", "Gateway[Tasklist,Admin]", "Operate"],
      after: ["Brokers[Gateway,Tasklist,Admin]", "Operate"],
      marks: { before: { gateway: "removed" }, after: {} },
    },
  ])(
    "gives the boxes before and after: $name",
    ({ spec, picks, before, after, marks }) => {
      const change = topologyChange(input(picks, spec));
      expect(shape(change.before)).toEqual(before);
      expect(shape(change.after)).toEqual(after);
      expect(change.marks).toEqual(marks);
    },
  );

  it("shows Connectors as their own box in both, without a mode row", () => {
    const change = topologyChange(
      input([["gateway", "Embedded"]], { connectors: { enabled: true } }),
    );
    expect(shape(change.before)).toContain("Connectors");
    expect(shape(change.after)).toContain("Connectors");
    expect(change.modes.map((m) => m.process)).toEqual([...MODE_PROCESSES]);
  });

  it("gives each mode row its mode before and after and the host it runs in when embedded", () => {
    const change = topologyChange(input([["gateway", "Embedded"]]));
    expect(change.modes).toEqual([
      {
        process: "gateway",
        label: "Gateway",
        before: "Standalone",
        after: "Embedded",
        host: "zeebe",
      },
      {
        process: "operate",
        label: "Operate",
        before: "Embedded",
        after: "Embedded",
        host: "zeebe",
      },
      {
        process: "tasklist",
        label: "Tasklist",
        before: "Embedded",
        after: "Embedded",
        host: "zeebe",
      },
      {
        process: "admin",
        label: "Admin",
        before: "Embedded",
        after: "Embedded",
        host: "zeebe",
      },
    ]);
  });

  it("gives a web app the gateway as host while the gateway stays standalone", () => {
    const row = topologyChange(input([["operate", "Standalone"]])).modes.find(
      (m) => m.process === "tasklist",
    );
    expect(row?.host).toBe("gateway");
  });

  it("gives a mode row the lock of its claim path", () => {
    const locks: LockProvider = (path) =>
      path === "spec.cluster.operate.mode" ? { reason: "admin" } : undefined;
    const row = topologyChange(input([], {}, locks)).modes.find(
      (m) => m.process === "operate",
    );
    expect(row?.lock).toEqual({ reason: "admin" });
  });

  it("gives a mode row the lock of a field above its mode", () => {
    const locks: LockProvider = (path) =>
      path === "spec.cluster.admin" ? { reason: "release" } : undefined;
    const row = topologyChange(input([], {}, locks)).modes.find(
      (m) => m.process === "admin",
    );
    expect(row?.lock).toEqual({ reason: "release" });
  });
});

// presetInput is input over a claim and a live cluster on the preset
// medium, whose session moves the claim to the preset large. large runs
// Operate standalone, and the claim holds no mode.
function presetInput(): ClusterChangeInput {
  const preset = (name: string, cluster: object) =>
    ({
      kind: "CamundaClusterPreset",
      metadata: { name },
      spec: { cluster },
    }) as unknown as Resource<"CamundaClusterPreset">;
  const base = input([], { presetRef: "medium" });
  const c = {
    ...base.session.claim,
    spec: { presetRef: "medium", cluster: {} },
  };
  const s = openSession({
    claim: c,
    resource: base.live,
    locks: () => undefined,
  })!;
  const r = setValue(s, [{ field: "spec" }, { field: "presetRef" }], "large");
  if (!("session" in r)) throw new Error("refused");
  return {
    ...base,
    session: r.session,
    presets: [
      preset("medium", {}),
      preset("large", { operate: { mode: "Standalone" } }),
    ],
  };
}

describe("modeWrite", () => {
  it("sets the picked mode at spec.cluster.<process>.mode", () => {
    expect(modeWrite(input([]), "operate", "Standalone")).toEqual({
      set: [
        { field: "spec" },
        { field: "cluster" },
        { field: "operate" },
        { field: "mode" },
      ],
      value: "Standalone",
    });
  });

  it("resets a pick of the current mode when the claim holds no mode", () => {
    expect(modeWrite(input([]), "operate", "Embedded")).toEqual({
      reset: modeSteps("operate"),
    });
  });

  it("resets a pick back to the mode of the cluster after a pending pick", () => {
    expect(
      modeWrite(input([["operate", "Standalone"]]), "operate", "Embedded"),
    ).toEqual({ reset: modeSteps("operate") });
  });

  it("sets the current mode when the claim holds another one", () => {
    expect(
      modeWrite(
        input([], { operate: { mode: "Standalone" } }),
        "operate",
        "Embedded",
      ),
    ).toEqual({ set: modeSteps("operate"), value: "Embedded" });
  });

  it("sets a pick that a pending preset change would override", () => {
    expect(modeWrite(presetInput(), "operate", "Embedded")).toEqual({
      set: modeSteps("operate"),
      value: "Embedded",
    });
  });

  it("resets a pick of the mode that a pending preset change gives", () => {
    expect(modeWrite(presetInput(), "operate", "Standalone")).toEqual({
      reset: modeSteps("operate"),
    });
  });
});

describe("othersPending", () => {
  it("is false with only mode changes and true with any other change", () => {
    const modes = input([["gateway", "Embedded"]]).session;
    expect(othersPending(modes)).toBe(false);
    const r = setValue(
      modes,
      [
        { field: "spec" },
        { field: "cluster" },
        { field: "zeebe" },
        { field: "replicas" },
      ],
      5,
    );
    if (!("session" in r)) throw new Error("refused");
    expect(othersPending(r.session)).toBe(true);
  });
});

describe("a topology switch and the brokers", () => {
  it.each([
    ["the gateway becomes embedded", {}, [["gateway", "Embedded"]]],
    [
      "the gateway becomes standalone",
      { gateway: { mode: "Embedded" } },
      [["gateway", "Standalone"]],
    ],
    [
      "an app leaves the brokers",
      { gateway: { mode: "Embedded" } },
      [["operate", "Standalone"]],
    ],
    [
      "an app moves into the brokers",
      { gateway: { mode: "Embedded" }, admin: { mode: "Standalone" } },
      [["admin", "Embedded"]],
    ],
  ] as [string, Record<string, unknown>, Pick[]][])(
    "restarts the brokers when %s",
    (_, spec, picks) => expect(restartsBrokers(input(picks, spec))).toBe(true),
  );

  it("does not restart the brokers when an app moves between the gateway and its own Deployment", () => {
    expect(restartsBrokers(input([["operate", "Standalone"]]))).toBe(false);
  });
});
