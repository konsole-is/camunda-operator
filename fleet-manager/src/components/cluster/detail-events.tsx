"use client";

import { EventList } from "@/components/fleet/event-list";
import { useResources } from "@/lib/k8s/hooks";
import type { Resource } from "@/lib/k8s/kinds";
import { type EventRef, componentEvents } from "@/lib/observe/events";

/**
 * DetailEvents is the Events tab of a component detail: the events of the
 * pods, the Services and PersistentVolumeClaims that selector selects,
 * the workloads of the pods, and refs, by the rule of componentEvents.
 * Each event names its object. It reads the events of namespace only, so
 * a ref in another namespace gets no events. It needs a
 * QueryClientProvider above it.
 */
export function DetailEvents({
  namespace,
  selector,
  pods,
  refs,
  now,
}: {
  namespace: string;
  selector: string;
  pods: readonly Resource<"Pod">[];
  refs: readonly EventRef[];
  now: string;
}) {
  const events = useResources("Event", { namespace }, []);
  const selected = { namespace, labelSelector: selector };
  const services = useResources("Service", selected, []);
  const claims = useResources("PersistentVolumeClaim", selected, []);
  const own = componentEvents(events.items, {
    namespace,
    pods,
    services: services.items.map((s) => s.metadata?.name ?? ""),
    claims: claims.items.map((c) => c.metadata?.name ?? ""),
    refs,
  });
  const failed = (
    [
      ["Event", events],
      ["Service", services],
      ["PersistentVolumeClaim", claims],
    ] as const
  ).flatMap(([kind, result]) =>
    result.error === undefined
      ? []
      : [`Could not read ${kind}: ${result.error}`],
  );
  return (
    <div className="flex flex-col gap-2 pt-2">
      {failed.length > 0 && (
        <div className="flex flex-col gap-1 text-sm text-warn">
          <p>The list can miss events, because a read failed.</p>
          <ul className="flex flex-col gap-1">
            {failed.map((line) => (
              <li key={line}>{line}</li>
            ))}
          </ul>
        </div>
      )}
      {(own.length > 0 || events.error === undefined) && (
        <EventList events={own} now={now} showObject />
      )}
    </div>
  );
}
