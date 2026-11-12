import type { CamundaRelease } from "@/generated/crds/camundarelease";
import type { Environment } from "@/generated/crds/environment";
import type { Resource } from "@/lib/k8s/kinds";
import { releaseMove } from "@/lib/policy";
import { claimAfter, type Session } from "@/lib/session";
import type { ClusterSlotContext } from "../slots";
import type { ClusterClaim } from "./cluster-edit";

/**
 * downgradeDone says whether the cluster reports the version that
 * allowVersionDowngrade of the claim names, so that the field can go.
 */
export function downgradeDone(
  cluster: Resource<"CamundaCluster">,
  claim: ClusterClaim,
): boolean {
  const target = claim.spec?.allowVersionDowngrade;
  return Boolean(target) && cluster.status?.management?.version === target;
}

/**
 * downgradeOf returns the versions of the downgrade that session makes or
 * allows: a move to a lower release, or a new allowVersionDowngrade for the
 * lower release that the claim names already. It is undefined for any
 * other session, a release that is not in the context, and a cluster that
 * reports no version. A claim without a release runs the default release of
 * the Environment.
 */
export function downgradeOf(
  context: ClusterSlotContext,
  session: Session<ClusterClaim>,
): { from: string; to: string } | undefined {
  const fallback = context.environment?.spec.releases?.default;
  const spec = claimAfter(session).spec;
  const before = session.claim.spec?.releaseRef || fallback;
  const after = spec?.releaseRef || fallback;
  const allows =
    spec?.allowVersionDowngrade !== undefined &&
    spec.allowVersionDowngrade !== session.claim.spec?.allowVersionDowngrade;
  if (!after || (after === before && !allows)) return undefined;
  const release = context.releases.find((r) => r.metadata?.name === after);
  if (!release) return undefined;
  const move = releaseMove(
    context.cluster,
    // The cache objects differ from the generated types only in their
    // timestamps, which releaseMove does not read.
    release as unknown as CamundaRelease,
    context.environment as unknown as Environment | undefined,
    context.backups,
  );
  return move.downgrade && move.current
    ? { from: move.current, to: move.target }
    : undefined;
}
