"use client";

import type { ReactNode } from "react";
import { DataTable } from "@/components/fleet/data-table";
import { describeReady } from "@/components/fleet/status";
import { StatusChip } from "@/components/fleet/status-chip";
import { type ReadError, unknownUsersText } from "@/lib/k8s/unread";
import { type PlatformRow, platformRows, unreadUserKinds } from "./model";
import { type PlatformSnapshot, usePlatformSources } from "./sources";

const METHOD_LABEL = { basic: "Basic", oidc: "OIDC" } as const;

/**
 * PlatformList is the list page of the platform configs. Each of errors
 * shows above the table, and the rows stay. stale holds the kinds whose
 * watch is not fresh (freshKey), as usePlatformSources
 * gives them.
 */
export function PlatformList({
  rows,
  errors,
  stale = [],
}: {
  rows: PlatformRow[];
  errors: ReadError[];
  stale?: readonly string[];
}): ReactNode {
  const unread = unreadUserKinds({ errors, stale });
  return (
    <div className="flex flex-col gap-6">
      <h1 className="font-heading text-3xl font-bold tracking-[-0.02em]">
        Platform
      </h1>
      {errors.map((e) => (
        <p key={e.kind} role="alert" className="text-sm text-bad">
          Could not read {e.kind}: {e.message}
        </p>
      ))}
      {unread.length > 0 && (
        <p className="text-sm text-muted">{unknownUsersText(unread)}</p>
      )}
      <DataTable
        label="Platform configs"
        emptyText={
          rows.length === 0
            ? "No platform config yet. A cluster needs one. The guide shows how to apply one."
            : undefined
        }
        columns={[
          { key: "name", label: "Name", sortable: true },
          { key: "method", label: "Authentication", sortable: true },
          { key: "users", label: "Users", sortable: true },
          { key: "status", label: "Status", sortable: true },
        ]}
        rows={rows.map((row) => ({
          id: row.name,
          label: row.name,
          href: `/setup/platform/${encodeURIComponent(row.name)}`,
          search: row.name,
          sort: {
            name: row.name,
            method: METHOD_LABEL[row.method],
            users: row.users ?? -1,
            status: describeReady(row.ready).label,
          },
          cells: {
            name: <span className="font-mono">{row.name}</span>,
            method: METHOD_LABEL[row.method],
            users: row.users ?? "Not known",
            status: <StatusChip condition={row.ready} />,
          },
        }))}
      />
    </div>
  );
}

/** PlatformListLive is PlatformList on the live sources, starting from initial. */
export function PlatformListLive({ initial }: { initial: PlatformSnapshot }) {
  const sources = usePlatformSources(initial);
  return (
    <PlatformList
      rows={platformRows(sources)}
      errors={sources.errors}
      stale={sources.stale}
    />
  );
}
