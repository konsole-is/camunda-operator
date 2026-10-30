import type { CamundaClusterSpec } from "@/generated/crds/camundacluster";
import type { Resource } from "@/lib/k8s/kinds";
import type { Process } from "./topology";

/**
 * PodContext is the platform config and the secondary storage of a
 * cluster. Each one is optional: without it, podInputs keeps each setting
 * that it could render.
 */
export type PodContext = {
  platform?: Resource<"CamundaPlatformConfig">;
  storage?: Resource<"SecondaryStorageConfig">;
};

/**
 * podInputs returns the settings of a merged cluster spec that the
 * operator renders into the environment of process p, as render in
 * pkg/components/camundacluster/render.go does. When the results for the
 * spec before and after a change differ, the pods of p restart.
 */
export function podInputs(
  spec: CamundaClusterSpec,
  ctx: PodContext,
  p: Process,
): unknown {
  const method = ctx.platform
    ? (ctx.platform.spec.auth?.method ?? "basic")
    : undefined;
  const oidc = method === "basic" ? undefined : oidcOf(spec, ctx.platform);

  if (p === "connectors")
    return {
      gatewayEmbedded: spec.gateway?.mode === "Embedded",
      client: oidc && [oidc.clientId, oidc.audience, oidc.secret],
      // A new passwordRotation gives a new admin password, and the config
      // hash of the connectors takes the hash of that password.
      rotation:
        method === "oidc" ? undefined : spec.auth?.basic?.passwordRotation,
    };

  return {
    identity: [
      spec.zeebe?.replicas ?? 1,
      spec.zeebe?.partitions ?? 1,
      spec.zeebe?.replicationFactor ?? 1,
    ],
    indexReplicas: indexReplicas(spec, ctx.storage),
    oidc: oidc && {
      ...oidc,
      redirect: spec.externalUrl?.replace(/\/+$/, ""),
      connectorsClient:
        oidc.connectorsClaim && spec.connectors?.enabled === true,
    },
    backup: backupOf(spec, ctx.storage, p),
  };
}

// oidcOf follows ResolveAuth in pkg/components/camundacluster/input.go.
function oidcOf(
  spec: CamundaClusterSpec,
  platform: Resource<"CamundaPlatformConfig"> | undefined,
) {
  const base = platform?.spec.auth?.oidc;
  if (platform && !base) return undefined;
  const own = spec.auth;
  let clientId = base?.clientId;
  let audience = base?.audience;
  let secret: unknown = base?.clientSecretRef;
  if (own?.clientId) {
    clientId = own.clientId;
    audience = undefined;
    secret = undefined;
  }
  if (own?.audience) audience = own.audience;
  if (own?.clientSecretRef) secret = own.clientSecretRef;
  return {
    clientId,
    audience: audience || clientId,
    secret,
    admin: own?.admin,
    connectorsClaim: !platform || Boolean(base?.clientIdClaim),
  };
}

// indexReplicas follows IndexReplicas of ElasticsearchStorage in api/v1.
function indexReplicas(
  spec: CamundaClusterSpec,
  storage: Resource<"SecondaryStorageConfig"> | undefined,
): number | undefined {
  if (!storage) return spec.indexReplicas;
  if (storage.spec.type !== "elasticsearch") return undefined;
  const nodes = storage.spec.elasticsearch?.nodeCount;
  return (
    spec.indexReplicas ??
    (nodes === undefined ? undefined : Math.min(1, nodes - 1))
  );
}

// backupOf follows backupEnv in pkg/components/camundacluster/backup.go and
// the defaults of PrimaryStorageBackup.
function backupOf(
  spec: CamundaClusterSpec,
  storage: Resource<"SecondaryStorageConfig"> | undefined,
  p: Process,
): unknown {
  if (!spec.backupStorageRef) return undefined;
  if (storage && storage.spec.type !== "rdbms") return undefined;
  const ps = spec.backup?.primaryStorage ?? {};
  const schedule = ps.schedule || "PT1H";
  const idMode = [
    ps.continuous ?? schedule !== "none",
    schedule,
    ps.checkpointInterval || "PT15M",
  ];
  if (p === "gateway") return idMode;
  if (p !== "zeebe") return undefined;
  return [
    ...idMode,
    ps.retention?.window || "P7D",
    ps.retention?.cleanupSchedule || "PT1H",
  ];
}
