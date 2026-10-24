import { describe, expect, it } from "vitest";
import type { Environment } from "@/generated/crds/environment";
import type {
  PluginRegistration,
  PluginRegistrationSpec,
} from "@/generated/crds/pluginregistration";
import type { PluginSection } from "@/generated/crds/pluginsection";
import { claimEditable, lockedSummary, sectionOptions } from "./options";

const READY = [
  { type: "Accepted", status: "True" },
  { type: "Ready", status: "True" },
] as const;

function reg(
  spec: Partial<PluginRegistrationSpec> = {},
  name = "s3",
  conditions: readonly { type: string; status: string }[] = READY,
): PluginRegistration {
  return {
    apiVersion: "plugins.camunda.io/v1alpha1",
    kind: "PluginRegistration",
    metadata: { name },
    spec: {
      sections: ["backup-storage"],
      mode: "Optional",
      label: `${name} label`,
      deletionPolicies: ["Delete"],
      serviceAccount: { name, namespace: "plugins" },
      ...spec,
    },
    status: {
      conditions: conditions.map((c) => ({
        ...c,
        status: c.status as "True" | "False" | "Unknown",
        reason: "Test",
        message: "",
        lastTransitionTime: "2026-10-21T00:00:00Z",
      })),
    },
  };
}

function envWith(
  parameters: NonNullable<Environment["spec"]["parameters"]>[string] = {},
  spec: Partial<Environment["spec"]> = {},
): Environment {
  return {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "Environment",
    metadata: { name: "dev" },
    spec: { parameters: { s3: parameters }, ...spec },
  };
}

function section(spec: Partial<PluginSection["spec"]> = {}): PluginSection {
  return {
    apiVersion: "plugins.camunda.io/v1alpha1",
    kind: "PluginSection",
    metadata: { name: "backup-storage" },
    spec: {
      appliesTo: ["CamundaClusterClaim"],
      label: "Backup storage",
      selection: "One",
      ...spec,
    },
  };
}

