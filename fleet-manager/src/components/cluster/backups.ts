import type { StatusTone } from "@/components/fleet/status";
import { formatBytes } from "@/components/fleet/units";
import type { Resource } from "@/lib/k8s/kinds";
import { BACKUP_SCHEDULE_LABEL } from "@/lib/labels";
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
