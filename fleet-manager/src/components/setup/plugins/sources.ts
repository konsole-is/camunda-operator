"use client";

import type { V1Namespace } from "@kubernetes/client-node";
import type { ReadError } from "@/components/planes/planes-table";
import type { Environment } from "@/generated/crds/environment";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import type { PluginRequest } from "@/generated/crds/pluginrequest";
import type { PluginSection } from "@/generated/crds/pluginsection";
import { useResources, useWatchesFresh } from "@/lib/k8s/hooks";
import type { KindName, Resource } from "@/lib/k8s/kinds";
import { REGISTRATION_SOURCE_KINDS } from "./source-kinds";

/** RegistrationSources holds every list that the Plugins page reads, in all namespaces. */
export type RegistrationSources = {
  sections: PluginSection[];
  registrations: PluginRegistration[];
  requests: PluginRequest[];
  namespaces: V1Namespace[];
  environments: Environment[];
};

/**
 * useRegistrationSources follows each list of initial live. errors holds
 * the error of each failed watch. The lists then keep their last items,
 * which can be out of date. stale holds the kind of each list whose watch
 * has no snapshot on its current connection.
 */
export function useRegistrationSources(
  initial: RegistrationSources,
): RegistrationSources & { errors: ReadError[]; stale: string[] } {
  const results = {
    sections: useLive("PluginSection", initial.sections),
    registrations: useLive("PluginRegistration", initial.registrations),
    requests: useLive("PluginRequest", initial.requests),
    namespaces: useLive("Namespace", initial.namespaces),
    environments: useLive("Environment", initial.environments),
  };
  const keys = Object.keys(results) as (keyof RegistrationSources)[];
  const kinds = keys.map((key) => REGISTRATION_SOURCE_KINDS[key]);
  const fresh = useWatchesFresh(kinds.map((kind) => ({ kind, opts: {} })));
  const errors = keys.flatMap((key) => {
    const error = results[key].error;
    return error
      ? [{ kind: REGISTRATION_SOURCE_KINDS[key], message: error }]
      : [];
  });
  return {
    sections: results.sections.items,
    registrations: results.registrations.items,
    requests: results.requests.items,
    namespaces: results.namespaces.items,
    environments: results.environments.items,
    errors,
    stale: kinds.filter((_, i) => !fresh[i]),
  };
}

// A Resource is plain JSON. It differs from its model type only in the
// timestamps of a V1Namespace, which the Plugins page does not read.
function useLive<K extends KindName, T>(
  kind: K,
  initial: T[],
): { items: T[]; error?: string } {
  const { items, error } = useResources(
    kind,
    {},
    initial as unknown as Resource<K>[],
  );
  return { items: items as unknown as T[], ...(error && { error }) };
}