describe("claimEditable", () => {
  it("returns a locked parameter with its summary and no value", () => {
    const [rule] = claimEditable(
      reg({ parameterValues: { policies: ["a", "b", "c"] } }),
      undefined,
    );

    expect(rule).toMatchObject({
      name: "policies",
      label: "policies",
      claimEditable: false,
      summary: "3 items",
    });
    expect(rule).not.toHaveProperty("value");
  });

  it("returns an editable parameter with its value and the names and labels of its choices", () => {
    const rules = claimEditable(
      reg({ parameterValues: { access: "sqs" } }),
      envWith({
        access: {
          claimEditable: true,
          allowed: [
            {
              name: "sqs",
              label: "SQS access",
              value: "arn:aws:iam::aws:policy/AmazonSQSFullAccess",
            },
          ],
        },
      }),
    );

    expect(rules[0]).toMatchObject({
      claimEditable: true,
      value: "sqs",
      allowed: [{ name: "sqs", label: "SQS access" }],
    });
    expect(rules[0].claimEditable && rules[0].allowed?.[0]).not.toHaveProperty(
      "value",
    );
  });

  it("returns the keys of claimEditable of the registration as editable without an environment", () => {
    const rules = claimEditable(
      reg({ parameterValues: { tier: "standard" }, claimEditable: ["tier"] }),
      undefined,
    );

    expect(rules).toEqual([
      { name: "tier", label: "tier", claimEditable: true, value: "standard" },
    ]);
  });

  it("gives a key that is not a property the schema of additionalProperties, and keeps its name as the label", () => {
    const value = {
      type: "string",
      title: "Any value",
      "x-kubernetes-validations": [{ rule: "self != ''" }],
    };
    const rules = claimEditable(
      reg({
        parameters: {
          type: "object",
          properties: { region: { type: "string" } },
          additionalProperties: value,
        },
        claimEditable: ["bucket"],
        parameterValues: { region: "eu" },
      }),
      undefined,
    );

    expect(rules).toEqual([
      {
        name: "region",
        label: "region",
        schema: { type: "string" },
        claimEditable: false,
        summary: "eu",
      },
      { name: "bucket", label: "bucket", schema: value, claimEditable: true },
    ]);
  });

  it("lists the schema properties in schema order with their title, schema, and required, then the other keys by name", () => {
    const bucket = { type: "string", title: "Bucket name" };
    const rules = claimEditable(
      reg({
        parameters: {
          type: "object",
          properties: { bucket, region: { type: "string" } },
          required: ["bucket"],
        },
        parameterValues: { zone: "a", extra: 1, region: "eu-west-1" },
      }),
      undefined,
    );

    expect(rules.map((r) => r.name)).toEqual([
      "bucket",
      "region",
      "extra",
      "zone",
    ]);
    expect(rules[0]).toEqual({
      name: "bucket",
      label: "Bucket name",
      schema: bucket,
      required: true,
      claimEditable: false,
      summary: "Not set",
    });
    expect(rules[1]).not.toHaveProperty("required");
  });

  it.each([
    ["a value", "arn:sqs", "sqs"],
    ["a list of values", ["arn:sqs", "arn:sns"], ["sqs", "sns"]],
  ])(
    "gives an editable parameter with choices %s as the names of the choices",
    (_, value, names) => {
      const [rule] = claimEditable(
        reg({ parameterValues: { access: value } }),
        envWith({
          access: {
            claimEditable: true,
            allowed: [
              { name: "sqs", label: "SQS access", value: "arn:sqs" },
              { name: "sns", label: "SNS access", value: "arn:sns" },
            ],
          },
        }),
      );

      expect(rule).toMatchObject({ claimEditable: true, value: names });
    },
  );

  it("gives a list value that one choice holds as the name of that choice", () => {
    const [rule] = claimEditable(
      reg({ parameterValues: { zones: [1, 2] } }),
      envWith({
        zones: {
          claimEditable: true,
          allowed: [{ name: "standard", label: "Standard", value: [1, 2] }],
        },
      }),
    );

    expect(rule).toMatchObject({ value: "standard" });
  });

  it("marks a parameter whose every choice is a list as one that takes one name", () => {
    const [rule] = claimEditable(
      reg({ parameterValues: { zones: ["a", "b"] } }),
      envWith({
        zones: {
          claimEditable: true,
          allowed: [
            { name: "standard", label: "Standard", value: ["a", "b"] },
            { name: "wide", label: "Wide", value: ["a", "b", "c"] },
          ],
        },
      }),
    );

    expect(rule).toMatchObject({ value: "standard", allowedLists: true });
  });

  it.each([
    ["one choice is not a list", ["a", "b"]],
    ["no choice is a list", "a"],
  ])(
    "does not mark a parameter as one that takes one name when %s",
    (_, value) => {
      const [rule] = claimEditable(
        reg(),
        envWith({
          zones: {
            claimEditable: true,
            allowed: [
              { name: "standard", label: "Standard", value },
              { name: "single", label: "Single", value: "c" },
            ],
          },
        }),
      );

      expect(rule).not.toHaveProperty("allowedLists");
    },
  );

  it("keeps a value of an editable parameter that no choice has", () => {
    const [rule] = claimEditable(
      reg({ parameterValues: { access: ["arn:sqs", "arn:other"] } }),
      envWith({
        access: {
          claimEditable: true,
          allowed: [{ name: "sqs", label: "SQS access", value: "arn:sqs" }],
        },
      }),
    );

    expect(rule).toMatchObject({ value: ["arn:sqs", "arn:other"] });
  });

  it("returns a rule for a parameter named constructor that only the policy has", () => {
    expect(
      claimEditable(
        reg({
          parameterValues: { constructor: "default" },
          claimEditable: ["constructor"],
        }),
        undefined,
      ),
    ).toEqual([
      {
        name: "constructor",
        label: "constructor",
        claimEditable: true,
        value: "default",
      },
    ]);
  });

  it("counts parameterValues that is not an object as no values", () => {
    expect(
      claimEditable(
        reg({ parameterValues: "broken", claimEditable: ["a"] }),
        undefined,
      ),
    ).toEqual([{ name: "a", label: "a", claimEditable: true }]);
  });
});

