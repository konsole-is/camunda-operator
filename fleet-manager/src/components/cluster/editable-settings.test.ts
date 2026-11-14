import { describe, expect, it } from "vitest";
import {
  accessSection,
  type SettingsPage,
  settingsFieldsOf,
  settingsGroupOf,
  steps,
} from "../../../test/fakes/settings";
import { type Setting, settingChange, settingsOf } from "./editable-settings";
import { componentSettings } from "./settings";
import type { Process } from "./topology";

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

  it("a value that the editor of a field cannot keep opens as YAML", () => {
    const toleration = { key: "spot", operator: "Exists", extra: true };
    const tolerations = setting(
      processSettings("zeebe", "zeebe", {
        preset: {
          zeebe: {
            scheduling: {
              tolerations: [toleration],
            },
          },
        },
      }),
      "scheduling.tolerations",
    );
    expect(tolerations.editor).toEqual({
      kind: "yaml",
      schema: { type: "array", items: { type: "object" } },
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

  it("a change inside a block keeps the pending changes of the block", () => {
    const pending = {
      managementAuthConfigRef: "keycloak",
      webapp: { replicas: 2 },
    };
    const { fields } = settingsFieldsOf({
      claim: { attachments: { optimize: pending } },
      optimize: {
        clusterRef: { name: "orders" },
        managementAuthRef: "keycloak",
        webapp: {},
      },
    });
    const [group] = componentSettings(fields, "optimize", {});
    const settings = settingsOf({ group, spec: pending });
    const resources = { requests: { cpu: "1" } };
    expect(
      settingChange(setting(settings, "webapp.resources"), resources),
    ).toEqual({
      steps: steps("spec", "attachments", "optimize", "webapp"),
      value: { replicas: 2, resources },
    });
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
  const labels = () =>
    setting(
      processSettings("zeebe", "zeebe", {
        claim: { cluster: { zeebe: { podLabels: { team: "a" } } } },
      }),
      "podLabels",
    );

  it("no value resets the field", () => {
    expect(settingChange(labels(), undefined)).toEqual({
      steps: steps("spec", "cluster", "zeebe", "podLabels"),
      value: undefined,
    });
  });

  it("an empty map writes {}", () => {
    expect(settingChange(labels(), {})).toEqual({
      steps: steps("spec", "cluster", "zeebe", "podLabels"),
      value: {},
    });
  });
});
