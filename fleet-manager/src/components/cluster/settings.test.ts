import { describe, expect, it } from "vitest";
import type { ClusterField, FieldGroup } from "@/lib/fields";
import { claimFieldHref, componentSettings } from "./settings";

function field(group: FieldGroup): ClusterField {
  return {
    key: `CamundaCluster:spec.${group}.replicas`,
    target: { kind: "CamundaCluster" },
    group,
    path: `spec.${group}.replicas`,
    label: "replicas",
    value: 1,
    state: { kind: "preset" },
  };
}

const fields = [field("zeebe"), field("gateway"), field("operate")];

function shape(groups: ReturnType<typeof componentSettings>) {
  return groups.map((g) => [g.label, g.embedded, g.fields.length]);
}

describe("componentSettings", () => {
  it("the gateway hosts the web apps when it is standalone", () => {
    const spec = { gateway: { mode: "Standalone" as const } };
    expect(shape(componentSettings(fields, "gateway", spec))).toEqual([
      ["Gateway", false, 1],
      ["Operate", true, 1],
      ["Tasklist", true, 0],
      ["Admin", true, 0],
    ]);
    expect(shape(componentSettings(fields, "operate", spec))).toEqual([
      ["Operate", false, 1],
    ]);
  });

  it("the brokers host the gateway and the web apps when the gateway is embedded", () => {
    const spec = { gateway: { mode: "Embedded" as const } };
    expect(
      componentSettings(fields, "zeebe", spec).map((g) => g.label),
    ).toEqual(["Brokers", "Gateway", "Operate", "Tasklist", "Admin"]);
  });

  it("a standalone web app is not embedded", () => {
    const spec = { operate: { mode: "Standalone" as const } };
    expect(
      componentSettings(fields, "gateway", spec).map((g) => g.label),
    ).toEqual(["Gateway", "Tasklist", "Admin"]);
  });

  it("Optimize has its own group only", () => {
    const optimize = { ...field("optimize"), key: "CamundaOptimize:x" };
    expect(
      shape(componentSettings([...fields, optimize], "optimize", {})),
    ).toEqual([["Optimize", false, 1]]);
  });
});

describe("claimFieldHref", () => {
  it("encodes the path", () => {
    expect(
      claimFieldHref("team-a", "orders", "spec.cluster.extraEnv[name=TZ]"),
    ).toBe(
      "/fleet/clusters/team-a/orders?tab=claim&field=spec.cluster.extraEnv%5Bname%3DTZ%5D",
    );
  });
});
