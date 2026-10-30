import type { ReactNode } from "react";
import {
  type ConditionLike,
  DOT_TONE,
  TEXT_TONE,
} from "@/components/fleet/status";
import { StatusDot } from "@/components/fleet/status-dot";
import type { PodDot, Process, TopologyBox } from "./topology";

/**
 * TopologyView draws the boxes of a cluster with their embedded processes
 * and the name and state of each pod. actions is the content of the
 * cluster.topology.actions slot. conditions gives the condition of each
 * process. A box with one shows its dot.
 */
export function TopologyView({
  boxes,
  actions,
  conditions,
}: {
  boxes: TopologyBox[];
  actions?: ReactNode;
  conditions?: Partial<Record<Process, ConditionLike>>;
}) {
  return (
    <section
      aria-label="Topology"
      className="flex flex-col gap-3 rounded-lg border border-border bg-panel p-4"
    >
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h2 className="font-heading text-base font-semibold">Topology</h2>
        {actions}
      </div>
      <ul className="grid gap-3 sm:grid-cols-[repeat(auto-fill,minmax(14rem,1fr))]">
        {boxes.map((box) => (
          <li
            key={box.process}
            className="flex flex-col gap-3 rounded-md border border-border bg-panel-2 p-3"
          >
            <div className="flex items-baseline justify-between gap-2">
              <span className="flex items-center gap-2 font-medium text-text">
                {conditions?.[box.process] && (
                  <StatusDot
                    label={box.label}
                    condition={conditions[box.process]}
                  />
                )}
                {box.label}
              </span>
              <span className="text-xs text-muted">{box.workload}</span>
            </div>
            {box.embedded.length > 0 && (
              <ul className="flex flex-wrap gap-1.5">
                {box.embedded.map((e) => (
                  <li
                    key={e.process}
                    className="rounded border border-dashed border-faint px-2 py-0.5 text-xs text-text-2"
                  >
                    Embedded: {e.label}
                  </li>
                ))}
              </ul>
            )}
            {box.pods.length > 0 ? (
              <ul
                aria-label={`Pods of ${box.label}`}
                className="flex flex-col gap-1"
              >
                {box.pods.map((pod) => (
                  <PodState key={pod.name} pod={pod} />
                ))}
              </ul>
            ) : (
              <p className="text-xs text-muted">No pods</p>
            )}
          </li>
        ))}
      </ul>
    </section>
  );
}

function PodState({ pod }: { pod: PodDot }) {
  return (
    <li className="flex min-w-0 items-center gap-2 text-xs">
      <span
        aria-hidden
        className={`inline-block size-2 shrink-0 rounded-full ${DOT_TONE[pod.tone]}`}
      />
      <span className="truncate text-text-2">{pod.name}</span>{" "}
      <span className={`shrink-0 ${TEXT_TONE[pod.tone]}`}>{pod.state}</span>
    </li>
  );
}
