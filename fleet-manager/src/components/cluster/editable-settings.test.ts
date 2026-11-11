import { describe, expect, it } from "vitest";
import { fullSchemas } from "@/generated/fieldschema";
import { fieldSchemaOf } from "@/lib/fields";
import {
  accessSection,
  type SettingsPage,
  settingsFieldsOf,
  settingsGroupOf,
  steps,
} from "../../../test/fakes/settings";
import {
  claimValueOf,
  editorOf,
  editorValueOf,
  fieldSchemaFromOpenAPI,
  type Setting,
  type SettingEditor,
  settingChange,
  settingsOf,
} from "./editable-settings";
import { componentSettings } from "./settings";
import type { Process } from "./topology";

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

  it.each<[string, SettingEditor]>([
    ["replicas", { kind: "number", integer: true, minimum: 1 }],
    ["storageSize", { kind: "storageSize" }],
    ["resources", { kind: "resources" }],
    ["scheduling", { kind: "yaml" }],
    ["podLabels", { kind: "keyValue", format: "label" }],
  ])("reads the registration schema of the override %s", (name, editor) => {
    expect(editorOf(name, fieldSchemaFromOpenAPI(OVERRIDES[name]))).toEqual(
      editor,
    );
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

function processSettings(host: Process, process: Process, page: SettingsPage) {
  return settingsGroupOf(host, process, page).settings;
}

function clusterSettings(page: SettingsPage) {
  return settingsGroupOf("cluster", "cluster", page).settings;
}

function setting(settings: Setting[], label: string): Setting {
  const found = settings.find((s) => s.label === label);
  expect(found, `a setting ${label}`).toBeDefined();
  return found!;
}

describe("settingsOf", () => {
  it("a component offers a field that no layer sets", () => {
    const replicas = setting(
      processSettings("tasklist", "tasklist", {}),
      "replicas",
    );
    expect(replicas.editor).toEqual({
      kind: "number",
      integer: true,
      minimum: 0,
    });
    expect(replicas.value).toBeUndefined();
    expect(replicas.state).toBeUndefined();
    expect(replicas.claimSet).toBe(false);
    expect(replicas.steps).toEqual(
      steps("spec", "cluster", "tasklist", "replicas"),
    );
  });

  it("a field that a layer sets takes its state", () => {
    const preset = setting(
      processSettings("zeebe", "zeebe", { preset: { zeebe: { replicas: 3 } } }),
      "replicas",
    );
    expect(preset).toMatchObject({
      value: 3,
      state: { kind: "preset" },
      claimSet: false,
    });
    const claimed = setting(
      processSettings("zeebe", "zeebe", {
        preset: { zeebe: { replicas: 3 } },
        claim: { cluster: { zeebe: { replicas: 5 } } },
      }),
      "replicas",
    );
    expect(claimed).toMatchObject({
      value: 5,
      state: { kind: "claim" },
      claimSet: true,
    });
    expect(claimed.resetLabel).toBe("Reset to preset");
  });

  it("the leaves of one editor share one setting", () => {
    const settings = processSettings("zeebe", "zeebe", {
      claim: {
        cluster: { zeebe: { resources: { requests: { cpu: "500m" } } } },
      },
    });
    expect(setting(settings, "resources")).toMatchObject({
      value: { requests: { cpu: "500m" } },
      claimSet: true,
      editor: { kind: "resources" },
    });
    expect(settings.some((s) => s.label.startsWith("resources."))).toBe(false);
  });

  it("settingChange writes the whole block that a layer sets", () => {
    const nodeAffinity = {
      requiredDuringSchedulingIgnoredDuringExecution: {
        nodeSelectorTerms: [
          {
            matchExpressions: [
              { key: "pool", operator: "In", values: ["camunda"] },
            ],
          },
        ],
      },
    };
    const t1 = { key: "dedicated", operator: "Exists" };
    const t2 = { key: "spot", operator: "Exists", effect: "NoSchedule" };
    const tolerations = setting(
      processSettings("zeebe", "zeebe", {
        preset: { zeebe: { scheduling: { nodeAffinity, tolerations: [t1] } } },
      }),
      "scheduling.tolerations",
    );
    expect(tolerations.state).toEqual({ kind: "preset" });
    expect(settingChange(tolerations, [t2])).toEqual({
      steps: steps("spec", "cluster", "zeebe", "scheduling"),
      value: { nodeAffinity, tolerations: [t2] },
    });
    expect(settingChange(tolerations, [])).toEqual({
      steps: steps("spec", "cluster", "zeebe", "scheduling"),
      value: { nodeAffinity },
    });
  });

  it("a list that each layer appends to edits the list of the claim", () => {
    const a = { configMapRef: { name: "preset-env" } };
    const b = { secretRef: { name: "claim-env" } };
    const c = { configMapRef: { name: "more-env" } };
    const envFrom = setting(
      processSettings("zeebe", "zeebe", {
        preset: { zeebe: { extraEnvFrom: [a] } },
        claim: { cluster: { zeebe: { extraEnvFrom: [b] } } },
      }),
      "extraEnvFrom",
    );
    expect(envFrom).toMatchObject({ value: [b], inherited: 1 });
    expect(settingChange(envFrom, [b, c])).toEqual({
      steps: steps("spec", "cluster", "zeebe", "extraEnvFrom"),
      value: [b, c],
    });
  });

  it("a filled field is locked", () => {
    const url = setting(
      clusterSettings({
        claim: {
          plugins: [{ section: "access", registration: "gateway-access" }],
        },
        sections: [accessSection()],
      }),
      "externalUrl",
    );
    expect(url.lock).toEqual({
      reason: "filled",
      section: "access",
      sectionLabel: "Access",
    });
    expect(url.editor).toBeUndefined();
    expect(url.steps).toBeUndefined();
  });

  it("a Secret reference is one setting with a Secret picker", () => {
    const settings = clusterSettings({
      claim: {
        cluster: {
          auth: {
            clientId: "orders",
            clientSecretRef: { name: "orders-oidc", key: "secret" },
          },
        },
      },
    });
    expect(setting(settings, "auth.clientSecretRef")).toMatchObject({
      editor: { kind: "secretRef" },
      value: { name: "orders-oidc", key: "secret" },
      steps: steps("spec", "cluster", "auth", "clientSecretRef"),
      claimSet: true,
    });
    expect(
      settings.some((s) => s.label.startsWith("auth.clientSecretRef.")),
    ).toBe(false);
  });

  it("Optimize settings use the claim names", () => {
    const attachment = {
      managementAuthConfigRef: "keycloak",
      importer: { replicas: 1 },
    };
    const { fields } = settingsFieldsOf({
      claim: { attachments: { optimize: attachment } },
      optimize: {
        clusterRef: { name: "orders" },
        managementAuthRef: "keycloak",
        importer: { replicas: 1 },
      },
    });
    const [group] = componentSettings(fields, "optimize", {});
    const settings = settingsOf({ group, spec: attachment });
    expect(setting(settings, "managementAuthConfigRef")).toMatchObject({
      state: { kind: "claim" },
      value: "keycloak",
    });
    expect(settingChange(setting(settings, "importer.replicas"), 0)).toEqual({
      steps: steps("spec", "attachments", "optimize", "importer", "replicas"),
      value: 0,
    });
    const clusterRef = setting(settings, "clusterRef.name");
    expect(clusterRef.lock).toEqual({ reason: "claimController" });
    expect(clusterRef.editor).toBeUndefined();
  });

  it("a field outside the schema stays read only", () => {
    const version = setting(
      processSettings("connectors", "connectors", {
        cluster: { connectors: { version: "8.9.1" } },
      }),
      "version",
    );
    expect(version.lock).toEqual({ reason: "release" });
    expect(version.editor).toBeUndefined();
    expect(version.value).toBe("8.9.1");
  });

  it("an embedded app writes to its own block", () => {
    const replicas = setting(
      processSettings("gateway", "tasklist", {
        cluster: { gateway: { mode: "Standalone" } },
      }),
      "replicas",
    );
    expect(replicas.steps).toEqual(
      steps("spec", "cluster", "tasklist", "replicas"),
    );
  });
});

describe("settingChange", () => {
  it("an empty value resets the field", () => {
    const labels = setting(
      processSettings("zeebe", "zeebe", {
        claim: { cluster: { zeebe: { podLabels: { team: "a" } } } },
      }),
      "podLabels",
    );
    expect(settingChange(labels, {})).toEqual({
      steps: steps("spec", "cluster", "zeebe", "podLabels"),
      value: undefined,
    });
  });
});
