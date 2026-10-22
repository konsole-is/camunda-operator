import { Lock } from "lucide-react";
import Link from "next/link";
import { FieldHelp } from "@/components/fleet/field-help";
import {
  Table,
  TableBody,
  TableHead,
  TableHeader,
  TableRow,
  TableCell,
} from "@/components/ui/table";
import type { FieldDoc } from "@/generated/fielddocs";
import {
  lockLabel,
  stateLabel,
  stateNote,
  type ClusterField,
} from "@/lib/fields";
import { claimFieldHref, type SettingsGroup } from "./settings";

/**
 * ComponentSettings shows the fields of one component read only: each
 * group as a table of the field with its help, the value, and where the
 * value comes from, with the lock and a link to the field in the Claim tab.
 * docs holds the help of a field by its key.
 */
export function ComponentSettings({
  groups,
  docs,
  operatorVersion,
  cluster,
}: {
  groups: SettingsGroup[];
  docs: Record<string, FieldDoc>;
  operatorVersion: string;
  cluster: { namespace: string; name: string };
}) {
  const empty = groups.every((g) => g.fields.length === 0);
  return (
    <section aria-label="Settings" className="flex min-w-0 flex-col gap-6">
      {empty ? (
        <p className="text-sm text-muted">No settings.</p>
      ) : (
        groups.map((group) => (
          <div key={group.label} className="flex min-w-0 flex-col gap-2">
            <h3 className="font-heading text-sm font-semibold">
              {group.embedded ? `${group.label} (embedded)` : group.label}
            </h3>
            {group.fields.length === 0 ? (
              <p className="text-sm text-muted">No settings.</p>
            ) : (
              <Table label={`${group.label} settings`}>
                <TableHeader>
                  <TableRow>
                    <TableHead>Field</TableHead>
                    <TableHead>Value</TableHead>
                    <TableHead>State</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {group.fields.map((field) => (
                    <TableRow key={field.key}>
                      <th
                        scope="row"
                        className="px-2 py-2 text-left align-top font-normal"
                      >
                        <span className="inline-flex items-center gap-1">
                          <code className="text-text">{field.label}</code>
                          <FieldHelp
                            label={field.label}
                            doc={docs[field.key]}
                            operatorVersion={operatorVersion}
                          />
                        </span>
                      </th>
                      <TableCell className="max-w-80 align-top break-all whitespace-normal">
                        <Value value={field.value} />
                      </TableCell>
                      <TableCell className="align-top whitespace-normal">
                        <State field={field} cluster={cluster} />
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
          </div>
        ))
      )}
    </section>
  );
}

function Value({ value }: { value: unknown }) {
  if (value === undefined) return <span className="text-muted">Not set</span>;
  if (typeof value === "string") return <>{value}</>;
  if (typeof value === "number" || typeof value === "boolean")
    return <>{String(value)}</>;
  return <code className="text-xs text-text-2">{JSON.stringify(value)}</code>;
}

function State({
  field,
  cluster,
}: {
  field: ClusterField;
  cluster: { namespace: string; name: string };
}) {
  const note = stateNote(field.state);
  return (
    <div className="flex flex-col gap-1">
      <span className="text-text-2">{stateLabel(field.state)}</span>
      {note && <span className="text-xs text-muted">{note}</span>}
      {field.lock && (
        <span className="flex items-center gap-1.5 text-xs text-muted">
          <Lock aria-hidden className="size-3.5 shrink-0" />
          {lockLabel(field.lock)}
        </span>
      )}
      {field.claimPath && (
        <Link
          href={claimFieldHref(
            cluster.namespace,
            cluster.name,
            field.claimPath,
          )}
          className="text-xs text-info underline-offset-4 hover:underline"
        >
          Open in the Claim tab
        </Link>
      )}
    </div>
  );
}
