import { isDNSSubdomain, SUSPENSION_HOLD_PREFIX } from "@/lib/labels";
import type { BackupKind } from "@/lib/policy/backups";
import { parseVersion } from "@/lib/policy/release";
import { refKey } from "@/lib/write/refs";
import type { Change } from "@/lib/write/types";
import type { BackupRow, BackupStorage, RestoreInterval } from "./backups";

// The backup and restore kinds are all in this group and version. The kind
// table of src/lib/k8s is server only, and this module runs in the browser.
const CORE_V1 = "core.camunda.io/v1";

/** DIRECT_NOTE says that the backup and restore writes skip Git. */
export const DIRECT_NOTE =
  "Backups and restores go straight to the cluster, also in Git mode.";

/**
 * BackupContext is what the backup and restore actions of one cluster read.
 * suspension is suspensionOf the cluster, unset when it runs. running is
 * the operation that a new backup of the cluster waits behind: the backup
 * in phase Running, else an Elasticsearch backup that ended as ResumeFailed
 * (with resumeFailed set), which holds the cluster until it is deleted,
 * else the restore that runs, a Pending one included, else a Pending
 * backup. restoring is the name of the restore of the cluster that is not
 * Completed or Failed. backupNames holds the name of every backup of the
 * namespace, of both kinds, and restoreNames the name of every restore, of
 * the three kinds. readingRestores holds each logical restore of the
 * namespace that is not Completed or Failed, with the kind and the name of
 * the backup that it reads. clusterVersion is clusterVersionOf the
 * cluster. retentionWindow is spec.backup.primaryStorage.retention.window
 * of the merged spec. changesOff is the reason that the page cannot write,
 * and unset when it can. now is an RFC 3339 time.
 */
export type BackupContext = {
  namespace: string;
  cluster: string;
  storage: BackupStorage;
  suspension?: Suspension;
  running?: { what: "Backup" | "Restore"; name: string; resumeFailed?: true };
  restoring?: string;
  backupNames: string[];
  restoreNames: string[];
  readingRestores: { name: string; kind: BackupKind; backup: string }[];
  clusterVersion?: string;
  retentionWindow?: string;
  changesOff?: string;
  now: string;
};

/** backupKindOf gives the kind of a backup on storage, or undefined for none and unknown storage. */
export function backupKindOf(storage: BackupStorage): BackupKind | undefined {
  if (storage.kind === "elasticsearch") return "LogicalBackupElasticsearch";
  if (storage.kind === "rdbms") return "LogicalBackupRDBMS";
  return undefined;
}

/** stamp writes the RFC 3339 time now in UTC as "20261110-143005". */
export function stamp(now: string): string {
  const text = new Date(now).toISOString();
  return `${text.slice(0, 10).replaceAll("-", "")}-${text.slice(11, 19).replaceAll(":", "")}`;
}

/** manualBackupName gives the name that Back up now offers: "<cluster>-manual-<stamp of now>". */
export function manualBackupName(cluster: string, now: string): string {
  return `${cluster}-manual-${stamp(now)}`;
}

/**
 * backupNameErrors returns every error of name as the name of a new backup,
 * or none. The name must be a DNS-1123 subdomain and not in taken. An
 * empty name gives only "Enter a name".
 */
export function backupNameErrors(
  name: string,
  taken: readonly string[],
): string[] {
  if (name === "") return ["Enter a name"];
  const errors: string[] = [];
  if (!isDNSSubdomain(name)) {
    errors.push(
      "Use only lowercase letters, digits, '-', and '.'. Start and end with a letter or a digit.",
    );
  }
  if (taken.includes(name)) {
    errors.push(`A backup named ${name} exists. Choose another name.`);
  }
  return errors;
}

/**
 * Suspension is why a CamundaCluster is suspended. held is a suspension
 * hold annotation, requested is spec.suspend, and the other two are the
 * Ready reasons with which the operator keeps the cluster off its
 * secondary storage.
 */
export type Suspension =
  "held" | "requested" | "StorageAlreadyAttached" | "WaitingForHandover";

/**
 * suspensionOf gives why a CamundaCluster is suspended, as
 * CamundaCluster.Suspended in api/v1 counts it, or undefined when it is
 * not. A hold comes before spec.suspend, because the cluster stays
 * suspended while it has a hold, whatever spec.suspend says.
 */
