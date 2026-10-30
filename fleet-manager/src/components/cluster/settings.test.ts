import { describe, expect, it } from "vitest";
import type { ClusterField, FieldGroup } from "@/lib/fields";
import type { Resource } from "@/lib/k8s/kinds";
import { mergeCluster } from "@/lib/merge";
import {
  claimFieldHref,
  clusterFieldsInputOf,
  componentSettings,
  operatorVersionOf,
} from "./settings";

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

function registration(
  name: string,
  controller: string,
  version: string,
): Resource<"PluginRegistration"> {
  return {
    kind: "PluginRegistration",
    metadata: { name },
    spec: { controller: { name: controller, version } },
  } as unknown as Resource<"PluginRegistration">;
}

describe("operatorVersionOf", () => {
  it("gives the version of a built-in registration", () =>
    expect(
      operatorVersionOf([
        registration("aws-ebs-kms", "aws-plugins", "1.4.0"),
        registration("existing-databases", "camunda-operator", "v0.2.0"),
      ]),
    ).toBe("v0.2.0"));

  it("gives an empty version without a built-in registration", () =>
    expect(
      operatorVersionOf([registration("aws-ebs-kms", "aws-plugins", "1.4.0")]),
    ).toBe(""));
});

describe("clusterFieldsInputOf", () => {
  const cluster = {
    kind: "CamundaCluster",
    metadata: { name: "orders", namespace: "team-a" },
    spec: {},
  } as Resource<"CamundaCluster">;
  const schedule = (name: string, clusterName: string) =>
    ({
      kind: "BackupSchedule",
      metadata: { name, namespace: "team-a" },
      spec: { clusterRef: { name: clusterName }, schedule: "@daily" },
    }) as unknown as Resource<"BackupSchedule">;
  const policy = { deletionPolicy: "Orphan" };
  const claim = {
    kind: "CamundaClusterClaim",
    metadata: { name: "orders", namespace: "team-a" },
    spec: {},
    status: { resolvedPolicy: policy },
  } as unknown as Resource<"CamundaClusterClaim">;

  it("picks the first schedule of the cluster by name and the policy of the claim", () => {
    const input = clusterFieldsInputOf({
      cluster,
      merged: mergeCluster(undefined, undefined, {}),
      claim,
      sections: [],
      registrations: [],
      schedules: [
        schedule("z-orders", "orders"),
        schedule("a-payments", "payments"),
        schedule("b-orders", "orders"),
      ],
    });
    expect(input.backupSchedule?.metadata?.name).toBe("b-orders");
    expect(input.policy).toEqual(policy);
  });

  it("gives no schedule and no policy when there are none", () => {
    const input = clusterFieldsInputOf({
      cluster,
      merged: mergeCluster(undefined, undefined, {}),
      sections: [],
      registrations: [],
      schedules: [schedule("a-payments", "payments")],
    });
    expect(input.backupSchedule).toBeUndefined();
    expect(input.policy).toBeUndefined();
  });
});
