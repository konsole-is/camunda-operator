import { describe, expect, it } from "vitest";
import type { Environment } from "@/generated/crds/environment";
import type {
  PluginRegistration,
  PluginRegistrationSpec,
} from "@/generated/crds/pluginregistration";
import type { PluginSection } from "@/generated/crds/pluginsection";
import type { SecondaryStorageConfig } from "@/generated/crds/secondarystorageconfig";
import type { ClusterDraft } from "./draft";
import {
  type StorageData,
  draftAccessPatterns,
  storageErrors,
  storageViews,
} from "./storage";

function section(
  name: string,
  label: string,
  spec: Partial<PluginSection["spec"]> = {},
): PluginSection {
  return {
    apiVersion: "plugins.camunda.io/v1alpha1",
    kind: "PluginSection",
    metadata: { name },
    spec: {
      appliesTo: ["CamundaClusterClaim"],
      label,
      selection: "One",
      ...spec,
    },
  };
}

function registration(
  name: string,
  spec: Partial<PluginRegistrationSpec> = {},
): PluginRegistration {
  return {
    apiVersion: "plugins.camunda.io/v1alpha1",
    kind: "PluginRegistration",
    metadata: { name },
    spec: {
      sections: ["access"],
      mode: "Optional",
      label: name,
      deletionPolicies: ["Delete"],
      serviceAccount: { name, namespace: "plugins" },
      ...spec,
    },
    status: {
      conditions: ["Accepted", "Ready"].map((type) => ({
        type,
        status: "True" as const,
        reason: "Test",
        message: "",
        lastTransitionTime: "2026-11-10T00:00:00Z",
      })),
    },
  };
}

const SECTIONS = [
  section("volume-encryption", "Volume encryption", {
    order: 30,
    emptyOptionLabel: "Unencrypted volumes",
  }),
  section("encryption-key", "Encryption key", {
    order: 20,
    emptyOptionLabel: "No key",
  }),
  section("access", "Access", { order: 60, emptyOptionLabel: "No access" }),
  section("secondary-storage", "Secondary storage", { order: 50 }),
  section("backup-storage", "Backup storage", { order: 50 }),
  section("management-database", "Databases", {
    order: 10,
    appliesTo: ["CamundaManagementClusterClaim"],
  }),
];

const REGISTRATIONS = [
  registration("kms", { sections: ["encryption-key"] }),
  registration("ebs", {
    sections: ["volume-encryption"],
    inputs: [{ name: "keyRef", from: "encryption-key/keyRef", required: true }],
  }),
  registration("gateway", {
    mode: "Default",
    parameterValues: { hostnamePattern: "{name}.{namespace}.example.com" },
  }),
  registration("internal", {
    parameterValues: {
      hostnamePattern: "{name}.{namespace}.internal.example.com",
    },
  }),
  registration("existing-secondary-storage", {
    sections: ["secondary-storage"],
    controller: { name: "camunda-operator", version: "0.0.0" },
    claimEditable: ["name"],
    parameters: {
      type: "object",
      properties: { name: { type: "string", title: "Name" } },
    },
  }),
];

function config(name: string, namespace: string): SecondaryStorageConfig {
  return {
    apiVersion: "core.camunda.io/v1",
    kind: "SecondaryStorageConfig",
    metadata: { name, namespace },
    spec: { type: "rdbms" },
  } as SecondaryStorageConfig;
}

const DATA: StorageData = {
  environment: undefined,
  sections: SECTIONS,
  registrations: REGISTRATIONS,
  objects: {
    SecondaryStorageConfig: [config("shared", "team-a"), config("x", "b")],
  },
};

function draftWith(storage?: ClusterDraft["storage"]): ClusterDraft {
  return {
    namespace: { kind: "existing", name: "team-a" },
    claim: { metadata: { name: "c1" }, spec: { presetRef: "small" } },
    ...(storage && { storage }),
  };
}

const viewNamed = (draft: ClusterDraft, name: string, data = DATA) =>
  storageViews(draft, data).find((v) => v.name === name)!;

describe("storageViews", () => {
  it("shows the sections of a cluster claim by order, then name", () => {
    expect(storageViews(draftWith(), DATA).map((v) => v.name)).toEqual([
      "encryption-key",
      "volume-encryption",
      "backup-storage",
      "secondary-storage",
      "access",
    ]);
  });

  it("disables an option whose input section has no selection in the draft", () => {
    expect(
      viewNamed(draftWith(), "volume-encryption").options[0].disabledReason,
    ).toBe("Needs an encryption key");
    expect(
      viewNamed(
        draftWith({
          "encryption-key": [
            { section: "encryption-key", registration: "kms" },
          ],
        }),
        "volume-encryption",
      ).options[0].disabledReason,
    ).toBeUndefined();
  });

  it("enables a default whose input section has an enabled default that the step has not reported yet", () => {
    const defaults = {
      ...DATA,
      environment: {
        apiVersion: "fleet.camunda.io/v1alpha1",
        kind: "Environment",
        metadata: { name: "prod" },
        spec: {
          sections: {
            "encryption-key": { allowed: ["kms"], default: "kms" },
            "volume-encryption": { allowed: ["ebs"], default: "ebs" },
          },
        },
      } satisfies Environment,
    };

    expect(
      viewNamed(draftWith(), "volume-encryption", defaults).options[0]
        .disabledReason,
    ).toBeUndefined();
    expect(
      viewNamed(
        draftWith({ "encryption-key": [] }),
        "volume-encryption",
        defaults,
      ).options[0].disabledReason,
    ).toBe("Needs an encryption key");
  });

  it("decorates a built-in option in the namespace of the draft", () => {
    const [option] = viewNamed(draftWith(), "secondary-storage").options;

    expect(option.parameters[0]).toMatchObject({
      name: "name",
      allowed: [{ name: "shared", label: "shared" }],
    });
  });

  it("shows a registration that is not ready as disabled", () => {
    const data = {
      ...DATA,
      registrations: [
        ...REGISTRATIONS.filter((r) => r.metadata.name !== "internal"),
        {
          ...registration("internal"),
          status: {
            conditions: [
              {
                type: "Accepted",
                status: "True" as const,
                reason: "Test",
                message: "",
                lastTransitionTime: "2026-11-10T00:00:00Z",
              },
            ],
          },
        },
      ],
    };

    expect(
      viewNamed(draftWith(), "access", data).options.map((o) => [
        o.registration,
        o.disabledReason,
      ]),
    ).toEqual([
      ["gateway", undefined],
      ["internal", "Unavailable right now"],
    ]);
  });
});

