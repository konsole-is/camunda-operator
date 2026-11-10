import type { ReviewConfirm } from "@/components/fleet/claim-review";
import { rollSensitiveBackup, type RollBackupLike } from "@/lib/policy/backups";
import { claimAfter, discard } from "@/lib/session";
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
 * backup of the cluster, by the backup controllers of the operator. none: no
 * backup with phase Running. note: a muted line, when the backup cannot fail
 * from the change: the change does not roll the brokers, the partition
 * backup of an Elasticsearch backup is final, or the change only suspends a
 * cluster with an Elasticsearch backup, which waits for the resume.
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
  const suspends = suspendsCluster(input);
  const rolls = rollsZeebe(
    suspends
      ? { ...input, session: discard(input.session, "spec.suspend") }
      : input,
  );
  if (!suspends && !rolls)
    return {
      kind: "note",
      note: `${running} This change does not roll the brokers, so it does not affect the backup.`,
    };
  const caution = (lines: string[]): BackupWarning => ({
    kind: "caution",
    lines: [
      ...lines,
      ...(mode === "git"
        ? [
            "GitOps applies the change later. If a backup runs then, it can fail.",
          ]
        : []),
    ],
    confirm: { label: "Change anyway", gitLabel: "Push anyway", danger: true },
  });
  const head = `CAUTION: Wait until backup ${backup.name} finishes.`;
  if (backup.kind === "LogicalBackupRDBMS" && suspends)
    return caution([
      `${head} This change suspends the cluster, which can fail a running backup.`,
      "The backup waits 10 minutes for the cluster to resume, then fails.",
      "A backup that starts after the change waits until the cluster resumes.",
    ]);
  if (backup.kind === "LogicalBackupElasticsearch") {
    if (!rolls)
      return {
        kind: "note",
        note: `${running} The backup waits until the cluster resumes.`,
      };
    if (backup.partitionsFinal)
      return {
        kind: "note",
        note: `${running} The backup of its Zeebe partitions is finished. A roll of the brokers does not affect the backup.`,
      };
  }
  const waits =
    backup.kind === "LogicalBackupRDBMS" && !backup.hasZeebeBackupId;
  return caution([
    `${head} This change can restart the brokers, which can fail a running backup.`,
    waits
      ? "The backup waits up to 10 minutes for the rollout to finish. It fails if the rollout takes longer, or if Zeebe then runs another configuration or Camunda version."
      : "The backup fails at once if the change gives Zeebe another configuration or Camunda version.",
    "A backup that starts after the change waits until the rollout is done.",
  ]);
}

function suspendsCluster(input: ClusterChangeInput): boolean {
  return (
    claimAfter(input.session).spec.suspend === true &&
    input.session.claim.spec.suspend !== true
  );
}
