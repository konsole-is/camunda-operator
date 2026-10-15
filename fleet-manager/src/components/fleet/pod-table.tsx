import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import type { PodUsage, Unavailable } from "@/lib/observe/types";
import { age } from "./age";
import { UsageBars } from "./usage-bars";

/** PodLike is the part of a Pod that the table reads. V1Pod of @kubernetes/client-node fits it. */
export type PodLike = {
  metadata?: { name?: string; creationTimestamp?: string | Date };
  spec?: { containers?: unknown[] };
  status?: {
    phase?: string;
    containerStatuses?: { ready: boolean; restartCount: number }[];
  };
};

/**
 * PodTable lists pods with their ready containers, phase, restarts, and age,
 * and the CPU and memory of each pod. When usage is "unavailable", the usage
 * column is hidden and a note says why. A pod without usage data shows "-"
 * in that column.
 */
export function PodTable({
  pods,
  usage,
  now,
}: {
  pods: PodLike[];
  usage: PodUsage[] | Unavailable;
  now: string;
}) {
  const showUsage = usage !== "unavailable";
  return (
    <div className="flex min-w-0 flex-col gap-2">
      <Table label="Pods">
        <TableHeader>
          <TableRow>
            <TableHead>Name</TableHead>
            <TableHead>Ready</TableHead>
            <TableHead>Status</TableHead>
            <TableHead>Restarts</TableHead>
            <TableHead>Age</TableHead>
            {showUsage && <TableHead>Usage</TableHead>}
          </TableRow>
        </TableHeader>
        <TableBody>
          {pods.map((pod) => {
            const name = pod.metadata?.name ?? "";
            const containers = pod.status?.containerStatuses ?? [];
            const podUsage = showUsage
              ? usage.find((u) => u.pod === name)
              : undefined;
            return (
              <TableRow key={name}>
                <th
                  scope="row"
                  className="px-2 py-2 text-left font-medium text-text"
                >
                  {name}
                </th>
                <TableCell>{`${containers.filter((c) => c.ready).length}/${pod.spec?.containers?.length ?? containers.length}`}</TableCell>
                <TableCell className="text-text-2">
                  {pod.status?.phase ?? "-"}
                </TableCell>
                <TableCell>
                  {containers.reduce((n, c) => n + c.restartCount, 0)}
                </TableCell>
                <TableCell className="text-muted">
                  {age(pod.metadata?.creationTimestamp, now)}
                </TableCell>
                {showUsage && (
                  <TableCell>
                    {podUsage ? <UsageBars usage={podUsage} /> : "-"}
                  </TableCell>
                )}
              </TableRow>
            );
          })}
        </TableBody>
      </Table>
      {!showUsage && (
        <p className="text-xs text-muted">
          Usage is not available. The cluster has no metrics server.
        </p>
      )}
    </div>
  );
}
