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
 * requestedParameters returns each object of parameters that the request of
 * selection of claim can get. When the policy of the claim resolves for the
 * selection, it is the parameters of that request. When it does not, it is
 * the parameters of the selection and, when the policy resolves without
 * them, the parameters that the policy alone gives the request. So a value
 * that the recorded policy holds still counts while the claim is refused.
 */
export function requestedParameters(
  claim: Claim,
  selection: Selection,
  sources: RequestedSources,
): unknown[] {
  const resolved = resolvedParameters(claim, selection, sources);
  if (resolved !== undefined) return [resolved];
  const { parameters, ...withoutParameters } = selection;
  const byPolicy = resolvedParameters(claim, withoutParameters, sources);
  return byPolicy === undefined ? [parameters] : [parameters, byPolicy];
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
