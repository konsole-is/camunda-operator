import type { CamundaClusterSpec } from "@/generated/crds/camundacluster";
import type { CamundaClusterPresetSpec } from "@/generated/crds/camundaclusterpreset";
import type { CamundaReleaseSpec } from "@/generated/crds/camundarelease";
import {
  applyLayer,
  asObject,
  isObject,
  mergeLayers,
  objectField,
  type CustomMerge,
  type JsonObject,
  type Merged,
  type RulesOf,
} from "./engine";
import { child } from "./path";

type Spec = CamundaClusterSpec;
type Workload = NonNullable<Spec["operate"]>;
type Backup = NonNullable<Spec["backup"]>;
type PrimaryStorage = NonNullable<Backup["primaryStorage"]>;

const resources = {
  fields: {
    requests: "byKey",
    limits: "byKey",
    claims: "block",
  } satisfies RulesOf<NonNullable<Workload["resources"]>>,
} as const;

// The rules of WorkloadSpec, which every component block holds.
const workload = {
  replicas: "pointer",
  resources,
  extraEnv: "byName",
  extraEnvFrom: "concat",
  podLabels: "byKey",
  podAnnotations: "byKey",
  scheduling: "block",
} as const;

const webApp = {
  fields: { ...workload, mode: "value" } satisfies RulesOf<Workload>,
} as const;

const authRules = {
  clientId: "value",
  audience: "value",
  clientSecretRef: "block",
  admin: "block",
  basic: "block",
} as const satisfies RulesOf<NonNullable<Spec["auth"]>>;

// mergeAuth follows the client rule of the operator: a layer that sets
// clientId replaces the whole client, so the audience and the secret of the
// layers before it go with it.
const mergeAuth: CustomMerge = (target, name, over, path, ctx) => {
  if (!isObject(over)) return;
  const auth = objectField(target, name);
  if (typeof over.clientId === "string" && over.clientId !== "") {
    for (const field of ["audience", "clientSecretRef"]) {
      delete auth[field];
      delete ctx.source[child(path, field)];
    }
  }
  applyLayer(authRules, auth, over, path, ctx);
};

const clusterRules = {
  platformConfigRef: { instance: "value" },
  presetRef: { instance: "value" },
  releaseRef: { instance: "value" },
  version: "value",
  externalUrl: { instance: "value" },
  serviceAccount: { instance: "block" },
  auth: { merge: mergeAuth },
  zeebe: {
    fields: {
      ...workload,
      partitions: "pointer",
      replicationFactor: "pointer",
      storageClassName: "pointer",
      storageSize: "pointer",
      persistentVolumeClaimRetentionPolicy: "pointer",
    } satisfies RulesOf<NonNullable<Spec["zeebe"]>>,
  },
  gateway: {
    fields: { ...workload, mode: "value" } satisfies RulesOf<
      NonNullable<Spec["gateway"]>
    >,
  },
  operate: webApp,
  tasklist: webApp,
  admin: webApp,
  connectors: {
    fields: {
      ...workload,
      enabled: "pointer",
      version: "value",
      serviceAccount: {
        fields: {
          name: "value",
          create: "pointer",
          annotations: "byKey",
        } satisfies RulesOf<
          NonNullable<NonNullable<Spec["connectors"]>["serviceAccount"]>
        >,
      },
    } satisfies RulesOf<NonNullable<Spec["connectors"]>>,
  },
  extraEnv: "byName",
  extraEnvFrom: "concat",
  podLabels: "byKey",
  podAnnotations: "byKey",
  scheduling: "block",
  storageRef: { instance: "value" },
  indexReplicas: "pointer",
  backupStorageRef: { instance: "value" },
  documentStorageRef: { instance: "value" },
  backup: {
    fields: {
      primaryStorage: {
        fields: {
          continuous: "pointer",
          schedule: "value",
          checkpointInterval: "value",
          retention: {
            fields: {
              window: "value",
              cleanupSchedule: "value",
            } satisfies RulesOf<NonNullable<PrimaryStorage["retention"]>>,
          },
        } satisfies RulesOf<PrimaryStorage>,
      },
      dump: {
        fields: {
          resources,
          extraEnv: "byName",
          extraEnvFrom: "concat",
          podLabels: "byKey",
          podAnnotations: "byKey",
          scheduling: "block",
          scratchVolume: "block",
          activeDeadlineSeconds: "pointer",
          postgresImage: "value",
        } satisfies RulesOf<NonNullable<Backup["dump"]>>,
      },
    } satisfies RulesOf<Backup>,
  },
  monitoring: { instance: "block" },
  suspend: { instance: "value" },
  pause: { instance: "value" },
} as const satisfies RulesOf<Spec>;

/**
 * mergeCluster merges a CamundaCluster spec over its baselines: spec.cluster
 * of the preset, then the release over it, then the overrides over both, by
 * the rules of MergeSpec in pkg/components/camundacluster. A release takes
 * part with its version, its connectors version, and its environment only.
 * The instance-bound fields come from the overrides only. A preset or a
 * release of undefined is empty. The result shares no memory with the
 * inputs.
 */
export function mergeCluster(
  preset: CamundaClusterPresetSpec | undefined,
  release: CamundaReleaseSpec | undefined,
  overrides: CamundaClusterSpec,
): Merged<CamundaClusterSpec> {
  const { value, source } = mergeLayers(
    clusterRules,
    [
      ["preset", asObject(preset?.cluster)],
      ["release", release === undefined ? undefined : releaseLayer(release)],
      ["overrides", asObject(overrides)],
    ],
    "spec",
  );
  return { spec: value as CamundaClusterSpec, source };
}

type ReleaseEnv = { extraEnv?: unknown[]; extraEnvFrom?: unknown[] };

// releaseLayer puts a release in the shape of a cluster spec, as releaseLayer
// in Go does. A component block is there only when the release sets
// environment under it, so that `operate: {}` adds no block.
function releaseLayer(release: CamundaReleaseSpec): JsonObject {
  const layer: JsonObject = {
    version: release.version,
    extraEnv: release.extraEnv,
    extraEnvFrom: release.extraEnvFrom,
  };
  for (const name of [
    "zeebe",
    "gateway",
    "operate",
    "tasklist",
    "admin",
  ] as const) {
    const env = releaseWorkload(release[name]);
    if (env !== undefined) layer[name] = env;
  }
  const connectors = release.connectors;
  if (connectors !== undefined) {
    const env = releaseWorkload(connectors);
    if (connectors.version || env !== undefined) {
      layer.connectors = { ...env, version: connectors.version };
    }
  }
  return layer;
}

function releaseWorkload(env: ReleaseEnv | undefined): JsonObject | undefined {
  if (
    env === undefined ||
    (!env.extraEnv?.length && !env.extraEnvFrom?.length)
  ) {
    return undefined;
  }
  return { extraEnv: env.extraEnv, extraEnvFrom: env.extraEnvFrom };
}
