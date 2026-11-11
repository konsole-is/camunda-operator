import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import type { Resource } from "@/lib/k8s/kinds";
import { valueImports } from "../../../test/imports/scan";
import {
  type EventScope,
  componentEvents,
  regards,
  workloadNames,
} from "./events";

function podOwnedBy(
  name: string,
  kind: string,
  owner: string,
  labels: Record<string, string> = {},
): Resource<"Pod"> {
  return {
    metadata: {
      name,
      namespace: "team-a",
      labels,
      ownerReferences: [
        { apiVersion: "apps/v1", kind, name: owner, uid: `u-${owner}` },
      ],
    },
  } as Resource<"Pod">;
}

function ev(
  kind: string,
  name: string,
  uid?: string,
  namespace = "team-a",
): Resource<"Event"> {
  return {
    metadata: { name: `${name}.1`, namespace },
    regarding: { kind, name, namespace, ...(uid && { uid }) },
  } as Resource<"Event">;
}

const scope: EventScope = {
  namespace: "team-a",
  pods: [
    podOwnedBy("orders-zeebe-0", "StatefulSet", "orders-zeebe"),
    podOwnedBy("orders-gateway-7c9d-x2", "ReplicaSet", "orders-gateway-7c9d", {
      "pod-template-hash": "7c9d",
    }),
  ],
  services: ["orders-zeebe"],
  claims: ["data-orders-zeebe-0"],
  refs: [
    { kind: "CamundaCluster", name: "orders", namespace: "team-a", uid: "u-1" },
  ],
};

describe("componentEvents", () => {
  it.each([
    ["a pod of the component", ev("Pod", "orders-zeebe-0")],
    ["a Service with the component labels", ev("Service", "orders-zeebe")],
    [
      "a PVC with the component labels",
      ev("PersistentVolumeClaim", "data-orders-zeebe-0"),
    ],
    ["the StatefulSet named after the workload", ev("StatefulSet", "orders-zeebe")],
    ["the Deployment of a pod's ReplicaSet", ev("Deployment", "orders-gateway")],
    ["an older ReplicaSet of that Deployment", ev("ReplicaSet", "orders-gateway-55aa")],
    ["the owner resource", ev("CamundaCluster", "orders", "u-1")],
  ])("keeps the event of %s", (_, event) =>
    expect(componentEvents([event], scope)).toEqual([event]),
  );

  it.each([
    ["a pod of another component", ev("Pod", "orders-gateway-other")],
    [
      "a ReplicaSet of another workload with the same prefix",
      ev("ReplicaSet", "orders-zeebe-gateway-5d8f"),
    ],
    ["the owner resource with another UID", ev("CamundaCluster", "orders", "u-old")],
    [
      "a pod of the same name in another namespace",
      ev("Pod", "orders-zeebe-0", undefined, "team-b"),
    ],
    ["a StatefulSet that no pod and no Service names", ev("StatefulSet", "orders-elastic")],
    ["a ConfigMap named after the workload", ev("ConfigMap", "orders-zeebe")],
  ])("leaves out the event of %s", (_, event) =>
    expect(componentEvents([event], scope)).toEqual([]),
  );

  it("keeps the event of a ref in another namespace", () => {
    const ref = { kind: "PluginRequest", name: "r", namespace: "team-b" };
    const event = ev("PluginRequest", "r", undefined, "team-b");

    expect(componentEvents([event], { ...scope, refs: [ref] })).toEqual([event]);
  });

  it("keeps the events in their order", () => {
    const events = [ev("Pod", "orders-zeebe-0"), ev("Service", "orders-zeebe")];

    expect(componentEvents(events, scope)).toEqual(events);
  });
});

describe("workloadNames", () => {
  it("gives the workload names of the Services and of the pod owners", () =>
    expect(workloadNames(scope.pods, scope.services)).toEqual([
      "orders-gateway",
      "orders-zeebe",
    ]));

  it("leaves out a ReplicaSet whose name does not end in the pod template hash", () =>
    expect(
      workloadNames(
        [podOwnedBy("x-abc", "ReplicaSet", "x", { "pod-template-hash": "abc" })],
        [],
      ),
    ).toEqual([]));
});

describe("regards", () => {
  const event = ev("Pod", "p", "2");

  it.each([
    ["the same kind, name, and namespace", { kind: "Pod", name: "p", namespace: "team-a" }, true],
    ["the same UID", { kind: "Pod", name: "p", namespace: "team-a", uid: "2" }, true],
    ["another UID", { kind: "Pod", name: "p", namespace: "team-a", uid: "1" }, false],
    ["another kind", { kind: "Service", name: "p", namespace: "team-a" }, false],
    ["another namespace", { kind: "Pod", name: "p", namespace: "team-b" }, false],
    ["no namespace", { kind: "Pod", name: "p" }, false],
  ])("a ref with %s gives %s", (_, ref, holds) =>
    expect(regards(event, ref)).toBe(holds),
  );
});

it("events.ts imports values only from its own directory", () =>
  expect(
    valueImports(
      readFileSync(new URL("./events.ts", import.meta.url), "utf8"),
    ).filter((s) => !s.startsWith("./")),
  ).toEqual([]));
