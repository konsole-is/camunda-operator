import { describe, expect, it } from "vitest";
import type { Environment } from "@/generated/crds/environment";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import { BUILTIN_CONTROLLER } from "@/lib/labels";
import {
  allowedErrors,
  optionSettings,
  settingErrors,
  withParameterPolicy,
} from "./settings";

const registration = (
  name: string,
  sections: string[],
  extra: Partial<PluginRegistration["spec"]> = {},
): PluginRegistration =>
  ({
    apiVersion: "plugins.camunda.io/v1alpha1",
    kind: "PluginRegistration",
    metadata: { name },
    spec: {
      label: `Label of ${name}`,
      sections,
      mode: "Optional",
      deletionPolicies: ["Delete"],
      serviceAccount: { name: "sa", namespace: "ns" },
      ...extra,
    },
  }) as PluginRegistration;

const role = registration("aws-connectors-role", ["connectors-permissions"], {
  parameters: {
    type: "object",
    properties: {
      replicas: { type: "integer", minimum: 1, maximum: 9, title: "Replicas" },
      tier: { type: "string", enum: ["gold", "silver"] },
      inlinePolicy: { type: "object", "x-fleet-template": true },
    },
  },
  parameterValues: { replicas: 3 },
  limits: [{ label: "Permissions boundary", value: "arn:aws:iam::1:policy/b" }],
});

const allowing = (
  sections: Record<string, string[]>,
  parameters?: Environment["spec"]["parameters"],
): Environment["spec"] => ({
  sections: Object.fromEntries(
    Object.entries(sections).map(([name, allowed]) => [name, { allowed }]),
  ),
  ...(parameters && { parameters }),
});

describe("optionSettings", () => {
  it("returns one entry for each registration that a section allows, by name, with its label and limits", () => {
    const settings = optionSettings(
      [
        role,
        registration("rustfs", ["backup-storage"]),
        registration("s3", ["backup-storage"]),
      ],
      allowing({
        "connectors-permissions": ["aws-connectors-role"],
        "backup-storage": ["s3", "gone"],
      }),
    );
    expect(settings.map((s) => [s.registration, s.label])).toEqual([
      ["aws-connectors-role", "Label of aws-connectors-role"],
      ["s3", "Label of s3"],
    ]);
    expect(settings[0].limits).toEqual([
      { label: "Permissions boundary", value: "arn:aws:iam::1:policy/b" },
    ]);
  });

  it("lists the parameters of each allowed registration in schema order, then the keys of the policy", () => {
    const [settings] = optionSettings(
      [role],
      allowing(
        { "connectors-permissions": ["aws-connectors-role"] },
        { "aws-connectors-role": { zone: { value: "a" }, extra: {} } },
      ),
    );
    expect(settings.parameters.map((p) => p.name)).toEqual([
      "replicas",
      "tier",
      "inlinePolicy",
      "extra",
      "zone",
    ]);
  });

  it("takes the label, the type, the range, and the enum from the schema", () => {
    const [settings] = optionSettings(
      [role],
      allowing({ "connectors-permissions": ["aws-connectors-role"] }),
    );
    expect(settings.parameters[0]).toMatchObject({
      label: "Replicas",
      schema: { type: "integer", minimum: 1, maximum: 9, title: "Replicas" },
    });
    expect(settings.parameters[1]).toMatchObject({
      label: "tier",
      schema: { type: "string", enum: ["gold", "silver"] },
    });
  });

  it("gives the summary of the registration value and the policy of the environment", () => {
    const [settings] = optionSettings(
      [role],
      allowing(
        { "connectors-permissions": ["aws-connectors-role"] },
        {
          "aws-connectors-role": {
            tier: {
              value: "gold",
              claimEditable: true,
              allowed: [{ name: "g", label: "Gold", value: "gold" }],
            },
          },
        },
      ),
    );
    expect(settings.parameters[0]).toMatchObject({
      registrationSummary: "3",
      claimEditable: false,
      allowed: [],
    });
    expect(settings.parameters[0].value).toBeUndefined();
    expect(settings.parameters[1]).toMatchObject({
      registrationSummary: "Not set",
      value: "gold",
      claimEditable: true,
      allowed: [{ name: "g", label: "Gold", value: "gold" }],
    });
  });

  it("hides the parameters that the built-in table hides", () => {
    const elasticsearch = registration("elasticsearch", ["secondary-storage"], {
      controller: { name: BUILTIN_CONTROLLER },
      parameters: {
        type: "object",
        properties: {
          preset: { type: "string" },
          releaseRef: { type: "string" },
        },
      },
    } as Partial<PluginRegistration["spec"]>);
    const [settings] = optionSettings(
      [elasticsearch],
      allowing({ "secondary-storage": ["elasticsearch"] }),
    );
    expect(settings.parameters.map((p) => p.name)).toEqual(["preset"]);
  });

  it("names the preset kind of the parameter that lists presets", () => {
    const database = registration("database-server", ["secondary-storage"], {
      controller: { name: BUILTIN_CONTROLLER },
      parameters: {
        type: "object",
        properties: { preset: { type: "string" } },
      },
    } as Partial<PluginRegistration["spec"]>);
    const [settings] = optionSettings(
      [database],
      allowing({ "secondary-storage": ["database-server"] }),
    );
    expect(settings.parameters[0].presetKind).toBe("DatabaseServerPreset");
  });

  it("marks the access patterns and the template parameters", () => {
    const gateway = registration("gateway-public", ["access"], {
      parameters: {
        type: "object",
        properties: {
          hostnamePattern: { type: "string" },
          other: { type: "string" },
        },
      },
    });
    const settings = optionSettings(
      [role, gateway],
      allowing({
        access: ["gateway-public"],
        "connectors-permissions": ["aws-connectors-role"],
      }),
    );
    const flags = settings.flatMap((s) =>
      s.parameters.map((p) => [p.name, p.hostPattern, p.template]),
    );
    expect(flags).toEqual([
      ["replicas", false, false],
      ["tier", false, false],
      ["inlinePolicy", false, true],
      ["hostnamePattern", true, false],
      ["other", false, false],
    ]);
  });
});

