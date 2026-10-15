import { describe, expect, it } from "vitest";
import type { DatabaseServerSpec } from "@/generated/crds/databaseserver";
import type { DatabaseServerPresetSpec } from "@/generated/crds/databaseserverpreset";
import type { ElasticsearchClusterSpec } from "@/generated/crds/elasticsearchcluster";
import type { ElasticsearchClusterPresetSpec } from "@/generated/crds/elasticsearchclusterpreset";
import { mergeDatabaseServer, mergeElasticsearch } from "./storage";

function deepFreeze<T>(value: T): T {
  if (typeof value === "object" && value !== null) {
    Object.values(value).forEach(deepFreeze);
    Object.freeze(value);
  }
  return value;
}

const esPreset: ElasticsearchClusterPresetSpec = {
  cluster: {
    replicas: 3,
    snapshotStorageRef: "preset-snapshots",
    resources: { requests: { cpu: "1", memory: "2Gi" } },
    extraEnv: [{ name: "A", value: "preset" }],
    podLabels: { "app.kubernetes.io/part-of": "camunda" },
  },
};

describe("mergeElasticsearch", () => {
  it("replaces a block as a whole, with one source for the block", () => {
    const { spec, source } = mergeElasticsearch(esPreset, {
      resources: { limits: { memory: "4Gi" } },
    });

    expect(spec.resources).toEqual({ limits: { memory: "4Gi" } });
    expect(source["spec.resources"]).toBe("overrides");
    expect(
      Object.keys(source).filter((p) => p.startsWith("spec.resources.")),
    ).toEqual([]);
  });

  it("keeps the preset list or map for an empty one, with the preset as source", () => {
    const { spec, source } = mergeElasticsearch(esPreset, {
      extraEnv: [],
      podLabels: {},
    });

    expect(spec.extraEnv).toEqual([{ name: "A", value: "preset" }]);
    expect(spec.podLabels).toEqual({ "app.kubernetes.io/part-of": "camunda" });
    expect(source["spec.extraEnv"]).toBe("preset");
    expect(source["spec.podLabels"]).toBe("preset");
  });

  it("a pointer zero replaces the preset value", () => {
    const { spec, source } = mergeElasticsearch(esPreset, { replicas: 0 });

    expect(spec.replicas).toBe(0);
    expect(source["spec.replicas"]).toBe("overrides");
  });

  it("an empty value field keeps the preset value", () => {
    const { spec, source } = mergeElasticsearch(esPreset, {
      snapshotStorageRef: "",
    });

    expect(spec.snapshotStorageRef).toBe("preset-snapshots");
    expect(source["spec.snapshotStorageRef"]).toBe("preset");
  });

  it("takes the instance-bound fields from the overrides only", () => {
    const preset: ElasticsearchClusterPresetSpec = {
      cluster: { suspend: true, secondaryStorageConfig: "preset-config" },
    };

    const { spec, source } = mergeElasticsearch(preset, {
      secondaryStorageConfig: "mine",
    });

    expect(spec).toEqual({ secondaryStorageConfig: "mine" });
    expect(source).toEqual({ "spec.secondaryStorageConfig": "overrides" });
  });

  it("without a preset, every value comes from the overrides", () => {
    const { spec, source } = mergeElasticsearch(undefined, {
      version: "9.2.4",
      replicas: 3,
    });

    expect(spec).toEqual({ version: "9.2.4", replicas: 3 });
    expect(source).toEqual({
      "spec.version": "overrides",
      "spec.replicas": "overrides",
    });
  });

  it("does not change its inputs or share memory with them", () => {
    const preset = deepFreeze(structuredClone(esPreset));
    const overrides: ElasticsearchClusterSpec = deepFreeze({
      version: "9.2.4",
    });

    const { spec } = mergeElasticsearch(preset, overrides);

    expect(() => {
      spec.extraEnv![0].value = "changed";
      spec.podLabels!.team = "changed";
    }).not.toThrow();
  });
});

describe("mergeDatabaseServer", () => {
  const preset: DatabaseServerPresetSpec = {
    server: {
      platformConfigRef: "preset-platform",
      instances: 2,
      archive: { objectStorageRef: "preset-bucket", retentionPeriodDays: 30 },
      podAnnotations: { note: "preset" },
    },
  };

  it("replaces the archive block as a whole", () => {
    const { spec, source } = mergeDatabaseServer(preset, {
      archive: { objectStorageRef: "mine", retentionPeriodDays: 7 },
    });

    expect(spec.archive).toEqual({
      objectStorageRef: "mine",
      retentionPeriodDays: 7,
    });
    expect(source["spec.archive"]).toBe("overrides");
  });

  it("merges platformConfigRef like any value field", () => {
    const kept = mergeDatabaseServer(preset, {});
    const replaced = mergeDatabaseServer(preset, { platformConfigRef: "mine" });

    expect(kept.spec.platformConfigRef).toBe("preset-platform");
    expect(kept.source["spec.platformConfigRef"]).toBe("preset");
    expect(replaced.spec.platformConfigRef).toBe("mine");
    expect(replaced.source["spec.platformConfigRef"]).toBe("overrides");
  });

  it("takes databaseServerConfig from the overrides only", () => {
    const overrides: DatabaseServerSpec = { databaseServerConfig: "my-config" };

    const { spec, source } = mergeDatabaseServer(
      { server: { databaseServerConfig: "preset-config" } },
      overrides,
    );

    expect(spec).toEqual({ databaseServerConfig: "my-config" });
    expect(source).toEqual({ "spec.databaseServerConfig": "overrides" });
  });

  it("does not change its inputs or share memory with them", () => {
    const frozen = deepFreeze(structuredClone(preset));

    const { spec } = mergeDatabaseServer(frozen, deepFreeze({}));

    expect(() => {
      spec.archive!.objectStorageRef = "changed";
    }).not.toThrow();
  });
});
