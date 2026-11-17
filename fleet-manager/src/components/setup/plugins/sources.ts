"use client";

import type { V1Namespace } from "@kubernetes/client-node";
import type { Environment } from "@/generated/crds/environment";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import type { PluginRequest } from "@/generated/crds/pluginrequest";
import type { PluginSection } from "@/generated/crds/pluginsection";
import { useAnswered } from "@/lib/k8s/answered";
import { useWatchesFresh } from "@/lib/k8s/hooks";
import { useLive } from "@/lib/k8s/live";
import { currentReadErrors, type ReadError } from "@/lib/k8s/unread";
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
 * RegistrationSnapshot is the first snapshot of the Plugins pages. readErrors
 * holds the error of each kind that the server could not read.
 */
export type RegistrationSnapshot = RegistrationSources & {
  readErrors?: ReadError[];
};

/**
 * useRegistrationSources follows each list of initial live. errors holds the
 * error of each failed watch, and the error of initial.readErrors of each other
 * kind until the watch of that kind answers. The lists then keep their last
 * items, which can be out of date. stale holds the kind of each list whose
 * watch is not fresh (freshKey).
 */
export function useRegistrationSources(
  initial: RegistrationSnapshot,
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
  const answered = useAnswered(kinds);
  const live = keys.flatMap((key) => {
    const error = results[key].error;
    return error
      ? [{ kind: REGISTRATION_SOURCE_KINDS[key], message: error }]
      : [];
  });
  const errors = currentReadErrors(live, initial.readErrors, answered);
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
