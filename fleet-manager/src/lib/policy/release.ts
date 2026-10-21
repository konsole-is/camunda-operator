import type { CamundaRelease } from "@/generated/crds/camundarelease";
import type { Environment } from "@/generated/crds/environment";
import { releaseAllowed } from "./allowed";
import { type BackupLike, type ClusterLike, runningBackup } from "./backups";
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
    downgrade: Boolean(want && have && below(want, have)),
    ...(skip && { skippedMinor: `${have[0]}.${have[1] + 1}` }),
    ...(backup && { backup }),
  };
}

function parseVersion(
  version: string | undefined,
): [number, number, number] | undefined {
  const match = /^(\d+)\.(\d+)\.(\d+)$/.exec(version ?? "");
  return match
    ? [Number(match[1]), Number(match[2]), Number(match[3])]
    : undefined;
}

function below(want: readonly number[], have: readonly number[]): boolean {
  const i = want.findIndex((n, j) => n !== have[j]);
  return i >= 0 && want[i] < have[i];
}