export function suspensionOf(cluster: {
  spec?: { suspend?: boolean };
  metadata?: { annotations?: Record<string, string> };
  status?: { conditions?: { type: string; reason?: string }[] };
}): Suspension | undefined {
  const held = Object.keys(cluster.metadata?.annotations ?? {}).some((key) =>
    key.startsWith(SUSPENSION_HOLD_PREFIX),
  );
  if (held) return "held";
  if (cluster.spec?.suspend === true) return "requested";
  const reason = cluster.status?.conditions?.find(
    (c) => c.type === "Ready",
  )?.reason;
  return reason === "StorageAlreadyAttached" || reason === "WaitingForHandover"
    ? reason
    : undefined;
}

const SUSPENDED: Record<Suspension, string> = {
  requested:
    "The cluster is suspended. A backup waits until it runs again, so resume the cluster first.",
  held: "A suspension hold keeps the cluster suspended, and a backup waits until the cluster runs again. A failed restore keeps its hold until you delete that restore.",
  StorageAlreadyAttached:
    "Another cluster holds the secondary storage of this cluster, so the operator keeps this cluster suspended, and a backup waits until it runs again. Give this cluster storage of its own, or delete the cluster that holds the storage.",
  WaitingForHandover:
    "Something else still writes the secondary storage of this cluster, so the operator keeps this cluster suspended, and a backup waits until it runs again. Wait, or remove what the Ready condition of the cluster names.",
};

/**
 * BackUpNow is the state of the Back up now action. disabled is the reason
 * when the action is off, and then kind is unset. Else kind is the backup
 * kind to create, and waits is the note that the new backup waits behind
 * the running operation.
 */
export type BackUpNow = {
  kind?: BackupKind;
  disabled?: string;
  waits?: string;
};

/**
 * backUpNow gives the state of Back up now. It is off when the page cannot
 * write, without backup storage, with storage of an unknown kind, and on a
 * suspended cluster, with the remedy of the suspension. A running restore
 * suspends its cluster, so a suspension while a restore runs does not turn
 * the action off.
 */
export function backUpNow(context: BackupContext): BackUpNow {
  const { storage, suspension, running, changesOff } = context;
  if (changesOff !== undefined) return { disabled: changesOff };
  if (storage.kind === "none") {
    return { disabled: "This cluster has no backup storage." };
  }
  const kind = backupKindOf(storage);
  if (kind === undefined) {
    const missing =
      ("missing" in storage && storage.missing) || "The secondary storage";
    return {
      disabled: `${missing} not found, so the kind of backup is not known.`,
    };
  }
  if (suspension !== undefined && running?.what !== "Restore") {
    return { disabled: SUSPENDED[suspension] };
  }
  const waits = waitNote(running, "backup");
  return waits === undefined ? { kind } : { kind, waits };
}

/** waitNote says that a new backup or restore waits behind running, or gives undefined when nothing runs. */
export function waitNote(
  running: BackupContext["running"],
  what: "backup" | "restore",
): string | undefined {
  if (running === undefined) return undefined;
  if (running.resumeFailed) {
    return `Backup ${running.name} ended as ResumeFailed and still holds the cluster. The new ${what} waits until you delete that backup.`;
  }
  return `${running.what} ${running.name} is running. The new ${what} waits until it ends.`;
}

/**
 * backupChange is the change of Back up now: one new backup of kind named
 * name for the cluster of context, straight to the cluster.
 */
export function backupChange(
  context: BackupContext,
  kind: BackupKind,
  name: string,
): Change {
  const { namespace, cluster } = context;
  return {
    intent: `Back up ${cluster}`,
    apply: [
      {
        apiVersion: CORE_V1,
        kind,
        metadata: { name, namespace },
        spec: { clusterRef: { name: cluster } },
      },
    ],
    create: [refKey({ kind, namespace, name })],
    directOnly: true,
  };
}

/** deleteBackupChange is the change that deletes the backup of row in namespace, straight to the cluster. */
export function deleteBackupChange(namespace: string, row: BackupRow): Change {
  return {
    intent: `Delete backup ${row.name}`,
    apply: [],
    delete: [
      {
        apiVersion: CORE_V1,
        kind: row.kind,
        namespace,
        name: row.name,
      },
    ],
    directOnly: true,
  };
}

/**
 * deleteBlock gives the reason that Delete is off for row, or undefined.
 * Delete is off when the page cannot write, and while a restore in
 * readingRestores reads the backup of row.
 */
export function deleteBlock(
  context: BackupContext,
  row: BackupRow,
): string | undefined {
  if (context.changesOff !== undefined) return context.changesOff;
  const reader = context.readingRestores.find(
    (r) => r.kind === row.kind && r.backup === row.name,
  );
  return reader
    ? `Restore ${reader.name} reads this backup. Delete the backup after the restore ends.`
    : undefined;
}

