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

  it("checks a changed release against the Environment without a resolved policy", () => {
    const environment = {
      apiVersion: "fleet.camunda.io/v1alpha1",
      kind: "Environment",
      metadata: { name: "prod" },
      spec: { releases: { allowed: ["camunda-8-9-5"] } },
    } as unknown as Resource<"Environment">;
    const s = set(open(), [f("spec"), f("releaseRef")], "camunda-8-10-1");
    expect(validateClusterClaim(s, ctx({ environment }))).toEqual([
      {
        message:
          'CamundaRelease "camunda-8-10-1" is not in spec.releases.allowed of Environment "prod"',
      },
    ]);
  });

  it("checks the default release of the Environment after a release reset", () => {
    const environment = {
      apiVersion: "fleet.camunda.io/v1alpha1",
      kind: "Environment",
      metadata: { name: "prod" },
      spec: {
        releases: {
          allowed: ["camunda-8-9-5", "camunda-8-11-0"],
          default: "camunda-8-11-0",
        },
      },
    } as unknown as Resource<"Environment">;
    const result = resetValue(open(), [f("spec"), f("releaseRef")]);
    if (!("session" in result)) throw new Error("refused");
    expect(validateClusterClaim(result.session, ctx({ environment }))).toEqual([
      {
        path: "spec.releaseRef",
        message: "Skips 8.10. Move to an 8.10 release first.",
      },
    ]);
  });

  it("does not check a parameter with choices against the schema", () => {
    const policy = {
      sections: { access: { allowed: ["gateway-access"] } },
      parameters: {
        "gateway-access": {
          hostnamePattern: { claimEditable: true, value: "x" },
          replicas: {
            claimEditable: true,
            allowed: [{ name: "small", label: "Small", value: 1 }],
          },
        },
      },
    } as unknown as ResolvedPolicy;
    const s = set(
      open(claim({}, policy)),
      [...ACCESS, f("parameters"), f("replicas")],
      "small",
    );
    expect(validateClusterClaim(s, ctx())).toEqual([]);
  });

  it("validates a claim whose spec a reset empties", () => {
    const only = {
      ...claim(),
      spec: { presetRef: "medium" },
    } as unknown as Parameters<typeof open>[0];
    const result = resetValue(open(only), [f("spec"), f("presetRef")]);
    if (!("session" in result)) throw new Error("refused");
    expect(validateClusterClaim(result.session, ctx())).toEqual([]);
  });

  it("checks a release that a change of the whole spec sets", () => {
    const s = set(open(), [f("spec")], {
      ...claim().spec,
      releaseRef: "camunda-8-11-0",
    });
    expect(validateClusterClaim(s, ctx())).toEqual([
      {
        path: "spec.releaseRef",
        message: "Skips 8.10. Move to an 8.10 release first.",
      },
    ]);
  });

  it("checks a parameter by the additionalProperties schema of its registration", () => {
    const c = ctx();
    c.registrations = [
      {
        apiVersion: "plugins.camunda.io/v1alpha1",
        kind: "PluginRegistration",
        metadata: { name: "gateway-access" },
        spec: {
          parameters: {
            type: "object",
            additionalProperties: { type: "integer" },
          },
        },
      } as unknown as Resource<"PluginRegistration">,
    ];
    const steps = [...ACCESS, f("parameters"), f("replicas")];
    const errors = validateClusterClaim(set(open(), steps, "three"), c);
    expect(errors).toHaveLength(1);
    expect(errors[0].path).toBe(pathFrom("", steps));
  });

  it("resolves a reference of the registration schema", () => {
    const c = ctx();
    c.registrations = [
      {
        apiVersion: "plugins.camunda.io/v1alpha1",
        kind: "PluginRegistration",
        metadata: { name: "gateway-access" },
        spec: {
          parameters: {
            type: "object",
            definitions: { Port: { type: "integer" } },
            properties: { port: { $ref: "#/definitions/Port" } },
          },
        },
      } as unknown as Resource<"PluginRegistration">,
    ];
    const steps = [...ACCESS, f("parameters"), f("port")];
    expect(validateClusterClaim(set(open(), steps, 8080), c)).toEqual([]);
    expect(validateClusterClaim(set(open(), steps, "x"), c)).toHaveLength(1);
  });

  it("resolves a reference to another parameter of the schema", () => {
    const c = ctx();
    c.registrations = [
      {
        apiVersion: "plugins.camunda.io/v1alpha1",
        kind: "PluginRegistration",
        metadata: { name: "gateway-access" },
        spec: {
          parameters: {
            type: "object",
            properties: {
              port: { type: "integer" },
              adminPort: { $ref: "#/properties/port" },
            },
          },
        },
      } as unknown as Resource<"PluginRegistration">,
    ];
    const steps = [...ACCESS, f("parameters"), f("adminPort")];
    expect(validateClusterClaim(set(open(), steps, 9090), c)).toEqual([]);
  });

  it("does not report a constraint of the whole parameters at one parameter", () => {
    const c = ctx();
    c.registrations = [
      {
        apiVersion: "plugins.camunda.io/v1alpha1",
        kind: "PluginRegistration",
        metadata: { name: "gateway-access" },
        spec: {
          parameters: {
            type: "object",
            minProperties: 3,
            properties: { replicas: { type: "integer" } },
          },
        },
      } as unknown as Resource<"PluginRegistration">,
    ];
    const steps = [...ACCESS, f("parameters"), f("replicas")];
    expect(validateClusterClaim(set(open(), steps, 2), c)).toEqual([]);
  });

  describe("checks a parameter beside the resolved values of the others", () => {
    // replicas may be at most 3 only while tier is 1.
    const tiered = (): ClusterClaimContext => {
      const c = ctx();
      c.registrations = [
        {
          apiVersion: "plugins.camunda.io/v1alpha1",
          kind: "PluginRegistration",
          metadata: { name: "gateway-access" },
          spec: {
            parameters: {
              type: "object",
              if: { required: ["tier"], properties: { tier: { const: 1 } } },
              then: { properties: { replicas: { maximum: 3 } } },
              else: { properties: { replicas: { minimum: 5 } } },
            },
          },
        } as unknown as Resource<"PluginRegistration">,
      ];
      return c;
    };
    const steps = [...ACCESS, f("parameters"), f("replicas")];

    it("the value of a sibling choice", () => {
      const policy = {
        sections: { access: { allowed: ["gateway-access"] } },
        parameters: {
          "gateway-access": {
            tier: {
              claimEditable: true,
              allowed: [{ name: "small", label: "Small", value: 1 }],
            },
            replicas: { claimEditable: true },
          },
        },
      } as unknown as ResolvedPolicy;
      const c = claim(
        {
          plugins: [
            {
              section: "access",
              registration: "gateway-access",
              parameters: { tier: "small" },
            },
          ],
        },
        policy,
      );
      expect(validateClusterClaim(set(open(c), steps, 2), tiered())).toEqual(
        [],
      );
    });

    it("a value that the policy sets", () => {
      const policy = {
        sections: { access: { allowed: ["gateway-access"] } },
        parameters: {
          "gateway-access": {
            tier: { value: 1 },
            replicas: { claimEditable: true },
          },
        },
      } as unknown as ResolvedPolicy;
      const c = claim(
        {
          plugins: [{ section: "access", registration: "gateway-access" }],
        },
        policy,
      );
      expect(validateClusterClaim(set(open(c), steps, 2), tiered())).toEqual(
        [],
      );
      expect(validateClusterClaim(set(open(c), steps, 4), tiered())).toEqual([
        { path: pathFrom("", steps), message: expect.any(String) },
      ]);
    });
  });

  it("refuses a parameter of a registration whose schema does not compile", () => {
    const c = ctx();
    c.registrations = [
      {
        apiVersion: "plugins.camunda.io/v1alpha1",
        kind: "PluginRegistration",
        metadata: { name: "gateway-access" },
        spec: {
          parameters: {
            type: "object",
            properties: { replicas: { $ref: "#/definitions/Missing" } },
          },
        },
      } as unknown as Resource<"PluginRegistration">,
    ];
    const steps = [...ACCESS, f("parameters"), f("replicas")];
    const errors = validateClusterClaim(set(open(), steps, 2), c);
    expect(errors).toEqual([
      {
        path: pathFrom("", steps),
        message: expect.stringMatching(/^The schema is not valid: /),
      },
    ]);
  });

  it("checks the values that a reset leaves", () => {
    const s = open(claim({ cluster: { externalUrl: "https://x" } }));
    const result = resetValue(s, [f("spec"), f("cluster"), f("externalUrl")]);
    if (!("session" in result)) throw new Error("refused");
    expect(validateClusterClaim(result.session, ctx())).toEqual([]);
  });
});
