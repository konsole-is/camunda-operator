"use client";

import type { ReactNode } from "react";
import { namesIn } from "@/lib/claims/names";
import { runningBackup, runningRestore } from "@/lib/policy/backups";
import { type BackupContext, clusterSuspended } from "./backup-actions";
import { BackupHeaderActions, BackupRowActions } from "./backup-actions-view";
import type { BackupRow } from "./backups";
import type { Change, WriteResult } from "@/lib/write/types";
import { useClusterEdit } from "./edit/cluster-edit";
import type { ClusterSlotContext, ClusterSlots } from "./slots";

/**
 * backupContextOf reads the BackupContext of the backup actions from the
 * slot context. running is the running backup of the cluster, else its
 * running restore.
 */
export function backupContextOf(context: ClusterSlotContext): BackupContext {
  const { cluster, backups, restores } = context;
  const namespace = cluster.metadata?.namespace ?? "";
  const backup = runningBackup(cluster, backups);
  const restore = runningRestore(cluster, restores);
  const running = backup
    ? { what: "Backup" as const, name: backup.name }
    : restore
      ? { what: "Restore" as const, name: restore.name }
      : undefined;
  return {
    namespace,
    cluster: cluster.metadata?.name ?? "",
    storage: context.backupStorage,
    suspended: clusterSuspended(cluster),
    ...(running ? { running } : {}),
    backupNames: namesIn(namespace, backups),
    now: context.now,
  };
}

type Apply = (change: Change) => Promise<WriteResult>;

/**
 * backupSlots fills cluster.backups.actions with Back up now and Change
 * schedule, and cluster.backups.row with Delete. Each write goes through
 * apply, the server action of backup changes. The slots need a
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
