"use client";

import { useId } from "react";
import type { FleetCamundaManagementClusterClaimConditionType } from "@/generated/conditions";
import type { CamundaManagementClusterClaim } from "@/generated/crds/camundamanagementclusterclaim";
import type { PluginSection } from "@/generated/crds/pluginsection";
import { PrecheckLine } from "@/components/fleet/precheck-line";
import { ProgressRows } from "@/components/fleet/progress-rows";
import { useResource } from "@/lib/k8s/hooks";
import { planeProgressRows } from "@/lib/planes/progress";

const READY: FleetCamundaManagementClusterClaimConditionType = "Ready";

/**
 * PlaneProgress follows the written claim live after the create and shows
 * it with PlaneProgressView. Until the claim exists, it shows claim as
 * written, with every row pending.
 */
export function PlaneProgress({
  claim,
  sections,
}: {
  claim: CamundaManagementClusterClaim;
  sections: readonly PluginSection[];
}) {
  const { namespace, name = "" } = claim.metadata;
  const { item } = useResource("CamundaManagementClusterClaim", {
    namespace,
    name,
  });
  return (
    <PlaneProgressView
      name={name}
      claim={(item as CamundaManagementClusterClaim | undefined) ?? claim}
      sections={sections}
    />
  );
}

/**
 * PlaneProgressView shows the progress of the plane claim named name: its
 * Ready condition, planeProgressRows, and the refusal of the claim
 * controller.
 */
export function PlaneProgressView({
  name,
  claim,
  sections,
}: {
  name: string;
  claim?: CamundaManagementClusterClaim;
  sections: readonly PluginSection[];
}) {
  const id = useId();
  const ready = claim?.status?.conditions?.find((c) => c.type === READY);
  return (
    <section
      aria-labelledby={id}
      className="flex max-w-3xl min-w-0 flex-col gap-4"
    >
      <h2 id={id} className="font-heading text-base font-semibold">
        {`Creating the management plane ${name}`}
      </h2>
      <ProgressRows ready={ready} rows={planeProgressRows(claim, sections)} />
      {claim && <PrecheckLine claim={claim} />}
    </section>
  );
}
