import { describe, expect, it } from "vitest";
import {
  accessSection,
  type SettingsPage,
  settingsFieldsOf,
  settingsGroupOf,
  steps,
} from "../../../test/fakes/settings";
import type { FieldState, Step } from "@/lib/fields";
import {
  blockingMergedErrors,
  resourceWrites,
  type Setting,
  settingChange,
  settingsOf,
  settingsSummary,
} from "./editable-settings";
import { componentSettings, type SettingsGroup } from "./settings";
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

  it.each([
    [false, true],
    [true, undefined],
  ])(
    "leaves the rows that the editor opens with unchecked in the labels of the ServiceMonitor of Optimize only while it is off (enabled: %s)",
    (enabled, unchecked) => {
      const attachment = {
        managementAuthConfigRef: "keycloak",
        monitoring: { serviceMonitor: { enabled, labels: { "bad key": "x" } } },
      };
      const { fields } = settingsFieldsOf({
        claim: { attachments: { optimize: attachment } },
        optimize: { clusterRef: { name: "orders" }, ...attachment },
      });
      const [group] = componentSettings(fields, "optimize", {});
      const labels = setting(
        settingsOf({ group, spec: attachment }),
        "monitoring.serviceMonitor.labels",
      );
      expect(labels.openedUnchecked).toBe(unchecked);
      expect(labels.mergedErrors).toBeUndefined();
    },
  );

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
        blocks: true,
      },
    ]);
  });

  it.each([
    [
      "both values come from the preset",
      {
        requests: { "ephemeral-storage": "2Gi" },
        limits: { "ephemeral-storage": "1Gi" },
      },
      undefined,
      "The ephemeral-storage limit of 1Gi and the ephemeral-storage request of 2Gi come from the preset. The limit is below the request. Pick another preset, or ask your admin to correct the preset.",
    ],
    [
      "the claim sets one value",
      { limits: { "ephemeral-storage": "1Gi" } },
      { requests: { "ephemeral-storage": "2Gi" } },
      "The ephemeral-storage limit of 1Gi comes from the preset and is below the ephemeral-storage request of 2Gi. Select Reset to preset to remove this value from the claim. Pick another preset, or ask your admin to correct the preset.",
    ],
    [
      "the claim holds both values, as after an edit of another amount",
      {
        requests: { "ephemeral-storage": "2Gi" },
        limits: { "ephemeral-storage": "1Gi" },
      },
      {
        requests: { "ephemeral-storage": "2Gi" },
        limits: { "ephemeral-storage": "1Gi" },
      },
      "The ephemeral-storage limit of 1Gi is below the ephemeral-storage request of 2Gi. Select Reset to preset to remove these values from the claim.",
    ],
  ])(
    "says how to correct a resource without a field when %s",
    (_, preset, claim, message) => {
      expect(messages(brokers(preset, claim))).toEqual([message]);
    },
  );

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

  const FAULT = { requests: { cpu: "2" }, limits: { cpu: "1" } };
  const BOTH =
    "The CPU limit of 1 and the CPU request of 2 come from the preset. The limit is below the request.";

  it.each([
    ["brokers", "zeebe", "zeebe", {}, true, ""],
    [
      "a standalone gateway",
      "gateway",
      "gateway",
      { gateway: { mode: "Standalone" } },
      true,
      "",
    ],
    [
      "a gateway embedded in the brokers",
      "zeebe",
      "gateway",
      { gateway: { mode: "Embedded" } },
      false,
      " Correct it before Gateway runs standalone.",
    ],
    [
      "a standalone Tasklist",
      "tasklist",
      "tasklist",
      { tasklist: { mode: "Standalone" } },
      true,
      "",
    ],
    [
      "Connectors that are enabled",
      "connectors",
      "connectors",
      { connectors: { enabled: true } },
      true,
      "",
    ],
    [
      "Connectors that are not enabled",
      "connectors",
      "connectors",
      {},
      false,
      " Correct it before Connectors are turned on.",
    ],
  ] as const)(
    "blocks on the resources of %s only when its workload runs",
    (_, host, process, cluster, blocks, later) => {
      const resources = setting(
        processSettings(host, process, {
          cluster: cluster as SettingsPage["cluster"],
          preset: { [process]: { resources: FAULT } } as SettingsPage["preset"],
        }),
        "resources",
      );
      expect(resources.mergedErrors).toEqual([
        expect.objectContaining({ message: BOTH + later, blocks }),
      ]);
    },
  );

  it("blocks on the resources of the dump", () => {
    const dump = setting(
      clusterSettings({ preset: { backup: { dump: { resources: FAULT } } } }),
      "backup.dump.resources",
    );
    expect(dump.mergedErrors).toEqual([
      expect.objectContaining({ message: BOTH, blocks: true }),
    ]);
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
    ).toEqual([]);
    expect(brokers().mergedErrors).toEqual([]);
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

describe("settingsOf claim layers", () => {
  const preset = { zeebe: { resources: { requests: { cpu: "1" } } } };

  it("gives no layers where no claim merges, as on the preset page", () => {
    const { group } = settingsGroupOf("zeebe", "zeebe", { preset });
    const { merged, below } = settingsFieldsOf({ preset });
    expect(
      setting(settingsOf({ group, spec: merged.spec, below }), "resources")
        .layers,
    ).toBeUndefined();
  });
});

describe("settingsOf of a preset body", () => {
  const elasticsearch: SettingsGroup = {
    label: "Elasticsearch",
    component: "elasticsearch",
    embedded: false,
    fields: [],
  };
  const server: SettingsGroup = {
    label: "PostgreSQL",
    component: "databaseServer",
    embedded: false,
    fields: [],
  };

  it("gives each field of an Elasticsearch preset body its editor and its steps under spec.cluster", () => {
    const settings = settingsOf({
      group: elasticsearch,
      spec: { replicas: 3 },
    });
    const replicas = setting(settings, "replicas");
    expect(replicas).toMatchObject({
      value: 3,
      editor: { kind: "number", integer: true, minimum: 1 },
    });
    expect(replicas.steps).toEqual(steps("spec", "cluster", "replicas"));
    expect(setting(settings, "extraEnv").editor?.kind).toBe("env");
    expect(setting(settings, "resources").editor?.kind).toBe("resources");
  });

  it("gives each field of a database server preset body its steps under spec.server", () => {
    const settings = settingsOf({
      group: server,
      spec: {},
      live: { instances: 2 },
    });
    const instances = setting(settings, "instances");
    expect(instances.value).toBeUndefined();
    expect(instances.current).toBe(2);
    expect(instances.steps).toEqual(steps("spec", "server", "instances"));
    expect(setting(settings, "archive.objectStorageRef").steps).toEqual(
      steps("spec", "server", "archive", "objectStorageRef"),
    );
  });
});

describe("settingsOf merged entry errors", () => {
  const PRESET =
    "comes from the preset. The claim cannot remove it. Pick another preset, or ask your admin to correct the preset.";

  // brokerSetting returns the setting label of the brokers under page.
  const brokerSetting = (label: string, page: SettingsPage) =>
    setting(processSettings("zeebe", "zeebe", page), label);

  it("reports a label key of the preset that is not valid, with the entry", () => {
    const labels = brokerSetting("podLabels", {
      preset: { zeebe: { podLabels: { "bad key": "x", team: "a" } } },
    });
    expect(labels.mergedErrors).toEqual([
      {
        message: `The label key "bad key" ${PRESET}`,
        blocks: true,
        entry: { key: "bad key", value: "x" },
      },
    ]);
  });

  it("keeps the error when the claim leaves the key out", () => {
    const labels = brokerSetting("podLabels", {
      preset: { zeebe: { podLabels: { "bad key": "x", team: "a" } } },
      claim: { cluster: { zeebe: { podLabels: { team: "a" } } } },
    });
    expect(labels.mergedErrors?.map((e) => e.message)).toEqual([
      `The label key "bad key" ${PRESET}`,
    ]);
  });

  it("reports an entry that the claim holds as one that the claim can correct, with no entry for the editor to leave out", () => {
    const labels = brokerSetting("podLabels", {
      preset: { zeebe: { podLabels: { "bad key": "x" } } },
      claim: { cluster: { zeebe: { podLabels: { "bad key": "y" } } } },
    });
    expect(labels.mergedErrors).toEqual([
      {
        message:
          'The label key "bad key" is not valid. Correct it, or remove its row.',
        blocks: true,
      },
    ]);
  });

  it("says that the claim can set a valid value for a label value of the preset that is not valid", () => {
    const labels = brokerSetting("podLabels", {
      preset: { zeebe: { podLabels: { team: "not valid" } } },
    });
    expect(labels.mergedErrors).toEqual([
      {
        message:
          'The value of the label "team" comes from the preset and is not valid. Set a valid value for "team" in the claim, pick another preset, or ask your admin to correct the preset.',
        blocks: true,
        entry: { key: "team", value: "not valid" },
      },
    ]);
  });

  it("reports one error for an entry whose key and value are not valid", () => {
    const labels = brokerSetting("podLabels", {
      preset: { zeebe: { podLabels: { "bad key": "not valid" } } },
    });
    expect(labels.mergedErrors?.map((e) => e.message)).toEqual([
      `The label key "bad key" ${PRESET}`,
    ]);
  });

  it("reports an empty annotation key of the preset", () => {
    const annotations = brokerSetting("podAnnotations", {
      preset: { zeebe: { podAnnotations: { "": "x", note: "any text" } } },
    });
    expect(annotations.mergedErrors).toEqual([
      {
        message: `An empty annotation key ${PRESET}`,
        blocks: true,
        entry: { key: "", value: "x" },
      },
    ]);
  });

  it("reports an annotation key of the preset that is not valid", () => {
    const annotations = brokerSetting("podAnnotations", {
      preset: { zeebe: { podAnnotations: { "bad key": "x" } } },
    });
    expect(annotations.mergedErrors?.map((e) => e.message)).toEqual([
      `The annotation key "bad key" ${PRESET}`,
    ]);
  });

  it("reports an empty key of the preset with an empty value, which the editor ignores", () => {
    const annotations = brokerSetting("podAnnotations", {
      preset: { zeebe: { podAnnotations: { "": "" } } },
    });
    expect(annotations.mergedErrors).toEqual([
      {
        message: `An empty annotation key ${PRESET}`,
        blocks: true,
        entry: { key: "", value: "" },
      },
    ]);
  });

  it("reports an empty annotation key of the Connectors ServiceAccount", () => {
    const annotations = setting(
      processSettings("connectors", "connectors", {
        cluster: { connectors: { enabled: true } },
        preset: {
          connectors: { serviceAccount: { annotations: { "": "x" } } },
        },
      }),
      "serviceAccount.annotations",
    );
    expect(annotations.mergedErrors?.map((e) => e.message)).toEqual([
      `An empty annotation key ${PRESET}`,
    ]);
  });

  it("reports a variable of the preset without a name", () => {
    const env = brokerSetting("extraEnv", {
      preset: { zeebe: { extraEnv: [{ name: "", value: "x" }] } },
      claim: {
        cluster: { zeebe: { extraEnv: [{ name: "TZ", value: "UTC" }] } },
      },
    });
    expect(env.mergedErrors).toEqual([
      {
        message: `A variable without a name ${PRESET}`,
        blocks: true,
        entry: { name: "", value: "x" },
      },
    ]);
  });

  it("reports a variable without a name that the claim holds, with no entry for the editor to leave out", () => {
    const env = brokerSetting("extraEnv", {
      preset: { zeebe: { extraEnv: [{ name: "", value: "x" }] } },
      claim: { cluster: { zeebe: { extraEnv: [{ name: "", value: "y" }] } } },
    });
    expect(env.mergedErrors).toEqual([
      {
        message: "A variable without a name is not valid. Remove its row.",
        blocks: true,
      },
    ]);
    expect(env.openedUnchecked).toBe(true);
  });

  it("asks the admin to correct a variable of the release", () => {
    const env = setting(
      clusterSettings({
        release: { version: "8.9.1", extraEnv: [{ name: "", value: "x" }] },
      }),
      "extraEnv",
    );
    expect(env.mergedErrors?.map((e) => e.message)).toEqual([
      "A variable without a name comes from the release. The claim cannot remove it. Ask your admin to correct the release.",
    ]);
  });

  it("blocks on the labels, the annotations, and the variables of the whole cluster", () => {
    const settings = clusterSettings({
      preset: {
        podLabels: { "bad key": "x" },
        podAnnotations: { "": "x" },
        extraEnv: [{ name: "", value: "x" }],
      },
    });
    for (const label of ["podLabels", "podAnnotations", "extraEnv"])
      expect(setting(settings, label).mergedErrors).toEqual([
        expect.objectContaining({ blocks: true }),
      ]);
  });

  it("does not block on the labels of an embedded app, and asks to correct them before it runs standalone", () => {
    const labels = setting(
      processSettings("gateway", "tasklist", {
        cluster: { gateway: { mode: "Standalone" } },
        preset: { tasklist: { podLabels: { "bad key": "x" } } },
      }),
      "podLabels",
    );
    expect(labels.mergedErrors).toEqual([
      expect.objectContaining({
        message: `The label key "bad key" ${PRESET} Correct it before Tasklist runs standalone.`,
        blocks: false,
      }),
    ]);
  });

  it("blocks on the variables of an embedded app, which its host gets", () => {
    const env = setting(
      processSettings("gateway", "tasklist", {
        cluster: { gateway: { mode: "Standalone" } },
        preset: { tasklist: { extraEnv: [{ name: "", value: "x" }] } },
      }),
      "extraEnv",
    );
    expect(env.mergedErrors).toEqual([
      expect.objectContaining({ blocks: true }),
    ]);
  });

  it("does not block on the variables of Connectors that are not enabled", () => {
    const env = setting(
      processSettings("connectors", "connectors", {
        preset: { connectors: { extraEnv: [{ name: "", value: "x" }] } },
      }),
      "extraEnv",
    );
    expect(env.mergedErrors).toEqual([
      expect.objectContaining({
        message: `A variable without a name ${PRESET} Correct it before Connectors are turned on.`,
        blocks: false,
      }),
    ]);
  });

  it("blocks on the labels and the variables of the dump", () => {
    const settings = clusterSettings({
      preset: {
        backup: {
          dump: {
            podLabels: { "bad key": "x" },
            extraEnv: [{ name: "", value: "x" }],
          },
        },
      },
    });
    for (const label of ["backup.dump.podLabels", "backup.dump.extraEnv"])
      expect(setting(settings, label).mergedErrors).toEqual([
        expect.objectContaining({ blocks: true }),
      ]);
  });

  // monitorLabels returns the setting monitoring.serviceMonitor.labels of
  // a claim whose ServiceMonitor is enabled as given and holds a label key
  // that is not valid.
  const monitorLabels = (enabled: boolean) =>
    setting(
      clusterSettings({
        claim: {
          cluster: {
            monitoring: {
              serviceMonitor: { enabled, labels: { "bad key": "x" } },
            },
          },
        },
      }),
      "monitoring.serviceMonitor.labels",
    );
  const HELD_KEY =
    'The label key "bad key" is not valid. Correct it, or remove its row.';

  it("warns about a label key of the claim that is not valid in a ServiceMonitor that is off", () => {
    expect(monitorLabels(false).mergedErrors).toEqual([
      {
        message: `${HELD_KEY} Correct it before you turn on the ServiceMonitor.`,
        blocks: false,
      },
    ]);
  });

  it("warns, and does not block, about an empty key of the claim in a ServiceMonitor that is off", () => {
    const labels = setting(
      clusterSettings({
        claim: {
          cluster: {
            monitoring: {
              serviceMonitor: { enabled: false, labels: { "": "" } },
            },
          },
        },
      }),
      "monitoring.serviceMonitor.labels",
    );
    expect(labels.openedUnchecked).toBe(true);
    expect(labels.mergedErrors).toEqual([
      {
        message:
          "An empty label key is not valid. Correct it, or remove its row. Correct it before you turn on the ServiceMonitor.",
        blocks: false,
      },
    ]);
  });

  it("blocks on a label key of the claim that is not valid in a ServiceMonitor that is on", () => {
    expect(monitorLabels(true).mergedErrors).toEqual([
      { message: HELD_KEY, blocks: true },
    ]);
  });

  it("keeps the rule of Kubernetes in the editor of a map that reaches no object, and leaves the rows it opens with to the merged check", () => {
    const labels = monitorLabels(false);
    expect(labels.editor).toEqual({ kind: "keyValue", format: "label" });
    expect(labels.openedUnchecked).toBe(true);
  });

  it("blocks on a label key of the claim that is not valid in the pod labels of a process that runs", () => {
    const labels = brokerSetting("podLabels", {
      claim: { cluster: { zeebe: { podLabels: { "bad key": "x" } } } },
    });
    expect(labels.mergedErrors).toEqual([{ message: HELD_KEY, blocks: true }]);
  });

  it("says that the value of a label of the claim that is not valid needs a correction", () => {
    const labels = brokerSetting("podLabels", {
      claim: { cluster: { zeebe: { podLabels: { team: "not valid" } } } },
    });
    expect(labels.mergedErrors).toEqual([
      {
        message: 'The value of the label "team" is not valid. Correct it.',
        blocks: true,
      },
    ]);
  });

  it("checks every row of a map in the editor without the claim, as on the preset page", () => {
    const page = {
      preset: {
        monitoring: { serviceMonitor: { labels: { "bad key": "x" } } },
      },
    };
    const { group } = settingsGroupOf("cluster", "cluster", page);
    const { merged } = settingsFieldsOf(page);
    const labels = setting(
      settingsOf({ group, spec: merged.spec }),
      "monitoring.serviceMonitor.labels",
    );
    expect(labels.editor).toEqual({ kind: "keyValue", format: "label" });
    expect(labels.openedUnchecked).toBeUndefined();
  });

  it("warns about an annotation key of the preset on a Connectors ServiceAccount that the operator does not create", () => {
    const annotations = setting(
      processSettings("connectors", "connectors", {
        cluster: { connectors: { enabled: true } },
        preset: {
          connectors: {
            serviceAccount: { create: false, annotations: { "bad key": "x" } },
          },
        },
      }),
      "serviceAccount.annotations",
    );
    expect(annotations.mergedErrors).toEqual([
      expect.objectContaining({
        message: `The annotation key "bad key" ${PRESET} Correct it before the operator creates the ServiceAccount.`,
        blocks: false,
      }),
    ]);
  });

  it("has no merged entry errors without the claim, as on the preset page", () => {
    const page = { preset: { zeebe: { podLabels: { "bad key": "x" } } } };
    const { group } = settingsGroupOf("zeebe", "zeebe", page);
    const { merged } = settingsFieldsOf(page);
    expect(
      setting(settingsOf({ group, spec: merged.spec }), "podLabels")
        .mergedErrors,
    ).toBeUndefined();
  });
});

describe("resourceWrites", () => {
  const preset = {
    zeebe: {
      resources: { requests: { memory: "1Gi" }, limits: { cpu: "500m" } },
    },
  };
  const saved = { zeebe: { resources: { limits: { cpu: 2 } } } };
  const at = (...names: string[]) =>
    steps("spec", "cluster", "zeebe", "resources", ...names);
  // resources returns the resources setting over the saved claim, with the
  // steps of pending.
  function resources(pending: Step[][]) {
    const { group } = settingsGroupOf("zeebe", "zeebe", { preset });
    const { merged, below } = settingsFieldsOf({
      preset,
      claim: { cluster: saved },
    });
    return setting(
      settingsOf({
        group,
        spec: merged.spec,
        claim: saved,
        liveClaim: saved,
        pending,
        below,
      }),
      "resources",
    );
  }

  it("writes each amount at its own key, a value or a reset", () => {
    expect(
      resourceWrites(
        resources([]),
        {
          "requests.memory": "1024Mi",
          "limits.cpu": "2000m",
          "requests.cpu": "3",
          "limits.memory": "",
        },
        [],
      ),
    ).toEqual([
      { steps: at("requests", "memory"), value: undefined },
      { steps: at("limits", "cpu"), value: 2 },
      { steps: at("requests", "cpu"), value: "3" },
      { steps: at("limits", "memory"), value: undefined },
    ]);
  });

  it("resets an amount equal to the preset under a pending change of the block", () => {
    expect(
      resourceWrites(resources([at()]), { "requests.memory": "1Gi" }, []),
    ).toEqual([{ steps: at("requests", "memory"), value: undefined }]);
  });

  it("holds no amount for a pending change of the amount itself or of another block", () => {
    const setting = resources([
      at("requests", "cpu"),
      steps("spec", "cluster", "gateway", "resources"),
    ]);
    expect(setting.layers?.pending).toEqual([at("requests", "cpu")]);
    expect(resourceWrites(setting, { "limits.cpu": "2" }, [])).toEqual([
      { steps: at("limits", "cpu"), value: 2 },
    ]);
  });

  it("holds an amount for a pending change of the block when the row mounted", () => {
    expect(
      resourceWrites(resources([]), { "limits.cpu": "500m" }, [at()]),
    ).toEqual([{ steps: at("limits", "cpu"), value: undefined }]);
  });

  it("holds no amount for a row mounted with nothing pending", () => {
    expect(resourceWrites(resources([]), { "limits.cpu": "500m" }, [])).toEqual(
      [{ steps: at("limits", "cpu"), value: "500m" }],
    );
  });
});

describe("blockingMergedErrors", () => {
  it("gives a preset entry that fails the rule of its editor with the path of its setting", () => {
    const { fields, merged } = settingsFieldsOf({
      preset: { zeebe: { podLabels: { "bad key": "x" } } },
    });
    expect(blockingMergedErrors(fields, merged.spec, {})).toEqual([
      'zeebe.podLabels: The label key "bad key" comes from the preset. The claim cannot remove it. Pick another preset, or ask your admin to correct the preset.',
    ]);
  });

  it("gives each merged error that blocks with the path of its setting", () => {
    const page = {
      cluster: { gateway: { mode: "Standalone" } },
      preset: {
        zeebe: { resources: { requests: { cpu: "2" }, limits: { cpu: "1" } } },
        tasklist: {
          resources: { requests: { cpu: "2" }, limits: { cpu: "1" } },
        },
      },
    } as const;
    const { fields, merged } = settingsFieldsOf(page as SettingsPage);
    expect(blockingMergedErrors(fields, merged.spec, {})).toEqual([
      "zeebe.resources: The CPU limit of 1 and the CPU request of 2 come from the preset. The limit is below the request.",
    ]);
  });

  it("gives nothing for a merged value that holds", () => {
    const { fields, merged } = settingsFieldsOf({
      preset: { zeebe: { resources: { limits: { cpu: "4" } } } },
    });
    expect(blockingMergedErrors(fields, merged.spec, {})).toEqual([]);
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
