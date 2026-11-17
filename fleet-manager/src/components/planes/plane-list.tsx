"use client";

import Link from "next/link";
import type { ReactNode } from "react";
import { DataTable } from "@/components/fleet/data-table";
import { NotRead, notReadText } from "@/components/fleet/not-read";
import { describeReady } from "@/components/fleet/status";
import { StatusChip } from "@/components/fleet/status-chip";
import { Button } from "@/components/ui/button";
import { useAnswered } from "@/lib/k8s/answered";
import { useWatchesFresh, useWatchesOverdue } from "@/lib/k8s/hooks";
import type { KindName } from "@/lib/k8s/kinds";
import { type ReadError, unreadKinds } from "@/lib/k8s/unread";
import { PlaneClusters, PlaneName, PlaneServes } from "./cells";
import { NotAttachedChip } from "./not-attached-chip";
import { optimizeLabel, type Plane, type PlaneSources } from "./plane";
import { usePlanes } from "./planes-table";

const MODES = ["Keycloak", "External Keycloak", "OIDC"];
const PLANE_KINDS = [
  "CamundaManagementCluster",
  "CamundaManagementClusterClaim",
];
// usePlanes follows these lists.
const FOLLOWED: KindName[] = [
  "CamundaManagementCluster",
  "CamundaManagementClusterClaim",
  "Namespace",
  "Environment",
];
const STATUSES = ["Ready", "Not ready", "Unknown", "No status"];

/**
 * PlaneList is the page of every management plane, with search, filters,
 * sort, and pages. Each of errors shows above the table, and the rows stay.
 * stale holds the kinds whose list is not current.
 */
export function PlaneList({
  planes,
  errors = [],
  stale = [],
}: {
  planes: Plane[];
  errors?: ReadError[];
  stale?: readonly string[];
}): ReactNode {
  const planesUnread = unreadKinds({ errors, stale }).filter((kind) =>
    PLANE_KINDS.includes(kind),
  );
  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="font-heading text-3xl font-bold tracking-[-0.02em]">
          Management planes
        </h1>
        <Button asChild>
          <Link href="/fleet/planes/new">New management plane</Link>
        </Button>
      </div>
      {errors.map((e) => (
        <p key={e.kind} role="alert" className="text-sm text-bad">
          Could not read {e.kind}: {e.message}
        </p>
      ))}
      {planes.length > 0 && (
        <NotRead
          kinds={stale.filter((kind) => !errors.some((e) => e.kind === kind))}
        />
      )}
      <DataTable
        label="Management planes"
        emptyText={
          planes.length > 0
            ? undefined
            : planesUnread.length > 0
              ? notReadText(planesUnread)
              : "No management plane yet."
        }
        columns={[
          { key: "plane", label: "Management plane", sortable: true },
          { key: "identity", label: "Identity" },
          { key: "components", label: "Components" },
          { key: "serves", label: "Serves" },
          { key: "clusters", label: "Clusters", sortable: true },
          { key: "optimize", label: "Optimize" },
          { key: "contract", label: "Contract" },
          { key: "status", label: "Status", sortable: true },
          { key: "warnings", label: "Warnings" },
        ]}
        filters={[
          {
            key: "mode",
            label: "Identity",
            options: MODES.map((m) => ({ value: m, label: m })),
          },
          {
            key: "status",
            label: "Status",
            options: STATUSES.map((s) => ({ value: s, label: s })),
          },
        ]}
        rows={planes.map((plane) => {
          const path = `${plane.namespace}/${plane.name}`;
          const status = describeReady(plane.ready).label;
          return {
            id: plane.id,
            label: plane.name,
            href: `/fleet/planes/${path}`,
            search: [
              plane.name,
              plane.namespace,
              "environments" in plane.serves
                ? plane.serves.environments.join(" ")
                : plane.serves.selector,
            ].join(" "),
            sort: { plane: path, clusters: plane.attached, status },
            filters: { mode: plane.mode ?? "", status },
            cells: {
              plane: <PlaneName plane={plane} />,
              identity: plane.mode ?? "—",
              components: plane.components.join(", "),
              serves: <PlaneServes plane={plane} />,
              clusters: <PlaneClusters plane={plane} />,
              optimize: optimizeLabel(plane),
              contract: plane.contract ?? "—",
              status: <StatusChip condition={plane.ready} readyReason />,
              warnings: <NotAttachedChip clusters={plane.notAttached} />,
            },
          };
        })}
      />
    </div>
  );
}

/** PlaneListLive is PlaneList on the live planes of usePlanes, starting from initial. */
export function PlaneListLive({ initial }: { initial: PlaneSources }) {
  const { planes, errors } = usePlanes(initial);
  const lists = FOLLOWED.map((kind) => ({ kind, opts: {} }));
  const answered = useAnswered(FOLLOWED);
  const fresh = useWatchesFresh(lists);
  const overdue = useWatchesOverdue(lists);
  // Before its first answer, a list can be the cache of an earlier visit.
  const stale = FOLLOWED.filter((kind, i) =>
    answered.has(kind) ? !fresh[i] : overdue[i],
  );
  return <PlaneList planes={planes} errors={errors} stale={stale} />;
}
