import { describe, expect, it } from "vitest";
import { componentPodSelector } from "./selectors";

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
