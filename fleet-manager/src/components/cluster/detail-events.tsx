"use client";

import { EventList } from "@/components/fleet/event-list";
import { useResources } from "@/lib/k8s/hooks";
import type { Resource } from "@/lib/k8s/kinds";
import { type EventRef, componentEvents } from "@/lib/observe/events";

type EventsProps = {
  namespace: string;
  pods: readonly Resource<"Pod">[];
  podsError?: string;
  refs: readonly EventRef[];
  now: string;
};

/**
 * DetailEvents is the Events tab of a component detail: the events of the
 * pods, the Services and PersistentVolumeClaims that selector selects,
 * the workloads of the pods, and refs, by the rule of componentEvents.
 * A missing or empty selector selects no Service and no
 * PersistentVolumeClaim. Each event names its object. It reads the events
 * of namespace only, so a ref in another namespace gets no events. It needs
 * a QueryClientProvider above it.
 */
export function DetailEvents({
  selector,
  ...props
}: EventsProps & { selector?: string }) {
  return selector ? (
    <SelectedEvents {...props} selector={selector} />
  ) : (
    <EventsView {...props} services={NO_LIST} claims={NO_LIST} />
  );
}

type List = {
  items: readonly { metadata?: { name?: string } }[];
  error?: string;
};

const NO_LIST: List = { items: [] };

function SelectedEvents({
  selector,
  ...props
}: EventsProps & { selector: string }) {
  const selected = { namespace: props.namespace, labelSelector: selector };
  const services = useResources("Service", selected, []);
  const claims = useResources("PersistentVolumeClaim", selected, []);
  return <EventsView {...props} services={services} claims={claims} />;
}

function EventsView({
  namespace,
  pods,
  podsError,
  refs,
  now,
  services,
  claims,
}: EventsProps & { services: List; claims: List }) {
  const events = useResources("Event", { namespace }, []);
  const own = componentEvents(events.items, {
    namespace,
    pods,
    services: services.items.map((s) => s.metadata?.name ?? ""),
    claims: claims.items.map((c) => c.metadata?.name ?? ""),
    refs,
  });
  const failed = (
    [
      ["Pod", { error: podsError }],
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
