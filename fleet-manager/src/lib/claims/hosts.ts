// This module runs in the browser and on the server.
import type { Environment } from "@/generated/crds/environment";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import type { PluginRequest } from "@/generated/crds/pluginrequest";
import type { PluginSection } from "@/generated/crds/pluginsection";
import { ACCESS_SECTION } from "@/lib/labels";
import {
  type ClaimKind,
  isObject,
  parameterPolicies,
  sectionOptions,
} from "@/lib/policy";

/** HOST_PATTERN_PARAMETERS are the access parameters that hold a host pattern (contract D15). */
export const HOST_PATTERN_PARAMETERS = [
  "hostnamePattern",
  "grpcHostnamePattern",
  "optimizeHostnamePattern",
  "managementHostnamePattern",
] as const;

/** RenderedHost is the host that one pattern parameter gives a claim. */
export type RenderedHost = { parameter: string; host: string };

/**
 * renderHosts returns the host of each pattern parameter in patterns, in the
 * order of HOST_PATTERN_PARAMETERS, as RenderHosts of the gateway-access
 * plugin renders it (contract D15): {name} and {namespace} replaced, in
 * lower case. A parameter that is not a non-empty string gives no host. It
 * returns no host while name or namespace is empty, so a host never shows
 * with an empty label.
 */
export function renderHosts(
  patterns: Record<string, unknown>,
  name: string,
  namespace: string,
): RenderedHost[] {
  if (name === "" || namespace === "") return [];
  const hosts: RenderedHost[] = [];
  for (const parameter of HOST_PATTERN_PARAMETERS) {
    const pattern = Object.hasOwn(patterns, parameter)
      ? patterns[parameter]
      : undefined;
    if (typeof pattern !== "string" || pattern === "") continue;
    const host = pattern
      .replaceAll("{name}", name)
      .replaceAll("{namespace}", namespace)
      .toLowerCase();
    hosts.push({ parameter, host });
  }
  return hosts;
}

/**
 * defaultAccessPatterns returns the host patterns of the default option of
 * the access section for a claim of claimKind under environment, by
 * parameter, with only the patterns that have a value. It returns undefined
 * when no access section applies to claimKind, when the section has no
 * default option, or when the policy of the default registration is
 * malformed. A create flow shows these hosts before the admin picks an
 * access option, so the step that picks the option must check its hosts
 * again (decision 13).
 */
export function defaultAccessPatterns(
  sections: readonly PluginSection[],
  registrations: readonly PluginRegistration[],
  environment: Environment | undefined,
  claimKind: ClaimKind,
): Record<string, unknown> | undefined {
  const section = sections.find((s) => s.metadata.name === ACCESS_SECTION);
  if (!section) return undefined;
  const option = sectionOptions(
    section,
    registrations,
    environment,
    claimKind,
  ).options.find((o) => o.isDefault);
  const registration = registrations.find(
    (r) => r.metadata.name === option?.registration,
  );
  if (!registration) return undefined;
  const result = parameterPolicies(registration, environment);
  if (!("value" in result)) return undefined;
  const patterns: Record<string, unknown> = {};
  for (const parameter of HOST_PATTERN_PARAMETERS) {
    const policy = Object.hasOwn(result.value, parameter)
      ? result.value[parameter]
      : undefined;
    if (policy?.value !== undefined && policy.value !== null) {
      patterns[parameter] = policy.value;
    }
  }
  return patterns;
}

/**
 * takenHosts returns each host that an access request holds, in lower case,
 * with the <namespace>/<name> of its claim. A request holds the host of each
 * output that is an http or https URL or a bare host, and the hosts that its
 * parameters render, so a request without outputs yet holds its hosts too.
 * When two requests give one host, the first one keeps it.
 */
export function takenHosts(
  requests: readonly PluginRequest[],
): Map<string, string> {
  const taken = new Map<string, string>();
  for (const request of requests) {
    const namespace = request.metadata.namespace ?? "";
    const claim = request.spec.claimRef.name;
    const owner = `${namespace}/${claim}`;
    const hold = (host: string | undefined) => {
      if (host && !taken.has(host)) taken.set(host, owner);
    };
    for (const value of Object.values(request.status?.outputs ?? {})) {
      hold(hostOf(value));
    }
    if (isObject(request.spec.parameters)) {
      for (const { host } of renderHosts(
        request.spec.parameters,
        claim,
        namespace,
      )) {
        hold(host);
      }
    }
  }
  return taken;
}

/**
 * hostConflicts returns one message for each host of hosts that taken holds,
 * once per host, naming the claim that holds it.
 */
export function hostConflicts(
  hosts: readonly RenderedHost[],
  taken: ReadonlyMap<string, string>,
): string[] {
  const conflicts: string[] = [];
  const seen = new Set<string>();
  for (const { host } of hosts) {
    const claim = taken.get(host);
    if (claim === undefined || seen.has(host)) continue;
    seen.add(host);
    conflicts.push(
      `The host ${host} is taken by ${claim}. Choose another name.`,
    );
  }
  return conflicts;
}

function hostOf(value: string): string | undefined {
  if (URL.canParse(value)) {
    const url = new URL(value);
    if (url.protocol === "http:" || url.protocol === "https:") {
      return url.hostname.toLowerCase();
    }
  }
  if (value === "" || value.includes("/") || /\s/.test(value)) {
    return undefined;
  }
  return value.toLowerCase();
}
