import type { ReactNode } from "react";
import { Panel } from "@/components/cluster/claim-tab";
import { ConditionsTable } from "@/components/fleet/conditions-table";
import { EventList } from "@/components/fleet/event-list";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import type { Resource } from "@/lib/k8s/kinds";
import { NotRead } from "@/components/fleet/not-read";
import { ValueError } from "./tab-parts";
import { conditionsOf } from "./values";

const HEALTH = ["Accepted", "Ready"];

/**
 * HealthTab shows the Accepted and Ready conditions of registration, with
 * their messages, and events, the events about it. unread holds the kinds
 * that are not read: PluginRegistration hides the conditions, and Event
 * marks the events as not complete. now is the time that the ages count to.
 */
export function HealthTab({
  registration,
  events,
  unread,
  now,
}: {
  registration: PluginRegistration;
  events: readonly Resource<"Event">[];
  unread: readonly string[];
  now: string;
}): ReactNode {
  const read = conditionsOf(registration.status?.conditions);
  const eventsUnread = unread.includes("Event");
  return (
    <div className="flex flex-col gap-4">
      <Panel label="Health">
        {unread.includes("PluginRegistration") ? (
          <NotRead kinds={["PluginRegistration"]} />
        ) : "error" in read ? (
          <ValueError text={read.error} />
        ) : (
          <ConditionsTable
            conditions={read.conditions.filter((c) => HEALTH.includes(c.type))}
            now={now}
          />
        )}
      </Panel>
      <Panel label="Events">
        {eventsUnread && events.length === 0 ? (
          <NotRead kinds={["Event"]} />
        ) : (
          <>
            {eventsUnread && (
              <p className="text-sm text-muted">
                The list can miss events: Event is not read.
              </p>
            )}
            <EventList events={[...events]} now={now} />
          </>
        )}
      </Panel>
    </div>
  );
}
