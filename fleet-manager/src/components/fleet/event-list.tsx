import type { Resource } from "@/lib/k8s/kinds";
import { cn } from "@/lib/utils";
import { age } from "./age";

/**
 * EventList lists the events.k8s.io/v1 Events of an object, newest first,
 * with the reason, the note, how many times it happened, and its age. A
 * Warning event is marked. An empty list says "No events".
 */
export function EventList({
  events,
  now,
}: {
  events: Resource<"Event">[];
  now: string;
}) {
  if (events.length === 0)
    return <p className="text-sm text-muted">No events</p>;
  // An Event that the API server converted from a core/v1 Event has a null
  // eventTime and keeps its times in the deprecated fields.
  const time = (e: Resource<"Event">): string | undefined =>
    e.series?.lastObservedTime ??
    e.deprecatedLastTimestamp ??
    (e.eventTime as string | null) ??
    e.deprecatedFirstTimestamp ??
    e.metadata?.creationTimestamp;
  const sorted = [...events].sort(
    (a, b) =>
      new Date(time(b) ?? 0).getTime() - new Date(time(a) ?? 0).getTime(),
  );
  return (
    <ul className="flex flex-col divide-y divide-divider">
      {sorted.map((event, i) => {
        const warning = event.type === "Warning";
        const count = event.series?.count ?? event.deprecatedCount;
        return (
          <li
            key={event.metadata?.uid ?? i}
            className="flex flex-col gap-0.5 py-2 text-sm sm:flex-row sm:gap-4"
          >
            <span className="w-12 shrink-0 text-muted">
              {age(time(event), now)}
            </span>
            <span
              className={cn(
                "w-40 shrink-0 font-medium",
                warning ? "text-warn" : "text-text",
              )}
            >
              {event.reason ?? "-"}
              {warning && (
                <span className="ml-2 rounded-sm bg-warn/15 px-1.5 py-0.5 text-xs font-normal text-warn">
                  Warning
                </span>
              )}
            </span>
            <span className="min-w-0 flex-1 text-text-2">
              {event.note}
              {count && count > 1 ? (
                <span className="text-muted">{` (${count} times)`}</span>
              ) : null}
            </span>
          </li>
        );
      })}
    </ul>
  );
}
