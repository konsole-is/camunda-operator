import type { CamundaRelease } from "@/generated/crds/camundarelease";
import type { Environment } from "@/generated/crds/environment";
import { releaseAllowed } from "./allowed";
import {
  type BackupLike,
  type BackupRef,
  type ClusterLike,
  runningBackup,
} from "./backups";
import type { MoveCheck } from "./types";

/** MovingCluster is the part of a CamundaCluster that releaseMove reads. */
export type MovingCluster = ClusterLike & {
  status?: { management?: { version?: string } };
};

/**
 * releaseMove checks a move of cluster to target. It compares
 * target.spec.version with status.management.version of the cluster, never
 * with the release the cluster runs now. A version that is not
 * major.minor.patch counts as no version, as in the operator.
 */
export function releaseMove(
  cluster: MovingCluster,
  target: CamundaRelease,
  environment: Environment | undefined,
  backups: readonly BackupLike[],
): MoveCheck {
  const current = cluster.status?.management?.version || undefined;
  const want = parseVersion(target.spec.version);
  const have = parseVersion(current);
  const backup = runningBackup(cluster, backups);
  const skip = want && have && want[0] === have[0] && want[1] > have[1] + 1;

  return {
    allowed: releaseAllowed(target.metadata.name ?? "", environment),
    target: target.spec.version,
    ...(current && { current }),
    downgrade: Boolean(want && have && versionBelow(want, have)),
    ...(skip && { skippedMinor: `${have[0]}.${have[1] + 1}` }),
    ...(backup && { backup }),
  };
}

/** SuspendableCluster is the part of a CamundaCluster that moveStatus reads. */
export type SuspendableCluster = MovingCluster & {
  spec?: { suspend?: boolean };
};

/** MoveStatus is the one status of a move check, with the text that a page shows. */
export type MoveStatus =
  | { kind: "notAllowed"; text: string }
  | { kind: "backupRunning"; backup: BackupRef; text: string }
  | { kind: "waitsForResume"; text: string }
  | { kind: "waitsForVersion"; text: string }
  | { kind: "skipsMinor"; minor: string; text: string }
  | { kind: "downgrade"; from: string; to: string; text: string }
  | { kind: "canMove"; text: string };

/**
 * versionCheckWait says why no version check of a move of cluster can run
 * now: the cluster is suspended, or it reports no running version. A
 * version that is not major.minor.patch counts as no version, as in the
 * operator. It is undefined when the checks can run.
 */
export function versionCheckWait(
  cluster: SuspendableCluster,
): string | undefined {
  if (cluster.spec?.suspend)
    return "The version checks run when the cluster resumes.";
  const version: unknown = cluster.status?.management?.version;
  if (typeof version !== "string" || !parseVersion(version))
    return "The version checks run when the cluster reports its version.";
  return undefined;
}

/**
 * moveStatus maps move, the check of a move of cluster, to one status. The
 * first that holds wins, in this order: the environment does not allow the
 * release, a backup runs, the version checks wait (versionCheckWait), the
 * move skips a minor, the move is a downgrade, and else the cluster can
 * move.
 */
export function moveStatus(
  move: MoveCheck,
  cluster: SuspendableCluster,
): MoveStatus {
  if (!move.allowed)
    return {
      kind: "notAllowed",
      text: "The environment does not allow this release.",
    };
  if (move.backup)
    return {
      kind: "backupRunning",
      backup: move.backup,
      text: `Backup ${move.backup.name} is running. The move waits until it ends.`,
    };
  const wait = versionCheckWait(cluster);
  if (wait)
    return {
      kind: cluster.spec?.suspend ? "waitsForResume" : "waitsForVersion",
      text: wait,
    };
  if (move.skippedMinor)
    return {
      kind: "skipsMinor",
      minor: move.skippedMinor,
      text: `Skips ${move.skippedMinor}. Move to an ${move.skippedMinor} release first.`,
    };
  if (move.downgrade && move.current)
    return {
      kind: "downgrade",
      from: move.current,
      to: move.target,
      text: `${move.current} to ${move.target} is a downgrade.`,
    };
  return { kind: "canMove", text: "Can move." };
}

/** parseVersion reads a version of the form major.minor.patch, or gives undefined, as the operator does. */
export function parseVersion(
  version: string | undefined,
): [number, number, number] | undefined {
  const match = /^(\d+)\.(\d+)\.(\d+)$/.exec(version ?? "");
  return match
    ? [Number(match[1]), Number(match[2]), Number(match[3])]
    : undefined;
}

/** versionBelow reports whether the parsed version want is lower than have. */
export function versionBelow(
  want: readonly number[],
  have: readonly number[],
): boolean {
  const i = want.findIndex((n, j) => n !== have[j]);
  return i >= 0 && want[i] < have[i];
}
