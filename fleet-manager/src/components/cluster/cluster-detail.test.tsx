// @vitest-environment jsdom
import { cleanup, render } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { ConditionRow } from "@/components/fleet/conditions-table";
import type { CamundaClusterSpec } from "@/generated/crds/camundacluster";
import type { Resource } from "@/lib/k8s/kinds";
import { componentCards } from "./cards";
import { ClusterComponentDetail } from "./cluster-detail";

const seen = vi.hoisted(() => ({ props: [] as Record<string, unknown>[] }));
vi.mock("./component-detail", () => ({
  ComponentDetail: (props: Record<string, unknown>) => {
    seen.props.push(props);
    return null;
  },
}));

beforeEach(() => {
  seen.props = [];
});
afterEach(cleanup);

const cluster = {
  metadata: { name: "orders", namespace: "team-a", uid: "u-cluster" },
  spec: {},
} as unknown as Resource<"CamundaCluster">;
const optimize = {
  metadata: { name: "orders-optimize", namespace: "team-a", uid: "u-1" },
  spec: { clusterRef: { name: "orders" } },
} as unknown as Resource<"CamundaOptimize">;
const cards = componentCards({
  spec: {
    gateway: { mode: "Standalone" },
    operate: { mode: "Embedded" },
  } as CamundaClusterSpec,
  pods: [],
});

function detailOf(
  id: Parameters<typeof ClusterComponentDetail>[0]["id"],
  conditions: ConditionRow[] = [],
  ofOptimize = optimize,
) {
  render(
    <ClusterComponentDetail
      id={id}
      cards={cards}
      cluster={cluster}
      namespace="team-a"
      podSelector="camunda.io/cluster=orders"
      conditions={conditions}
      optimize={ofOptimize}
      partitions={undefined}
      settings={null}
      now="2026-11-10T12:00:00Z"
    />,
  );
  return seen.props.at(-1)!;
}

describe("ClusterComponentDetail", () => {
  it("asks the charts of the brokers of the cluster", () =>
    expect(detailOf("zeebe").metrics).toEqual({
      cluster: "orders",
      component: "zeebe",
    }));

  it("asks the charts of the host for an embedded app", () =>
    expect(detailOf("operate").metrics).toEqual({
      cluster: "orders",
      component: "gateway",
    }));

  it("asks the charts of the Optimize pods", () =>
    expect(detailOf("optimize").metrics).toEqual({
      cluster: "orders",
      component: "optimize",
    }));

  it("gives the brokers the events of the cluster", () =>
    expect(detailOf("zeebe").eventRefs).toEqual([
      {
        kind: "CamundaCluster",
        name: "orders",
        namespace: "team-a",
        uid: "u-cluster",
      },
    ]));

  it("gives Optimize the events of its CamundaOptimize", () =>
    expect(detailOf("optimize").eventRefs).toEqual([
      {
        kind: "CamundaOptimize",
        name: "orders-optimize",
        namespace: "team-a",
        uid: "u-1",
      },
    ]));

  it("selects the pods of an embedded app by its host", () =>
    expect(detailOf("operate").podSelector).toBe(
      "camunda.io/cluster=orders,camunda.io/component=gateway",
    ));

  it("selects the Services and PVCs of Optimize without the UID that only its pods carry", () =>
    expect(detailOf("optimize").relatedSelector).toBe(
      "camunda.io/cluster=orders,camunda.io/component in (optimize-webapp,optimize-importer)",
    ));

  it("selects the Services and PVCs of the brokers as their pods", () =>
    expect(detailOf("zeebe").relatedSelector).toBe(
      "camunda.io/cluster=orders,camunda.io/component=zeebe",
    ));

  it("selects the pods of the CamundaOptimize by its UID", () =>
    expect(detailOf("optimize").podSelector).toBe(
      "camunda.io/cluster=orders,camunda.io/component in (optimize-webapp,optimize-importer),camunda.io/optimize-uid=u-1",
    ));

  it("sums up the brokers by their own condition, not by the Ready of the cluster", () =>
    expect(
      detailOf("zeebe", [
        { type: "Ready", status: "False", reason: "Degraded" },
        { type: "ZeebeReady", status: "False", reason: "Degraded" },
      ]).ready,
    ).toMatchObject({ type: "ZeebeReady" }));

  it("sums up an embedded app by its own condition", () =>
    expect(
      detailOf("operate", [
        { type: "GatewayReady", status: "True", reason: "Healthy" },
        { type: "OperateReady", status: "True", reason: "Disabled" },
      ]).ready,
    ).toMatchObject({ type: "OperateReady" }));

  it("sums up Optimize by its Ready, not by the condition of an app", () =>
    expect(
      detailOf("optimize", [], {
        ...optimize,
        status: {
          conditions: [
            { type: "WebappReady", status: "False", reason: "Degraded" },
            { type: "Ready", status: "False", reason: "Degraded" },
          ],
        },
      } as unknown as Resource<"CamundaOptimize">).ready,
    ).toMatchObject({ type: "Ready" }));
});
