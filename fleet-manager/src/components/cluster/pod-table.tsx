import { CHIP_TONE, DOT_TONE } from "@/components/fleet/status";
import { UsageMeter } from "@/components/fleet/usage-bars";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import type { PodUsage, Unavailable } from "@/lib/observe/types";
import { cn } from "@/lib/utils";
import { type ContainerStateLike, type PodStateLike, podDot } from "./topology";

/**
 * PodLike is the part of a Pod that the pod views read. V1Pod of
 * @kubernetes/client-node fits it.
 */
export type PodLike = {
  metadata?: NonNullable<PodStateLike["metadata"]> & {
    creationTimestamp?: string | Date;
  };
  spec?: { containers?: unknown[]; nodeName?: string };
  status?: Omit<
    NonNullable<PodStateLike["status"]>,
    "containerStatuses" | "initContainerStatuses"
  > & {
    initContainerStatuses?: ContainerStatusLike[];
    containerStatuses?: ContainerStatusLike[];
  };
};

type ContainerStatusLike = ContainerStateLike & {
  ready: boolean;
  restartCount: number;
};

/** podRestarts counts the restarts of all containers of pod, init containers too. */
export function podRestarts(pod: PodLike): number {
  return [
    ...(pod.status?.initContainerStatuses ?? []),
    ...(pod.status?.containerStatuses ?? []),
  ].reduce((n, c) => n + c.restartCount, 0);
}

/**
 * USAGE_UNAVAILABLE says why a table shows no usage. The usage reader gives
 * the same answer for a cluster without metrics-server and for a read that
 * failed, so the sentence names both.
 */
export const USAGE_UNAVAILABLE =
  "Usage is not available. The cluster has no metrics server, or the app could not read the usage.";

/** USAGE_FOOTNOTE says what the CPU and memory of PodTable are. */
export const USAGE_FOOTNOTE =
  "CPU and memory show the use now, against the limit of the pod, or its request without a limit. They come from the metrics API of Kubernetes, so they need no Prometheus.";

/**
 * PodTable lists pods with their node, their state in the words of the
 * topology, the restarts of their containers, and their CPU and memory.
 * When usage is "unavailable", the CPU and memory columns are hidden and a
 * note says why. A pod without usage data shows "-" in those columns.
 */
export function PodTable({
  pods,
  usage,
}: {
  pods: PodLike[];
  usage: PodUsage[] | Unavailable;
}) {
  const showUsage = usage !== "unavailable";
  return (
    <div className="flex min-w-0 flex-col gap-2">
      <Table label="Pods">
        <TableHeader>
          <TableRow>
            <TableHead>Pod</TableHead>
            <TableHead>State</TableHead>
            <TableHead>Restarts</TableHead>
            {showUsage && <TableHead>CPU</TableHead>}
            {showUsage && <TableHead>Memory</TableHead>}
          </TableRow>
        </TableHeader>
        <TableBody>
          {pods.map((pod) => {
            const name = pod.metadata?.name ?? "";
            const state = podDot(pod);
            const podUsage = showUsage
              ? usage.find((u) => u.pod === name)
              : undefined;
            return (
              <TableRow key={name}>
                <th
                  scope="row"
                  className="px-2 py-2 text-left align-top font-normal whitespace-nowrap"
                >
                  <span className="block font-mono text-xs text-text">
                    {name}
                  </span>
                  <span className="block text-xs text-muted">
                    {pod.spec?.nodeName ?? "-"}
                  </span>
                </th>
                <TableCell className="align-top">
                  <span
                    className={cn(
                      "inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-xs font-medium",
                      CHIP_TONE[state.tone],
                    )}
                  >
                    <span
                      aria-hidden
                      className={cn(
                        "size-1.5 shrink-0 rounded-full",
                        DOT_TONE[state.tone],
                      )}
                    />
                    {state.state}
                  </span>
                </TableCell>
                <TableCell className="align-top tabular-nums">
                  {podRestarts(pod)}
                </TableCell>
                {showUsage &&
                  (["cpu", "memory"] as const).map((resource) => (
                    <TableCell key={resource} className="align-top">
                      {podUsage ? (
                        <UsageMeter usage={podUsage} resource={resource} />
                      ) : (
                        "-"
                      )}
                    </TableCell>
                  ))}
              </TableRow>
            );
          })}
        </TableBody>
      </Table>
      <p className="text-xs text-muted">
        {showUsage ? USAGE_FOOTNOTE : USAGE_UNAVAILABLE}
      </p>
    </div>
  );
}
