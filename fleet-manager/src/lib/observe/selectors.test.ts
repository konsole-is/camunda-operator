import { describe, expect, it } from "vitest";
import { componentObjectSelector, componentPodSelector } from "./selectors";

describe("componentPodSelector", () => {
  it("selects the pods of one component of the cluster", () =>
    expect(componentPodSelector("camunda.io/cluster=orders", "gateway")).toBe(
      "camunda.io/cluster=orders,camunda.io/component=gateway",
    ));

  it("selects the webapp and importer pods of one CamundaOptimize by its UID", () =>
    expect(
      componentPodSelector("camunda.io/cluster=orders", "optimize", "u-1"),
    ).toBe(
      "camunda.io/cluster=orders,camunda.io/component in (optimize-webapp,optimize-importer),camunda.io/optimize-uid=u-1",
    ));
});

describe("componentObjectSelector", () => {
  it("selects the objects of one component of the cluster", () =>
    expect(componentObjectSelector("camunda.io/cluster=orders", "zeebe")).toBe(
      "camunda.io/cluster=orders,camunda.io/component=zeebe",
    ));

  it("selects the objects of Optimize without the UID that only its pods carry", () =>
    expect(
      componentObjectSelector("camunda.io/cluster=orders", "optimize"),
    ).toBe(
      "camunda.io/cluster=orders,camunda.io/component in (optimize-webapp,optimize-importer)",
    ));
});
