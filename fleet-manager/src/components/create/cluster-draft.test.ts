import { describe, expect, it } from "vitest";
import type { CamundaClusterPreset } from "@/generated/crds/camundaclusterpreset";
import type { Environment } from "@/generated/crds/environment";
import {
  type BasicsContext,
  type ClusterDraft,
  basicsErrors,
  offeredPresets,
  pickerEnvironment,
} from "./cluster-draft";

function environment(
  name: string,
  over: Partial<Environment["spec"]> = {},
  ready?: { status: "True" | "False"; message: string },
): Environment {
  return {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "Environment",
    metadata: { name },
    spec: over,
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

function preset(name: string): CamundaClusterPreset {
  return {
    apiVersion: "core.camunda.io/v1",
    kind: "CamundaClusterPreset",
    metadata: { name },
    spec: { cluster: {} },
  };
}

const prod = environment("prod", { displayName: "Production" });

const complete: ClusterDraft = {
  environment: "prod",
  namespace: { kind: "existing", name: "team-a" },
  claim: { metadata: { name: "orders" }, spec: { presetRef: "small" } },
};
const context: BasicsContext = {
  environments: [prod],
  takenNames: [],
  conflicts: [],
  presets: ["small"],
};

describe("basicsErrors", () => {
  it("passes a complete draft", () => {
    expect(basicsErrors(complete, context)).toEqual([]);
  });

  it("asks for an environment while one exists", () => {
    expect(
      basicsErrors({ ...complete, environment: undefined }, context),
    ).toEqual(["Choose an environment"]);
  });

  it("asks for an environment when the picked one is gone", () => {
    expect(basicsErrors({ ...complete, environment: "old" }, context)).toEqual([
      "Choose an environment",
    ]);
  });

  it("asks for no environment when none exists", () => {
    expect(
      basicsErrors(
        { ...complete, environment: undefined },
        { ...context, environments: [] },
      ),
    ).toEqual([]);
  });

  it("refuses an environment that is no longer ready", () => {
    const notReady = environment(
      "prod",
      { displayName: "Production" },
      { status: "False", message: "x" },
    );
    expect(
      basicsErrors(complete, { ...context, environments: [notReady] }),
    ).toEqual(["The environment Production is not ready: x"]);
  });

  it("gives the name errors", () => {
    expect(
      basicsErrors(
        { ...complete, claim: { ...complete.claim, metadata: { name: "" } } },
        context,
      ),
    ).toEqual(["Enter a name"]);
  });

  it("refuses a taken name", () => {
    expect(
      basicsErrors(complete, { ...context, takenNames: ["orders"] }),
    ).toEqual(["A cluster named orders already exists in this namespace."]);
  });

  it("gives each host conflict", () => {
    const conflict =
      "The host orders.team-a.example.com is taken by team-b/billing. Choose another name.";
    expect(
      basicsErrors(complete, { ...context, conflicts: [conflict] }),
    ).toEqual([conflict]);
  });

  it("asks for a namespace", () => {
    expect(
      basicsErrors({ ...complete, namespace: undefined }, context),
    ).toEqual(["Choose a namespace"]);
  });

  it("asks for a preset when none is picked", () => {
    expect(
      basicsErrors(
        { ...complete, claim: { ...complete.claim, spec: {} } },
        context,
      ),
    ).toEqual(["Choose a preset"]);
  });

  it("asks for a preset when the picked one is not offered", () => {
    expect(basicsErrors(complete, { ...context, presets: ["large"] })).toEqual([
      "Choose a preset",
    ]);
  });

  it("gives the errors in screen order", () => {
    expect(
      basicsErrors(
        { claim: { metadata: { name: "" }, spec: {} } },
        { ...context, conflicts: ["c"] },
      ),
    ).toEqual([
      "Choose an environment",
      "Enter a name",
      "c",
      "Choose a namespace",
      "Choose a preset",
    ]);
  });
});

describe("offeredPresets", () => {
  const all = [preset("small"), preset("large"), preset("medium")];

  it("keeps the order of the environment and skips a missing name", () => {
    expect(
      offeredPresets(
        all,
        environment("prod", {
          presets: { camundaCluster: ["medium", "gone", "small"] },
        }),
      ).map((p) => p.metadata.name),
    ).toEqual(["medium", "small"]);
  });

  it("offers no preset when the environment lists none", () => {
    expect(offeredPresets(all, prod)).toEqual([]);
  });

  it("offers every preset by name without an environment", () => {
    expect(offeredPresets(all, undefined).map((p) => p.metadata.name)).toEqual([
      "large",
      "medium",
      "small",
    ]);
  });
});

describe("pickerEnvironment", () => {
  it("passes the name, the display name, and the namespace creation", () => {
    const creation = { allowed: true, requiredLabelKeys: ["team"] };
    expect(
      pickerEnvironment(
        environment("prod", {
          displayName: "Production",
          namespaceCreation: creation,
        }),
      ),
    ).toEqual({
      name: "prod",
      displayName: "Production",
      namespaceCreation: creation,
    });
  });
});
