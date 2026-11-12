import { describe, expect, it } from "vitest";
import type { DatabaseServerPreset } from "@/generated/crds/databaseserverpreset";
import type { ElasticsearchClusterPreset } from "@/generated/crds/elasticsearchclusterpreset";
import type { Environment } from "@/generated/crds/environment";
import type {
  PluginRegistration,
  PluginRegistrationSpec,
} from "@/generated/crds/pluginregistration";
import type { PluginSection } from "@/generated/crds/pluginsection";
import type { SecondaryStorageConfig } from "@/generated/crds/secondarystorageconfig";
import {
  BUILTIN_OPTIONS,
  type BuiltinContext,
  builtinEntry,
  builtinOption,
  bucketNote,
} from "./builtins";
import { sectionOptions } from "./options";

const PRESET_SCHEMA = {
  type: "object",
  required: ["preset", "releaseRef"],
  properties: {
    preset: { type: "string", title: "Preset" },
    releaseRef: { type: "string", title: "Release" },
    overrides: { type: "object", title: "Overrides" },
  },
};

function builtin(
  name: string,
  spec: Partial<PluginRegistrationSpec> = {},
): PluginRegistration {
  return {
    apiVersion: "plugins.camunda.io/v1alpha1",
    kind: "PluginRegistration",
    metadata: { name },
    spec: {
      sections: ["secondary-storage"],
      mode: "Optional",
      label: name,
      deletionPolicies: ["Delete", "Orphan"],
      serviceAccount: { name: "operator", namespace: "system" },
      controller: { name: "camunda-operator", version: "0.0.0" },
      parameters: PRESET_SCHEMA,
      inputs: [
        {
          name: "objectStorageConfig",
          from: "backup-storage/objectStorageConfig",
        },
      ],
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

function section(name: string, label: string): PluginSection {
  return {
    apiVersion: "plugins.camunda.io/v1alpha1",
    kind: "PluginSection",
    metadata: { name },
    spec: { appliesTo: ["CamundaClusterClaim"], label, selection: "One" },
  };
}

function esPreset(
  name: string,
  cluster: ElasticsearchClusterPreset["spec"]["cluster"],
): ElasticsearchClusterPreset {
  return {
    apiVersion: "core.camunda.io/v1",
    kind: "ElasticsearchClusterPreset",
    metadata: { name },
    spec: { cluster },
  };
}

function dbPreset(
  name: string,
  server: DatabaseServerPreset["spec"]["server"],
): DatabaseServerPreset {
  return {
    apiVersion: "core.camunda.io/v1",
    kind: "DatabaseServerPreset",
    metadata: { name },
    spec: { server },
  };
}

function storageConfig(
  name: string,
  namespace: string,
): SecondaryStorageConfig {
  return {
    apiVersion: "core.camunda.io/v1",
    kind: "SecondaryStorageConfig",
    metadata: { name, namespace },
    spec: { type: "rdbms" },
  } as SecondaryStorageConfig;
}

function environment(spec: Partial<Environment["spec"]>): Environment {
  return {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "Environment",
    metadata: { name: "prod" },
    spec,
  };
}

const small = esPreset("small", {
  replicas: 1,
  resources: { requests: { cpu: "1", memory: "2Gi" } },
  storageSize: "20Gi",
});
const large = esPreset("large", { replicas: 3 });
const shared = storageConfig("shared", "team-a");
const other = storageConfig("other", "team-b");

const SECONDARY = section("secondary-storage", "Secondary storage");
const SECTIONS = [SECONDARY, section("backup-storage", "Backup storage")];

function optionOf(
  registration: PluginRegistration,
  env: Environment | undefined,
) {
  const [option] = sectionOptions(
    SECONDARY,
    [registration],
    env,
    "CamundaClusterClaim",
  ).options;
  return option;
}

function ctx(env: Environment | undefined, namespace?: string): BuiltinContext {
  return {
    environment: env,
    ...(namespace && { namespace }),
    objects: {
      ElasticsearchClusterPreset: [small, large],
      SecondaryStorageConfig: [shared, other],
    },
  };
}

const editablePreset = (
  extra: Partial<Environment["spec"]> = {},
  preset: NonNullable<Environment["spec"]["parameters"]>[string][string] = {
    claimEditable: true,
  },
) =>
  environment({
    sections: { "secondary-storage": { allowed: ["elasticsearch"] } },
    parameters: { elasticsearch: { preset } },
    ...extra,
  });

describe("builtinEntry", () => {
  it("gives the entry of a built-in registration", () => {
    expect(builtinEntry(builtin("elasticsearch"))).toBe(
      BUILTIN_OPTIONS.elasticsearch,
    );
  });

  it("gives nothing for a registration that another controller serves", () => {
    expect(
      builtinEntry(
        builtin("elasticsearch", {
          controller: { name: "someone-else", version: "1.0.0" },
        }),
      ),
    ).toBeUndefined();
  });

  it("gives nothing for a built-in name that the table does not have", () => {
    expect(builtinEntry(builtin("constructor"))).toBeUndefined();
  });
});

describe("builtinOption", () => {
  it("hides releaseRef of the built-in Elasticsearch", () => {
    const es = builtin("elasticsearch");

    const option = builtinOption(optionOf(es, undefined), [es], ctx(undefined));

    expect(option.parameters.map((r) => r.name)).toEqual([
      "preset",
      "overrides",
    ]);
  });

  it("lists the presets that the Environment allows as cards, in its order, with their facts", () => {
    const es = builtin("elasticsearch");
    const env = editablePreset({
      presets: { elasticsearchCluster: ["large", "gone", "small"] },
    });

    const option = builtinOption(optionOf(es, env), [es], ctx(env));

    expect(option.choices).toEqual({
      parameter: "preset",
      items: [
        { name: "large", specs: [{ label: "Nodes", value: "3" }] },
        {
          name: "small",
          specs: [
            { label: "Nodes", value: "1" },
            { label: "CPU", value: "1" },
            { label: "Memory", value: "2Gi" },
            { label: "Storage", value: "20Gi" },
          ],
        },
      ],
    });
    expect(option.disabledReason).toBeUndefined();
  });

  it("lists every preset by name without an Environment", () => {
    const es = builtin("elasticsearch", { claimEditable: ["preset"] });

    const option = builtinOption(optionOf(es, undefined), [es], ctx(undefined));

    expect(option.choices?.items.map((i) => i.name)).toEqual([
      "large",
      "small",
    ]);
  });

  it("gives the facts of a PostgreSQL preset", () => {
    const pg = builtin("database-server", { claimEditable: ["preset"] });

    const option = builtinOption(optionOf(pg, undefined), [pg], {
      environment: undefined,
      objects: {
        DatabaseServerPreset: [
          dbPreset("pg", {
            instances: 2,
            resources: { requests: { cpu: "500m", memory: "1Gi" } },
            storageSize: "10Gi",
          }),
        ],
      },
    });

    expect(option.choices).toEqual({
      parameter: "preset",
      items: [
        {
          name: "pg",
          specs: [
            { label: "Instances", value: "2" },
            { label: "CPU", value: "500m" },
            { label: "Memory", value: "1Gi" },
            { label: "Storage", value: "10Gi" },
          ],
        },
      ],
    });
  });

  it("keeps a locked preset as it is", () => {
    const es = builtin("elasticsearch");
    const option = optionOf(es, undefined);

    const decorated = builtinOption(option, [es], ctx(undefined));

    expect(decorated.choices).toBeUndefined();
    expect(decorated.parameters.find((r) => r.name === "preset")).toEqual(
      option.parameters.find((r) => r.name === "preset"),
    );
  });

  it("keeps the allowed list that the Environment gives a preset", () => {
    const es = builtin("elasticsearch");
    const env = editablePreset(
      { presets: { elasticsearchCluster: ["small"] } },
      {
        claimEditable: true,
        allowed: [{ name: "tiny", label: "Tiny", value: "small" }],
      },
    );
    const option = optionOf(es, env);

    const decorated = builtinOption(option, [es], ctx(env));

    expect(decorated.choices).toBeUndefined();
    expect(decorated.parameters.find((r) => r.name === "preset")).toEqual(
      option.parameters.find((r) => r.name === "preset"),
    );
  });

  it.each([
    ["lists no preset", { presets: { elasticsearchCluster: [] } }],
    ["has no preset list", {}],
    [
      "names only unknown presets",
      { presets: { elasticsearchCluster: ["x"] } },
    ],
  ])(
    "disables the option when the Environment %s",
    (_, extra: Partial<Environment["spec"]>) => {
      const es = builtin("elasticsearch");
      const env = editablePreset(extra);

      const option = builtinOption(optionOf(es, env), [es], ctx(env));

      expect(option.disabledReason).toBe(
        "No ElasticsearchClusterPreset is offered",
      );
    },
  );

  it("lists the config objects of the namespace for a claim-editable name", () => {
    const existing = builtin("existing-secondary-storage", {
      claimEditable: ["name"],
      parameters: {
        type: "object",
        properties: { name: { type: "string", title: "Name" } },
      },
    });

    const option = builtinOption(
      optionOf(existing, undefined),
      [existing],
      ctx(undefined, "team-a"),
    );

    expect(option.parameters).toEqual([
      expect.objectContaining({
        name: "name",
        claimEditable: true,
        allowed: [{ name: "shared", label: "shared" }],
      }),
    ]);
    expect(option.disabledReason).toBeUndefined();
  });

  it("disables the option when the namespace has no object of the kind", () => {
    const existing = builtin("existing-secondary-storage", {
      claimEditable: ["name"],
    });

    const option = builtinOption(
      optionOf(existing, undefined),
      [existing],
      ctx(undefined, "team-c"),
    );

    expect(option.disabledReason).toBe("No SecondaryStorageConfig in team-c");
  });

  it("gives a config parameter no list without a namespace", () => {
    const existing = builtin("existing-secondary-storage", {
      claimEditable: ["name"],
    });
    const option = optionOf(existing, undefined);

    const decorated = builtinOption(option, [existing], ctx(undefined));

    expect(decorated).toEqual(option);
  });

  it("keeps the reason of an option that is disabled already", () => {
    const existing = builtin("existing-secondary-storage", {
      claimEditable: ["name"],
    });
    const option = {
      ...optionOf(existing, undefined),
      disabledReason: "Unavailable right now",
    };

    expect(
      builtinOption(option, [existing], ctx(undefined, "team-c"))
        .disabledReason,
    ).toBe("Unavailable right now");
  });

  it("changes no option of a registration that another controller serves", () => {
    const foreign = builtin("elasticsearch", {
      controller: { name: "someone-else", version: "1.0.0" },
      claimEditable: ["preset"],
    });
    const option = optionOf(foreign, undefined);

    expect(builtinOption(option, [foreign], ctx(undefined))).toBe(option);
  });

  it("hides databases of the built-in management database server", () => {
    const server = builtin("management-database-server", {
      parameters: {
        type: "object",
        properties: {
          preset: { type: "string" },
          version: { type: "string" },
          databases: { type: "array" },
        },
      },
    });

    const option = builtinOption(
      optionOf(server, undefined),
      [server],
      ctx(undefined),
    );

    expect(option.parameters.map((r) => r.name)).toEqual(["preset", "version"]);
  });
});

describe("bucketNote", () => {
  const BUCKET = [{ section: "backup-storage", registration: "s3" }];
  const objects = {
    ElasticsearchClusterPreset: [
      esPreset("with-snapshots", { snapshotStorageRef: "preset-bucket" }),
      esPreset("no-snapshots", { replicas: 1 }),
    ],
    DatabaseServerPreset: [
      dbPreset("with-archive", {
        archive: { objectStorageRef: "preset-bucket", retentionPeriodDays: 7 },
      }),
      dbPreset("no-archive", { instances: 1 }),
    ],
  };
  const locked = (registration: string, preset: string): BuiltinContext => ({
    environment: environment({
      parameters: { [registration]: { preset: { value: preset } } },
    }),
    objects,
  });
  const es = builtin("elasticsearch");
  const pg = builtin("database-server");

  it.each([
    ["with-snapshots", BUCKET, "Snapshots: From Backup storage"],
    ["no-snapshots", BUCKET, "Snapshots: From Backup storage"],
    ["with-snapshots", [], "Snapshots: From the preset"],
    ["no-snapshots", [], "Snapshots: No snapshots"],
  ])(
    "says the snapshot bucket of Elasticsearch with preset %s and backup storage %j",
    (preset, selections, note) => {
      expect(
        bucketNote(es, selections, SECTIONS, locked("elasticsearch", preset)),
      ).toBe(note);
    },
  );

  it.each([
    ["with-archive", BUCKET, "Archive: From Backup storage"],
    ["no-archive", BUCKET, "Archive: No archive"],
    ["with-archive", [], "Archive: From the preset"],
    ["no-archive", [], "Archive: No archive"],
  ])(
    "says the archive bucket of PostgreSQL with preset %s and backup storage %j",
    (preset, selections, note) => {
      expect(
        bucketNote(pg, selections, SECTIONS, locked("database-server", preset)),
      ).toBe(note);
    },
  );

  it("says From Backup storage for Elasticsearch whose preset is not known", () => {
    expect(
      bucketNote(es, BUCKET, SECTIONS, { environment: undefined, objects }),
    ).toBe("Snapshots: From Backup storage");
  });

  it.each([
    ["Elasticsearch without backup storage", es, []],
    ["PostgreSQL without backup storage", pg, []],
    ["PostgreSQL with backup storage", pg, BUCKET],
  ])(
    "gives no line for %s while the preset is not known",
    (_, registration, selections) => {
      expect(
        bucketNote(registration, selections, SECTIONS, {
          environment: undefined,
          objects,
        }),
      ).toBeUndefined();
    },
  );

  it("gives no line for a preset that does not exist", () => {
    expect(
      bucketNote(pg, [], SECTIONS, locked("database-server", "gone")),
    ).toBeUndefined();
  });

  it("reads the preset that the claim picks when the preset is claim-editable", () => {
    const context = {
      environment: environment({
        parameters: {
          "database-server": {
            preset: { claimEditable: true, value: "no-archive" },
          },
        },
      }),
      objects,
    };
    const picked = {
      section: "secondary-storage",
      registration: "database-server",
      parameters: { preset: "with-archive" },
    };

    expect(bucketNote(pg, [picked], SECTIONS, context)).toBe(
      "Archive: From the preset",
    );
  });

  it("ignores the pick of a locked preset", () => {
    const picked = {
      section: "secondary-storage",
      registration: "database-server",
      parameters: { preset: "with-archive" },
    };

    expect(
      bucketNote(
        pg,
        [picked],
        SECTIONS,
        locked("database-server", "no-archive"),
      ),
    ).toBe("Archive: No archive");
  });

  it("gives nothing for a registration without a bucket entry", () => {
    expect(
      bucketNote(builtin("existing-secondary-storage"), [], SECTIONS, {
        environment: undefined,
        objects,
      }),
    ).toBeUndefined();
  });

  it("gives nothing for a registration that another controller serves", () => {
    expect(
      bucketNote(
        builtin("elasticsearch", {
          controller: { name: "someone-else", version: "1.0.0" },
        }),
        BUCKET,
        SECTIONS,
        locked("elasticsearch", "with-snapshots"),
      ),
    ).toBeUndefined();
  });
});
