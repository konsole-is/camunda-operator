import { sha256 } from "@noble/hashes/sha2.js";
import { bytesToHex, utf8ToBytes } from "@noble/hashes/utils.js";
import type { StatusTone } from "@/components/fleet/status";
import type { CamundaClusterSpec } from "@/generated/crds/camundacluster";
import type { DatabaseServerSpec } from "@/generated/crds/databaseserver";
import { formatBytes } from "@/components/fleet/units";
import type { Resource } from "@/lib/k8s/kinds";
import { BACKUP_SCHEDULE_LABEL } from "@/lib/labels";
import { mergeDatabaseServer } from "@/lib/merge";
import {
  type BackupKind,
  type ClusterLike,
  type RestoreKind,
  clusterBackups,
  clusterRestores,
} from "@/lib/policy/backups";

export type BackupObject =
  Resource<"LogicalBackupElasticsearch"> | Resource<"LogicalBackupRDBMS">;
export type RestoreObject =
  | Resource<"LogicalRestoreElasticsearch">
  | Resource<"LogicalRestoreRDBMS">
  | Resource<"PointInTimeRestore">;

/** Detail is one labelled value of an expanded backup row. */
export type Detail = { label: string; value: string };

/**
 * BackupRow is one backup as the Backups table shows it. Each text field is
 * "-" when the backup has no value for it. details holds only the values
 * that the backup has.
 */
export type BackupRow = {
  kind: BackupKind;
  name: string;
  origin: "Scheduled" | "Manual";
  started: string;
  finished: string;
  duration: string;
  phase: string;
  tone: StatusTone;
  step: string;
  version: string;
  details: Detail[];
};

/** RestoreRow is one restore as the Restores table shows it. */
export type RestoreRow = {
  kind: RestoreKind;
  name: string;
  label: "Restore" | "Point-in-time restore";
  source: string;
  started: string;
  phase: string;
  tone: StatusTone;
  running: boolean;
  failureMessage?: string;
};

/** RECENT_RESTORES is the number of finished restores that restoreRowsOf keeps. */
export const RECENT_RESTORES = 5;

const DASH = "-";
const FINAL = ["Completed", "Failed"];

/**
 * dateTime writes an RFC 3339 time as "2026-10-18 11:58 UTC", the same on
 * every machine. It writes "-" for an unset or bad time.
 */
export function dateTime(iso: string | undefined): string {
  const ms = parse(iso);
  if (ms === undefined) return DASH;
  const text = new Date(ms).toISOString();
  return `${text.slice(0, 10)} ${text.slice(11, 16)} UTC`;
}

/**
 * duration writes the time from from to to in its two largest units, such as
 * "12m 5s" or "2d 4h", and leaves out a second unit of zero. It writes "-"
 * when a time is unset or bad, and "0s" when to is before from.
 */
export function duration(
  from: string | undefined,
  to: string | undefined,
): string {
  const start = parse(from);
  const end = parse(to);
  if (start === undefined || end === undefined) return DASH;
  const seconds = Math.max(0, Math.floor((end - start) / 1000));
  const units: [number, string][] = [
    [Math.floor(seconds / 86400), "d"],
    [Math.floor(seconds / 3600) % 24, "h"],
    [Math.floor(seconds / 60) % 60, "m"],
    [seconds % 60, "s"],
  ];
  const first = units.findIndex(([n]) => n > 0);
  if (first === -1) return "0s";
  return units
    .slice(first, first + 2)
    .filter(([n], i) => i === 0 || n > 0)
    .map(([n, unit]) => `${n}${unit}`)
    .join(" ");
}

/**
 * backupRowsOf returns the rows of the backups of cluster, newest first,
 * then by name. A backup that is not Completed or Failed counts its
 * duration to now.
 */
export function backupRowsOf(
  cluster: ClusterLike,
  backups: readonly BackupObject[],
  now: string,
): BackupRow[] {
  return newestFirst(clusterBackups(cluster, backups)).map((b) => {
    const status = b.status ?? {};
    const created = b.metadata?.creationTimestamp;
    const phase = status.phase ?? "Pending";
    const final = FINAL.includes(phase);
    return {
      kind: b.kind,
      name: b.metadata?.name ?? "",
      origin: b.metadata?.labels?.[BACKUP_SCHEDULE_LABEL]
        ? "Scheduled"
        : "Manual",
      started: dateTime(created),
      finished: dateTime(status.completionTime),
      duration: duration(created, final ? status.completionTime : now),
      phase,
      tone: phaseTone(phase),
      step: status.step ?? DASH,
      version: status.version ?? DASH,
      details: detailsOf(b),
    };
  });
}

