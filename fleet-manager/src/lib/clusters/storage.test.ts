import { describe, expect, it } from "vitest";
import type { ElasticsearchClusterPreset } from "@/generated/crds/elasticsearchclusterpreset";
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
  clusterDeletion,
  draftAccessPatterns,
  storageErrors,
  storageSummary,
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
  ) => storageErrors(draft, data, cardErrors, conflicts);

  it("asks for an option in a required section", () => {
    expect(errorsOf(draftWith({ "secondary-storage": [] }))).toEqual([
      "Choose an option for Secondary storage.",
    ]);
  });

  it("asks for an option in a required section that the step has not reported yet", () => {
    expect(errorsOf(draftWith())).toEqual([
      "Choose an option for Secondary storage.",
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

    expect(storageErrors(draft, withDefault, {}, [])).toEqual([]);
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
      "Choose another option for Volume encryption.",
    ]);
  });

  it("asks for another option when the picked one is gone", () => {
    const draft = draftWith({
      "secondary-storage": [
        { section: "secondary-storage", registration: "removed" },
      ],
    });

    expect(errorsOf(draft)).toEqual([
      "Choose another option for Secondary storage.",
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
    ).toEqual(["Fix the Secondary storage settings."]);
  });

  it("does not repeat the required error that the card also reports", () => {
    expect(
      errorsOf(draftWith({ "secondary-storage": [] }), {
        "secondary-storage": [
          { path: "secondary-storage", message: "Choose an option" },
        ],
      }),
    ).toEqual(["Choose an option for Secondary storage."]);
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
      "Choose an option for Secondary storage.",
      "Choose another option for gone.",
    ]);
  });

  it("appends the host conflicts", () => {
    expect(
      errorsOf(draftWith({ "secondary-storage": [] }), {}, [
        "The host is taken",
      ]),
    ).toEqual(["Choose an option for Secondary storage.", "The host is taken"]);
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

describe("storageErrors and a live list that changed after the pick", () => {
  const builtin = { name: "camunda-operator", version: "0.0.0" };
  const elasticsearch = registration("elasticsearch", {
    sections: ["secondary-storage"],
    label: "New Elasticsearch cluster",
    controller: builtin,
    parameters: {
      type: "object",
      required: ["preset", "releaseRef"],
      properties: {
        preset: { type: "string", minLength: 1, title: "Preset" },
        releaseRef: { type: "string", minLength: 1, title: "Release" },
        overrides: {
          type: "object",
          properties: { replicas: { type: "integer", minimum: 1 } },
        },
      },
    },
  });
  const existing = registration("existing-secondary-storage", {
    sections: ["secondary-storage"],
    label: "Existing secondary storage",
    controller: builtin,
    parameters: {
      type: "object",
      required: ["name"],
      properties: { name: { type: "string", minLength: 1, title: "Name" } },
    },
  });
  const environment = (spec: Partial<Environment["spec"]>): Environment => ({
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "Environment",
    metadata: { name: "prod" },
    spec: {
      releases: { allowed: ["r1"], default: "r1" },
      sections: {
        "secondary-storage": {
          allowed: ["elasticsearch", "existing-secondary-storage"],
        },
      },
      ...spec,
    },
  });
  const preset = (name: string) =>
    ({
      metadata: { name },
      spec: { cluster: { replicas: 1 } },
    }) as ElasticsearchClusterPreset;
  const dataOf = (
    spec: Partial<Environment["spec"]>,
    objects: StorageData["objects"],
  ): StorageData => ({
    environment: environment(spec),
    sections: [section("secondary-storage", "Secondary storage")],
    registrations: [elasticsearch, existing],
    objects,
  });
  const picking = (registration: string, parameters: Record<string, unknown>) =>
    draftWith({
      "secondary-storage": [
        { section: "secondary-storage", registration, parameters },
      ],
    });
  const named = {
    parameters: {
      "existing-secondary-storage": { name: { claimEditable: true } },
    },
  };

  it("refuses a picked SecondaryStorageConfig that is gone while another one is left, and names it", () => {
    const draft = picking("existing-secondary-storage", { name: "orders" });

    expect(
      storageErrors(
        draft,
        dataOf(named, {
          SecondaryStorageConfig: [config("orders", "team-a")],
        }),
        {},
        [],
      ),
    ).toEqual([]);
    expect(
      storageErrors(
        draft,
        dataOf(named, {
          SecondaryStorageConfig: [config("other", "team-a")],
        }),
        {},
        [],
      ),
    ).toEqual([
      "orders is not available for Name of Existing secondary storage. Choose another.",
    ]);
  });

  it("refuses a picked SecondaryStorageConfig that is gone with the last one, and names it", () => {
    expect(
      storageErrors(
        picking("existing-secondary-storage", { name: "orders" }),
        dataOf(named, { SecondaryStorageConfig: [] }),
        {},
        [],
      ),
    ).toEqual([
      "Choose another option for Secondary storage.",
      "orders is not available for Name of Existing secondary storage. Choose another.",
    ]);
  });

  it("refuses a SecondaryStorageConfig that an allowed name of the Environment gives once it is gone", () => {
    const alias = {
      parameters: {
        "existing-secondary-storage": {
          name: {
            claimEditable: true,
            allowed: [{ name: "fast", label: "Fast", value: "orders" }],
          },
        },
      },
    };
    const draft = picking("existing-secondary-storage", { name: "fast" });

    expect(
      storageErrors(
        draft,
        dataOf(alias, { SecondaryStorageConfig: [config("orders", "team-a")] }),
        {},
        [],
      ),
    ).toEqual([]);
    expect(
      storageErrors(
        draft,
        dataOf(alias, { SecondaryStorageConfig: [config("other", "team-a")] }),
        {},
        [],
      ),
    ).toEqual([
      "Pick another SecondaryStorageConfig for Name. orders is not in team-a.",
    ]);
  });

  it("refuses a preset that an allowed name of the Environment gives once the Environment no longer lists it", () => {
    const withPresets = (listed: string[]) =>
      dataOf(
        {
          presets: { camundaCluster: ["small"], elasticsearchCluster: listed },
          parameters: {
            elasticsearch: {
              preset: {
                claimEditable: true,
                allowed: [
                  { name: "small-option", label: "Small", value: "small" },
                ],
              },
            },
          },
        },
        { ElasticsearchClusterPreset: [preset("small"), preset("large")] },
      );
    const draft = picking("elasticsearch", { preset: "small-option" });

    expect(
      storageErrors(draft, withPresets(["small", "large"]), {}, []),
    ).toEqual([]);
    expect(storageErrors(draft, withPresets(["large"]), {}, [])).toEqual([
      'ElasticsearchClusterPreset "small" is not in spec.presets.elasticsearchCluster of Environment "prod"',
    ]);
  });

  it("refuses a preset that an allowed name of the Environment gives once the preset is deleted", () => {
    const data = (presets: ElasticsearchClusterPreset[]) =>
      dataOf(
        {
          presets: { elasticsearchCluster: ["small", "large"] },
          parameters: {
            elasticsearch: {
              preset: {
                claimEditable: true,
                allowed: [
                  { name: "small-option", label: "Small", value: "small" },
                ],
              },
            },
          },
        },
        { ElasticsearchClusterPreset: presets },
      );
    const draft = picking("elasticsearch", { preset: "small-option" });

    expect(
      storageErrors(draft, data([preset("small"), preset("large")]), {}, []),
    ).toEqual([]);
    expect(storageErrors(draft, data([preset("large")]), {}, [])).toEqual([
      "Pick another ElasticsearchClusterPreset for Preset. small does not exist.",
    ]);
  });

  it("leaves the release to a later step while no release is known", () => {
    const data = dataOf(
      {
        releases: { allowed: ["r1"] },
        presets: { elasticsearchCluster: ["small"] },
        parameters: { elasticsearch: { preset: { claimEditable: true } } },
      },
      { ElasticsearchClusterPreset: [preset("small")] },
    );

    expect(
      storageErrors(
        picking("elasticsearch", { preset: "small" }),
        data,
        {},
        [],
      ),
    ).toEqual([]);
  });

  it("refuses a locked policy value that the schema of its registration refuses", () => {
    const locked = (replicas: number) =>
      dataOf(
        {
          presets: { elasticsearchCluster: ["small"] },
          parameters: {
            elasticsearch: {
              preset: { value: "small" },
              overrides: { value: { replicas } },
            },
          },
        },
        { ElasticsearchClusterPreset: [preset("small")] },
      );
    const draft = picking("elasticsearch", {});

    expect(storageErrors(draft, locked(1), {}, [])).toEqual([]);
    expect(storageErrors(draft, locked(0), {}, [])).toEqual([
      expect.stringMatching(
        /^The policy gives New Elasticsearch cluster a value that its schema refuses: .+\. Pick another option, or ask your admin to change the policy of the Environment\.$/,
      ),
    ]);
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

describe("clusterDeletion", () => {
  const keeping = (name: string, sections: string[]) =>
    registration(name, { sections, deletionPolicies: ["Delete", "Orphan"] });
  const data: StorageData = {
    environment: undefined,
    sections: SECTIONS,
    registrations: [
      keeping("s3", ["backup-storage"]),
      registration("ephemeral", {
        sections: ["backup-storage"],
        label: "Ephemeral bucket",
      }),
      keeping("internal", ["access"]),
    ],
    objects: {},
  };
  const withEnvironment = (
    deletionPolicy?: "Delete" | "Orphan",
    backupDefault?: string,
  ): StorageData => ({
    ...data,
    environment: {
      apiVersion: "fleet.camunda.io/v1alpha1",
      kind: "Environment",
      metadata: { name: "prod" },
      spec: {
        sections: {
          "backup-storage": {
            allowed: ["s3", "ephemeral"],
            ...(backupDefault && { default: backupDefault }),
          },
          access: { allowed: ["internal"] },
        },
        ...(deletionPolicy && { deletionPolicy }),
      },
    } satisfies Environment,
  });
  const picking = (
    backup: string,
    policy?: "Delete" | "Orphan",
  ): ClusterDraft => ({
    ...draftWith({
      "backup-storage": [{ section: "backup-storage", registration: backup }],
      access: [{ section: "access", registration: "internal" }],
    }),
    ...(policy && {
      claim: {
        metadata: { name: "c1" },
        spec: { presetRef: "small", deletionPolicy: policy },
      },
    }),
  });

  it("does not offer Keep when checkDeletionPolicy refuses Orphan", () => {
    const choice = clusterDeletion(picking("ephemeral"), data);

    expect(choice.keepAllowed).toBe(false);
    expect(choice.alwaysDeleted).toEqual([
      { section: "backup-storage", label: "Ephemeral bucket (Backup storage)" },
    ]);
  });

  it("offers Keep when every registration supports Orphan", () => {
    const choice = clusterDeletion(picking("s3"), data);

    expect(choice.keepAllowed).toBe(true);
    expect(choice.alwaysDeleted).toEqual([]);
  });

  it("does not offer Keep for a default that the step has not reported yet", () => {
    expect(
      clusterDeletion(draftWith(), withEnvironment(undefined, "ephemeral"))
        .keepAllowed,
    ).toBe(false);
  });

  it("offers Keep for the picked empty option", () => {
    expect(
      clusterDeletion(draftWith({ "backup-storage": [], access: [] }), data)
        .keepAllowed,
    ).toBe(true);
  });

  it("takes Delete as the default without an Environment", () => {
    expect(clusterDeletion(picking("s3"), data)).toMatchObject({
      defaultPolicy: "Delete",
      policy: "Delete",
    });
  });

  it("takes Delete as the default of an Environment that sets none", () => {
    expect(
      clusterDeletion(picking("s3"), withEnvironment()).defaultPolicy,
    ).toBe("Delete");
  });

  it("takes the deletion default of the Environment", () => {
    expect(
      clusterDeletion(picking("s3"), withEnvironment("Orphan")),
    ).toMatchObject({ defaultPolicy: "Orphan", policy: "Orphan" });
  });

  it("takes the pick of the draft over the default", () => {
    expect(
      clusterDeletion(picking("s3", "Delete"), withEnvironment("Orphan"))
        .policy,
    ).toBe("Delete");
  });

  it("falls back to Delete when the default keeps and a pick cannot", () => {
    expect(
      clusterDeletion(picking("ephemeral"), withEnvironment("Orphan")),
    ).toMatchObject({ defaultPolicy: "Orphan", policy: "Delete" });
  });

  it("falls back to Delete when the draft keeps and a pick cannot", () => {
    expect(clusterDeletion(picking("ephemeral", "Orphan"), data).policy).toBe(
      "Delete",
    );
  });
});

describe("storageSummary", () => {
  const builtin = { name: "camunda-operator", version: "0.0.0" };
  const data: StorageData = {
    environment: {
      apiVersion: "fleet.camunda.io/v1alpha1",
      kind: "Environment",
      metadata: { name: "prod" },
      spec: {
        sections: {
          "backup-storage": { allowed: ["s3"] },
          "secondary-storage": {
            allowed: [
              "elasticsearch",
              "database-server",
              "existing-secondary-storage",
              "plugin-store",
            ],
          },
          access: { allowed: ["gateway", "internal"] },
          "connectors-permissions": { allowed: ["aws-connectors-role"] },
        },
        parameters: {
          "existing-secondary-storage": { name: { claimEditable: true } },
          "aws-connectors-role": {
            managedPolicyArns: {
              value: ["arn:aws:iam::aws:policy/AmazonEC2ReadOnlyAccess"],
            },
            extraPolicyArns: {
              claimEditable: true,
              allowed: [
                {
                  name: "sqs",
                  label: "SQS access",
                  value: "arn:aws:iam::aws:policy/AmazonSQSFullAccess",
                },
                {
                  name: "s3",
                  label: "S3 read",
                  value: "arn:aws:iam::aws:policy/AmazonS3ReadOnlyAccess",
                },
              ],
            },
            roleArn: { claimEditable: true },
          },
        },
      },
    },
    sections: [
      section("backup-storage", "Backup storage", {
        order: 40,
        slot: "backupStorage",
        emptyOptionLabel: "No backups",
      }),
      section("secondary-storage", "Secondary storage", {
        order: 50,
        slot: "secondaryStorage",
        emptyOptionLabel: "None",
      }),
      section("access", "Access", {
        order: 60,
        slot: "externalAccess",
        emptyOptionLabel: "No access",
      }),
      section("connectors-permissions", "Connectors permissions", {
        order: 70,
        slot: "connectorsIdentity",
        emptyOptionLabel: "None",
      }),
    ],
    registrations: [
      registration("s3", { sections: ["backup-storage"] }),
      registration("elasticsearch", {
        sections: ["secondary-storage"],
        controller: builtin,
      }),
      registration("database-server", {
        sections: ["secondary-storage"],
        controller: builtin,
      }),
      registration("existing-secondary-storage", {
        sections: ["secondary-storage"],
        controller: builtin,
        parameterValues: { name: "shared" },
        parameters: {
          type: "object",
          properties: { name: { type: "string", title: "Name" } },
        },
      }),
      registration("plugin-store", { sections: ["secondary-storage"] }),
      registration("gateway", {
        outputs: [{ key: "externalUrl" }, { key: "optimizeUrl" }],
      }),
      registration("internal", { outputs: [{ key: "externalUrl" }] }),
      registration("aws-connectors-role", {
        sections: ["connectors-permissions"],
        label: "Role for each cluster",
      }),
    ],
    objects: {
      SecondaryStorageConfig: [
        config("shared", "team-a"),
        { ...config("other", "team-a"), spec: { type: "elasticsearch" } },
        { ...config("shared", "team-b"), spec: { type: "elasticsearch" } },
      ] as SecondaryStorageConfig[],
    },
  };
  const pick = (
    section: string,
    registration: string,
    parameters?: Record<string, unknown>,
  ) => ({ section, registration, ...(parameters && { parameters }) });
  const summaryOf = (storage: ClusterDraft["storage"]) =>
    storageSummary(
      draftWith({
        "backup-storage": [],
        "secondary-storage": [],
        access: [],
        "connectors-permissions": [],
        ...storage,
      }),
      data,
    );

  it("names the backup storage selection by the slot of its section", () => {
    expect(
      summaryOf({ "backup-storage": [pick("backup-storage", "s3")] })
        .backupStorage,
    ).toEqual(pick("backup-storage", "s3"));
    expect(summaryOf({}).backupStorage).toBeUndefined();
  });

  it("knows the type of built-in Elasticsearch and PostgreSQL", () => {
    expect(
      summaryOf({
        "secondary-storage": [pick("secondary-storage", "elasticsearch")],
      }).secondaryStorageType,
    ).toBe("elasticsearch");
    expect(
      summaryOf({
        "secondary-storage": [pick("secondary-storage", "database-server")],
      }).secondaryStorageType,
    ).toBe("rdbms");
  });

  it("reads the type of the named SecondaryStorageConfig in the namespace of the draft", () => {
    expect(
      summaryOf({
        "secondary-storage": [
          pick("secondary-storage", "existing-secondary-storage", {
            name: "other",
          }),
        ],
      }).secondaryStorageType,
    ).toBe("elasticsearch");
  });

  it("reads the type of the SecondaryStorageConfig that the policy names", () => {
    expect(
      summaryOf({
        "secondary-storage": [
          pick("secondary-storage", "existing-secondary-storage"),
        ],
      }).secondaryStorageType,
    ).toBe("rdbms");
  });

  it("does not know the type of a SecondaryStorageConfig that is not there", () => {
    expect(
      summaryOf({
        "secondary-storage": [
          pick("secondary-storage", "existing-secondary-storage", {
            name: "missing",
          }),
        ],
      }).secondaryStorageType,
    ).toBeUndefined();
  });

  it("does not know the type of another registration", () => {
    expect(
      summaryOf({
        "secondary-storage": [pick("secondary-storage", "plugin-store")],
      }).secondaryStorageType,
    ).toBeUndefined();
  });

  it("says that an access option gives the Optimize URL when it lists that output", () => {
    expect(summaryOf({ access: [pick("access", "gateway")] }).optimizeUrl).toBe(
      true,
    );
    expect(
      summaryOf({ access: [pick("access", "internal")] }).optimizeUrl,
    ).toBe(false);
    expect(summaryOf({}).optimizeUrl).toBe(false);
  });

  it("lists the slots that the selections fill, in section order", () => {
    expect(
      summaryOf({
        access: [pick("access", "gateway")],
        "backup-storage": [pick("backup-storage", "s3")],
      }).slots,
    ).toEqual(["backupStorage", "externalAccess"]);
  });

  it("lists the picked labels of an allowed list and no ARN", () => {
    const summary = summaryOf({
      "connectors-permissions": [
        pick("connectors-permissions", "aws-connectors-role", {
          extraPolicyArns: ["sqs", "s3"],
        }),
      ],
    });

    expect(summary.picks).toContainEqual({
      section: "connectors-permissions",
      sectionLabel: "Connectors permissions",
      registration: "aws-connectors-role",
      optionLabel: "Role for each cluster",
      values: [{ label: "extraPolicyArns", value: "SQS access, S3 read" }],
    });
    expect(JSON.stringify(summary)).not.toMatch(/arn:aws/);
  });

  it("lists the policy value of an editable parameter that the selection leaves out", () => {
    const withDefault: StorageData = {
      ...data,
      environment: {
        ...data.environment!,
        spec: {
          ...data.environment!.spec,
          parameters: {
            "aws-connectors-role": {
              extraPolicyArns: {
                claimEditable: true,
                value: ["arn:aws:iam::aws:policy/AmazonSQSFullAccess"],
                allowed: [
                  {
                    name: "sqs",
                    label: "SQS access",
                    value: "arn:aws:iam::aws:policy/AmazonSQSFullAccess",
                  },
                ],
              },
            },
          },
        },
      },
    };
    const summary = storageSummary(
      draftWith({
        "connectors-permissions": [
          pick("connectors-permissions", "aws-connectors-role"),
        ],
      }),
      withDefault,
    );

    expect(summary.picks.at(-1)?.values).toEqual([
      { label: "extraPolicyArns", value: "SQS access" },
    ]);
    expect(JSON.stringify(summary)).not.toMatch(/arn:aws/);
  });

  it("shows a policy value that is no name of the allowed list as Set by your admin", () => {
    const outside: StorageData = {
      ...data,
      environment: {
        ...data.environment!,
        spec: {
          ...data.environment!.spec,
          parameters: {
            "aws-connectors-role": {
              extraPolicyArns: {
                claimEditable: true,
                value: "arn:aws:iam::aws:policy/AdministratorAccess",
                allowed: [
                  {
                    name: "sqs",
                    label: "SQS access",
                    value: "arn:aws:iam::aws:policy/AmazonSQSFullAccess",
                  },
                ],
              },
            },
          },
        },
      },
    };
    const summary = storageSummary(
      draftWith({
        "connectors-permissions": [
          pick("connectors-permissions", "aws-connectors-role"),
        ],
      }),
      outside,
    );

    expect(summary.picks.at(-1)?.values).toEqual([
      { label: "extraPolicyArns", value: "Set by your admin" },
    ]);
  });

  it("shows an ARN that the user entered as An ARN that you entered", () => {
    const summary = summaryOf({
      "connectors-permissions": [
        pick("connectors-permissions", "aws-connectors-role", {
          roleArn: "arn:aws:iam::123456789012:role/x",
        }),
      ],
    });

    expect(summary.picks[0].values).toEqual([
      { label: "roleArn", value: "An ARN that you entered" },
    ]);
  });

  it("shows an ARN of the policy as Set by your admin", () => {
    const withRole: StorageData = {
      ...data,
      environment: {
        ...data.environment!,
        spec: {
          ...data.environment!.spec,
          parameters: {
            "aws-connectors-role": {
              roleArn: {
                claimEditable: true,
                value: "arn:aws:iam::123456789012:role/admin",
              },
            },
          },
        },
      },
    };
    const summary = storageSummary(
      draftWith({
        "connectors-permissions": [
          pick("connectors-permissions", "aws-connectors-role"),
        ],
      }),
      withRole,
    );

    expect(summary.picks.at(-1)?.values).toEqual([
      { label: "roleArn", value: "Set by your admin" },
    ]);
  });

  it("shows another typed value as written", () => {
    const summary = summaryOf({
      "connectors-permissions": [
        pick("connectors-permissions", "aws-connectors-role", {
          roleArn: "connectors",
        }),
      ],
    });

    expect(summary.picks[0].values).toEqual([
      { label: "roleArn", value: "connectors" },
    ]);
  });

  it("gives one pick for each selection, in section order, with the label of the parameter", () => {
    const summary = summaryOf({
      access: [pick("access", "gateway")],
      "secondary-storage": [
        pick("secondary-storage", "existing-secondary-storage", {
          name: "other",
        }),
      ],
    });

    expect(summary.picks).toEqual([
      {
        section: "secondary-storage",
        sectionLabel: "Secondary storage",
        registration: "existing-secondary-storage",
        optionLabel: "existing-secondary-storage",
        values: [{ label: "Name", value: "other" }],
      },
      {
        section: "access",
        sectionLabel: "Access",
        registration: "gateway",
        optionLabel: "gateway",
        values: [],
      },
    ]);
  });
});
