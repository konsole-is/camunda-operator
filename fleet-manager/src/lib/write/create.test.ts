import { describe, expect, it } from "vitest";
import { createChangeOf } from "./create";

describe("createChangeOf", () => {
  const claim = {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "CamundaManagementClusterClaim",
    metadata: { name: "plane-a", namespace: "platform" },
    spec: {},
  };

  it("puts a new namespace first, with its labels, and lists both in create", () => {
    expect(
      createChangeOf("Create the plane plane-a", claim, {
        name: "platform",
        labels: { "fleet.camunda.io/environment": "prod" },
      }),
    ).toEqual({
      intent: "Create the plane plane-a",
      apply: [
        {
          apiVersion: "v1",
          kind: "Namespace",
          metadata: {
            name: "platform",
            labels: { "fleet.camunda.io/environment": "prod" },
          },
        },
        claim,
      ],
      create: [
        "Namespace//platform",
        "CamundaManagementClusterClaim/platform/plane-a",
      ],
    });
  });

  it("lists only the claim without a new namespace", () => {
    expect(createChangeOf("x", claim)).toEqual({
      intent: "x",
      apply: [claim],
      create: ["CamundaManagementClusterClaim/platform/plane-a"],
    });
  });
});