/** deleteLines gives the lines of the delete confirm of row: what goes with the backup, and what stays. */
export function deleteLines(row: BackupRow): {
  goes: string[];
  stays: string[];
} {
  // The delete of a backup that is not Completed or Failed also stops its run.
  const runs = row.phase !== "Completed" && row.phase !== "Failed";
  const goes: string[] = [];
  const stays: string[] = [];
  if (row.kind === "LogicalBackupElasticsearch") {
    goes.push(
      `Backup ${row.name}, with its Elasticsearch snapshots and its Zeebe partition backup in the bucket.`,
    );
    if (runs) {
      goes.push(
        "The run of the backup. If it paused exporting on the cluster, exporting resumes.",
      );
    }
  } else {
    goes.push(`Backup ${row.name}, with its database dump in the bucket.`);
    if (runs) goes.push("The run of the backup, with its dump Job.");
    stays.push(
      "The Zeebe backups. Zeebe removes them after the retention window of the cluster.",
    );
  }
  if (row.origin === "Scheduled") {
    stays.push(
      "The other backups of the schedule. The schedule deletes the oldest of them beyond the counts it keeps.",
    );
  }
  return { goes, stays };
}

/** LogicalRestoreKind is the kind of a restore from a backup. */
export type LogicalRestoreKind =
  "LogicalRestoreElasticsearch" | "LogicalRestoreRDBMS";

/** restoreKindOf gives the kind of the restore that reads a backup of kind. */
export function restoreKindOf(kind: BackupKind): LogicalRestoreKind {
  return kind === "LogicalBackupElasticsearch"
    ? "LogicalRestoreElasticsearch"
    : "LogicalRestoreRDBMS";
}

/**
 * clusterVersionOf gives the Camunda version of a cluster: the version that
 * it reports, else the version of its merged spec. A suspended cluster
 * reports none, and the operator then compares against the version of the
 * spec, which sets the tag of the broker image.
 */
export function clusterVersionOf(
  cluster: { status?: { management?: { version?: string } } },
  spec: { version?: string },
): string | undefined {
  return cluster.status?.management?.version || spec.version || undefined;
}

/**
 * restoreBlock gives the reason that Restore is off for the Completed
 * backup of row, or undefined. It is off when the page cannot write, while
 * a restore of the cluster has not ended, and when the backup recorded no
 * x.y.z version. A relational backup also needs a cluster on its Camunda
 * minor or one minor newer, in the same major. An Elasticsearch restore
 * sets the version of the backup on the cluster, so it has no such rule.
 */
export function restoreBlock(
  context: BackupContext,
  row: BackupRow,
): string | undefined {
  if (context.changesOff !== undefined) return context.changesOff;
  if (context.restoring !== undefined) return restoringNote(context.restoring);
  const backup = parseVersion(row.version);
  if (backup === undefined) {
    return `Backup ${row.name} recorded no Camunda version, so it cannot be restored.`;
  }
  if (row.kind === "LogicalBackupElasticsearch") return undefined;
  const cluster = parseVersion(context.clusterVersion);
  if (cluster === undefined) {
    return "The cluster has no Camunda version yet, so the version rule cannot be checked.";
  }
  const [major, minor] = backup;
  if (
    cluster[0] === major &&
    (cluster[1] === minor || cluster[1] === minor + 1)
  ) {
    return undefined;
  }
  return `Backup ${row.name} has Camunda ${row.version}. It restores only into Camunda ${major}.${minor} or ${major}.${minor + 1}, and the cluster has ${context.clusterVersion}.`;
}

function restoringNote(restore: string): string {
  return `Restore ${restore} of this cluster has not ended. Start a new restore after it ends.`;
}

/** freeName gives base when taken does not hold it, else the first of base-2, base-3, … that is free. */
export function freeName(base: string, taken: readonly string[]): string {
  let name = base;
  for (let n = 2; taken.includes(name); n++) name = `${base}-${n}`;
  return name;
}

/**
 * restoreNameOf gives the name of a new restore of the cluster:
 * "<cluster>-<what>-<stamp of now>", made free among restoreNames.
 */
export function restoreNameOf(
  context: BackupContext,
  what: "restore" | "pitr",
): string {
  return freeName(
    `${context.cluster}-${what}-${stamp(context.now)}`,
    context.restoreNames,
  );
}

/**
 * restoreChange is the change of Restore: one new logical restore named
 * name that restores the backup of row into the cluster of context,
 * straight to the cluster.
 */
