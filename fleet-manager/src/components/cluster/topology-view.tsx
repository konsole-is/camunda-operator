import type { ReactNode } from "react";
import { type ConditionLike, DOT_TONE } from "@/components/fleet/status";
import { StatusDot } from "@/components/fleet/status-dot";
import type { PodDot, TopologyBox } from "./topology";

/**
 * TopologyView draws the boxes of a cluster with their embedded processes
 * and one dot for each pod. actions is the content of the
 * cluster.topology.actions slot.
 */
export function TopologyView({
  boxes,
  actions,
}: {
  boxes: TopologyBox[];
  actions?: ReactNode;
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
              <span className="font-medium text-text">{box.label}</span>
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
              <div className="flex flex-wrap gap-2">
                {box.pods.map((pod) => (
                  <PodState key={pod.name} pod={pod} />
                ))}
              </div>
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
  // StatusDot names an off dot "Not used", but a pod in this state is going
  // away, so its name says its own state.
  if (pod.tone === "off") {
    return (
      <span
        role="img"
        aria-label={`${pod.name}: ${pod.state}`}
        className={`inline-block size-2 shrink-0 rounded-full ${DOT_TONE.off}`}
      />
    );
  }
  const condition: ConditionLike =
    pod.tone === "ok"
      ? { status: "True" }
      : pod.tone === "bad"
        ? { status: "False", reason: pod.state }
        : { status: "Unknown", reason: pod.state };
  return <StatusDot label={pod.name} condition={condition} />;
}
