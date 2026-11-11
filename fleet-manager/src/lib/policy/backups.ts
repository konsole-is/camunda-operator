export type BackupKind = "LogicalBackupElasticsearch" | "LogicalBackupRDBMS";
export type RestoreKind =
  "LogicalRestoreElasticsearch" | "LogicalRestoreRDBMS" | "PointInTimeRestore";

/** BackupRef names one backup object. */
export type BackupRef = { kind: BackupKind; namespace: string; name: string };
/** RestoreRef names one restore object. */
export type RestoreRef = { kind: RestoreKind; namespace: string; name: string };

type Meta = { metadata?: { namespace?: string; name?: string } };
type Phased = { kind?: string; status?: { phase?: string } };

/** ClusterLike is the part of a CamundaCluster that the functions read. */
export type ClusterLike = Meta;
/** BackupLike is the part of a backup object that runningBackup reads. */
export type BackupLike = Meta &
  Phased & { spec: { clusterRef: { name: string } } };
/**
 * RestoreLike is the part of a restore object that runningRestore reads. A
 * logical restore names its cluster in targetClusterRef, a point-in-time
 * restore in clusterRef.
 */
export type RestoreLike = Meta &
  Phased & {
    spec: {
      targetClusterRef?: { name: string };
      clusterRef?: { name: string };
    };
  };

const BACKUP_KINDS: readonly string[] = [
  "LogicalBackupElasticsearch",
  "LogicalBackupRDBMS",
] satisfies BackupKind[];
const RESTORE_KINDS: readonly string[] = [
  "LogicalRestoreElasticsearch",
  "LogicalRestoreRDBMS",
  "PointInTimeRestore",
] satisfies RestoreKind[];

/**
 * clusterBackups returns the backups of cluster, in their input order. A
 * backup belongs to the cluster when it is in the namespace of the cluster
 * and its spec.clusterRef names the cluster. An object of another kind is
 * dropped.
 */
export function clusterBackups<T extends BackupLike>(
  cluster: ClusterLike,
  backups: readonly T[],
): T[] {
  return ofCluster(
    cluster,
    backups,
    BACKUP_KINDS,
    (b) => b.spec.clusterRef.name,
  );
}

/**
 * clusterRestores returns the restores of cluster by the rules of
 * clusterBackups. A logical restore names the cluster in
 * spec.targetClusterRef, a PointInTimeRestore in spec.clusterRef.
 */
export function clusterRestores<T extends RestoreLike>(
  cluster: ClusterLike,
  restores: readonly T[],
): T[] {
  return ofCluster(cluster, restores, RESTORE_KINDS, (r) =>
    r.kind === "PointInTimeRestore"
      ? r.spec.clusterRef?.name
      : r.spec.targetClusterRef?.name,
  );
}

/**
 * runningBackup returns the running backup of cluster in backups, or
 * undefined. A backup belongs to the cluster when it is in the namespace of
 * the cluster and its spec.clusterRef names the cluster. It runs until its
 * phase is Completed or Failed, so a missing phase and Pending count. An
 * object of another kind is skipped. With more than one, the result is the
 * first by name.
 */
export function runningBackup(
  cluster: ClusterLike,
  backups: readonly BackupLike[],
): BackupRef | undefined {
  return firstRunning(cluster, clusterBackups(cluster, backups)) as
    BackupRef | undefined;
}

/** RollBackupLike is the part of a backup object that rollSensitiveBackup reads. */
export type RollBackupLike = BackupLike & {
  status?: { zeebeBackupId?: number; runtime?: { state?: string } };
};

/**
 * RollSensitiveBackup is a backup with phase Running, with the facts that
 * the backup warning needs.
 */
export type RollSensitiveBackup =
  | (BackupRef & {
      kind: "LogicalBackupElasticsearch";
      partitionsFinal: boolean;
    })
  | (BackupRef & { kind: "LogicalBackupRDBMS"; hasZeebeBackupId: boolean });

/**
 * rollSensitiveBackup returns the backup of cluster that a roll of the
 * brokers can fail, or undefined. It finds the backups of the cluster by the
 * rules of clusterBackups, and takes only a backup with phase Running: a
 * pending backup waits for the rollout. With more than one, the result is
 * the first by name. partitionsFinal is true when the backup of the Zeebe
 * partitions of an Elasticsearch backup is Completed or Failed.
 * hasZeebeBackupId is true when a PostgreSQL backup has set
 * status.zeebeBackupId.
 */
export function rollSensitiveBackup(
  cluster: ClusterLike,
  backups: readonly RollBackupLike[],
): RollSensitiveBackup | undefined {
  const [first] = clusterBackups(cluster, backups)
    .filter((b) => b.status?.phase === "Running")
    .sort(byName);
  if (!first) return undefined;
  const ref = {
    namespace: cluster.metadata?.namespace ?? "",
    name: first.metadata?.name ?? "",
  };
  if (first.kind === "LogicalBackupElasticsearch") {
    const state = first.status?.runtime?.state;
    return {
      kind: first.kind,
      ...ref,
      partitionsFinal: state === "Completed" || state === "Failed",
    };
  }
  return {
    kind: "LogicalBackupRDBMS",
    ...ref,
    hasZeebeBackupId: typeof first.status?.zeebeBackupId === "number",
  };
}

/**
 * runningRestore returns the running restore of cluster in restores, or
 * undefined, by the rules of runningBackup. A logical restore names the
 * cluster in spec.targetClusterRef, a PointInTimeRestore in spec.clusterRef.
 */
export function runningRestore(
  cluster: ClusterLike,
  restores: readonly RestoreLike[],
): RestoreRef | undefined {
  return firstRunning(cluster, clusterRestores(cluster, restores)) as
    RestoreRef | undefined;
}

function ofCluster<T extends Meta & Phased>(
  cluster: ClusterLike,
  objects: readonly T[],
  kinds: readonly string[],
  clusterName: (object: T) => string | undefined,
): T[] {
  const { namespace = "", name = "" } = cluster.metadata ?? {};
  return objects.filter(
    (o) =>
      kinds.includes(o.kind ?? "") &&
      o.metadata?.namespace === namespace &&
      clusterName(o) === name,
  );
}

function firstRunning(
  cluster: ClusterLike,
  objects: readonly (Meta & Phased)[],
): { kind: string; namespace: string; name: string } | undefined {
  const [first] = objects
    .filter(
      (o) => o.status?.phase !== "Completed" && o.status?.phase !== "Failed",
    )
    .sort(byName);
  return first
    ? {
        kind: first.kind!,
        namespace: cluster.metadata?.namespace ?? "",
        name: first.metadata?.name ?? "",
      }
    : undefined;
}

function byName(a: Meta, b: Meta): number {
  return (a.metadata?.name ?? "").localeCompare(b.metadata?.name ?? "");
}
