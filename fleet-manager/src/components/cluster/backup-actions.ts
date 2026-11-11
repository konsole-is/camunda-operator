import { apiVersionOf, KindMap } from "@/lib/k8s/kinds";
import { isDNSSubdomain, SUSPENSION_HOLD_PREFIX } from "@/lib/labels";
import type { BackupKind } from "@/lib/policy/backups";
import { refKey } from "@/lib/write/refs";
import type { Change } from "@/lib/write/types";
import type { BackupRow, BackupStorage } from "./backups";

/** DIRECT_NOTE says that the backup and restore writes skip Git. */
export const DIRECT_NOTE =
  "Backups and restores go straight to the cluster, also in Git mode.";

/**
 * BackupContext is what the backup actions of one cluster read. suspended
 * is clusterSuspended of the cluster. running is the backup or restore of
 * the cluster that runs, a Pending one included. backupNames holds the
 * name of every backup of the namespace, of both kinds. now is an RFC 3339
 * time.
 */
export type BackupContext = {
  namespace: string;
  cluster: string;
  storage: BackupStorage;
  suspended: boolean;
  running?: { what: "Backup" | "Restore"; name: string };
  backupNames: string[];
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
 * clusterSuspended says whether a CamundaCluster is suspended: spec.suspend
 * is true, or an annotation key starts with SUSPENSION_HOLD_PREFIX.
 */
export function clusterSuspended(cluster: {
  spec?: { suspend?: boolean };
  metadata?: { annotations?: Record<string, string> };
}): boolean {
  return (
    cluster.spec?.suspend === true ||
    Object.keys(cluster.metadata?.annotations ?? {}).some((key) =>
      key.startsWith(SUSPENSION_HOLD_PREFIX),
    )
  );
}

/**
 * BackUpNow is the state of the Back up now action. disabled is the reason
 * when the action is off, and then kind is unset. Else kind is the backup
 * kind to create, and waits is the note that the new backup waits behind
 * the running operation.
 */
export type BackUpNow = { kind?: BackupKind; disabled?: string; waits?: string };

/**
 * backUpNow gives the state of Back up now. It is off without backup
 * storage, with storage of an unknown kind, and on a suspended cluster. A
 * running restore suspends its cluster, so a suspension while a restore
 * runs does not turn the action off.
 */
export function backUpNow(context: BackupContext): BackUpNow {
  const { storage, suspended, running } = context;
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
  if (suspended && running?.what !== "Restore") {
    return {
      disabled:
        "The cluster is suspended. A backup waits until it runs again, so resume the cluster first.",
    };
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
        apiVersion: apiVersionOf(KindMap[kind]),
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
        apiVersion: apiVersionOf(KindMap[row.kind]),
        kind: row.kind,
        namespace,
        name: row.name,
      },
    ],
    directOnly: true,
  };
}

/** deleteLines gives the lines of the delete confirm of row: what goes with the backup, and what stays. */
export function deleteLines(row: BackupRow): {
  goes: string[];
  stays: string[];
} {
  const stays: string[] = [];
  let goes: string;
  if (row.kind === "LogicalBackupElasticsearch") {
    goes = `Backup ${row.name}, with its Elasticsearch snapshots and its Zeebe partition backup in the bucket.`;
  } else {
    goes = `Backup ${row.name}, with its database dump in the bucket.`;
    stays.push(
      "The Zeebe backups. Zeebe removes them after the retention window of the cluster.",
    );
  }
  if (row.origin === "Scheduled") {
    stays.push(
      "The other backups of the schedule. The schedule deletes the oldest of them beyond the counts it keeps.",
    );
  }
  return { goes: [goes], stays };
}
