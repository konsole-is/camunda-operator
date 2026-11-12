"use client";

import Link from "next/link";
import type { ReactNode } from "react";
import { DataTable } from "@/components/fleet/data-table";
import { describeReady } from "@/components/fleet/status";
import { StatusChip } from "@/components/fleet/status-chip";
import type { ReadError } from "@/components/planes/planes-table";
import { Button } from "@/components/ui/button";
import { EnvironmentName } from "./environment-name";
import {
  type EnvironmentRow,
  environmentRows,
  type UnassignedNamespace,
  unassignedNamespaces,
} from "./rows";
import { type EnvironmentSources, useEnvironmentSources } from "./sources";
import { UnassignedBanner } from "./unassigned-banner";

/**
 * EnvironmentList is the list page of the environments. Each of errors
 * shows above the table, and the rows stay.
 */
export function EnvironmentList({
  rows,
  unassigned,
  errors,
}: {
  rows: EnvironmentRow[];
  unassigned: UnassignedNamespace[];
  errors: ReadError[];
}): ReactNode {
  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="font-heading text-3xl font-bold tracking-[-0.02em]">
          Environments
        </h1>
        <Button asChild>
          <Link href="/setup/environments/new">New environment</Link>
        </Button>
      </div>
      {errors.map((e) => (
        <p key={e.kind} role="alert" className="text-sm text-bad">
          Could not read {e.kind}: {e.message}
        </p>
      ))}
      <UnassignedBanner namespaces={unassigned} />
      <DataTable
        label="Environments"
        emptyText={
          rows.length === 0
            ? "No environment yet. Without one, every namespace takes the policy of the registrations."
            : undefined
        }
        columns={[
          { key: "environment", label: "Environment", sortable: true },
          { key: "namespaces", label: "Namespaces", sortable: true },
          { key: "clusters", label: "Clusters", sortable: true },
          { key: "planes", label: "Management planes", sortable: true },
          { key: "status", label: "Status", sortable: true },
        ]}
        rows={rows.map((row) => ({
          id: row.name,
          label: row.displayName ?? row.name,
          href: `/setup/environments/${encodeURIComponent(row.name)}`,
          search: `${row.name} ${row.displayName ?? ""}`,
          sort: {
            environment: row.displayName ?? row.name,
            namespaces: row.namespaces,
            clusters: row.clusters,
            planes: row.planes,
            status: describeReady(row.ready).label,
          },
          cells: {
            environment: <EnvironmentName {...row} />,
            namespaces: row.namespaces,
            clusters: row.clusters,
            planes: row.planes,
            status: <StatusChip condition={row.ready} />,
          },
        }))}
      />
    </div>
  );
}

/** EnvironmentListLive is EnvironmentList on the live sources, starting from initial. */
export function EnvironmentListLive({
  initial,
}: {
  initial: EnvironmentSources;
}) {
  const sources = useEnvironmentSources(initial);
  return (
    <EnvironmentList
      rows={environmentRows(sources)}
      unassigned={unassignedNamespaces(sources)}
      errors={sources.errors}
    />
  );
}
