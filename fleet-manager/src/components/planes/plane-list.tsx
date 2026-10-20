"use client";

import Link from "next/link";
import type { ReactNode } from "react";
import { DataTable } from "@/components/fleet/data-table";
import { describeReady } from "@/components/fleet/status";
import { StatusChip } from "@/components/fleet/status-chip";
import { Button } from "@/components/ui/button";
import { PlaneClusters, PlaneName, PlaneServes } from "./cells";
import { NotAttachedChip } from "./not-attached-chip";
import { optimizeLabel, type Plane, type PlaneSources } from "./plane";
import { usePlanes } from "./planes-table";

const MODES = ["Keycloak", "External Keycloak", "OIDC"];
const STATUSES = ["Ready", "Not ready", "Unknown", "No status"];

/**
 * PlaneList is the page of every management plane, with search, filters,
 * sort, and pages. error, when set, shows above the table, and the rows stay.
 */
export function PlaneList({
  planes,
  error,
}: {
  planes: Plane[];
  error?: string;
}): ReactNode {
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
      {error && (
        <p role="alert" className="text-sm text-bad">
          {error}
        </p>
      )}
      <DataTable
        label="Management planes"
        emptyText={planes.length === 0 ? "No management plane yet." : undefined}
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
          const id = `${plane.namespace}/${plane.name}`;
          const status = describeReady(plane.ready).label;
          return {
            id,
            label: plane.name,
            href: `/fleet/planes/${id}`,
            search: [
              plane.name,
              plane.namespace,
              "environments" in plane.serves
                ? plane.serves.environments.join(" ")
                : plane.serves.selector,
            ].join(" "),
            sort: { plane: id, clusters: plane.attached, status },
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
  const { planes, error } = usePlanes(initial);
  return <PlaneList planes={planes} error={error} />;
}