describe("storageErrors", () => {
  const required: Environment = {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "Environment",
    metadata: { name: "prod" },
    spec: {
      sections: {
        "secondary-storage": {
          allowed: ["existing-secondary-storage"],
          required: true,
        },
        "volume-encryption": { allowed: ["ebs"] },
      },
    },
  };
  const data = { ...DATA, environment: required };
  const errorsOf = (
    draft: ClusterDraft,
    cardErrors: Parameters<typeof storageErrors>[2] = {},
    conflicts: string[] = [],
  ) => storageErrors(draft, storageViews(draft, data), cardErrors, conflicts);

  it("asks for an option in a required section", () => {
    expect(errorsOf(draftWith({ "secondary-storage": [] }))).toEqual([
      "Choose an option for Secondary storage",
    ]);
  });

  it("asks for an option in a required section that the step has not reported yet", () => {
    expect(errorsOf(draftWith())).toEqual([
      "Choose an option for Secondary storage",
    ]);
  });

  it("counts the default of a required section that the step has not reported yet", () => {
    const withDefault = {
      ...data,
      environment: {
        ...required,
        spec: {
          sections: {
            "secondary-storage": {
              allowed: ["existing-secondary-storage"],
              default: "existing-secondary-storage",
              required: true,
            },
          },
        },
      },
    };
    const draft = draftWith();

    expect(
      storageErrors(draft, storageViews(draft, withDefault), {}, []),
    ).toEqual([]);
  });

  it("asks for another option when the picked one is disabled now", () => {
    const draft = draftWith({
      "secondary-storage": [
        {
          section: "secondary-storage",
          registration: "existing-secondary-storage",
        },
      ],
      "volume-encryption": [
        { section: "volume-encryption", registration: "ebs" },
      ],
    });

    expect(errorsOf(draft)).toEqual([
      "Choose another option for Volume encryption",
    ]);
  });

  it("asks for another option when the picked one is gone", () => {
    const draft = draftWith({
      "secondary-storage": [
        { section: "secondary-storage", registration: "removed" },
      ],
    });

    expect(errorsOf(draft)).toEqual([
      "Choose another option for Secondary storage",
    ]);
  });

  it("asks to fix the settings of a section whose card reports a field error", () => {
    const draft = draftWith({
      "secondary-storage": [
        {
          section: "secondary-storage",
          registration: "existing-secondary-storage",
        },
      ],
    });

    expect(
      errorsOf(draft, {
        "secondary-storage": [
          { path: "existing-secondary-storage/name", message: "is required" },
          { path: "existing-secondary-storage/name", message: "too long" },
        ],
      }),
    ).toEqual(["Fix the Secondary storage settings"]);
  });

  it("does not repeat the required error that the card also reports", () => {
    expect(
      errorsOf(draftWith({ "secondary-storage": [] }), {
        "secondary-storage": [
          { path: "secondary-storage", message: "Choose an option" },
        ],
      }),
    ).toEqual(["Choose an option for Secondary storage"]);
  });

  it("asks for another option for a pick of a section that step 2 no longer shows", () => {
    expect(
      errorsOf(
        draftWith({
          "secondary-storage": [],
          gone: [{ section: "gone", registration: "x" }],
          "also-gone": [],
        }),
      ),
    ).toEqual([
      "Choose an option for Secondary storage",
      "Choose another option for gone",
    ]);
  });

  it("appends the host conflicts", () => {
    expect(
      errorsOf(draftWith({ "secondary-storage": [] }), {}, [
        "The host is taken",
      ]),
    ).toEqual(["Choose an option for Secondary storage", "The host is taken"]);
  });

  it("gives no error for a complete draft", () => {
    expect(
      errorsOf(
        draftWith({
          "secondary-storage": [
            {
              section: "secondary-storage",
              registration: "existing-secondary-storage",
            },
          ],
        }),
      ),
    ).toEqual([]);
  });
});

describe("draftAccessPatterns", () => {
  it("takes the default option before step 2 showed the access section", () => {
    expect(draftAccessPatterns(draftWith(), DATA)).toEqual({
      hostnamePattern: "{name}.{namespace}.example.com",
    });
  });

  it("takes the picked option after", () => {
    expect(
      draftAccessPatterns(
        draftWith({
          access: [{ section: "access", registration: "internal" }],
        }),
        DATA,
      ),
    ).toEqual({ hostnamePattern: "{name}.{namespace}.internal.example.com" });
  });

  it("gives none for the picked empty option", () => {
    expect(
      draftAccessPatterns(draftWith({ access: [] }), DATA),
    ).toBeUndefined();
  });

  it("gives none for a picked option whose registration is gone", () => {
    expect(
      draftAccessPatterns(
        draftWith({ access: [{ section: "access", registration: "removed" }] }),
        DATA,
      ),
    ).toBeUndefined();
  });
});
