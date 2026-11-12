"use client";

import type { ReactNode } from "react";
import { namesIn } from "@/lib/claims/names";
import {
  clusterBackups,
  runningBackup,
  runningRestore,
} from "@/lib/policy/backups";
import {
  type BackupContext,
  clusterVersionOf,
  suspensionOf,
} from "./backup-actions";
import { BackupHeaderActions, BackupRowActions } from "./backup-actions-view";
import type { BackupObject, BackupRow } from "./backups";
import type { Change, WriteResult } from "@/lib/write/types";
import { CHANGES_OFF, useClusterEdit } from "./edit/cluster-edit";
import type { ClusterSlotContext, ClusterSlots } from "./slots";

/**
 * backupContextOf reads the BackupContext of the backup and restore actions
 * from the slot context. changesOff is CHANGES_OFF when the page has no
 * write mode.
 */
export function backupContextOf(context: ClusterSlotContext): BackupContext {
  const { cluster, backups, restores } = context;
  const spec = context.merged.spec;
  const namespace = cluster.metadata?.namespace ?? "";
  const running = holderOf(context);
  const suspension = suspensionOf(cluster);
  const restoring = runningRestore(cluster, restores)?.name;
  const clusterVersion = clusterVersionOf(cluster, spec);
  const retentionWindow = spec.backup?.primaryStorage?.retention?.window;
  return {
    namespace,
    cluster: cluster.metadata?.name ?? "",
    storage: context.backupStorage,
    ...(suspension ? { suspension } : {}),
    ...(running ? { running } : {}),
    ...(restoring ? { restoring } : {}),
    backupNames: namesIn(namespace, backups),
    restoreNames: namesIn(namespace, restores),
    readingRestores: restores.flatMap((r) => {
      const phase = r.status?.phase;
      if (phase === "Completed" || phase === "Failed") return [];
      if (r.kind === "PointInTimeRestore") return [];
      const kind = READ_KIND[r.kind];
      return [
        { name: r.metadata?.name ?? "", kind, backup: r.spec.backupRef.name },
      ];
    }),
    ...(clusterVersion ? { clusterVersion } : {}),
    ...(retentionWindow ? { retentionWindow } : {}),
    ...(context.writeMode === undefined ? { changesOff: CHANGES_OFF } : {}),
    now: context.now,
  };
}

// READ_KIND is the kind of the backup that a logical restore reads.
const READ_KIND = {
  LogicalRestoreElasticsearch: "LogicalBackupElasticsearch",
  LogicalRestoreRDBMS: "LogicalBackupRDBMS",
} as const;

// holderOf gives the running field of BackupContext. A restore stays in
// Pending while it suspends its cluster, so a Pending restore can hold the
// cluster, and a Pending backup only waits.
function holderOf(context: ClusterSlotContext): BackupContext["running"] {
  const { cluster, backups, restores } = context;
  const ofCluster = clusterBackups(cluster, backups);
  const [active] = namesOf(
    ofCluster.filter((b) => b.status?.phase === "Running"),
  );
  if (active !== undefined) return { what: "Backup", name: active };
  const [held] = namesOf(
    ofCluster.filter(
      (b) =>
        b.kind === "LogicalBackupElasticsearch" &&
        b.status?.terminalReason === "ResumeFailed",
    ),
  );
  if (held !== undefined) {
    return { what: "Backup", name: held, resumeFailed: true };
  }
  const restore = runningRestore(cluster, restores);
  if (restore) return { what: "Restore", name: restore.name };
  const queued = runningBackup(cluster, backups);
  return queued ? { what: "Backup", name: queued.name } : undefined;
}

function namesOf(objects: readonly BackupObject[]): string[] {
  return objects.map((o) => o.metadata?.name ?? "").sort();
}

type Apply = (
  cluster: { namespace: string; name: string },
  change: Change,
) => Promise<WriteResult>;

/**
 * backupSlots fills cluster.backups.actions with BackupHeaderActions, and
 * cluster.backups.row with BackupRowActions. Each write goes through
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
