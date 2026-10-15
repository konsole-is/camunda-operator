import type { DatabaseServerSpec } from "@/generated/crds/databaseserver";
import type { DatabaseServerPresetSpec } from "@/generated/crds/databaseserverpreset";
import type { ElasticsearchClusterSpec } from "@/generated/crds/elasticsearchcluster";
import type { ElasticsearchClusterPresetSpec } from "@/generated/crds/elasticsearchclusterpreset";
import { asObject, mergeLayers, type Merged, type RulesOf } from "./engine";

const elasticsearchRules = {
  presetRef: { instance: "value" },
  releaseRef: { instance: "value" },
  version: "value",
  replicas: "pointer",
  resources: "block",
  storageSize: "pointer",
  storageClassName: "pointer",
  serviceAccount: "block",
  extraEnv: "collection",
  extraEnvFrom: "collection",
  podLabels: "collection",
  podAnnotations: "collection",
  scheduling: "block",
  snapshotStorageRef: "value",
  secureSettings: "collection",
  secondaryStorageConfig: { instance: "value" },
  monitoring: "block",
  persistentVolumeClaimRetentionPolicy: "block",
  suspend: { instance: "value" },
} as const satisfies RulesOf<ElasticsearchClusterSpec>;

const databaseServerRules = {
  presetRef: { instance: "value" },
  releaseRef: { instance: "value" },
  platformConfigRef: "value",
  version: "value",
  instances: "pointer",
  resources: "block",
  storageSize: "pointer",
  storageClassName: "pointer",
  walStorageSize: "pointer",
  serviceAccount: "block",
  scheduling: "block",
  podLabels: "collection",
  podAnnotations: "collection",
  monitoring: "block",
  databaseServerConfig: { instance: "value" },
  archive: "block",
  suspend: { instance: "value" },
} as const satisfies RulesOf<DatabaseServerSpec>;

/**
 * mergeElasticsearch merges the overrides of an ElasticsearchCluster over
 * spec.cluster of its preset, by the rules of MergeSpec in
 * pkg/components/elasticsearchcluster. It takes no release, so the version
 * comes from the overrides only and no source is release. A preset of
 * undefined is empty.
 */
export function mergeElasticsearch(
  preset: ElasticsearchClusterPresetSpec | undefined,
  overrides: ElasticsearchClusterSpec,
): Merged<ElasticsearchClusterSpec> {
  const { value, source } = mergeLayers(
    elasticsearchRules,
    [
      ["preset", asObject(preset?.cluster)],
      ["overrides", asObject(overrides)],
    ],
    "spec",
  );
  return { spec: value as ElasticsearchClusterSpec, source };
}

/**
 * mergeDatabaseServer merges the overrides of a DatabaseServer over
 * spec.server of its preset, by the rules of MergeSpec in
 * pkg/components/databaseserver. It takes no release, so the version comes
 * from the overrides only and no source is release. A preset of undefined is
 * empty.
 */
export function mergeDatabaseServer(
  preset: DatabaseServerPresetSpec | undefined,
  overrides: DatabaseServerSpec,
): Merged<DatabaseServerSpec> {
  const { value, source } = mergeLayers(
    databaseServerRules,
    [
      ["preset", asObject(preset?.server)],
      ["overrides", asObject(overrides)],
    ],
    "spec",
  );
  return { spec: value as DatabaseServerSpec, source };
}