/**
 * restoreRowsOf returns the rows of the restores of cluster: every running
 * restore, then the RECENT_RESTORES newest finished ones. Each group is
 * newest first.
 */
export function restoreRowsOf(
  cluster: ClusterLike,
  restores: readonly RestoreObject[],
): RestoreRow[] {
  const rows = newestFirst(clusterRestores(cluster, restores)).map(
    (r): RestoreRow => {
      const phase = r.status?.phase ?? "Pending";
      const pitr = r.kind === "PointInTimeRestore";
      return {
        kind: r.kind,
        name: r.metadata?.name ?? "",
        label: pitr ? "Point-in-time restore" : "Restore",
        source: pitr
          ? `To ${dateTime(r.spec.timestamp)}`
          : `Backup ${r.spec.backupRef.name}`,
        started: dateTime(r.metadata?.creationTimestamp),
        phase,
        tone: phaseTone(phase),
        running: !FINAL.includes(phase),
        failureMessage: r.status?.failureMessage || undefined,
      };
    },
  );
  return [
    ...rows.filter((r) => r.running),
    ...rows.filter((r) => !r.running).slice(0, RECENT_RESTORES),
  ];
}

/** BucketView is an ObjectStorageConfig by name. location is unset when it is not found. */
export type BucketView = { name: string; location?: string };

/** ScheduleView is one BackupSchedule of the cluster. */
export type ScheduleView = {
  name: string;
  cron: string;
  completed: number;
  failed: number;
  lastBackup?: string;
};

/**
 * RestoreInterval is a span [from, to) that a point-in-time restore can
 * reach. to is unset when the span runs to now. failing marks a to that is
 * where the uploads of the write-ahead log started to fail.
 */
export type RestoreInterval = { from: string; to?: string; failing?: true };

/**
 * ArchiveView is the WAL archive of a DatabaseServer that the operator runs.
 * bucket is unset when the archive is off or its preset is not found. window
 * is set only when the DatabaseServerConfig offers a rollback by the
 * operator. It lists the intervals that a rollback can reach, oldest first,
 * and a time between two intervals is a gap that no restore reaches. An
 * empty window has no restore point.
 */
export type ArchiveView = {
  server: string;
  state: { tone: StatusTone; label: string; message?: string };
  bucket?: BucketView;
  window?: RestoreInterval[];
};

/**
 * DatabaseView is the PostgreSQL server of a cluster. missing names the
 * first object of the chain from the SecondaryStorageConfig that is not
 * found, such as "DatabaseConfig orders-db", and then no other field is set.
 * archive is set only when a DatabaseServer of the operator publishes the
 * DatabaseServerConfig.
 */
export type DatabaseView = {
  missing?: string;
  serverConfig?: string;
  endpoint?: string;
  pitr?: { enabled: boolean; recovery?: string; retentionDays?: number };
  archive?: ArchiveView;
};

/**
 * BackupStorage is the backup storage of a cluster. It is none when the
 * cluster has no backupStorageRef, and unknown when its
 * SecondaryStorageConfig is not found. missing then names that object, as
 * "SecondaryStorageConfig orders-es". repository is set only for
 * elasticsearch, and database only for rdbms.
 */
export type BackupStorage =
  | { kind: "none"; schedules: ScheduleView[] }
  | {
      kind: "elasticsearch" | "rdbms" | "unknown";
      missing?: string;
      bucket: BucketView;
      repository?: string;
      database?: DatabaseView;
      schedules: ScheduleView[];
    };

/**
 * BackupStorageInput holds what backupStorageOf reads. cluster is the live
 * object and spec its merged spec. The namespaced lists hold the objects of
 * the namespace of the cluster. now is the current time.
 */
export type BackupStorageInput = {
  cluster: Resource<"CamundaCluster">;
  spec: CamundaClusterSpec;
  storage: readonly Resource<"SecondaryStorageConfig">[];
  objectStorage: readonly Resource<"ObjectStorageConfig">[];
  databaseConfigs: readonly Resource<"DatabaseConfig">[];
  databaseServerConfigs: readonly Resource<"DatabaseServerConfig">[];
  databaseServers: readonly Resource<"DatabaseServer">[];
  databaseServerPresets: readonly Resource<"DatabaseServerPreset">[];
  schedules: readonly Resource<"BackupSchedule">[];
  now: string;
};

