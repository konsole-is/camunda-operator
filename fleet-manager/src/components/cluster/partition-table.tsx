import { type StatusTone, TEXT_TONE } from "@/components/fleet/status";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import type {
  PartitionHealth,
  PartitionHealthState,
  Unavailable,
} from "@/lib/observe/types";
import { cn } from "@/lib/utils";

const HEALTH: Record<PartitionHealthState, { text: string; tone: StatusTone }> =
  {
    healthy: { text: "Healthy", tone: "ok" },
    unhealthy: { text: "Unhealthy", tone: "warn" },
    dead: { text: "Dead", tone: "bad" },
    unknown: { text: "Unknown", tone: "muted" },
  };

/**
 * PartitionTable lists the partitions of the brokers with their leader,
 * followers, processor phase, and health. undefined means that the first
 * answer is not in yet.
 */
export function PartitionTable({
  partitions,
}: {
  partitions: PartitionHealth[] | Unavailable | undefined;
}) {
  return (
    <div className="flex min-w-0 flex-col gap-2">
      <h3 className="font-heading text-sm font-semibold text-text">
        Partitions
      </h3>
      {partitions === undefined ? (
        <p className="text-sm text-muted">Loading</p>
      ) : partitions === "unavailable" ? (
        <p className="text-sm text-muted">
          Partitions are not available. The management API of the brokers did
          not answer.
        </p>
      ) : (
        <>
          <Table label="Partitions">
            <TableHeader>
              <TableRow>
                <TableHead>Partition</TableHead>
                <TableHead>Leader</TableHead>
                <TableHead>Followers</TableHead>
                <TableHead>Processor phase</TableHead>
                <TableHead>Health</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {partitions.map((p) => (
                <TableRow key={p.partition}>
                  <th
                    scope="row"
                    className="px-2 py-2 text-left font-medium text-text tabular-nums"
                  >
                    {p.partition}
                  </th>
                  <TableCell className={p.leader ? "text-text" : "text-bad"}>
                    {p.leader ?? "-"}
                  </TableCell>
                  <TableCell className="text-text-2">
                    {p.followers.join(", ") || "-"}
                  </TableCell>
                  <TableCell className="text-text-2">
                    {p.processorPhase ?? "-"}
                  </TableCell>
                  <TableCell
                    className={cn(
                      "font-medium",
                      TEXT_TONE[HEALTH[p.health].tone],
                    )}
                  >
                    {HEALTH[p.health].text}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
          {partitions.every((p) => p.health === "unknown") && (
            <p className="text-xs text-muted">
              Health needs Prometheus. Set it in the settings.
            </p>
          )}
        </>
      )}
    </div>
  );
}
