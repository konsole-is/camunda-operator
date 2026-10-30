import { describe, expect, it } from "vitest";
import type { Environment } from "@/generated/crds/environment";
import { environmentReady } from ".";

function environment(ready?: {
  status: "True" | "False" | "Unknown";
  message: string;
}): Environment {
  return {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "Environment",
    metadata: { name: "x" },
    spec: {},
    ...(ready && {
      status: {
        conditions: [
          {
            type: "Ready",
            status: ready.status,
            reason: "Test",
            message: ready.message,
            lastTransitionTime: "2026-10-30T00:00:00Z",
          },
        ],
      },
    }),
  };
}

describe("environmentReady", () => {
  it("is ready without a status", () => {
    expect(environmentReady(environment())).toEqual({ ready: true });
  });

  it("is ready with Ready=Unknown", () => {
    expect(
      environmentReady(environment({ status: "Unknown", message: "wait" })),
    ).toEqual({ ready: true });
  });

  it("is not ready with Ready=False, with its message", () => {
    expect(
      environmentReady(environment({ status: "False", message: "gone" })),
    ).toEqual({ ready: false, message: "gone" });
  });
});
