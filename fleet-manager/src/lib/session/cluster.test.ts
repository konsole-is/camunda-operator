import { describe, expect, it } from "vitest";
import { pathFrom, type Step } from "@/lib/fields";
import { REFUSED_CLUSTER_FIELDS, refusedSteps } from "@/lib/fields/claim";
import type { Resource } from "@/lib/k8s/kinds";
import { CLAIM_LABEL } from "@/lib/labels";
import type { ResolvedPolicy } from "@/lib/policy";
import { validateClusterClaim, type ClusterClaimContext } from "./cluster";
import { openSession, resetValue, setValue, type Session } from "./session";

type Claim = Resource<"CamundaClusterClaim">;
const f = (field: string): Step => ({ field });
const ACCESS: Step[] = [
  f("spec"),
  f("plugins"),
  { item: { section: "access", registration: "gateway-access" } },
];

function claim(
  spec: Record<string, unknown> = {},
  resolvedPolicy?: ResolvedPolicy,
): Claim {
  return {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "CamundaClusterClaim",
    metadata: { name: "orders", namespace: "team-a", resourceVersion: "41" },
    spec: {
      presetRef: "medium",
      releaseRef: "camunda-8-9-5",
      plugins: [
        {
          section: "access",
          registration: "gateway-access",
          parameters: { hostnamePattern: "{name}.a.example.com" },
        },
      ],
      ...spec,
    },
    ...(resolvedPolicy && { status: { resolvedPolicy } }),
  } as unknown as Claim;
}

function open(c: Claim = claim()): Session<Claim> {
  return openSession({
    claim: c,
    resource: { metadata: { labels: { [CLAIM_LABEL]: "orders" } } },
    locks: () => undefined,
  })!;
}

function set(s: Session<Claim>, steps: Step[], value: unknown) {
  const result = setValue(s, steps, value);
  if (!("session" in result)) throw new Error("refused");
  return result.session;
}

function release(name: string, version: string): Resource<"CamundaRelease"> {
  return {
    apiVersion: "core.camunda.io/v1",
    kind: "CamundaRelease",
    metadata: { name },
    spec: { version },
  } as unknown as Resource<"CamundaRelease">;
}

function ctx(over: Partial<ClusterClaimContext> = {}): ClusterClaimContext {
  return {
    sections: [
      {
        apiVersion: "plugins.camunda.io/v1alpha1",
        kind: "PluginSection",
        metadata: { name: "access" },
        spec: {
          label: "Access",
          slot: "externalAccess",
          appliesTo: ["CamundaClusterClaim"],
          selection: "One",
        },
      } as unknown as Resource<"PluginSection">,
    ],
    registrations: [
      {
        apiVersion: "plugins.camunda.io/v1alpha1",
        kind: "PluginRegistration",
        metadata: { name: "gateway-access" },
        spec: {
          parameters: {
            type: "object",
            properties: {
              hostnamePattern: { type: "string" },
              replicas: { type: "integer" },
            },
          },
        },
      } as unknown as Resource<"PluginRegistration">,
    ],
    releases: [
      release("camunda-8-9-5", "8.9.5"),
      release("camunda-8-9-2", "8.9.2"),
      release("camunda-8-11-0", "8.11.0"),
      release("camunda-8-10-1", "8.10.1"),
    ],
    cluster: {
      metadata: { name: "orders", namespace: "team-a" },
      status: { management: { version: "8.9.5" } },
    },
    backups: [],
    ...over,
  };
}

const MESSAGES = {
  claim: (name: string) =>
    `The claim sets ${name} in its own field, not in spec.cluster.`,
  release: () =>
    "The release sets this field. Use allowVersionDowngrade for a downgrade.",
  claimController: () => "The claim controller sets this field.",
  section: () => "A selected section sets this field.",
};