describe("lockedSummary", () => {
  it.each([
    [undefined, "Not set"],
    [null, "Not set"],
    ["eu-west-1", "eu-west-1"],
    [3, "3"],
    [true, "true"],
    [[], "No items"],
    [["a"], "1 item"],
    [["a", "b", "c"], "3 items"],
    [{}, "No fields"],
    [{ a: 1 }, "1 field"],
    [{ a: 1, b: 2 }, "2 fields"],
  ])("summarizes %j as %s", (value, summary) => {
    expect(lockedSummary(value)).toBe(summary);
  });
});

describe("sectionOptions", () => {
  it("sets choices.parameter to preset for a registration with a preset parameter", () => {
    const view = sectionOptions(
      section(),
      [
        reg({
          parameters: {
            type: "object",
            properties: {
              preset: { type: "string" },
              overrides: { type: "object" },
            },
          },
          choices: [{ name: "small" }],
        }),
      ],
      undefined,
      "CamundaClusterClaim",
    );

    expect(view.options[0].choices).toEqual({
      parameter: "preset",
      items: [{ name: "small" }],
    });
  });

  it("sets choices.parameter to name for a registration without a preset parameter", () => {
    const view = sectionOptions(
      section(),
      [
        reg({
          parameters: {
            type: "object",
            properties: { name: { type: "string" } },
          },
          choices: [{ name: "shared-bucket" }],
        }),
      ],
      undefined,
      "CamundaClusterClaim",
    );

    expect(view.options[0].choices?.parameter).toBe("name");
  });

  it("sets no choices when the environment gives the choice parameter an allowed list", () => {
    const view = sectionOptions(
      section(),
      [
        reg({
          parameters: {
            type: "object",
            properties: { preset: { type: "string" } },
          },
          choices: [{ name: "small" }, { name: "large" }],
        }),
      ],
      envWith(
        {
          preset: {
            claimEditable: true,
            allowed: [{ name: "standard", label: "Standard", value: "small" }],
          },
        },
        { sections: { "backup-storage": { allowed: ["s3"] } } },
      ),
      "CamundaClusterClaim",
    );

    expect(view.options[0]).not.toHaveProperty("choices");
  });

  it("sets no choices for a registration without choices", () => {
    const view = sectionOptions(
      section(),
      [reg({ choices: [] })],
      undefined,
      "CamundaClusterClaim",
    );

    expect(view.options[0]).not.toHaveProperty("choices");
  });

  it("lists the allowed registrations in the order of the environment and marks its default", () => {
    const view = sectionOptions(
      section(),
      [reg({}, "a"), reg({}, "b"), reg({}, "c")],
      envWith(
        {},
        {
          sections: {
            "backup-storage": {
              allowed: ["c", "a"],
              default: "a",
              required: true,
            },
          },
        },
      ),
      "CamundaClusterClaim",
    );

    expect(view.required).toBe(true);
    expect(view.options.map((o) => [o.registration, o.isDefault])).toEqual([
      ["c", false],
      ["a", true],
    ]);
  });

  it("lists every registration of the section by name without an environment and marks the one Default", () => {
    const view = sectionOptions(
      section(),
      [
        reg({ mode: "Default" }, "b"),
        reg({}, "a"),
        reg({ sections: ["secondary-storage"] }, "other"),
      ],
      undefined,
      "CamundaClusterClaim",
    );

    expect(view.required).toBe(false);
    expect(view.options.map((o) => [o.registration, o.isDefault])).toEqual([
      ["a", false],
      ["b", true],
    ]);
  });

  it("offers only the empty option for a section that the environment does not list", () => {
    const view = sectionOptions(
      section({ emptyOptionLabel: "No backups" }),
      [reg()],
      envWith({}, { sections: {} }),
      "CamundaClusterClaim",
    );

    expect(view).toMatchObject({
      options: [],
      required: false,
      emptyOptionLabel: "No backups",
    });
  });

  it.each([
    ["Accepted is False", [{ type: "Accepted", status: "False" }, READY[1]]],
    ["Ready is False", [READY[0], { type: "Ready", status: "False" }]],
    ["Ready is missing", [READY[0]]],
  ])("leaves out a registration whose %s", (_, conditions) => {
    const view = sectionOptions(
      section(),
      [reg({}, "a", conditions), reg({}, "b")],
      undefined,
      "CamundaClusterClaim",
    );

    expect(view.options.map((o) => o.registration)).toEqual(["b"]);
  });

  it("leaves out an allowed registration that does not serve the section", () => {
    const view = sectionOptions(
      section(),
      [reg({}, "a"), reg({ sections: ["secondary-storage"] }, "elastic")],
      envWith(
        {},
        { sections: { "backup-storage": { allowed: ["elastic", "a"] } } },
      ),
      "CamundaClusterClaim",
    );

    expect(view.options.map((o) => o.registration)).toEqual(["a"]);
  });

  it("leaves out an allowed registration that does not exist", () => {
    const view = sectionOptions(
      section(),
      [reg({}, "a")],
      envWith(
        {},
        { sections: { "backup-storage": { allowed: ["gone", "a"] } } },
      ),
      "CamundaClusterClaim",
    );

    expect(view.options.map((o) => o.registration)).toEqual(["a"]);
  });

  it("has no options for a section that does not apply to the claim kind", () => {
    const view = sectionOptions(
      section({ appliesTo: ["CamundaManagementClusterClaim"] }),
      [reg()],
      envWith(
        {},
        { sections: { "backup-storage": { allowed: ["s3"], required: true } } },
      ),
      "CamundaClusterClaim",
    );

    expect(view).toMatchObject({ options: [], required: false });
  });

  it("hides nothing that the section says: label, description, selection, emptyOptionLabel, required", () => {
    const view = sectionOptions(
      section({
        description: "Where backups go",
        selection: "Many",
        emptyOptionLabel: "None",
      }),
      [],
      undefined,
      "CamundaClusterClaim",
    );

    expect(view).toEqual({
      name: "backup-storage",
      label: "Backup storage",
      description: "Where backups go",
      selection: "Many",
      emptyOptionLabel: "None",
      required: false,
      options: [],
    });
  });

  it("gives an option the label, description, docs link, and parameter rules of its registration", () => {
    const registration = reg({
      description: "S3 buckets",
      docsUrl: "https://example.com/s3",
      parameterValues: { region: "eu-west-1" },
    });

    const [option] = sectionOptions(
      section(),
      [registration],
      undefined,
      "CamundaClusterClaim",
    ).options;

    expect(option).toEqual({
      registration: "s3",
      label: "s3 label",
      description: "S3 buckets",
      docsUrl: "https://example.com/s3",
      isDefault: false,
      parameters: claimEditable(registration, undefined),
    });
  });

  it("passes the parameter schema of the registration to the option", () => {
    const parameters = {
      type: "object",
      properties: { min: { type: "integer" }, max: { type: "integer" } },
      "x-kubernetes-validations": [
        { rule: "self.min <= self.max", message: "min must not be above max" },
      ],
    };

    const [withSchema] = sectionOptions(
      section(),
      [reg({ parameters })],
      undefined,
      "CamundaClusterClaim",
    ).options;
    const [withoutSchema] = sectionOptions(
      section(),
      [reg()],
      undefined,
      "CamundaClusterClaim",
    ).options;

    expect(withSchema.schema).toEqual(parameters);
    expect(withoutSchema.schema).toBeUndefined();
  });
});
