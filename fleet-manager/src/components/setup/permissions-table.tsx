import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import type { AppPermissions } from "@/lib/settings";

/**
 * PermissionsTable shows the rules of the app, read only. The chart sets
 * them. An error shows as an alert in place of the table.
 */
export function PermissionsTable({
  permissions,
}: {
  permissions: AppPermissions | { error: string };
}) {
  if ("error" in permissions) {
    return (
      <p role="alert" className="text-sm text-bad">
        Could not read the permissions: {permissions.error}
      </p>
    );
  }
  return (
    <div className="space-y-2">
      <Table label="Permissions of the fleet manager">
        <TableHeader>
          <TableRow>
            <TableHead>API group</TableHead>
            <TableHead>Resources</TableHead>
            <TableHead>Names</TableHead>
            <TableHead>Verbs</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {permissions.rules.map((rule) => (
            <TableRow
              key={`${rule.apiGroups.join(",")}|${rule.resources.join(",")}|${rule.resourceNames.join(",")}|${rule.verbs.join(",")}`}
            >
              <TableCell>
                {rule.apiGroups.map((g) => g || "core").join(", ")}
              </TableCell>
              <TableCell>{rule.resources.join(", ")}</TableCell>
              <TableCell
                className={rule.resourceNames.length ? "" : "text-muted"}
              >
                {rule.resourceNames.length
                  ? rule.resourceNames.join(", ")
                  : "any"}
              </TableCell>
              <TableCell>{rule.verbs.join(", ")}</TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
      {permissions.incomplete && (
        <p className="text-sm text-warn">
          The API server could not list every rule.
        </p>
      )}
    </div>
  );
}