/**
 * bucketLocation writes where an ObjectStorageConfig points, as
 * s3://bucket/path (region r), gs://bucket/path, or
 * azure://account/container/path.
 */
export function bucketLocation(
  config: Resource<"ObjectStorageConfig">,
): string {
  const { s3, gcs, azureBlob } = config.spec;
  const path = (base: string, basePath?: string) =>
    basePath ? `${base}/${basePath}` : base;
  if (config.spec.type === "S3" && s3) {
    const location = path(`s3://${s3.bucketName}`, s3.basePath);
    return s3.region ? `${location} (region ${s3.region})` : location;
  }
  if (config.spec.type === "GCS" && gcs) {
    return path(`gs://${gcs.bucketName}`, gcs.basePath);
  }
  if (azureBlob) {
    return path(
      `azure://${azureBlob.accountName}/${azureBlob.container}`,
      azureBlob.basePath,
    );
  }
  return config.spec.type;
}

/**
 * backupStorageOf follows a cluster to its backup bucket, its snapshot
 * repository or its PostgreSQL server, and its schedules. A reference to an
 * object that is not found gives its name without the object, never an
 * error. The schedules are sorted by name.
 */
export function backupStorageOf(input: BackupStorageInput): BackupStorage {
  const name = input.cluster.metadata?.name;
  const schedules = input.schedules
    .filter((s) => s.spec.clusterRef.name === name)
    .map((s): ScheduleView => ({
      name: s.metadata?.name ?? "",
      cron: s.spec.schedule,
      // The defaults of RetainedBackups in api/v1.
      completed: s.spec.retained?.completed ?? 7,
      failed: s.spec.retained?.failed ?? 3,
      lastBackup: s.status?.lastBackupName,
    }))
    .sort((a, b) => a.name.localeCompare(b.name));
  const ref = input.spec.backupStorageRef;
  if (!ref) return { kind: "none", schedules };

  const storage = byName(input.storage, input.spec.storageRef);
  const bucket = bucketOf(input.objectStorage, ref);
  switch (storage?.spec.type) {
    case "elasticsearch":
      return {
        kind: "elasticsearch",
        bucket,
        repository:
          input.cluster.status?.management?.backupRepository ||
          storage.spec.elasticsearch?.snapshotRepository ||
          undefined,
        schedules,
      };
    case "rdbms":
      return {
        kind: "rdbms",
        bucket,
        database: databaseOf(input, storage.spec.rdbms?.databaseConfigRef),
        schedules,
      };
    default:
      return {
        kind: "unknown",
        missing: `SecondaryStorageConfig ${input.spec.storageRef ?? ""}`,
        bucket,
        schedules,
      };
  }
}

function databaseOf(
  input: BackupStorageInput,
  databaseConfigName: string | undefined,
): DatabaseView {
  const databaseConfig = byName(input.databaseConfigs, databaseConfigName);
  if (!databaseConfig) {
    return { missing: `DatabaseConfig ${databaseConfigName ?? ""}` };
  }
  const serverName = databaseConfig.spec.serverRef;
  const serverConfig = byName(input.databaseServerConfigs, serverName);
  if (!serverConfig) return { missing: `DatabaseServerConfig ${serverName}` };

  const { host, port, pitr } = serverConfig.spec;
  const view: DatabaseView = {
    serverConfig: serverName,
    endpoint: `${host}:${port}`,
    pitr: {
      enabled: pitr?.enabled ?? false,
      recovery: pitr?.recovery,
      retentionDays: pitr?.retentionPeriodDays,
    },
  };
  // The operator rejects a second server that asks for the same config
  // (contractTaken in internal/controller/databaseserver), so only the
  // controller of the config publishes it.
  const publisher = serverConfig.metadata?.ownerReferences?.find(
    (r) => r.controller && r.kind === "DatabaseServer",
  );
  const server =
    publisher &&
    input.databaseServers.find((s) => s.metadata?.uid === publisher.uid);
  if (server) {
    const rollback = pitr?.enabled === true && pitr.recovery === "operator";
    view.archive = archiveOf(input, server, rollback);
  }
  return view;
}

