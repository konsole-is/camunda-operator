import { describe, expect, it } from "vitest";
import { type FieldSchema, fullSchemas } from "@/generated/fieldschema";
import {
  claimValueOf,
  editorOf,
  editorValueOf,
  fieldSchemaFromOpenAPI,
  overrideEditorOf,
  parameterEditorOf,
  type SettingEditor,
} from "./editor";
import type { OpenAPISchema } from "@/lib/schema";
import { fieldSchemaOf } from "./schema";

// The overrides properties of the elasticsearch registration, as
// internal/pluginstartup/schemas.go writes them.
const OVERRIDES: Record<string, unknown> = {
  replicas: {
    type: "integer",
    format: "int32",
    minimum: 1,
    description: "The number of Elasticsearch nodes.",
  },
  resources: {
    type: "object",
    "x-kubernetes-preserve-unknown-fields": true,
    description: "The CPU and memory of each node.",
  },
  storageSize: {
    type: "string",
    description: "The volume size of each node, for example 50Gi.",
  },
  scheduling: {
    type: "object",
    "x-kubernetes-preserve-unknown-fields": true,
    description: "Where the nodes run.",
  },
  podLabels: {
    type: "object",
    additionalProperties: { type: "string" },
    description: "The labels to add to the Elasticsearch pods.",
  },
};

describe("editorOf", () => {
  it.each<[string, SettingEditor]>([
    ["tasklist.replicas", { kind: "number", integer: true, minimum: 0 }],
    ["gateway.mode", { kind: "select", options: ["Standalone", "Embedded"] }],
    ["connectors.enabled", { kind: "toggle" }],
    ["zeebe.resources", { kind: "resources" }],
    ["zeebe.storageSize", { kind: "storageSize" }],
    ["extraEnv", { kind: "env" }],
    ["podLabels", { kind: "keyValue", format: "label" }],
    ["podAnnotations", { kind: "keyValue" }],
    ["zeebe.scheduling.tolerations", { kind: "tolerations" }],
    [
      "zeebe.scheduling.nodeAffinity",
      { kind: "yaml", schema: fullSchemas.nodeAffinity },
    ],
    ["extraEnvFrom", { kind: "yaml", schema: fullSchemas.extraEnvFrom }],
    ["externalUrl", { kind: "text" }],
    [
      "auth.admin.users",
      { kind: "yaml", schema: { type: "array", items: { type: "string" } } },
    ],
  ])("picks the editor of the claim field %s", (path, editor) => {
    expect(editorOf(path, fieldSchemaOf("cluster", path))).toEqual(editor);
  });

  it("gives no editor to a field without a schema", () => {
    expect(editorOf("connectors.version", undefined)).toBeUndefined();
  });

  describe("with the value of the field", () => {
    const tolerations = "zeebe.scheduling.tolerations";
    const toleration = {
      key: "spot",
      operator: "Equal",
      value: "yes",
      effect: "NoExecute",
      tolerationSeconds: 60,
    };
    const secretEnv = {
      name: "DB_PASSWORD",
      valueFrom: { secretKeyRef: { name: "orders-db", key: "password" } },
    };
    const editorFor = (path: string, value: unknown) =>
      editorOf(path, fieldSchemaOf("cluster", path), { value });

    it.each<[string, string, unknown]>([
      [
        "a toleration with a field that the rows do not show",
        tolerations,
        [{ ...toleration, extra: true }],
      ],
      [
        "a toleration with a value for the operator Exists",
        tolerations,
        [{ key: "spot", operator: "Exists", value: "yes" }],
      ],
      [
        "a toleration with seconds for an effect other than NoExecute",
        tolerations,
        [{ key: "spot", effect: "NoSchedule", tolerationSeconds: 60 }],
      ],
      [
        "a toleration with seconds that are not whole",
        tolerations,
        [{ ...toleration, tolerationSeconds: 1.5 }],
      ],
      [
        "a variable with a field that the rows do not show",
        "extraEnv",
        [{ name: "TZ", value: "UTC", extra: true }],
      ],
      [
        "a variable with a value and a Secret key",
        "extraEnv",
        [{ ...secretEnv, value: "x" }],
      ],
      [
        "a Secret key with a field that the rows do not show",
        "extraEnv",
        [
          {
            name: "DB_PASSWORD",
            valueFrom: {
              secretKeyRef: {
                name: "orders-db",
                key: "password",
                optional: false,
              },
            },
          },
        ],
      ],
      [
        "a scratch size limit that is a number",
        "backup.dump.scratchVolume.sizeLimit",
        1073741824,
      ],
    ])("opens %s as YAML", (_, path, value) => {
      expect(editorFor(path, value)?.kind).toBe("yaml");
    });

    it.each<[string, unknown]>([
      ["backup.dump.scratchVolume.sizeLimit", 1073741824],
      ["zeebe.storageSize", { size: "50Gi" }],
    ])(
      "checks the YAML of the int-or-string %s against an integer or a string",
      (path, value) => {
        expect(editorFor(path, value)).toEqual({
          kind: "yaml",
          schema: { anyOf: [{ type: "integer" }, { type: "string" }] },
        });
      },
    );

    it("checks the YAML against the schema of the field", () => {
      expect(editorFor(tolerations, [{ ...toleration, extra: true }])).toEqual({
        kind: "yaml",
        schema: { type: "array", items: { type: "object" } },
      });
    });

    it.each<[string, string, unknown]>([
      [
        "tolerations with the fields that the rows show",
        tolerations,
        [
          toleration,
          { operator: "Exists" },
          { key: "a", effect: "NoSchedule" },
        ],
      ],
      [
        "variables with a value, a Secret key, or another source",
        "extraEnv",
        [
          { name: "TZ", value: "UTC" },
          secretEnv,
          {
            name: "A",
            valueFrom: {
              secretKeyRef: { name: "orders-db", key: "a", optional: true },
            },
          },
          {
            name: "B",
            valueFrom: { configMapKeyRef: { name: "c", key: "b" } },
          },
          {
            name: "C",
            valueFrom: { fileKeyRef: { volumeName: "v", path: "p", key: "c" } },
          },
          {
            name: "D",
            valueFrom: { fieldRef: { fieldPath: "metadata.name" } },
          },
        ],
      ],
      [
        "resources with claims and number amounts",
        "zeebe.resources",
        { requests: { cpu: 1, memory: "1Gi" }, claims: [{ name: "gpu" }] },
      ],
      ["a storage size that is a string", "zeebe.storageSize", "50Gi"],
      ["a storage size that is a number", "zeebe.storageSize", 10737418240],
      ["null", tolerations, null],
    ])("keeps the editor of the field for %s", (_, path, value) => {
      expect(editorFor(path, value)).toEqual(editorFor(path, undefined));
      expect(editorFor(path, value)?.kind).not.toBe("yaml");
    });
  });
});

