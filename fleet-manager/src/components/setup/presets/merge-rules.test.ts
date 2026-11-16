import { describe, expect, it } from "vitest";
import { cannotSet, presetParts } from "./merge-rules";

describe("cannotSet", () => {
  it("lists the fields that each CRD refuses in a preset", () => {
    expect(new Set(cannotSet("CamundaClusterPreset"))).toEqual(
      new Set([
        "platformConfigRef",
        "presetRef",
        "releaseRef",
        "externalUrl",
        "serviceAccount",
        "connectors.serviceAccount.name",
        "storageRef",
        "backupStorageRef",
        "documentStorageRef",
        "monitoring",
        "suspend",
        "pause",
        "version",
        "connectors.version",
      ]),
    );
    expect(new Set(cannotSet("ElasticsearchClusterPreset"))).toEqual(
      new Set([
        "presetRef",
        "releaseRef",
        "secondaryStorageConfig",
        "suspend",
        "version",
      ]),
    );
    expect(new Set(cannotSet("DatabaseServerPreset"))).toEqual(
      new Set([
        "presetRef",
        "releaseRef",
        "databaseServerConfig",
        "suspend",
        "version",
      ]),
    );
  });
});

describe("presetParts", () => {
  it("gives each field of a cluster preset its rule, by part", () => {
    expect(
      presetParts("CamundaClusterPreset", {
        podLabels: { team: "a" },
        zeebe: {
          replicas: 3,
          resources: { requests: { cpu: "1" } },
          extraEnv: [{ name: "TZ", value: "UTC" }],
        },
        gateway: { scheduling: { nodeSelector: { pool: "x" } } },
      }),
    ).toEqual([
      {
        title: "General",
        settings: [
          { path: "podLabels", value: { team: "a" }, rule: "Merged by key" },
        ],
      },
      {
        title: "Brokers",
        settings: [
          {
            path: "zeebe.replicas",
            value: 3,
            rule: "A resource value replaces it",
          },
          {
            path: "zeebe.resources.requests",
            value: { cpu: "1" },
            rule: "Merged by key",
          },
          {
            path: "zeebe.extraEnv",
            value: [{ name: "TZ", value: "UTC" }],
            rule: "Merged by name",
          },
        ],
      },
      {
        title: "Gateway",
        settings: [
          {
            path: "gateway.scheduling",
            value: { nodeSelector: { pool: "x" } },
            rule: "Replaces whole",
          },
        ],
      },
    ]);
  });

  it("labels the client fields and the blocks of auth apart", () => {
    const [auth] = presetParts("CamundaClusterPreset", {
      auth: { clientId: "c", basic: { adminEmail: "a@b.c" } },
    });
    expect(auth).toEqual({
      title: "Authentication",
      settings: [
        { path: "auth.clientId", value: "c", rule: "Replaced with the client" },
        {
          path: "auth.basic",
          value: { adminEmail: "a@b.c" },
          rule: "Replaces whole",
        },
      ],
    });
  });

  it("replaces the lists and maps of an Elasticsearch preset whole", () => {
    expect(
      presetParts("ElasticsearchClusterPreset", {
        replicas: 3,
        podLabels: { a: "b" },
      }),
    ).toEqual([
      {
        title: "General",
        settings: [
          { path: "replicas", value: 3, rule: "A resource value replaces it" },
          { path: "podLabels", value: { a: "b" }, rule: "Replaces whole" },
        ],
      },
    ]);
  });

  it("labels the entries of a list that the resource appends to", () => {
    const [general] = presetParts("CamundaClusterPreset", {
      extraEnvFrom: [{ configMapRef: { name: "shared" } }],
    });
    expect(general.settings[0].rule).toBe("Resource entries follow");
  });

  it("keeps an empty connectors service account block as one setting", () => {
    expect(
      presetParts("CamundaClusterPreset", {
        connectors: { serviceAccount: {} },
      }),
    ).toEqual([
      {
        title: "Connectors",
        settings: [
          {
            path: "connectors.serviceAccount",
            value: {},
            rule: "Merged by field",
          },
        ],
      },
    ]);
  });

  it("walks the nested blocks of the backup into one part", () => {
    expect(
      presetParts("CamundaClusterPreset", {
        backup: { primaryStorage: { retention: { window: "7d" } } },
      }),
    ).toEqual([
      {
        title: "Backup",
        settings: [
          {
            path: "backup.primaryStorage.retention.window",
            value: "7d",
            rule: "A resource value replaces it",
          },
        ],
      },
    ]);
  });

  it("shows no field that a preset cannot set", () => {
    expect(
      presetParts("DatabaseServerPreset", { suspend: true, presetRef: "x" }),
    ).toEqual([]);
  });

  it("has no part for an empty body", () => {
    expect(presetParts("DatabaseServerPreset", undefined)).toEqual([]);
    expect(presetParts("CamundaClusterPreset", {})).toEqual([]);
  });
});
