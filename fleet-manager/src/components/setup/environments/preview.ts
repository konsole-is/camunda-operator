import type { Environment } from "@/generated/crds/environment";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import type { PluginRequest } from "@/generated/crds/pluginrequest";
import type { PluginSection } from "@/generated/crds/pluginsection";
import { requestName } from "@/lib/claims/requests";
import { parseTemplate, type TemplateData } from "@/lib/gotemplate";
import {
  byCodeUnit,
  type ClaimKind,
  type PluginSelection,
  resolve,
  targetServiceAccount,
} from "@/lib/policy";
import type { ValidationError } from "@/lib/schema";
import { labelledNamespaces } from "./rows";
import type { EnvironmentSources } from "./sources";

/** PreviewClaim is a claim that a template preview renders for. example marks a claim that does not exist. */
export type PreviewClaim = {
  kind: ClaimKind;
  namespace: string;
  name: string;
  plugins: PluginSelection[];
  example?: true;
};

const KINDS: readonly ClaimKind[] = [
  "CamundaClusterClaim",
  "CamundaManagementClusterClaim",
];

/**
 * previewClaims returns the claims of the namespaces of environment whose
 * kind a section of the registration applies to, by namespace and name.
 * Without one it returns the claim example in the first namespace of the
 * environment, of the first such kind, and none when the environment has no
 * namespace.
 */
export function previewClaims(
  s: EnvironmentSources,
  environment: string,
  registration: string,
): PreviewClaim[] {
  const namespaces = labelledNamespaces(environment, s.namespaces);
  const served =
    s.registrations.find((r) => r.metadata.name === registration)?.spec
      .sections ?? [];
  const kinds = KINDS.filter((kind) =>
    s.sections.some(
      (section) =>
        served.includes(section.metadata.name ?? "") &&
        section.spec.appliesTo.includes(kind),
    ),
  );
  const claims = [...s.clusterClaims, ...s.planeClaims]
    .filter(
      (c) =>
        namespaces.includes(c.metadata.namespace ?? "") &&
        kinds.includes(c.kind),
    )
    .map(
      (c): PreviewClaim => ({
        kind: c.kind,
        namespace: c.metadata.namespace ?? "",
        name: c.metadata.name ?? "",
        plugins: (c.spec.plugins ?? []) as PluginSelection[],
      }),
    )
    .sort(
      (a, b) =>
        byCodeUnit(a.namespace, b.namespace) || byCodeUnit(a.name, b.name),
    );
  if (claims.length > 0 || namespaces.length === 0 || kinds.length === 0)
    return claims;
  return [
    {
      kind: kinds[0],
      namespace: namespaces[0],
      name: "example",
      plugins: [],
      example: true,
    },
  ];
}

/**
 * templateData returns the placeholder values of claim for the request of
 * registration in section. A live request of the claim gives its name and
 * its target account; else requestName and targetServiceAccount give them.
 * parameters are the request parameters that resolve gives under
 * environment, with the selection of the claim, or none. outputs is empty,
 * because a template parameter cannot read them. A policy that resolve
 * refuses gives no parameters.
 */
export function templateData(input: {
  claim: PreviewClaim;
  section: PluginSection;
  registration: PluginRegistration;
  environment: Environment;
  registrations: readonly PluginRegistration[];
  requests: readonly PluginRequest[];
}): TemplateData {
  const { claim, section, registration } = input;
  const sectionName = section.metadata.name ?? "";
  const registrationName = registration.metadata.name ?? "";
  const live = input.requests.find(
    (r) =>
      r.metadata.namespace === claim.namespace &&
      r.spec.claimRef.kind === claim.kind &&
      r.spec.claimRef.name === claim.name &&
      r.spec.section === sectionName &&
      r.spec.registration === registrationName,
  );
  const selection = claim.plugins.find(
    (p) => p.section === sectionName && p.registration === registrationName,
  ) ?? { section: sectionName, registration: registrationName };
  const resolved = resolve({
    environment: input.environment,
    registrations: [...input.registrations],
    selections: [selection],
  });
  const parameters =
    ("value" in resolved && resolved.value.requests?.[0]?.parameters) || {};
  return {
    namespace: claim.namespace,
    claim: claim.name,
    request:
      live?.metadata.name ??
      requestName(claim.name, selection, section.spec.selection),
    serviceAccount:
      live?.spec.target.serviceAccountName ??
      targetServiceAccount(claim.kind, claim.name, section),
    parameters,
    outputs: {},
  };
}

/**
 * templateErrors returns the error of each string of value that is not a
 * valid template parameter, with the path of the string in value, such as
 * "Statement[0].Resource", or "" for a string value.
 */
export function templateErrors(value: unknown, path = ""): ValidationError[] {
  if (typeof value === "string") {
    const parsed = parseTemplate(value, "parameter");
    return "error" in parsed ? [{ path, message: parsed.error }] : [];
  }
  if (Array.isArray(value))
    return value.flatMap((item, i) => templateErrors(item, `${path}[${i}]`));
  if (typeof value === "object" && value !== null)
    return Object.entries(value).flatMap(([key, item]) =>
      templateErrors(item, path === "" ? key : `${path}.${key}`),
    );
  return [];
}