export function restoreChange(
  context: BackupContext,
  row: BackupRow,
  name: string,
): Change {
  const { namespace, cluster } = context;
  const kind = restoreKindOf(row.kind);
  return {
    intent: `Restore ${cluster} from backup ${row.name}`,
    apply: [
      {
        apiVersion: CORE_V1,
        kind,
        metadata: { name, namespace },
        spec: {
          backupRef: { name: row.name },
          targetClusterRef: { name: cluster },
        },
      },
    ],
    create: [refKey({ kind, namespace, name })],
    directOnly: true,
  };
}

/** RestoreConfirm is the text of a restore confirm: a lead sentence and the lines of a list. */
export type RestoreConfirm = { lead: string; lines: string[] };

/**
 * restoreConfirm gives the confirm of a restore of the backup of row: what
 * the restore loses, what it deletes, whether the cluster starts again
 * after it, and what a failed restore leaves.
 */
export function restoreConfirm(
  context: BackupContext,
  row: BackupRow,
): RestoreConfirm {
  const es = row.kind === "LogicalBackupElasticsearch";
  const cluster = context.clusterVersion
    ? `The cluster has ${context.clusterVersion}.`
    : "The cluster reports no Camunda version yet.";
  const volumes =
    "It also deletes the broker volumes and creates them again from the Zeebe data of the backup.";
  return {
    lead: `The cluster goes back to backup ${row.name}. Everything that it wrote after that backup is lost.`,
    lines: [
      `Backup ${row.name} has Camunda ${row.version}. ${cluster} The restore sets the cluster to ${row.version}.`,
      es
        ? `The restore deletes the Camunda indices in Elasticsearch and restores the snapshots of the backup. ${volumes}`
        : `The restore runs pg_restore over the database of the cluster, which drops each table of the dump and creates it again. ${volumes}`,
      startsAgain(context.suspension),
      failed(
        es
          ? "the Camunda indices and the broker volumes"
          : "the database and the broker volumes",
      ),
    ],
  };
}

/**
 * pointInTimeConfirm gives the confirm of a restore to at, the point as
 * the page shows it, with the facts of restoreConfirm.
 */
export function pointInTimeConfirm(
  context: BackupContext,
  at: string,
): RestoreConfirm {
  return {
    lead: `The cluster goes back to ${at}. Everything that it wrote after that time is lost.`,
    lines: [
      "The database server rolls back to that time. The restore also deletes the broker volumes and creates them again from the Zeebe backup that covers the point.",
      startsAgain(context.suspension),
      "If no Zeebe backup covers the point, the restore fails after it erased the broker volumes.",
      failed("the broker volumes"),
    ],
  };
}

// A restore lifts only the suspension that it applied, or one that an
// earlier restore left, so a cluster that its owner suspended stays
// suspended.
function startsAgain(suspension: Suspension | undefined): string {
  if (suspension === undefined) {
    return "The restore suspends the cluster while it runs, and starts it again when it completes.";
  }
  if (suspension === "requested") {
    return "The cluster is suspended now. If its owner suspended it, it stays suspended after the restore, and the owner resumes it.";
  }
  return "Something else keeps the cluster suspended now. It stays suspended after the restore until that suspension ends.";
}

function failed(what: string): string {
  return `If the restore fails, ${what} can be empty or half written. Then the cluster has no usable data, and it stays suspended. Do not resume it. Correct the cause and restore again, as "When the cluster starts again" in the Operations guide shows.`;
}

const DURATION =
  /^P(?:(\d+)D)?(?:T(?:(\d+)H)?(?:(\d+)M)?(?:(\d+(?:\.\d+)?)S)?)?$/;

/**
 * retentionMs reads an ISO 8601 duration of days and time, such as P7D or
 * PT12H, in milliseconds. It gives undefined for any other text, a week,
 * month, or year unit included, because Camunda reads none of them.
 */
export function retentionMs(window: string): number | undefined {
  const match = DURATION.exec(window);
  if (!match || match.slice(1).every((n) => n === undefined)) return undefined;
  if (window.endsWith("T")) return undefined;
  const [days, hours, minutes, seconds] = match
    .slice(1)
    .map((n) => Number(n ?? 0));
  return (
    ((days * 24 + hours) * 60 + minutes) * 60_000 + Math.round(seconds * 1000)
  );
}

/** ZEEBE_WINDOW is the default of spec.backup.primaryStorage.retention.window. */
const ZEEBE_WINDOW = "P7D";

/**
 * PointInTimeWindow is the span that a point-in-time restore can reach,
 * as intervals of RestoreInterval, oldest first, or the reason that no
 * point can be offered.
 */
export type PointInTimeWindow =
  { intervals: RestoreInterval[] } | { unavailable: string };

