import { describe, expect, it } from "vitest";
import {
  accessSection,
  type SettingsPage,
  settingsFieldsOf,
  settingsGroupOf,
  steps,
} from "../../../test/fakes/settings";
import type { FieldState } from "@/lib/fields";
import {
  type Setting,
  settingChange,
  settingsOf,
  settingsSummary,
} from "./editable-settings";
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

describe("settingsOf merged errors", () => {
  // brokers returns the resources setting of the brokers when preset is
  // spec.cluster.zeebe.resources of the preset and claim that of the claim.
  function brokers(preset?: object, claim?: object): Setting {
    return setting(
      processSettings("zeebe", "zeebe", {
        ...(preset && { preset: { zeebe: { resources: preset } } }),
        ...(claim && { claim: { cluster: { zeebe: { resources: claim } } } }),
      }),
      "resources",
    );
  }

  const messages = (s: Setting) => s.mergedErrors?.map((e) => e.message);

  it("names the preset when its limit stays below the request of the claim", () => {
    expect(
      messages(brokers({ limits: { cpu: "1" } }, { requests: { cpu: "2" } })),
    ).toEqual([
      "The CPU limit of 1 comes from the preset and is below the CPU request of 2.",
    ]);
  });

  it("names the preset when its request stays above the limit of the claim", () => {
    expect(
      messages(brokers({ requests: { cpu: "2" } }, { limits: { cpu: "1" } })),
    ).toEqual([
      "The CPU request of 2 comes from the preset and is above the CPU limit of 1.",
    ]);
  });

  it("names the preset when it sets both and the claim changes nothing", () => {
    expect(
      messages(
        brokers({
          requests: { cpu: "2" },
          limits: { cpu: "1" },
        }),
      ),
    ).toEqual([
      "The CPU limit of 1 and the CPU request of 2 come from the preset. The limit is below the request.",
    ]);
  });

  it("names no source when the claim sets the limit and the request", () => {
    const setting = brokers(
      { requests: { cpu: "4" }, limits: { cpu: "8" } },
      { requests: { cpu: "2" }, limits: { cpu: "1" } },
    );
    expect(setting.mergedErrors).toEqual([
      {
        message: "The CPU limit of 1 is below the CPU request of 2.",
        belowRequest: { resource: "cpu", request: "2", limit: "1" },
        blocks: true,
      },
    ]);
  });

  it("names any resource that both the requests and the limits hold, and how to correct one without a field", () => {
    expect(
      messages(
        brokers(
          { limits: { "ephemeral-storage": "1Gi" } },
          { requests: { "ephemeral-storage": "2Gi" } },
        ),
      ),
    ).toEqual([
      "The ephemeral-storage limit of 1Gi comes from the preset and is below the ephemeral-storage request of 2Gi. Pick another preset, or ask your admin to correct the preset.",
    ]);
  });

  it("does not block on the resources of an embedded app", () => {
    const tasklist = setting(
      processSettings("gateway", "tasklist", {
        cluster: { gateway: { mode: "Standalone" } },
        preset: {
          tasklist: {
            resources: { requests: { cpu: "2" }, limits: { cpu: "1" } },
          },
        },
      }),
      "resources",
    );
    expect(tasklist.mergedErrors).toMatchObject([{ blocks: false }]);
    const zeebe = brokers({ requests: { cpu: "2" }, limits: { cpu: "1" } });
    expect(zeebe.mergedErrors).toMatchObject([{ blocks: true }]);
  });

  it("says that a value that no manager owns is already set on the cluster", () => {
    const resources = setting(
      processSettings("zeebe", "zeebe", {
        cluster: {
          zeebe: {
            resources: { requests: { cpu: "2" }, limits: { cpu: "1" } },
          },
        },
        claim: {
          cluster: { zeebe: { resources: { requests: { cpu: "2" } } } },
        },
      }),
      "resources",
    );
    expect(messages(resources)).toEqual([
      "The CPU limit of 1 is already set on the cluster and is below the CPU request of 2.",
    ]);
  });

  it("has no error when the merged limit is not below the request", () => {
    expect(
      brokers({ limits: { cpu: "4" } }, { requests: { cpu: "2" } })
        .mergedErrors,
    ).toBeUndefined();
    expect(brokers().mergedErrors).toBeUndefined();
  });

  it("compares the memory and quantities in other units", () => {
    expect(
      messages(
        brokers(
          { requests: { memory: "2Gi" }, limits: { cpu: "500m" } },
          { requests: { cpu: "1" }, limits: { memory: "1024Mi" } },
        ),
      ),
    ).toEqual([
      "The CPU limit of 500m comes from the preset and is below the CPU request of 1.",
      "The memory request of 2Gi comes from the preset and is above the memory limit of 1024Mi.",
    ]);
  });

  it("checks the resources of every component and of the dump", () => {
    const gateway = setting(
      processSettings("gateway", "gateway", {
        preset: { gateway: { resources: { limits: { memory: "1Gi" } } } },
        claim: {
          cluster: { gateway: { resources: { requests: { memory: "2Gi" } } } },
        },
      }),
      "resources",
    );
    expect(gateway.mergedErrors).toHaveLength(1);
    const dump = setting(
      clusterSettings({
        preset: {
          backup: { dump: { resources: { limits: { cpu: "1" } } } },
        },
        claim: {
          cluster: {
            backup: { dump: { resources: { requests: { cpu: "2" } } } },
          },
        },
      }),
      "backup.dump.resources",
    );
    expect(messages(dump)).toEqual([
      "The CPU limit of 1 comes from the preset and is below the CPU request of 2.",
    ]);
  });

  it("asks to correct a fault of an embedded app before it runs standalone", () => {
    const tasklist = setting(
      processSettings("gateway", "tasklist", {
        cluster: { gateway: { mode: "Standalone" } },
        preset: {
          tasklist: {
            resources: { requests: { cpu: "2" }, limits: { cpu: "1" } },
          },
        },
      }),
      "resources",
    );
    expect(messages(tasklist)).toEqual([
      "The CPU limit of 1 and the CPU request of 2 come from the preset. The limit is below the request. Correct it before Tasklist runs standalone.",
    ]);
  });

  it("gives the value of the claim to settingChange, which writes only the changed keys", () => {
    const resources = brokers(
      { requests: { cpu: "2", memory: "2Gi" }, limits: { cpu: "1" } },
      { limits: { memory: "4Gi" } },
    );
    expect(resources.claimed).toEqual({ limits: { memory: "4Gi" } });
    expect(
      settingChange(resources, {
        requests: { cpu: "2", memory: "2Gi" },
        limits: { cpu: "3", memory: "4Gi" },
      }).value,
    ).toEqual({ limits: { cpu: "3", memory: "4Gi" } });
    expect(
      settingChange(resources, {
        requests: { cpu: "2", memory: "2Gi" },
        limits: { cpu: "1" },
      }).value,
    ).toBeUndefined();
  });

  it("has no merged errors without the claim, as on the preset page", () => {
    const { group, settings } = settingsGroupOf("zeebe", "zeebe", {
      preset: {
        zeebe: { resources: { requests: { cpu: "2" }, limits: { cpu: "1" } } },
      },
    });
    expect(setting(settings, "resources").mergedErrors).toHaveLength(1);
    const { merged } = settingsFieldsOf({
      preset: {
        zeebe: { resources: { requests: { cpu: "2" }, limits: { cpu: "1" } } },
      },
    });
    expect(
      setting(settingsOf({ group, spec: merged.spec }), "resources")
        .mergedErrors,
    ).toBeUndefined();
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

describe("settingsSummary", () => {
  const of = (...states: (FieldState | undefined)[]): Setting[] =>
    states.map((state, i) => ({
      key: `k${i}`,
      label: `s${i}`,
      value: undefined,
      ...(state && { state }),
      claimSet: state?.kind === "claim",
      resetLabel: "Reset to preset",
    }));

  it("counts the settings by where their values come from, the changes last", () => {
    expect(
      settingsSummary(
        of(
          { kind: "claim" },
          { kind: "preset" },
          { kind: "default" },
          { kind: "preset" },
          { kind: "release" },
          { kind: "environment" },
          { kind: "admin" },
          undefined,
          { kind: "default" },
        ),
      ),
    ).toBe(
      "9 settings: 2 from the preset, 1 from the release, 1 from the environment, 1 set by your admin, 2 default values, 1 not set, 1 changed in the claim",
    );
  });

  it("leaves out a source that no setting has", () => {
    expect(settingsSummary(of({ kind: "preset" }, { kind: "preset" }))).toBe(
      "2 settings: 2 from the preset",
    );
  });

  it("uses the singular for one", () => {
    expect(settingsSummary(of({ kind: "default" }))).toBe(
      "1 setting: 1 default value",
    );
  });

  it("names the section of each filled setting and each writer", () => {
    expect(
      settingsSummary(
        of(
          { kind: "filled", section: "access", sectionLabel: "Access" },
          { kind: "filled", section: "identity", sectionLabel: "Identity" },
          { kind: "filled", section: "access", sectionLabel: "Access" },
          { kind: "setBy", manager: "kubectl", takeOver: true },
          { kind: "setBy", takeOver: false },
        ),
      ),
    ).toBe(
      "5 settings: 2 filled by Access, 1 filled by Identity, 1 set by kubectl, 1 set by an unknown writer",
    );
  });

  it("says that a group without settings has none", () => {
    expect(settingsSummary([])).toBe("No settings");
  });
});
