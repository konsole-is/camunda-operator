"use client";

import { type ReactNode, useMemo } from "react";
import { DataTable } from "@/components/fleet/data-table";
import { StatusChip } from "@/components/fleet/status-chip";
import { useResources } from "@/lib/k8s/hooks";
import { PlaneClusters, PlaneName, PlaneServes } from "./cells";
import { NotAttachedChip } from "./not-attached-chip";
import {
  type Plane,
  type PlaneSources,
  planesOf,
  servesEnvironment,
} from "./plane";

export type { Plane, PlaneSources } from "./plane";

/**
 * usePlanes follows the three lists of initial live and returns their
 * planes. error is the first error of the three watches. The planes then
 * keep the last lists, which can be out of date.
 */
export function usePlanes(initial: PlaneSources): {
  planes: Plane[];
  error?: string;
} {
  const clusters = useResources(
    "CamundaManagementCluster",
    {},
    initial.clusters,
  );
  const claims = useResources(
    "CamundaManagementClusterClaim",
    {},
    initial.claims,
  );
  const namespaces = useResources("Namespace", {}, initial.namespaces);
  const planes = useMemo(
    () =>
      planesOf({
        clusters: clusters.items,
        claims: claims.items,
        namespaces: namespaces.items,
      }),
    [clusters.items, claims.items, namespaces.items],
  );
  const error = clusters.error ?? claims.error ?? namespaces.error;
  return error === undefined ? { planes } : { planes, error };
}

/** PlanesTable shows the planes that serve environment, or nothing when none does. */
export function PlanesTable({
  planes,
  environment,
}: {
  planes: Plane[];
  environment: string;
}): ReactNode {
  const served = planes.filter((plane) =>
    servesEnvironment(plane, environment),
  );
  if (served.length === 0) return null;
  return (
    <DataTable
      label="Management planes"
      columns={[
        { key: "plane", label: "Management plane" },
        { key: "status", label: "Status" },
        { key: "identity", label: "Identity" },
        { key: "serves", label: "Serves" },
        { key: "clusters", label: "Clusters" },
        { key: "warnings", label: "Warnings" },
      ]}
      rows={served.map((plane) => ({
        id: plane.id,
        label: plane.name,
        href: `/fleet/planes/${plane.namespace}/${plane.name}`,
        search: `${plane.name} ${plane.namespace}`,
        cells: {
          plane: <PlaneName plane={plane} />,
          status: <StatusChip condition={plane.ready} readyReason />,
          identity: plane.mode ?? "—",
          serves: <PlaneServes plane={plane} />,
          clusters: <PlaneClusters plane={plane} />,
          warnings: <NotAttachedChip clusters={plane.notAttached} />,
        },
      }))}
    />
  );
}
