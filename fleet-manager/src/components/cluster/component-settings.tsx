import { FieldHelp } from "@/components/fleet/field-help";
import {
  FIELD_TABLE,
  FieldState,
  FieldValue,
} from "@/components/fleet/field-state";
import {
  TableBody,
  TableHead,
  TableHeader,
  TableRow,
  TableCell,
} from "@/components/ui/table";
import type { FieldDoc } from "@/generated/fielddocs";
import type { ClusterField } from "@/lib/fields";
import { cn } from "@/lib/utils";
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
              // Not the Table of the kit: its scroll box clips the help of
              // a field, so the cells wrap long values instead.
              <table
                aria-label={`${group.label} settings`}
                className="w-full text-sm"
              >
                <TableHeader>
                  <TableRow className={FIELD_TABLE.row}>
                    <TableHead>Field</TableHead>
                    <TableHead>Value</TableHead>
                    <TableHead className={FIELD_TABLE.stateHead}>
                      State
                    </TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {group.fields.map((field) => (
                    <TableRow key={field.key} className={FIELD_TABLE.row}>
                      <th
                        scope="row"
                        className={cn(
                          "px-2 py-2 text-left align-top font-normal",
                          FIELD_TABLE.field,
                        )}
                      >
                        <span className="inline-flex items-center gap-1">
                          <code className="text-text break-words">
                            {field.label}
                          </code>
                          <FieldHelp
                            label={field.label}
                            doc={docs[field.key]}
                            operatorVersion={operatorVersion}
                          />
                        </span>
                      </th>
                      <TableCell
                        className={cn(
                          "max-w-80 align-top break-words whitespace-normal",
                          FIELD_TABLE.value,
                        )}
                      >
                        <FieldValue value={field.value} />
                      </TableCell>
                      <TableCell
                        className={cn(
                          "align-top whitespace-normal",
                          FIELD_TABLE.state,
                        )}
                      >
                        <FieldState
                          field={field}
                          claimHref={claimHrefOf(field, cluster)}
                        />
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </table>
            )}
          </div>
        ))
      )}
    </section>
  );
}

function claimHrefOf(
  field: ClusterField,
  cluster: { namespace: string; name: string },
): string | undefined {
  return field.claimPath
    ? claimFieldHref(cluster.namespace, cluster.name, field.claimPath)
    : undefined;
}
