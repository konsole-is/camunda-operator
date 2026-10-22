import { describe, expect, it } from "vitest";
import type { Resource } from "@/lib/k8s/kinds";
import { CLAIM_LABEL } from "@/lib/labels";
import { mergeCluster } from "@/lib/merge";
import type { ResolvedPolicy } from "./cluster";
import { lockLabel, stateLabel } from "./labels";
import { parameterFields } from "./parameters";

const SECTION = "secondary-storage";
const REGISTRATION = "elasticsearch";

function fieldsOf({
  policy,
  parameters,
  registrations = [],
  labelled = true,
}: {
  policy?: Record<
    string,
    {
      claimEditable?: boolean;
      value?: unknown;
      allowed?: { name: string; label: string; value?: unknown }[];
    }
  >;
  parameters?: Record<string, unknown>;
  registrations?: Resource<"PluginRegistration">[];
  labelled?: boolean;
}) {
  return parameterFields({
    cluster: {
      kind: "CamundaCluster",
      metadata: {
        name: "orders",
        labels: labelled ? { [CLAIM_LABEL]: "orders" } : {},
      },
      spec: {},
    } as unknown as Resource<"CamundaCluster">,
    merged: mergeCluster(undefined, undefined, {}),
    claim: {
      kind: "CamundaClusterClaim",
      metadata: { name: "orders" },
      spec: {
        presetRef: "medium",
        plugins: [{ section: SECTION, registration: REGISTRATION, parameters }],
      },
    } as unknown as Resource<"CamundaClusterClaim">,
    policy: { parameters: { [REGISTRATION]: policy ?? {} } } as ResolvedPolicy,
    sections: [],
    registrations,
  });
}

function registration(
  properties: Record<string, unknown>,
): Resource<"PluginRegistration"> {
  return {
    kind: "PluginRegistration",
    metadata: { name: REGISTRATION },
    spec: { parameters: { type: "object", properties } },
  } as unknown as Resource<"PluginRegistration">;
}

describe("parameterFields", () => {
  it("a parameter that the claim cannot change is Set by your admin", () => {
    const [field] = fieldsOf({ policy: { preset: { value: "medium" } } });
    expect(field.value).toBe("medium");
    expect(stateLabel(field.state)).toBe("Set by your admin");
    expect(lockLabel(field.lock!)).toBe("Set by your admin");
    expect(field.target).toEqual({
      kind: "PluginRequest",
      section: SECTION,
      registration: REGISTRATION,
    });
    expect([field.group, field.label, field.path]).toEqual([
      "parameters",
      "preset",
      "spec.parameters.preset",
    ]);
  });

  it("a claim-editable parameter that the claim keeps is From environment", () => {
    const [field] = fieldsOf({
      policy: { overrides: { claimEditable: true, value: { replicas: 3 } } },
      parameters: {},
    });
    expect(field.value).toEqual({ replicas: 3 });
    expect(stateLabel(field.state)).toBe("From environment");
    expect(field.lock).toBeUndefined();
    expect(field.claimPath).toBe(
      "spec.plugins[section=secondary-storage,registration=elasticsearch].parameters.overrides",
    );
  });

  it("a claim-editable parameter that the claim sets is Changed in the claim", () => {
    const [field] = fieldsOf({
      policy: { overrides: { claimEditable: true, value: { replicas: 3 } } },
      parameters: { overrides: { replicas: 5 } },
    });
    expect(field.value).toEqual({ replicas: 5 });
    expect(stateLabel(field.state)).toBe("Changed in the claim");
  });

  it("a value that the claim sets on a locked parameter stays the admin value", () => {
    const [field] = fieldsOf({
      policy: { preset: { value: "medium" } },
      parameters: { preset: "large" },
    });
    expect(field.value).toBe("medium");
    expect(stateLabel(field.state)).toBe("Set by your admin");
  });

  it.each([
    ["one choice", "sqs", "arn:sqs"],
    ["a list of choices", ["sqs", "sns"], ["arn:sqs", "arn:sns"]],
  ])(
    "a claim that picks %s by name has the value of the choice",
    (_, pick, want) => {
      const [field] = fieldsOf({
        policy: {
          access: {
            claimEditable: true,
            allowed: [
              { name: "sqs", label: "SQS access", value: "arn:sqs" },
              { name: "sns", label: "SNS access", value: "arn:sns" },
            ],
          },
        },
        parameters: { access: pick },
      });
      expect(field.value).toEqual(want);
      expect(stateLabel(field.state)).toBe("Changed in the claim");
    },
  );

  it("lists a schema parameter that the policy does not name", () => {
    const fields = fieldsOf({
      registrations: [
        registration({
          storageSize: { type: "string", description: "The size of a volume." },
        }),
      ],
    });
    expect(fields).toHaveLength(1);
    expect(fields[0].label).toBe("storageSize");
    expect(fields[0].value).toBeUndefined();
    expect(stateLabel(fields[0].state)).toBe("Set by your admin");
    expect(fields[0].description).toBe("The size of a volume.");
  });

  it("lists the parameters of a selection whose registration is gone", () => {
    const fields = fieldsOf({
      policy: { preset: { value: "medium" }, zones: { value: 3 } },
    });
    expect(fields.map((f) => f.label)).toEqual(["preset", "zones"]);
    expect(fields.every((f) => f.description === undefined)).toBe(true);
  });

  it("an unmanaged cluster has no parameter fields", () => {
    expect(
      fieldsOf({ policy: { preset: { value: "medium" } }, labelled: false }),
    ).toEqual([]);
  });
});