describe("editorOf with choices", () => {
  it("picks names from the choices for a list", () => {
    expect(
      editorOf(
        "policies",
        { type: "array", items: { type: "string" } },
        { choices: ["sqs", "s3"] },
      ),
    ).toEqual({ kind: "choices", options: ["sqs", "s3"] });
  });

  it.each<[string, FieldSchema | undefined]>([
    ["a string", { type: "string" }],
    ["an enum", { type: "string", enum: ["gp3", "io2"] }],
    ["an object", {}],
    ["no schema", undefined],
  ])("picks one name from the choices for %s", (_, schema) => {
    expect(editorOf("access", schema, { choices: ["publish"] })).toEqual({
      kind: "select",
      options: ["publish"],
    });
  });

  it("takes the choices before the name of the field", () => {
    expect(
      editorOf("resources", { type: "object" }, { choices: ["small"] }),
    ).toEqual({ kind: "select", options: ["small"] });
  });
});

describe("parameterEditorOf", () => {
  it("gives a map of strings key and value rows", () => {
    expect(
      parameterEditorOf({
        type: "object",
        additionalProperties: { type: "string" },
      }),
    ).toEqual({ kind: "keyValue" });
  });

  it.each<[string, unknown, SettingEditor]>([
    [
      "resources",
      { type: "object", properties: { requests: { type: "object" } } },
      {
        kind: "yaml",
        schema: {
          type: "object",
          properties: { requests: { type: "object" } },
        },
      },
    ],
    ["storageSize", { type: "string" }, { kind: "text" }],
    [
      "extraEnv",
      { type: "array", items: { type: "object" } },
      { kind: "yaml", schema: { type: "array", items: { type: "object" } } },
    ],
    [
      "tolerations",
      { type: "array" },
      { kind: "yaml", schema: { type: "array" } },
    ],
    [
      "podLabels",
      { type: "object", additionalProperties: { type: "string" } },
      { kind: "keyValue" },
    ],
  ])(
    "gives the schema of a %s cluster field %o the editor of the schema",
    (_, schema, editor) => {
      expect(parameterEditorOf(schema)).toEqual(editor);
    },
  );

  it("checks YAML of an object against the whole registration schema", () => {
    const schema = {
      type: "object",
      properties: { instances: { type: "integer", minimum: 1 } },
    };
    expect(parameterEditorOf(schema)).toEqual({ kind: "yaml", schema });
  });

  it("checks YAML of a list against the whole registration schema", () => {
    const schema = {
      type: "array",
      maxItems: 10,
      items: { type: "string", pattern: "^arn:" },
    };
    expect(parameterEditorOf(schema)).toEqual({ kind: "yaml", schema });
  });

  it("edits a parameter whose schema has no type as YAML", () => {
    expect(parameterEditorOf({})).toEqual({ kind: "yaml", schema: {} });
  });

  it("gives no editor to a parameter without a schema", () => {
    expect(parameterEditorOf(undefined)).toBeUndefined();
  });

  it("picks from the choices whatever the schema", () => {
    expect(
      parameterEditorOf({ type: "array" }, { choices: ["sqs", "s3"] }),
    ).toEqual({ kind: "choices", options: ["sqs", "s3"] });
    expect(parameterEditorOf(undefined, { choices: ["small"] })).toEqual({
      kind: "select",
      options: ["small"],
    });
  });

  it("edits a map of strings with a value that is not a string as YAML", () => {
    const schema = { type: "object", additionalProperties: { type: "string" } };
    expect(parameterEditorOf(schema, { value: { team: 1 } })).toEqual({
      kind: "yaml",
      schema,
    });
  });

  it("edits an int-or-string with a value that is a number as YAML", () => {
    const schema = { "x-kubernetes-int-or-string": true };
    expect(parameterEditorOf(schema, { value: 1024 })).toEqual({
      kind: "yaml",
      schema,
    });
  });

  it("keeps the text input of an int-or-string for a string", () => {
    expect(
      parameterEditorOf(
        { "x-kubernetes-int-or-string": true },
        { value: "50Gi" },
      ),
    ).toEqual({ kind: "text" });
  });

  it.each([{ team: "orders" }, null])(
    "keeps the rows of a map of strings for the value %o",
    (value) => {
      expect(
        parameterEditorOf(
          { type: "object", additionalProperties: { type: "string" } },
          { value },
        ),
      ).toEqual({ kind: "keyValue" });
    },
  );
});

