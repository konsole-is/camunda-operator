// @vitest-environment jsdom
import { cleanup, render } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
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

function detailOf(id: Parameters<typeof ClusterComponentDetail>[0]["id"]) {
  render(
    <ClusterComponentDetail
      id={id}
      cards={cards}
      cluster={cluster}
      namespace="team-a"
      podSelector="camunda.io/cluster=orders"
      conditions={[]}
      optimize={optimize}
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

  it("selects the pods of an embedded app by its host", () =>
    expect(detailOf("operate").podSelector).toBe(
      "camunda.io/cluster=orders,camunda.io/component=gateway",
    ));

  it("selects the pods of the CamundaOptimize by its UID", () =>
    expect(detailOf("optimize").podSelector).toBe(
      "camunda.io/cluster=orders,camunda.io/component in (optimize-webapp,optimize-importer),camunda.io/optimize-uid=u-1",
    ));
});
