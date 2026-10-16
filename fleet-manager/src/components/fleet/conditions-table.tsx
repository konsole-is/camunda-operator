import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { cn } from "@/lib/utils";
import { age } from "./age";
import { CONDITION_TONE, type ConditionLike, TEXT_TONE } from "./status";

/** ConditionRow is the part of a Kubernetes condition that the table shows. */
export type ConditionRow = ConditionLike & {
  type: string;
  lastTransitionTime?: string | Date;
};

/**
 * ConditionsTable lists conditions with their type, status, reason, message,
 * and age. The conditions without a group come first, then each group under
 * its label, in the order given. now is the time that the ages count to.
 */
export function ConditionsTable({
  conditions,
  groups = [],
  now,
  label = "Conditions",
}: {
  conditions: ConditionRow[];
  groups?: { label: string; conditions: ConditionRow[] }[];
  now: string;
  label?: string;
}) {
  if (
    conditions.length === 0 &&
    groups.every((g) => g.conditions.length === 0)
  ) {
    return <p className="text-sm text-muted">No conditions</p>;
  }
  return (
    <Table label={label}>
      <TableHeader>
        <TableRow>
          <TableHead>Type</TableHead>
          <TableHead>Status</TableHead>
          <TableHead>Reason</TableHead>
          <TableHead>Message</TableHead>
          <TableHead>Age</TableHead>
        </TableRow>
      </TableHeader>
      {conditions.length > 0 && (
        <TableBody>
          {conditions.map((c) => (
            <Row key={c.type} condition={c} now={now} />
          ))}
        </TableBody>
      )}
      {groups.map((group) => (
        <TableBody key={group.label}>
          <TableRow>
            <th
              scope="rowgroup"
              colSpan={5}
              className="px-2 pt-4 pb-1 text-left font-heading text-sm font-semibold text-text"
            >
              {group.label}
            </th>
          </TableRow>
          {group.conditions.map((c) => (
            <Row key={c.type} condition={c} now={now} />
          ))}
        </TableBody>
      ))}
    </Table>
  );
}

function Row({ condition, now }: { condition: ConditionRow; now: string }) {
  return (
    <TableRow>
      <th
        scope="row"
        className="px-2 py-2 text-left font-medium whitespace-nowrap text-text"
      >
        {condition.type}
      </th>
      <TableCell
        className={cn(
          "font-medium",
          TEXT_TONE[CONDITION_TONE[condition.status]],
        )}
      >
        {condition.status}
      </TableCell>
      <TableCell className="text-text-2">{condition.reason ?? "-"}</TableCell>
      <TableCell className="min-w-48 whitespace-normal text-text-2">
        {condition.message || "-"}
      </TableCell>
      <TableCell className="text-muted">
        {age(condition.lastTransitionTime, now)}
      </TableCell>
    </TableRow>
  );
}
