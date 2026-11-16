"use client";

import { DataTable } from "@/components/fleet/data-table";
import { byCodeUnit } from "@/lib/policy";
import type { ServedCluster } from "./served-clusters";
import { UnreadNote, unreadText } from "./unread-note";

const NO_ENVIRONMENT = "No environment";
const NOT_KNOWN = "Not known";
const EMPTY = <span className="text-muted">-</span>;

/**
 * ServedClustersTab is the Clusters served tab of a plane: one row for each
 * cluster that the plane lists, with filters for the environment and for
 * attached, and a search. unread names the kinds that the page could not
 * read. While unread is not empty, the tab never says that the plane lists
 * no clusters.
 */
export function ServedClustersTab({
  rows,
  unread = [],
}: {
  rows: ServedCluster[];
  unread?: readonly string[];
}) {
  const environmentOf = (row: ServedCluster) =>
    row.environmentUnknown ? NOT_KNOWN : (row.environment ?? NO_ENVIRONMENT);
  const named = [
    ...new Set(rows.flatMap((r) => (r.environment ? [r.environment] : []))),
  ].sort(byCodeUnit);
  const environments = [
    ...named,
    ...(rows.some((r) => !r.environmentUnknown && !r.environment)
      ? [NO_ENVIRONMENT]
      : []),
    ...(rows.some((r) => r.environmentUnknown) ? [NOT_KNOWN] : []),
  ];
  return (
    <section
      aria-label="Clusters served"
      className="flex flex-col gap-3 rounded-lg border border-border bg-panel p-4"
    >
      <h2 className="font-heading text-base font-semibold">Clusters served</h2>
      {rows.length > 0 && <UnreadNote kinds={unread} />}
      <DataTable
        label="Clusters served table"
        emptyText={
          rows.length > 0
            ? undefined
            : unread.length > 0
              ? unreadText(unread)
              : "This plane lists no clusters yet."
        }
        columns={[
          { key: "cluster", label: "Cluster", sortable: true },
          { key: "environment", label: "Environment", sortable: true },
          { key: "namespace", label: "Namespace" },
          { key: "attached", label: "Attached" },
          { key: "reason", label: "Reason" },
          { key: "message", label: "Message" },
        ]}
        filters={[
          {
            key: "environment",
            label: "Environment",
            options: environments.map((e) => ({ value: e, label: e })),
          },
          {
            key: "attached",
            label: "Attached",
            options: [
              { value: "Attached", label: "Attached" },
              { value: "Not attached", label: "Not attached" },
            ],
          },
        ]}
        rows={rows.map((row) => ({
          id: `${row.namespace}/${row.name}`,
          label: row.name,
          href: row.href,
          search: [row.name, row.namespace, row.reason, row.message]
            .filter(Boolean)
            .join(" "),
          filters: {
            environment: environmentOf(row),
            attached: row.attached ? "Attached" : "Not attached",
          },
          sort: { cluster: row.name, environment: environmentOf(row) },
          cells: {
            cluster: row.name,
            environment: row.environment ?? (
              <span className="text-muted">{environmentOf(row)}</span>
            ),
            namespace: row.namespace,
            attached: row.attached ? "Yes" : "No",
            reason: row.reason ?? EMPTY,
            message: row.message ? (
              <span className="block min-w-48 whitespace-normal">
                {row.message}
              </span>
            ) : (
              EMPTY
            ),
          },
        }))}
      />
    </section>
  );
}
