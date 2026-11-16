import { describe, expect, it } from "vitest";
import type { Resource } from "@/lib/k8s/kinds";
import { ENVIRONMENT_LABEL } from "@/lib/labels";
import { servedClusters } from "./served-clusters";

function planeWith(clusters: unknown): Resource<"CamundaManagementCluster"> {
  return {
    apiVersion: "core.camunda.io/v1",
    kind: "CamundaManagementCluster",
    metadata: { namespace: "platform", name: "production" },
    spec: {},
    status: { clusters },
  } as unknown as Resource<"CamundaManagementCluster">;
}

const namespace = (name: string, environment?: string) =>
  ({
    metadata: {
      name,
      labels: environment ? { [ENVIRONMENT_LABEL]: environment } : {},
    },
  }) as unknown as Resource<"Namespace">;

const environments = [
  { metadata: { name: "production" }, spec: {} },
] as unknown as Resource<"Environment">[];

const namespaces = [
  namespace("team-orders", "production"),
  namespace("team-billing", "production"),
  namespace("team-lab"),
];

describe("servedClusters", () => {
  it("gives each row of status.clusters in its order, with a link to the cluster page", () => {
    const plane = planeWith([
      { namespace: "team-orders", name: "orders", attached: true },
      {
        namespace: "team-billing",
        name: "billing",
        attached: false,
        reason: "ClaimedElsewhere",
        message: "The plane platform/sandbox already serves this cluster",
      },
    ]);
    expect(servedClusters({ plane, namespaces, environments })).toEqual([
      {
        namespace: "team-orders",
        name: "orders",
        environment: "production",
        attached: true,
        href: "/fleet/clusters/team-orders/orders",
      },
      {
        namespace: "team-billing",
        name: "billing",
        environment: "production",
        attached: false,
        reason: "ClaimedElsewhere",
        message: "The plane platform/sandbox already serves this cluster",
        href: "/fleet/clusters/team-billing/billing",
      },
    ]);
  });

  it("keeps the reason of an attached row", () => {
    const plane = planeWith([
      {
        namespace: "team-orders",
        name: "orders",
        attached: true,
        reason: "BasicAuthUserFailed",
        message: "The Web Modeler user is missing",
      },
    ]);
    expect(
      servedClusters({ plane, namespaces, environments })[0],
    ).toMatchObject({ attached: true, reason: "BasicAuthUserFailed" });
  });

  it("gives no environment to a namespace without one or missing from the list", () => {
    const plane = planeWith([
      { namespace: "team-lab", name: "lab", attached: true },
      { namespace: "team-gone", name: "gone", attached: false },
    ]);
    expect(
      servedClusters({ plane, namespaces, environments }).map((r) => [
        r.environment,
        r.environmentUnknown,
      ]),
    ).toEqual([
      [undefined, undefined],
      [undefined, undefined],
    ]);
  });

  it("marks the environment as unknown, not as none, while the namespaces or environments are not read", () => {
    const plane = planeWith([
      { namespace: "team-orders", name: "orders", attached: true },
    ]);
    expect(
      servedClusters({
        plane,
        namespaces: [],
        environments,
        environmentsUnread: true,
      })[0],
    ).toMatchObject({ environmentUnknown: true });
    expect(
      servedClusters({
        plane,
        namespaces: [],
        environments,
        environmentsUnread: true,
      })[0].environment,
    ).toBeUndefined();
  });

  it("gives no rows without a plane or without status.clusters", () => {
    expect(
      servedClusters({ plane: undefined, namespaces, environments }),
    ).toEqual([]);
    expect(
      servedClusters({ plane: planeWith(undefined), namespaces, environments }),
    ).toEqual([]);
  });

  it("does not throw on values of the wrong type, and treats a row that is not attached true as not attached", () => {
    const plane = planeWith([
      null,
      7,
      { namespace: 3, name: "x", attached: true },
      {
        namespace: "team-orders",
        name: "orders",
        attached: "yes",
        reason: 5,
        message: null,
      },
    ]);
    expect(servedClusters({ plane, namespaces, environments })).toEqual([
      {
        namespace: "team-orders",
        name: "orders",
        environment: "production",
        attached: false,
        href: "/fleet/clusters/team-orders/orders",
      },
    ]);
    expect(
      servedClusters({ plane: planeWith("rows"), namespaces, environments }),
    ).toEqual([]);
  });
});
