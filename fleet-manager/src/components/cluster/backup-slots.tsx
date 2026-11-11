"use client";

import type { ReactNode } from "react";
import { namesIn } from "@/lib/claims/names";
import {
  clusterBackups,
  runningBackup,
  runningRestore,
} from "@/lib/policy/backups";
import { type BackupContext, clusterSuspended } from "./backup-actions";
import { BackupHeaderActions, BackupRowActions } from "./backup-actions-view";
import type { BackupRow } from "./backups";
import type { Change, WriteResult } from "@/lib/write/types";
import { CHANGES_OFF, useClusterEdit } from "./edit/cluster-edit";
import type { ClusterSlotContext, ClusterSlots } from "./slots";

/**
 * backupContextOf reads the BackupContext of the backup actions from the
 * slot context. changesOff is CHANGES_OFF when the page has no write mode.
 */
export function backupContextOf(context: ClusterSlotContext): BackupContext {
  const { cluster, backups, restores } = context;
  const namespace = cluster.metadata?.namespace ?? "";
  const running = holderOf(context);
  return {
    namespace,
    cluster: cluster.metadata?.name ?? "",
    storage: context.backupStorage,
    suspended: clusterSuspended(cluster),
    ...(running ? { running } : {}),
    backupNames: namesIn(namespace, backups),
    readingRestores: restores.flatMap((r) => {
      const phase = r.status?.phase;
      if (phase === "Completed" || phase === "Failed") return [];
      if (r.kind === "PointInTimeRestore") return [];
      const kind = READ_KIND[r.kind];
      return [
        { name: r.metadata?.name ?? "", kind, backup: r.spec.backupRef.name },
      ];
    }),
    ...(context.writeMode === undefined ? { changesOff: CHANGES_OFF } : {}),
    now: context.now,
  };
}

// READ_KIND is the kind of the backup that a logical restore reads.
const READ_KIND = {
  LogicalRestoreElasticsearch: "LogicalBackupElasticsearch",
  LogicalRestoreRDBMS: "LogicalBackupRDBMS",
} as const;

// holderOf gives the running field of BackupContext. runningBackup counts
// only the backups that run, so it misses a backup that ended as
// ResumeFailed.
function holderOf(context: ClusterSlotContext): BackupContext["running"] {
  const { cluster, backups, restores } = context;
  const backup = runningBackup(cluster, backups);
  if (backup) return { what: "Backup", name: backup.name };
  const [held] = clusterBackups(cluster, backups)
    .filter(
      (b) =>
        b.kind === "LogicalBackupElasticsearch" &&
        b.status?.terminalReason === "ResumeFailed",
    )
    .map((b) => b.metadata?.name ?? "")
    .sort();
  if (held !== undefined) {
    return { what: "Backup", name: held, resumeFailed: true };
  }
  const restore = runningRestore(cluster, restores);
  return restore ? { what: "Restore", name: restore.name } : undefined;
}

type Apply = (
  cluster: { namespace: string; name: string },
  change: Change,
) => Promise<WriteResult>;

/**
 * backupSlots fills cluster.backups.actions with Back up now and Change
 * schedule, and cluster.backups.row with Delete. Each write goes through
 * apply, the server action of backup changes, with the namespace and the
 * name of the cluster of the page. The slots need a
 * ClusterEditProvider above them, for the toast of the page.
 */
export function backupSlots(apply: Apply) {
  return {
    "cluster.backups.actions": (context: ClusterSlotContext): ReactNode => (
      <BackupHeaderSlot context={context} apply={apply} />
    ),
    "cluster.backups.row": (
      context: ClusterSlotContext,
      row: BackupRow,
    ): ReactNode => <BackupRowSlot context={context} row={row} apply={apply} />,
  } satisfies ClusterSlots;
}

function BackupHeaderSlot({
  context,
  apply,
}: {
  context: ClusterSlotContext;
  apply: Apply;
}) {
  const { show } = useClusterEdit();
  return (
    <BackupHeaderActions
      context={backupContextOf(context)}
      apply={apply}
      show={show}
    />
  );
}

function BackupRowSlot({
  context,
  row,
  apply,
}: {
  context: ClusterSlotContext;
  row: BackupRow;
  apply: Apply;
}) {
  const { show } = useClusterEdit();
  return (
    <BackupRowActions
      context={backupContextOf(context)}
      row={row}
      apply={apply}
      show={show}
    />
  );
}
