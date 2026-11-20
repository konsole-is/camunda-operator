import type { ReactNode } from "react";
import { describeReady } from "@/components/fleet/status";
import { DataTable } from "@/components/fleet/data-table";
import { StatusChip } from "@/components/fleet/status-chip";
import type { InheritingRow } from "./inheriting";

/**
 * InheritingTable lists the resources that inherit a preset with their
 * environment, namespace, own fields, and Ready condition. A field that a
 * slot fills names its section. When unread is not empty, it names those
 * kinds and shows no resource.
 */
export function InheritingTable({
  rows,
  unread,
}: {
  rows: InheritingRow[];
  unread: readonly string[];
}): ReactNode {
  if (unread.length > 0)
    return (
      <p role="alert" className="text-sm text-bad">
        The inheriting resources are not known while the fleet manager cannot
        read {unread.join(", ")}.
      </p>
    );
  return (
    <DataTable
      label="Inheriting resources"
      toolbar={false}
      emptyText="No resource inherits this preset."
      columns={[
        { key: "resource", label: "Resource" },
        { key: "environment", label: "Environment" },
        { key: "namespace", label: "Namespace" },
        { key: "own", label: "Own fields" },
        { key: "ready", label: "Ready" },
      ]}
      rows={rows.map((row) => ({
        id: `${row.kind}/${row.namespace}/${row.name}`,
        label: row.name,
        href: row.href,
        sort: { ready: describeReady(row.ready).label },
        cells: {
          resource: row.name,
          environment: row.environment ?? (
            <span className="text-muted">None</span>
          ),
          namespace: row.namespace,
          own:
            row.ownFields.length === 0 ? (
              <span className="text-muted">None</span>
            ) : (
              <ul className="flex flex-col gap-0.5">
                {row.ownFields.map((field) => (
                  <li key={field.path} className="text-sm">
                    <span className="font-mono text-xs">{field.path}</span>
                    {field.filledBy && (
                      <span className="text-muted">
                        {" "}
                        Filled by {field.filledBy}
                      </span>
                    )}
                  </li>
                ))}
              </ul>
            ),
          ready: <StatusChip condition={row.ready} />,
        },
      }))}
    />
  );
}