function archiveOf(
  input: BackupStorageInput,
  server: Resource<"DatabaseServer">,
  rollback: boolean,
): ArchiveView {
  const view: ArchiveView = {
    server: server.metadata?.name ?? "",
    state: { tone: "muted", label: "Off" },
  };
  const preset = byName(input.databaseServerPresets, server.spec.presetRef);
  if (server.spec.presetRef && !preset) {
    view.state = {
      tone: "bad",
      label: `DatabaseServerPreset "${server.spec.presetRef}" not found`,
    };
    return view;
  }
  const archive = heldArchive(
    server,
    mergeDatabaseServer(preset?.spec, server.spec).spec,
  );
  if (!archive) return view;

  const ready = server.status?.conditions?.find(
    (c) => c.type === "ArchiveReady",
  );
  const message = ready?.message || undefined;
  if (ready?.status === "True") {
    view.state = { tone: "ok", label: "Archiving" };
  } else if (ready?.status === "False") {
    view.state = { tone: "bad", label: ready.reason, message };
  } else {
    view.state = { tone: "warn", label: ready?.reason || "Waiting", message };
  }
  const bucket = archive.objectStorageRef;
  const config = byName(input.objectStorage, bucket);
  const live = config ? archiveLocation(config, server) : "";
  const held = heldLocation(server, live);
  view.bucket = held
    ? { name: bucket, location: held }
    : bucketOf(input.objectStorage, bucket);
  if (rollback) {
    view.window = restoreWindow(
      server,
      bucket,
      held ?? live,
      archive.retentionPeriodDays,
      input.now,
    );
  }
  return view;
}

// heldLocation is the location that an unfinished rollback recorded, when the
// bucket moved under it. The operator keeps the archive there until the
// rollback finishes (recoveryHoldsLocation in internal/controller/databaseserver).
function heldLocation(
  server: Resource<"DatabaseServer">,
  live: string,
): string | undefined {
  const running = server.status?.recovery;
  const recorded = running?.archive?.location;
  if (running?.completedAt || !recorded || !live || recorded === live) {
    return undefined;
  }
  return recorded;
}

// heldArchive is the archive that the operator runs: while a rollback is
// unfinished, it keeps the archive that status.recovery recorded against an
// edit of the spec (recoveryHoldsSpec and heldArchive in
// internal/controller/databaseserver).
function heldArchive(
  server: Resource<"DatabaseServer">,
  spec: DatabaseServerSpec,
): DatabaseServerSpec["archive"] {
  const running = server.status?.recovery;
  const recorded = running?.archive;
  if (!recorded || running.completedAt) return spec.archive;
  const contractHeld =
    !!running.contract && running.contract !== spec.databaseServerConfig;
  if (!spec.archive && !recorded.retentionPeriodDays && !contractHeld) {
    return undefined;
  }
  return {
    objectStorageRef: recorded.objectStorageRef,
    retentionPeriodDays:
      recorded.retentionPeriodDays || spec.archive?.retentionPeriodDays || 0,
    baseBackupSchedule:
      recorded.baseBackupSchedule || spec.archive?.baseBackupSchedule,
  };
}

// restoreWindow follows the operator: SelectArchive in
// pkg/components/databaseserver picks the record and refuses one at another
// location, and outOfReach in internal/controller/databaseserver refuses a
// point before the retention period or reachableFrom.
function restoreWindow(
  server: Resource<"DatabaseServer">,
  bucket: string,
  location: string,
  retentionDays: number,
  now: string,
): RestoreInterval[] {
  const archive = server.status?.archive;
  const nowMs = parse(now) ?? Date.now();
  const cutoff = nowMs - retentionDays * 86_400_000;
  const reachable = parse(archive?.reachableFrom);
  const [floor, floorText] =
    reachable !== undefined && reachable > cutoff
      ? [reachable, archive!.reachableFrom!]
      : [cutoff, new Date(cutoff).toISOString()];
  const window: RestoreInterval[] = [];
  for (const record of archive?.history ?? []) {
    const ref = record.objectStorageRef || bucket;
    const at = record.location || (ref === bucket ? location : "");
    const recordFrom = parse(record.from);
    if (!at || at !== location || recordFrom === undefined) continue;
    const from = recordFrom >= floor ? record.from : floorText;
    // The API says a restore cannot reach past unverifiedFrom.
    const interval: RestoreInterval = record.to
      ? { from, to: record.to }
      : record.unverifiedFrom
        ? { from, to: record.unverifiedFrom, failing: true }
        : { from };
    const end = interval.to ? (parse(interval.to) ?? -Infinity) : nowMs;
    if (Math.max(recordFrom, floor) < end) window.push(interval);
  }
  return window;
}

