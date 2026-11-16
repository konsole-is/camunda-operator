import { describe, expect, it } from "vitest";
import { claimChangeSchema } from "@/lib/write/run-change";
import { suspendChange } from "./suspend";

const claim = {
  apiVersion: "fleet.camunda.io/v1alpha1",
  kind: "CamundaManagementClusterClaim",
  metadata: { name: "plane-a", namespace: "platform", resourceVersion: "7" },
  spec: {
    environments: ["platform"],
    management: { identityProvider: { oidc: {} } },
  },
};

describe("suspendChange", () => {
  it("sets spec.suspend and keeps the rest of the claim", () => {
    const change = suspendChange(claim, true);
    expect(change.apply).toEqual([
      { ...claim, spec: { ...claim.spec, suspend: true } },
    ]);
    expect(change.intent).toBe("Suspend the management plane plane-a");
  });

  it("carries the resourceVersion that the page read", () => {
    expect(suspendChange(claim, true).preconditions).toEqual({
      "CamundaManagementClusterClaim/platform/plane-a": "7",
    });
  });

  it("resume sets spec.suspend to false", () => {
    const change = suspendChange(
      { ...claim, spec: { ...claim.spec, suspend: true } },
      false,
    );
    expect(change.apply[0]).toMatchObject({ spec: { suspend: false } });
    expect(change.intent).toBe("Resume the management plane plane-a");
  });

  it("a suspend change passes claimChangeSchema", () => {
    expect(
      claimChangeSchema.safeParse(suspendChange(claim, true)).success,
    ).toBe(true);
  });

  it("does not change the claim it got", () => {
    suspendChange(claim, true);
    expect(claim.spec).not.toHaveProperty("suspend");
  });
});
