"use client";

import Link from "next/link";
import { useId } from "react";
import type { FleetCamundaClusterClaimConditionType } from "@/generated/conditions";
import type { CamundaClusterClaim } from "@/generated/crds/camundaclusterclaim";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import type { PluginRequest } from "@/generated/crds/pluginrequest";
import type { PluginSection } from "@/generated/crds/pluginsection";
import { PrecheckLine } from "@/components/fleet/precheck-line";
import { ProgressRows } from "@/components/fleet/progress-rows";
import type { ClusterClaimObject } from "@/lib/clusters/draft";
import { clusterProgressRows } from "@/lib/clusters/progress";
import { useResource, useResources } from "@/lib/k8s/hooks";
import { CLAIM_LABEL } from "@/lib/labels";

const READY: FleetCamundaClusterClaimConditionType = "Ready";
const CLUSTER_READY: FleetCamundaClusterClaimConditionType = "ClusterReady";

/**
 * ClusterProgress follows the written claim and its PluginRequests live
 * after the create, and shows them with ClusterProgressView, with the error
 * of each failed read above it. Until the claim exists, it shows claim as
 * written, with every row pending.
 */
export function ClusterProgress({
  claim,
  sections,
  registrations,
}: {
  claim: ClusterClaimObject;
  sections: readonly PluginSection[];
  registrations: readonly PluginRegistration[];
}) {
  const { namespace, name } = claim.metadata;
  const live = useResource("CamundaClusterClaim", { namespace, name });
  const requests = useResources(
    "PluginRequest",
    { namespace, labelSelector: `${CLAIM_LABEL}=${name}` },
    [],
  );
  return (
    <div className="flex min-w-0 flex-col gap-4">
      {live.error && (
        <p role="alert" className="text-sm text-bad">
          Could not read CamundaClusterClaim: {live.error}
        </p>
      )}
      {requests.error && (
        <p role="alert" className="text-sm text-bad">
          Could not read PluginRequest: {requests.error}
        </p>
      )}
      <ClusterProgressView
        name={name}
        namespace={namespace}
        claim={
          (live.item as CamundaClusterClaim | undefined) ??
          (claim as unknown as CamundaClusterClaim)
        }
        sections={sections}
        registrations={registrations}
        requests={requests.items as unknown as PluginRequest[]}
      />
    </div>
  );
}

/**
 * ClusterProgressView shows the progress of the cluster claim named name in
 * namespace: its Ready condition, clusterProgressRows, and the precheck
 * refusal of the claim controller. The refusal links the Claim tab only
 * once status.cluster names the cluster, because the cluster page needs
 * the cluster. Once ClusterReady is True, a link opens the cluster page.
 */
export function ClusterProgressView({
  name,
  namespace,
  claim,
  sections,
  registrations,
  requests,
}: {
  name: string;
  namespace: string;
  claim?: CamundaClusterClaim;
  sections: readonly PluginSection[];
  registrations: readonly PluginRegistration[];
  requests: readonly PluginRequest[];
}) {
  const id = useId();
  const conditions = claim?.status?.conditions ?? [];
  const ready = conditions.find((c) => c.type === READY);
  const clusterReady =
    conditions.find((c) => c.type === CLUSTER_READY)?.status === "True";
  const page = `/fleet/clusters/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}`;
  return (
    <section
      aria-labelledby={id}
      className="flex max-w-3xl min-w-0 flex-col gap-4"
    >
      <h2 id={id} className="font-heading text-base font-semibold">
        {`Creating the cluster ${name}`}
      </h2>
      <ProgressRows
        ready={ready}
        rows={clusterProgressRows(claim, sections, registrations, requests)}
        done={
          clusterReady && (
            <Link
              href={page}
              className="relative self-start text-sm text-info underline-offset-4 hit-area hover:underline"
            >
              Open the cluster page
            </Link>
          )
        }
      />
      {claim && (
        <PrecheckLine
          claim={claim}
          href={claim.status?.cluster ? `${page}?tab=claim` : undefined}
        />
      )}
    </section>
  );
}