describe("validateClusterClaim", () => {
  it("accepts a session without refused values", () => {
    expect(validateClusterClaim(open(), ctx())).toEqual([]);
  });

  it.each(REFUSED_CLUSTER_FIELDS.map((row) => [row.names.join("."), row]))(
    "refuses %s in spec.cluster",
    (name, row) => {
      const value = name === "suspend" ? true : "x";
      const s = set(open(), refusedSteps(row), value);
      expect(validateClusterClaim(s, ctx())).toEqual([
        {
          path: pathFrom("", refusedSteps(row)),
          message: MESSAGES[row.setBy](name),
        },
      ]);
    },
  );

  it("suspend: false and an empty string count as unset", () => {
    const s = open(
      claim({ cluster: { suspend: false, presetRef: "", version: "" } }),
    );
    expect(validateClusterClaim(s, ctx())).toEqual([]);
  });

  it("refuses a parameter outside the claim-editable list", () => {
    const policy = {
      sections: { access: { allowed: ["gateway-access"] } },
      parameters: {
        "gateway-access": {
          hostnamePattern: { value: "{name}.a.example.com" },
        },
      },
    } as unknown as ResolvedPolicy;
    const s = set(
      open(claim({}, policy)),
      [...ACCESS, f("parameters"), f("hostnamePattern")],
      "{name}.b.example.com",
    );
    expect(validateClusterClaim(s, ctx())).toEqual([
      {
        message:
          'parameter "hostnamePattern" of registration "gateway-access" is not claim-editable',
      },
    ]);
  });

  it("refuses a field that a selected slot fills", () => {
    const s = set(
      open(),
      [f("spec"), f("cluster"), f("externalUrl")],
      "https://orders.example.com",
    );
    expect(validateClusterClaim(s, ctx())).toEqual([
      {
        message:
          'spec.cluster.externalUrl is filled by section "access", so the claim cannot set it',
      },
    ]);
  });

  it("refuses a parameter that fails the registration schema", () => {
    const steps = [...ACCESS, f("parameters"), f("replicas")];
    const s = set(open(), steps, "three");
    const errors = validateClusterClaim(s, ctx());
    expect(errors).toHaveLength(1);
    expect(errors[0].path).toBe(pathFrom("", steps));
    expect(errors[0].message).toMatch(/integer/);
  });

  it("refuses a release that skips a minor", () => {
    const s = set(open(), [f("spec"), f("releaseRef")], "camunda-8-11-0");
    expect(validateClusterClaim(s, ctx())).toEqual([
      {
        path: "spec.releaseRef",
        message: "Skips 8.10. Move to an 8.10 release first.",
      },
    ]);
  });

  it("refuses a release that does not exist", () => {
    const s = set(open(), [f("spec"), f("releaseRef")], "camunda-9-0-0");
    expect(validateClusterClaim(s, ctx())).toEqual([
      {
        path: "spec.releaseRef",
        message: "Release camunda-9-0-0 does not exist.",
      },
    ]);
  });

  it("refuses a downgrade without allowVersionDowngrade, accepts it with the target version", () => {
    const down = set(open(), [f("spec"), f("releaseRef")], "camunda-8-9-2");
    expect(validateClusterClaim(down, ctx())).toEqual([
      {
        path: "spec.releaseRef",
        message:
          "8.9.5 to 8.9.2 is a downgrade. A downgrade needs allowVersionDowngrade set to 8.9.2.",
      },
    ]);
    const allowed = set(down, [f("spec"), f("allowVersionDowngrade")], "8.9.2");
    expect(validateClusterClaim(allowed, ctx())).toEqual([]);
  });

  it("accepts a move to the next minor", () => {
    const s = set(open(), [f("spec"), f("releaseRef")], "camunda-8-10-1");
    expect(validateClusterClaim(s, ctx())).toEqual([]);
  });

  it("does not check an unchanged release against the Environment", () => {
    const environment = {
      apiVersion: "fleet.camunda.io/v1alpha1",
      kind: "Environment",
      metadata: { name: "prod" },
      spec: {
        presets: { camundaCluster: ["medium"] },
        releases: { allowed: ["camunda-8-10-1"] },
      },
    } as unknown as Resource<"Environment">;
    const policy = {
      sections: { access: { allowed: ["gateway-access"] } },
      parameters: {
        "gateway-access": {
          hostnamePattern: { claimEditable: true, value: "x" },
          replicas: { claimEditable: true },
        },
      },
    } as unknown as ResolvedPolicy;
    const s = set(
      open(claim({}, policy)),
      [...ACCESS, f("parameters"), f("replicas")],
      2,
    );
    expect(validateClusterClaim(s, ctx({ environment }))).toEqual([]);

    const moved = set(s, [f("spec"), f("releaseRef")], "camunda-8-9-2");
    expect(validateClusterClaim(moved, ctx({ environment }))).toContainEqual({
      message:
        'CamundaRelease "camunda-8-9-2" is not in spec.releases.allowed of Environment "prod"',
    });
  });

  it("checks the values that a reset leaves", () => {
    const s = open(claim({ cluster: { externalUrl: "https://x" } }));
    const result = resetValue(s, [f("spec"), f("cluster"), f("externalUrl")]);
    if (!("session" in result)) throw new Error("refused");
    expect(validateClusterClaim(result.session, ctx())).toEqual([]);
  });
});