describe("withParameterPolicy", () => {
  it("sets the policy of one parameter and keeps the others", () => {
    expect(
      withParameterPolicy(
        { parameters: { a: { x: { value: 1 } } } },
        "a",
        "y",
        { value: 2, claimEditable: true },
      ),
    ).toEqual({
      parameters: {
        a: { x: { value: 1 }, y: { value: 2, claimEditable: true } },
      },
    });
  });

  it("drops allowed when claimEditable turns off", () => {
    expect(
      withParameterPolicy({}, "a", "x", {
        value: 1,
        claimEditable: false,
        allowed: [{ name: "n", label: "N", value: 1 }],
      }),
    ).toEqual({ parameters: { a: { x: { value: 1 } } } });
  });

  it("keeps an empty allowed list out of the policy", () => {
    expect(
      withParameterPolicy({}, "a", "x", { claimEditable: true, allowed: [] }),
    ).toEqual({ parameters: { a: { x: { claimEditable: true } } } });
  });

  it("removes the entry of an empty policy and the registration without entries", () => {
    expect(
      withParameterPolicy(
        { displayName: "Staging", parameters: { a: { x: { value: 1 } } } },
        "a",
        "x",
        {},
      ),
    ).toEqual({ displayName: "Staging" });
    expect(
      withParameterPolicy(
        { parameters: { a: { x: { value: 1 } }, b: { y: { value: 2 } } } },
        "a",
        "x",
        { claimEditable: false },
      ),
    ).toEqual({ parameters: { b: { y: { value: 2 } } } });
  });

  it("does not change spec", () => {
    const spec: Environment["spec"] = {
      parameters: { a: { x: { value: 1 } } },
    };
    withParameterPolicy(spec, "a", "x", {});
    expect(spec).toEqual({ parameters: { a: { x: { value: 1 } } } });
  });
});

describe("allowedErrors", () => {
  it("refuses a duplicate name", () =>
    expect(
      allowedErrors([
        { name: "sqs", label: "SQS", value: 1 },
        { name: "sqs", label: "Other", value: 2 },
      ]),
    ).toEqual([{ path: "1.name", message: "Each name is used once." }]));

  it("refuses a duplicate label", () =>
    expect(
      allowedErrors([
        { name: "a", label: "SQS", value: 1 },
        { name: "b", label: "SQS", value: 2 },
      ]),
    ).toEqual([{ path: "1.label", message: "Each label is used once." }]));

  it("refuses an empty name or label", () =>
    expect(allowedErrors([{ name: "", label: " ", value: 1 }])).toEqual([
      { path: "0.name", message: "Give the choice a name." },
      { path: "0.label", message: "Give the choice a label." },
    ]));

  it("accepts distinct names and labels", () =>
    expect(
      allowedErrors([
        { name: "a", label: "A", value: 1 },
        { name: "b", label: "B", value: 1 },
      ]),
    ).toEqual([]));
});

describe("settingErrors", () => {
  it("names a value and an allowed value that their schema refuses", () => {
    const settings = optionSettings(
      [role],
      allowing(
        { "connectors-permissions": ["aws-connectors-role"] },
        {
          "aws-connectors-role": {
            replicas: {
              value: 12,
              claimEditable: true,
              allowed: [{ name: "a", label: "A", value: 0 }],
            },
          },
        },
      ),
    );
    expect(settingErrors(settings).map((e) => e.path)).toEqual([
      "spec.parameters.aws-connectors-role.replicas.value",
      "spec.parameters.aws-connectors-role.replicas.allowed.0.value",
    ]);
  });

  it("checks each allowed value of an array parameter against its items schema", () => {
    const list = registration("lists", ["s"], {
      parameters: {
        type: "object",
        properties: {
          arns: { type: "array", items: { type: "string", pattern: "^arn:" } },
        },
      },
    });
    const settings = optionSettings(
      [list],
      allowing(
        { s: ["lists"] },
        {
          lists: {
            arns: {
              claimEditable: true,
              allowed: [
                { name: "a", label: "A", value: "arn:x" },
                { name: "b", label: "B", value: "nope" },
              ],
            },
          },
        },
      ),
    );
    expect(settingErrors(settings).map((e) => e.path)).toEqual([
      "spec.parameters.lists.arns.allowed.1.value",
    ]);
  });
});

describe("settingErrors of a template parameter", () => {
  it("names a stored template that is not a valid template parameter, also in a choice", () => {
    const settings = optionSettings(
      [role],
      allowing(
        { "connectors-permissions": ["aws-connectors-role"] },
        {
          "aws-connectors-role": {
            inlinePolicy: {
              value: { Resource: "{{ .outputs.bucket }}" },
              claimEditable: true,
              allowed: [
                {
                  name: "a",
                  label: "A",
                  value: { Resource: "{{ .namespace }}" },
                },
                { name: "b", label: "B", value: { Resource: "{{ .nope }}" } },
              ],
            },
          },
        },
      ),
    );
    expect(settingErrors(settings)).toEqual([
      {
        path: "spec.parameters.aws-connectors-role.inlinePolicy.value",
        message:
          "Resource: the placeholder .outputs.bucket is not allowed: a template parameter cannot read the outputs",
      },
      {
        path: "spec.parameters.aws-connectors-role.inlinePolicy.allowed.1.value",
        message: "Resource: unknown placeholder .nope",
      },
    ]);
  });
});
