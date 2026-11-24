"use client";

import { claimNameOf } from "@/components/cluster/claim";
import {
  type ClusterPageData,
  ClusterView,
} from "@/components/cluster/cluster-view";
import { type PendingPush, useGitPushes } from "@/components/fleet/git-pushes";
import type { PushedObject } from "@/components/fleet/git-waiting-line";
import { refKey } from "@/lib/write/refs";
import type { WriteMode } from "@/lib/write/types";
import { applyClaimChange, previewClaimChange } from "./actions";
import { clusterSlots } from "./slots";

const claimActions = { preview: previewClaimChange, apply: applyClaimChange };

/**
 * ClusterPage is the client half of the cluster page. It gives ClusterView
 * the slot fills of the route, the claim actions, and the pending Git push
 * of the claim of the cluster. writeMode is undefined when the server could
 * not read it.
 */
export function ClusterPage({
  initial,
  now,
  writeMode,
}: {
  initial: ClusterPageData;
  now: string;
  writeMode?: WriteMode;
}) {
  const { pushes } = useGitPushes();
  const { namespace } = initial.ref;
  const claimName = claimNameOf(initial.cluster, initial.ref.name);
  const key =
    claimName === undefined
      ? undefined
      : refKey({ kind: "CamundaClusterClaim", namespace, name: claimName });
  const pushedClaim = pushes.find(
    (p): p is PendingPush & PushedObject => p.key === key && !("deleted" in p),
  );
  return (
    <ClusterView
      initial={initial}
      now={now}
      slots={clusterSlots}
      claimActions={claimActions}
      writeMode={writeMode}
      pushedClaim={pushedClaim}
    />
  );
}