describe("overrideEditorOf", () => {
  it.each<[string, SettingEditor]>([
    ["replicas", { kind: "number", integer: true, minimum: 1 }],
    ["storageSize", { kind: "storageSize" }],
    ["resources", { kind: "resources" }],
    [
      "scheduling",
      { kind: "yaml", schema: OVERRIDES.scheduling as OpenAPISchema },
    ],
    ["podLabels", { kind: "keyValue", format: "label" }],
  ])("reads the registration schema of the override %s", (name, editor) => {
    expect(overrideEditorOf(name, OVERRIDES[name])).toEqual(editor);
  });

  it("checks YAML against the registration schema also for a name of fullSchemaOf", () => {
    expect(overrideEditorOf("extraEnvFrom", { type: "array" })).toEqual({
      kind: "yaml",
      schema: { type: "array" },
    });
  });

  it.each(["constructor", "toString"])(
    "reads the schema of an override named %s",
    (name) => {
      expect(overrideEditorOf(name, { type: "string" })).toEqual({
        kind: "text",
      });
    },
  );

  it.each<[string, unknown, SettingEditor]>([
    ["extraEnv", { type: "array", items: { type: "object" } }, { kind: "env" }],
    ["extraEnv", undefined, { kind: "env" }],
    ["tolerations", { type: "array" }, { kind: "tolerations" }],
    [
      "resources",
      { type: "object", properties: { requests: { type: "object" } } },
      { kind: "resources" },
    ],
    [
      "storageSize",
      { "x-kubernetes-int-or-string": true },
      { kind: "storageSize" },
    ],
  ])(
    "gives %s the editor of its name for the schema %o",
    (name, schema, editor) => {
      expect(overrideEditorOf(name, schema)).toEqual(editor);
    },
  );

  it.each<[string, unknown, SettingEditor]>([
    [
      "extraEnv",
      { type: "object", additionalProperties: { type: "string" } },
      { kind: "keyValue" },
    ],
    [
      "extraEnv",
      { type: "array", items: { type: "string" } },
      { kind: "yaml", schema: { type: "array", items: { type: "string" } } },
    ],
    ["tolerations", { type: "string" }, { kind: "text" }],
    [
      "resources",
      { type: "object", additionalProperties: { type: "string" } },
      { kind: "keyValue" },
    ],
    [
      "resources",
      { type: "object", properties: { requests: { type: "string" } } },
      {
        kind: "yaml",
        schema: {
          type: "object",
          properties: { requests: { type: "string" } },
        },
      },
    ],
    ["storageSize", { type: "integer" }, { kind: "number", integer: true }],
  ])(
    "gives %s the editor of the schema %o, which the editor of the name cannot write",
    (name, schema, editor) => {
      expect(overrideEditorOf(name, schema)).toEqual(editor);
    },
  );

  it.each<[string, unknown, unknown]>([
    ["extraEnv", { type: "array" }, { TZ: "UTC" }],
    ["extraEnv", { type: "array" }, ["TZ=UTC"]],
    ["extraEnv", undefined, [{ name: "TZ", value: 1 }]],
    ["extraEnv", undefined, [{ name: "TZ", valueFrom: "secret" }]],
    ["extraEnv", undefined, [{ name: "TZ", value: "UTC", extra: true }]],
    ["tolerations", undefined, "all"],
    ["tolerations", { type: "array" }, [null]],
    ["tolerations", { type: "array" }, [{ tolerationSeconds: "60" }]],
    ["tolerations", { type: "array" }, [{ key: "a", extra: true }]],
    ["resources", {}, "big"],
    ["resources", { type: "object" }, { requests: "1" }],
    ["resources", { type: "object" }, { limits: { cpu: true } }],
    ["storageSize", undefined, { size: "1Gi" }],
    [
      "podLabels",
      { type: "object", additionalProperties: { type: "string" } },
      { team: 1 },
    ],
  ])(
    "edits a %s override with the schema %o and the value %o, which its editor cannot keep, as YAML",
    (name, schema, value) => {
      expect(overrideEditorOf(name, schema, { value })).toEqual({
        kind: "yaml",
        ...(schema !== undefined && { schema }),
      });
    },
  );

  it.each<[string, unknown, unknown, SettingEditor["kind"]]>([
    [
      "extraEnv",
      undefined,
      [
        { name: "A", value: "b" },
        { name: "B", valueFrom: { secretKeyRef: { name: "s", key: "k" } } },
      ],
      "env",
    ],
    [
      "tolerations",
      undefined,
      [{ key: "a", effect: "NoExecute", tolerationSeconds: 60 }],
      "tolerations",
    ],
    [
      "resources",
      undefined,
      { requests: { cpu: 1, memory: "1Gi" } },
      "resources",
    ],
    ["storageSize", undefined, "50Gi", "storageSize"],
    ["storageSize", undefined, 1024, "storageSize"],
    [
      "podLabels",
      { type: "object", additionalProperties: { type: "string" } },
      null,
      "keyValue",
    ],
  ])(
    "keeps the editor of a %s override with the schema %o for the value %o",
    (name, schema, value, kind) => {
      expect(overrideEditorOf(name, schema, { value })?.kind).toBe(kind);
    },
  );
});

