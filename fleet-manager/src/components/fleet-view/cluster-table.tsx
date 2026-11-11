import Link from "next/link";
import type { ReactNode } from "react";
import { age } from "@/components/fleet/age";
import { DataTable } from "@/components/fleet/data-table";
import { Button } from "@/components/ui/button";
import { CHIP_TONE } from "@/components/fleet/status";
import { StatusChip } from "@/components/fleet/status-chip";
import { cn } from "@/lib/utils";
import type { RefKey } from "@/lib/write/types";
import { ComponentDots } from "./component-dots";
import type { ClusterRow, WaitingRow } from "./model";

const CHIP =
  "inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium whitespace-nowrap";

// The groups of the fleet view stack several cluster tables. Fixed column
// widths keep their columns in line from one table to the next.
const COLUMN_WIDTHS =
  "md:[&_table]:table-fixed md:[&_th:nth-child(1)]:w-[24%] md:[&_th:nth-child(2)]:w-[22%] md:[&_th:nth-child(3)]:w-[14%] md:[&_th:nth-child(4)]:w-[22%]";

// The cells of a fixed column break a long name, which has no spaces.
const WRAP = "min-w-0 whitespace-normal break-words";

const CLUSTER_COLUMNS = [
  { key: "cluster", label: "Cluster" },
  { key: "status", label: "Status" },
  { key: "version", label: "Version" },
  { key: "components", label: "Components" },
  { key: "backup", label: "Last backup" },
];

/** ClusterTable is one cluster table. A row with href links its name to the cluster page. */
export function ClusterTable({
  label,
  rows,
  emptyText,
}: {
  label: string;
  rows: ClusterRow[];
  emptyText?: string;
}): ReactNode {
  return (
    <div className={COLUMN_WIDTHS}>
      <DataTable
        label={label}
        toolbar={false}
        emptyText={emptyText}
        columns={CLUSTER_COLUMNS}
        rows={rows.map((row) => ({
          id: row.id,
          label: row.name,
          cells: {
            cluster: (
              <Name name={row.name} namespace={row.namespace} href={row.href}>
                {!row.managed && (
                  <span className={cn(CHIP, "mt-1 w-fit", CHIP_TONE.muted)}>
                    Not managed
                  </span>
                )}
              </Name>
            ),
            status: <StatusChip condition={row.ready} readyReason />,
            version: (
              <span className={cn("flex flex-col", WRAP)}>
                <span className="text-text-2">{row.version ?? "-"}</span>
                {row.preset && (
                  <span className="text-xs text-muted">{row.preset}</span>
                )}
              </span>
            ),
            components: <ComponentDots dots={row.components} />,
            backup: row.lastBackup ? (
              <span className="flex flex-col gap-1">
                <span
                  className={cn(CHIP, "w-fit", CHIP_TONE[row.lastBackup.tone])}
                >
                  {row.lastBackup.phase}
                </span>
                <span className="text-xs whitespace-nowrap text-muted">
                  {row.lastBackup.started}
                </span>
              </span>
            ) : (
              <span className="text-muted">No backups</span>
            ),
          },
        }))}
      />
    </div>
  );
}

/**
 * WaitingTable lists the claims that wait for GitOps, with their file and the
 * age of the push. Stop waiting gives the key of the push to onStopWaiting.
 */
export function WaitingTable({
  rows,
  now,
  onStopWaiting,
}: {
  rows: WaitingRow[];
  now: string;
  onStopWaiting?: (key: RefKey) => void;
}): ReactNode {
  return (
    <DataTable
      label="Waiting for GitOps"
      toolbar={false}
      columns={[
        { key: "cluster", label: "Cluster" },
        { key: "status", label: "Status" },
        { key: "file", label: "File" },
        { key: "pushed", label: "Pushed" },
        { key: "stop", label: "" },
      ]}
      rows={rows.map((row) => ({
        id: row.id,
        label: row.name,
        cells: {
          cluster: <Name name={row.name} namespace={row.namespace} />,
          status: (
            <span className={cn(CHIP, "bg-info/15 text-info")}>
              Waiting for GitOps
            </span>
          ),
          file: <code className="font-mono text-xs break-all">{row.path}</code>,
          pushed: (
            <span className="whitespace-nowrap">{`${age(row.pushedAt, now)} ago`}</span>
          ),
          stop: (
            <Button
              variant="ghost"
              size="sm"
              onClick={() => onStopWaiting?.(row.key)}
            >
              Stop waiting
            </Button>
          ),
        },
      }))}
    />
  );
}

function Name({
  name,
  namespace,
  href,
  children,
}: {
  name: string;
  namespace: string;
  href?: string;
  children?: ReactNode;
}) {
  return (
    <span className={cn("flex flex-col", WRAP)}>
      {href ? (
        <Link href={href} className="font-medium text-text hover:text-info">
          {name}
        </Link>
      ) : (
        <span className="font-medium text-text">{name}</span>
      )}
      <span className="text-xs text-muted">{namespace}</span>
      {children}
    </span>
  );
}