/**
 * pointInTimeWindow cuts archive, the window of the WAL archive, to the
 * retention window in which Zeebe keeps its backups (P7D when unset) and
 * to now. An interval keeps its failing mark while its end stays. It
 * gives unavailable for a retention window that is not days and time, for
 * an empty archive, and when no interval is left.
 */
export function pointInTimeWindow(
  archive: readonly RestoreInterval[],
  retentionWindow: string | undefined,
  now: string,
): PointInTimeWindow {
  const window = retentionWindow ?? ZEEBE_WINDOW;
  const keep = retentionMs(window);
  if (keep === undefined) {
    return {
      unavailable: `The cluster keeps its Zeebe backups for ${window}, which the fleet manager cannot read, so it cannot offer a point.`,
    };
  }
  if (archive.length === 0) {
    return { unavailable: "The archive has no restore point yet." };
  }
  const nowMs = Date.parse(now);
  const floor = nowMs - keep;
  const intervals = archive.flatMap((interval): RestoreInterval[] => {
    const from = Date.parse(interval.from);
    const to = interval.to === undefined ? nowMs : Date.parse(interval.to);
    if (Math.max(from, floor) >= Math.min(to, nowMs)) return [];
    const cut: RestoreInterval = {
      from: from >= floor ? interval.from : new Date(floor).toISOString(),
    };
    if (interval.to !== undefined && to <= nowMs) {
      cut.to = interval.to;
      if (interval.failing) cut.failing = true;
    }
    return [cut];
  });
  return intervals.length > 0
    ? { intervals }
    : {
        unavailable: `No point is inside both the archive window and the ${window} in which Zeebe keeps its backups.`,
      };
}

/**
 * timestampOf reads the value of a datetime-local field as a time in UTC,
 * and gives it as RFC 3339 to the second, or undefined when it is no time.
 */
export function timestampOf(local: string): string | undefined {
  const ms = Date.parse(`${local}Z`);
  if (local === "" || Number.isNaN(ms)) return undefined;
  return `${new Date(ms).toISOString().slice(0, 19)}Z`;
}

/**
 * pointInTimeError gives the error of the point at, an RFC 3339 time or
 * undefined, or undefined when a restore can reach it. A point is inside
 * an interval from its from up to its to, which it excludes, or up to now
 * when the interval has no to.
 */
export function pointInTimeError(
  at: string | undefined,
  intervals: readonly RestoreInterval[],
  now: string,
): string | undefined {
  const t = at === undefined ? NaN : Date.parse(at);
  if (Number.isNaN(t)) return "Enter a date and time.";
  const nowMs = Date.parse(now);
  if (t > nowMs) return "Choose a point in the past.";
  const inside = intervals.some(
    (i) =>
      Date.parse(i.from) <= t &&
      (i.to === undefined ? t <= nowMs : t < Date.parse(i.to)),
  );
  return inside ? undefined : "Choose a point inside the restore window.";
}

/**
 * PointInTime is the state of Restore to a point in time: the intervals
 * that it can reach, or the reason that it is off.
 */
export type PointInTime =
  { intervals: RestoreInterval[] } | { disabled: string };

/**
 * pointInTime gives the state of Restore to a point in time, or undefined
 * when the cluster has no such action: it is offered only on PostgreSQL
 * whose database server the operator rolls back, which the window of the
 * archive shows. It is off when the page cannot write, while a restore of
 * the cluster has not ended, and with the reason of pointInTimeWindow.
 */
export function pointInTime(context: BackupContext): PointInTime | undefined {
  const { storage } = context;
  const archive =
    storage.kind === "rdbms" ? storage.database?.archive?.window : undefined;
  if (archive === undefined) return undefined;
  if (context.changesOff !== undefined) {
    return { disabled: context.changesOff };
  }
  if (context.restoring !== undefined) {
    return { disabled: restoringNote(context.restoring) };
  }
  const window = pointInTimeWindow(
    archive,
    context.retentionWindow,
    context.now,
  );
  return "unavailable" in window ? { disabled: window.unavailable } : window;
}

/**
 * pointInTimeChange is the change of Restore to a point in time: one new
 * PointInTimeRestore named name that rolls the cluster of context back to
 * timestamp, straight to the cluster.
 */
export function pointInTimeChange(
  context: BackupContext,
  name: string,
  timestamp: string,
): Change {
  const { namespace, cluster } = context;
  const kind = "PointInTimeRestore";
  return {
    intent: `Restore ${cluster} to ${timestamp}`,
    apply: [
      {
        apiVersion: CORE_V1,
        kind,
        metadata: { name, namespace },
        spec: { clusterRef: { name: cluster }, timestamp },
      },
    ],
    create: [refKey({ kind, namespace, name })],
    directOnly: true,
  };
}