describe("fieldSchemaFromOpenAPI", () => {
  it("keeps the type, the string enum, and the range", () => {
    expect(
      fieldSchemaFromOpenAPI({
        type: "string",
        enum: ["small", "large"],
        description: "x",
      }),
    ).toEqual({ type: "string", enum: ["small", "large"] });
    expect(
      fieldSchemaFromOpenAPI({ type: "number", minimum: 0.5, maximum: 2 }),
    ).toEqual({ type: "number", minimum: 0.5, maximum: 2 });
  });

  it("marks an int-or-string quantity", () => {
    expect(
      fieldSchemaFromOpenAPI({ "x-kubernetes-int-or-string": true }),
    ).toEqual({ intOrString: true });
  });

  it("gives an object with properties no type", () => {
    expect(
      fieldSchemaFromOpenAPI({
        type: "object",
        properties: { a: { type: "string" } },
      }),
    ).toEqual({});
  });

  it("gives a map of strings with declared strings the object type", () => {
    expect(
      fieldSchemaFromOpenAPI({
        type: "object",
        properties: { team: { type: "string" } },
        patternProperties: { "^x-": {} },
        additionalProperties: { type: "string" },
      }),
    ).toEqual({ type: "object" });
  });

  it.each([
    ["properties", { properties: { retries: { type: "integer" } } }],
    ["patternProperties", { patternProperties: { "^n-": { type: "number" } } }],
  ])(
    "gives a map of strings with declared %s that are not strings no type",
    (_, declared) => {
      expect(
        fieldSchemaFromOpenAPI({
          type: "object",
          ...declared,
          additionalProperties: { type: "string" },
        }),
      ).toEqual({});
    },
  );

  it("gives undefined for a value that is not a schema", () => {
    expect(fieldSchemaFromOpenAPI(undefined)).toBeUndefined();
    expect(fieldSchemaFromOpenAPI("string")).toBeUndefined();
  });
});

