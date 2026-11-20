// This module runs in the browser and on the server.
import type { Environment } from "@/generated/crds/environment";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import {
  environmentOf,
  type PluginSelection,
  type ResolvedPolicy,
  resolve,
} from "@/lib/policy";

type Claim = {
  metadata: { namespace?: string };
  status?: { resolvedPolicy?: ResolvedPolicy };
};

type Selection = {
  section: string;
  registration: string;
  parameters?: unknown;
};

/** RequestedSources holds the lists that resolve the policy of a claim. */
export type RequestedSources = {
  namespaces: readonly {
    metadata?: { name?: string; labels?: Record<string, string> };
  }[];
  environments: readonly Environment[];
  registrations: readonly PluginRegistration[];
};

/**
 * requestedParameters returns the parameters that the request of selection
 * of claim gets from the claim policy. When the parameters of the selection
 * are not a JSON object, it is what the policy alone gives the request. When
 * the policy does not resolve without them either, it is the parameters of
 * the selection, which can be any JSON value.
 */
export function requestedParameters(
  claim: Claim,
  selection: Selection,
  sources: RequestedSources,
): unknown {
  const { parameters, ...withoutParameters } = selection;
  return (
    resolvedParameters(claim, selection, sources) ??
    resolvedParameters(claim, withoutParameters, sources) ??
    parameters
  );
}

// resolvedParameters gives the parameters of the request of selection, or
// undefined when the policy of claim cannot be resolved for it.
function resolvedParameters(
  claim: Claim,
  selection: Selection,
  sources: RequestedSources,
): Record<string, unknown> | undefined {
  const namespace = sources.namespaces.find(
    (n) => n.metadata?.name === claim.metadata.namespace,
  );
  const resolved = resolve({
    recorded: claim.status?.resolvedPolicy,
    environment:
      namespace && environmentOf(namespace, sources.environments).environment,
    registrations: [...sources.registrations],
    selections: [selection as PluginSelection],
  });
  if (!("value" in resolved)) return undefined;
  return resolved.value.requests?.[0]?.parameters ?? {};
}
