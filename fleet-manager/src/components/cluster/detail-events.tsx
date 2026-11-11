"use client";

import { EventList } from "@/components/fleet/event-list";
import { useResources } from "@/lib/k8s/hooks";
import type { Resource } from "@/lib/k8s/kinds";
import { type EventRef, componentEvents } from "@/lib/observe/events";

/**
 * DetailEvents is the Events tab of a component detail: the events of the
 * pods, the Services and PersistentVolumeClaims that podSelector selects,
 * the workloads of the pods, and refs, by the rule of componentEvents.
 * Each event names its object. It reads the events of namespace only, so
 * a ref in another namespace gets no events. It needs a
 * QueryClientProvider above it.
 */
export function DetailEvents({
  namespace,
  podSelector,
  pods,
  refs,
  now,
}: {
  namespace: string;
  podSelector: string;
  pods: readonly Resource<"Pod">[];
  refs: readonly EventRef[];
  now: string;
}) {
  const events = useResources("Event", { namespace }, []);
  const selected = { namespace, labelSelector: podSelector };
  const services = useResources("Service", selected, []);
  const claims = useResources("PersistentVolumeClaim", selected, []);
  const own = componentEvents(events.items, {
    namespace,
    pods,
    services: services.items.map((s) => s.metadata?.name ?? ""),
    claims: claims.items.map((c) => c.metadata?.name ?? ""),
    refs,
  });
  return (
    <div className="pt-2">
      <EventList events={own} now={now} showObject />
    </div>
  );
}
