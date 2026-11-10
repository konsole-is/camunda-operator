import type { ReviewConfirm } from "@/components/fleet/claim-review";
import { equalJSON } from "@/lib/policy";
import { rollSensitiveBackup, type RollBackupLike } from "@/lib/policy/backups";
import { claimAfter } from "@/lib/session";
import type { WriteMode } from "@/lib/write/types";
import {
  mergedSpecs,
  restartsBrokers,
  type ClusterChangeInput,
} from "./impacts";

/**
 * rollsZeebe says whether the change rolls the brokers: restartsBrokers is
 * true, or the merged Camunda version changes. Like restartsBrokers, it is
 * true when the review cannot decide.
 */
export function rollsZeebe(input: ClusterChangeInput): boolean {
  if (restartsBrokers(input)) return true;
  const { before, after } = mergedSpecs(input);
  return before.spec.version !== after.spec.version;
}

/** BackupWarning is what the review says about a running backup. */
export type BackupWarning =
  | { kind: "none" }
  | { kind: "note"; note: string }
  | { kind: "caution"; lines: string[]; confirm: ReviewConfirm };

/**
 * backupWarning says what the review of a change says about the running
 * backup of the cluster, by the rules of "Changes" in
 * docs/crds/logicalbackupelasticsearch.md and docs/crds/logicalbackuprdbms.md.
 * none: no backup with phase Running. note: a muted line, when the change
 * does not roll the brokers, or when the partition backup of an
 * Elasticsearch backup is final and the change cannot move the storage or
 * the backup bucket.
 * caution: the lines of the caution and the danger confirm.
 */
export function backupWarning(
  input: ClusterChangeInput,
  backups: readonly RollBackupLike[],
  mode: WriteMode,
): BackupWarning {
  const backup = rollSensitiveBackup(input.live, backups);
  if (!backup) return { kind: "none" };
  const running = `Backup ${backup.name} is running.`;
  if (!rollsZeebe(input))
    return {
      kind: "note",
      note: `${running} This change does not roll the brokers, so it does not affect the backup.`,
    };
  if (
    backup.kind === "LogicalBackupElasticsearch" &&
    backup.partitionsFinal &&
    !movesStorage(input)
  )
    return {
      kind: "note",
      note: `${running} The backup of its Zeebe partitions is finished. A roll of the brokers does not affect the backup.`,
    };
  const waits =
    backup.kind === "LogicalBackupRDBMS" && !backup.hasZeebeBackupId;
  return {
    kind: "caution",
    lines: [
      `CAUTION: Wait until backup ${backup.name} finishes. This change rolls the brokers, and a roll fails a running backup.`,
      waits
        ? "The backup waits 10 minutes for the change to be undone, then fails."
        : "The backup fails at once.",
      "A backup that starts after the change waits until the rollout is done.",
      ...(mode === "git"
        ? ["GitOps applies the change later. If a backup runs then, it fails."]
        : []),
    ],
    confirm: { label: "Change anyway", gitLabel: "Push anyway", danger: true },
  };
}

// movesStorage says whether the change can move the secondary storage or the
// backup bucket of the cluster. Such a move fails a running backup after its
// partition backup too. A section fills these fields only after its outputs
// change, so a section change counts.
function movesStorage(input: ClusterChangeInput): boolean {
  const { before, after } = mergedSpecs(input);
  return (
    before.spec.storageRef !== after.spec.storageRef ||
    before.spec.backupStorageRef !== after.spec.backupStorageRef ||
    !equalJSON(
      input.session.claim.spec.plugins,
      claimAfter(input.session).spec.plugins,
    )
  );
}
