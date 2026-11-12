import type { Session } from "@/lib/session";
import type { ClusterChangeInput } from "../impacts";
import type { ClusterSlotContext } from "../slots";
import type { ClusterClaim } from "./cluster-edit";

/**
 * changeInputOf returns the input of clusterImpacts, mergedSpecs, and
 * restartsBrokers for session on the page of context: the live cluster, the
 * presets, releases, sections, and Environment of the page, the
 * CamundaPlatformConfig of spec.platformConfigRef of the live cluster as
 * platform, and context.storage as storage.
 */
export function changeInputOf(
  context: ClusterSlotContext,
  session: Session<ClusterClaim>,
): ClusterChangeInput {
  const ref = context.cluster.spec.platformConfigRef;
  return {
    session,
    live: context.cluster,
    presets: context.presets,
    releases: context.releases,
    sections: context.sections,
    environment: context.environment,
    platform:
      ref === undefined
        ? undefined
        : context.platformConfigs.find((p) => p.metadata?.name === ref),
    storage: context.storage,
  };
}