describe("the claim value of each editor", () => {
  const secretEnv = {
    name: "DB_PASSWORD",
    valueFrom: { secretKeyRef: { name: "orders-db", key: "password" } },
  };
  it.each<[string, SettingEditor, unknown, unknown]>([
    [
      "text",
      { kind: "text" },
      "https://orders.example.com",
      "https://orders.example.com",
    ],
    ["number", { kind: "number", integer: true }, 3, 3],
    ["toggle", { kind: "toggle" }, false, false],
    [
      "select",
      { kind: "select", options: ["Standalone", "Embedded"] },
      "Embedded",
      "Embedded",
    ],
    [
      "resources",
      { kind: "resources" },
      { requests: { cpu: "500m" }, limits: {} },
      { requests: { cpu: "500m" } },
    ],
    ["storageSize", { kind: "storageSize" }, "64Gi", "64Gi"],
    [
      "env",
      { kind: "env" },
      [{ name: "TZ", value: "UTC" }, secretEnv],
      [{ name: "TZ", value: "UTC" }, secretEnv],
    ],
    ["keyValue", { kind: "keyValue" }, { team: "orders" }, { team: "orders" }],
    ["empty keyValue", { kind: "keyValue" }, {}, {}],
    [
      "tolerations",
      { kind: "tolerations" },
      [{ key: "dedicated", operator: "Exists" }],
      [{ key: "dedicated", operator: "Exists" }],
    ],
    [
      "yaml",
      { kind: "yaml" },
      [{ configMapRef: { name: "env" } }],
      [{ configMapRef: { name: "env" } }],
    ],
    [
      "secretRef",
      { kind: "secretRef" },
      { name: "orders-oidc", key: "secret" },
      { name: "orders-oidc", key: "secret" },
    ],
  ])(
    "%s turns its input into the claim value and back",
    (_, editor, input, claim) => {
      expect(claimValueOf(editor, input)).toEqual(claim);
      if (editor.kind !== "resources")
        expect(editorValueOf(editor, claim)).toEqual(input);
    },
  );

  it.each<[string, SettingEditor, unknown]>([
    ["text", { kind: "text" }, ""],
    ["select", { kind: "select", options: ["a"] }, ""],
    ["storageSize", { kind: "storageSize" }, ""],
    ["number", { kind: "number", integer: true }, Number.NaN],
    ["number", { kind: "number", integer: true }, undefined],
    ["env", { kind: "env" }, []],
    ["tolerations", { kind: "tolerations" }, []],
    ["yaml list", { kind: "yaml" }, []],
    ["yaml object", { kind: "yaml" }, {}],
    ["yaml", { kind: "yaml" }, undefined],
    ["keyValue", { kind: "keyValue" }, undefined],
    ["resources", { kind: "resources" }, { requests: {}, limits: {} }],
    ["secretRef", { kind: "secretRef" }, undefined],
  ])("an empty %s is a reset", (_, editor, input) => {
    expect(claimValueOf(editor, input)).toBeUndefined();
  });

  it.each<[SettingEditor, unknown]>([
    [{ kind: "text" }, ""],
    [{ kind: "select", options: ["a"] }, ""],
    [{ kind: "storageSize" }, ""],
    [{ kind: "number", integer: true }, undefined],
    [{ kind: "toggle" }, false],
    [{ kind: "env" }, []],
    [{ kind: "tolerations" }, []],
    [{ kind: "keyValue" }, undefined],
    [{ kind: "resources" }, {}],
    [{ kind: "yaml" }, undefined],
    [{ kind: "secretRef" }, undefined],
  ])("an unset %o shows as its empty value", (editor, empty) => {
    expect(editorValueOf(editor, undefined)).toEqual(empty);
    if (editor.kind !== "yaml")
      expect(editorValueOf(editor, null)).toEqual(empty);
  });

  it("YAML shows null as it is", () => {
    expect(editorValueOf({ kind: "yaml" }, null)).toBeNull();
  });
});