/**
 * archiveLocation returns where the operator writes the archive of server
 * in config, as ArchiveLocation in pkg/components/databaseserver writes it.
 * status.archive.history records hold locations of this form.
 */
export function archiveLocation(
  config: Resource<"ObjectStorageConfig">,
  server: Resource<"DatabaseServer">,
): string {
  const { namespace = "", name = "", uid = "" } = server.metadata ?? {};
  const segment = `${name}-${bytesToHex(sha256(utf8ToBytes(uid))).slice(0, 8)}`;
  return locationOf(config, `databaseserver/${namespace}/${segment}`);
}

// locationOf is LocationOf of ObjectStorageConfig in api/v1.
function locationOf(
  config: Resource<"ObjectStorageConfig">,
  path: string,
): string {
  const trim = (text: string) => text.replace(/^\/+|\/+$/g, "");
  const trimRight = (text: string) => text.replace(/\/+$/, "");
  const { type, s3, gcs, azureBlob } = config.spec;
  const basePath =
    type === "S3"
      ? s3?.basePath
      : type === "GCS"
        ? gcs?.basePath
        : azureBlob?.basePath;
  const base = trim(`${trim(basePath ?? "")}/${trim(path)}`);
  if (type === "S3" && s3) {
    const endpoint = trimRight(s3.endpoint ?? "");
    const service = endpoint
      ? `endpoint ${endpoint}`
      : `region ${s3.region ?? ""}`;
    return `s3://${s3.bucketName}/${base} (${service})`;
  }
  if (type === "GCS" && gcs) return `gs://${gcs.bucketName}/${base}`;
  if (type === "AzureBlob" && azureBlob) {
    const endpoint =
      trimRight(azureBlob.endpoint ?? "") ||
      `https://${azureBlob.accountName}.blob.core.windows.net`;
    return `azblob://${azureBlob.accountName}/${azureBlob.container}/${base} (endpoint ${endpoint})`;
  }
  return "";
}

function bucketOf(
  objectStorage: readonly Resource<"ObjectStorageConfig">[],
  name: string,
): BucketView {
  const config = byName(objectStorage, name);
  return config ? { name, location: bucketLocation(config) } : { name };
}

function byName<T extends { metadata?: { name?: string } }>(
  objects: readonly T[],
  name: string | undefined,
): T | undefined {
  return name ? objects.find((o) => o.metadata?.name === name) : undefined;
}

function detailsOf(backup: BackupObject): Detail[] {
  const status = backup.status ?? {};
  const es =
    backup.kind === "LogicalBackupElasticsearch" ? backup.status : undefined;
  const rdbms =
    backup.kind === "LogicalBackupRDBMS" ? backup.status : undefined;
  const sizes = [
    ["Zeebe", status.storageSizes?.zeebe],
    ["Elasticsearch", status.storageSizes?.elasticsearch],
  ]
    .filter(([, size]) => size !== undefined && size !== "")
    .map(([name, size]) => `${name} ${quantity(size!)}`)
    .join(", ");
  const details: [string, string | number | undefined][] = [
    // backupId and partitionsCount are omitempty in Go, so 0 means unset.
    ["Backup ID", status.backupId || undefined],
    ["Object key", rdbms?.objectKey],
    ["Zeebe backup ID", rdbms?.zeebeBackupId],
    ["Repository", es?.repository],
    ["Partitions", es?.partitionsCount || undefined],
    ["Restore sizes", sizes],
    ["Failure", status.failureMessage],
    ["Resume failure", es?.resumeFailureMessage],
  ];
  return details
    .filter(([, value]) => value !== undefined && value !== "")
    .map(([label, value]) => ({ label, value: String(value) }));
}

function quantity(size: number | string): string {
  return typeof size === "number" ? formatBytes(size) : size;
}

function phaseTone(phase: string): StatusTone {
  if (phase === "Completed") return "ok";
  if (phase === "Failed") return "bad";
  return "warn";
}

function newestFirst<
  T extends { metadata?: { name?: string; creationTimestamp?: string } },
>(objects: readonly T[]): T[] {
  return [...objects].sort(
    (a, b) =>
      (parse(b.metadata?.creationTimestamp) ?? 0) -
        (parse(a.metadata?.creationTimestamp) ?? 0) ||
      (a.metadata?.name ?? "").localeCompare(b.metadata?.name ?? ""),
  );
}

function parse(iso: string | undefined): number | undefined {
  if (!iso) return undefined;
  const ms = Date.parse(iso);
  return Number.isNaN(ms) ? undefined : ms;
}
