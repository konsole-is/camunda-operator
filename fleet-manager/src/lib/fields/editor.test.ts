import { describe, expect, it } from "vitest";
import { type FieldSchema, fullSchemas } from "@/generated/fieldschema";
import {
  claimValueOf,
  editorOf,
  editorValueOf,
  fieldSchemaFromOpenAPI,
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
  it.each<[string, SettingEditor]>([
    ["replicas", { kind: "number", integer: true, minimum: 1 }],
    ["storageSize", { kind: "storageSize" }],
    ["resources", { kind: "resources" }],
    ["scheduling", { kind: "yaml", schema: OVERRIDES.scheduling as OpenAPISchema }],
    ["podLabels", { kind: "keyValue", format: "label" }],
  ])("reads the registration schema of the override %s", (name, editor) => {
    expect(parameterEditorOf(name, OVERRIDES[name])).toEqual(editor);
  });

  it("gives a map of strings key and value rows", () => {
    expect(
      parameterEditorOf("tags", {
        type: "object",
        additionalProperties: { type: "string" },
      }),
    ).toEqual({ kind: "keyValue" });
  });

  it("checks YAML of an object against the whole registration schema", () => {
    const schema = {
      type: "object",
      properties: { instances: { type: "integer", minimum: 1 } },
    };
    expect(parameterEditorOf("overrides", schema)).toEqual({
      kind: "yaml",
      schema,
    });
  });

  it("checks YAML of a list against the whole registration schema", () => {
    const schema = {
      type: "array",
      maxItems: 10,
      items: { type: "string", pattern: "^arn:" },
    };
    expect(parameterEditorOf("managedPolicyArns", schema)).toEqual({
      kind: "yaml",
      schema,
    });
  });

  it("edits a parameter whose schema has no type as YAML", () => {
    expect(parameterEditorOf("free", {})).toEqual({ kind: "yaml", schema: {} });
  });

  it("keeps the CRD schema of a field that the YAML editor knows by name", () => {
    expect(parameterEditorOf("extraEnvFrom", { type: "array" })).toEqual({
      kind: "yaml",
      schema: fullSchemas.extraEnvFrom,
    });
  });

  it("gives no editor to a parameter without a schema", () => {
    expect(parameterEditorOf("free", undefined)).toBeUndefined();
  });

  it("picks from the choices whatever the schema", () => {
    expect(
      parameterEditorOf("policies", { type: "array" }, ["sqs", "s3"]),
    ).toEqual({ kind: "choices", options: ["sqs", "s3"] });
    expect(parameterEditorOf("preset", undefined, ["small"])).toEqual({
      kind: "select",
      options: ["small"],
    });
  });
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
    ["keyValue", { kind: "keyValue" }, {}],
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
    [{ kind: "keyValue" }, {}],
    [{ kind: "resources" }, {}],
    [{ kind: "yaml" }, undefined],
    [{ kind: "secretRef" }, undefined],
  ])("an unset %o shows as its empty value", (editor, empty) => {
    expect(editorValueOf(editor, undefined)).toEqual(empty);
  });
});
