import Link from "next/link";
import type { ReactNode } from "react";
import { DataTable } from "@/components/fleet/data-table";
import { describeReady } from "@/components/fleet/status";
import { StatusChip } from "@/components/fleet/status-chip";
import { unknownUsersText } from "@/lib/k8s/unread";
import type { ContractUser } from "@/lib/usage";

/**
 * UsersTable lists the objects that name a contract, each with the fields
 * that name it, its environment, and its own Ready. The message of a user
 * that is not ready shows under its status. unread names the kinds that can
 * use the contract and that the page could not read or has not read yet:
 * the users are then not known, and the list shows the users that the page
 * read.
 */
export function UsersTable({
  users,
  unread = [],
}: {
  users: readonly ContractUser[];
  unread?: readonly string[];
}): ReactNode {
  const incomplete = unread.length > 0;
  return (
    <div className="flex flex-col gap-2">
      {incomplete && (
        <p className="text-sm text-bad">{unknownUsersText(unread)}</p>
      )}
      <DataTable
        label="Users of the contract"
        toolbar={false}
        emptyText={
          incomplete
            ? "No resource that the page could read uses this contract."
            : "No resource uses this contract."
        }
        columns={[
          { key: "kind", label: "Kind", sortable: true },
          { key: "name", label: "Name", sortable: true },
          { key: "fields", label: "Fields" },
          { key: "environment", label: "Environment", sortable: true },
          { key: "status", label: "Status", sortable: true },
        ]}
        rows={users.map((user) => {
          const name = `${user.namespace}/${user.name}`;
          const notReady = user.ready && user.ready.status !== "True";
          return {
            id: `${user.kind}/${name}`,
            label: name,
            sort: {
              kind: user.kind,
              name,
              environment: user.environment ?? "",
              status: describeReady(user.ready).label,
            },
            cells: {
              kind: user.kind,
              name:
                user.kind === "CamundaCluster" ? (
                  <Link
                    href={`/fleet/clusters/${encodeURIComponent(user.namespace)}/${encodeURIComponent(user.name)}`}
                    className="relative font-medium text-text hit-area hover:text-info"
                  >
                    {name}
                  </Link>
                ) : (
                  name
                ),
              fields: (
                <div className="flex flex-col gap-0.5">
                  {user.fields.map((field) => (
                    <code
                      key={field}
                      className="font-mono text-xs [overflow-wrap:anywhere]"
                    >
                      {field}
                    </code>
                  ))}
                  {user.preset && (
                    <span className="text-xs text-muted">
                      from preset {user.preset}
                    </span>
                  )}
                </div>
              ),
              environment: user.environment ?? (
                <span className="text-muted">No environment</span>
              ),
              status: (
                <div className="flex flex-col items-start gap-1">
                  <StatusChip condition={user.ready} />
                  {notReady && user.ready?.message && (
                    <span className="text-xs text-muted [overflow-wrap:anywhere]">
                      {user.ready.message}
                    </span>
                  )}
                </div>
              ),
            },
          };
        })}
      />
    </div>
  );
}
