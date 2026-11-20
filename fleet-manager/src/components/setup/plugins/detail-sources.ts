"use client";

import type { Grant } from "@/generated/crds/grant";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import { useResources, useWatchesFresh } from "@/lib/k8s/hooks";
import type { Resource } from "@/lib/k8s/kinds";
import type { ReadError } from "@/lib/k8s/unread";
import { REGISTRATION_LABEL } from "@/lib/labels";
import { regards } from "@/lib/observe/events";
import { type RegistrationSnapshot, useRegistrationSources } from "./sources";

/** RegistrationDetailData is what the page of one registration reads. */
export type RegistrationDetailData = {
  name: string;
  /** registration is the registration as the server read it for the page. */
  registration: PluginRegistration;
  sources: RegistrationSnapshot;
  /** grants are the Grants with REGISTRATION_LABEL=name, in every namespace. */
  grants: Grant[];
  /** events are the events that regard the registration. */
  events: Resource<"Event">[];
};

/**
 * useRegistrationDetail follows each list of initial live. events keeps
 * only the events of the registration with the uid of the live one, so an
 * event of an older registration of the same name stays out. errors and
 * stale are as useRegistrationSources gives them, with Grant and Event
 * added.
 */
export function useRegistrationDetail(
  initial: RegistrationDetailData,
): RegistrationDetailData & { errors: ReadError[]; stale: string[] } {
  const { name } = initial;
  const { errors, stale, ...sources } = useRegistrationSources(initial.sources);
  const selector = { labelSelector: `${REGISTRATION_LABEL}=${name}` };
  // A Resource is plain JSON. It differs from its model type only in its
  // timestamps, which the page does not read.
  const grants = useResources(
    "Grant",
    selector,
    initial.grants as unknown as Resource<"Grant">[],
  );
  const events = useResources("Event", {}, initial.events);
  const fresh = useWatchesFresh([
    { kind: "Grant", opts: selector },
    { kind: "Event", opts: {} },
  ]);
  const live = sources.registrations.find((r) => r.metadata.name === name);
  const uid = (live ?? initial.registration).metadata.uid;
  const ref = { kind: "PluginRegistration", name, uid };
  return {
    name,
    registration: initial.registration,
    sources,
    grants: grants.items as unknown as Grant[],
    events: events.items.filter((e) => regards(e, ref)),
    errors: [
      ...errors,
      ...(grants.error ? [{ kind: "Grant", message: grants.error }] : []),
      ...(events.error ? [{ kind: "Event", message: events.error }] : []),
    ],
    stale: [
      ...stale,
      ...(fresh[0] ? [] : ["Grant"]),
      ...(fresh[1] ? [] : ["Event"]),
    ],
  };
}
