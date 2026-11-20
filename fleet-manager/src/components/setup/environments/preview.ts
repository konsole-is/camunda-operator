import type { Environment } from "@/generated/crds/environment";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import type { PluginSection } from "@/generated/crds/pluginsection";
import type { TemplateClaim } from "@/lib/claims/templates";
import { parseTemplate } from "@/lib/gotemplate";
import { byCodeUnit, type ClaimKind, type PluginSelection } from "@/lib/policy";
import type { ValidationError } from "@/lib/schema";
import { labelledNamespaces } from "./rows";
import type { EnvironmentSources } from "./sources";

/** PreviewClaim is a claim that a template preview renders for. example marks a claim that does not exist. */
export type PreviewClaim = TemplateClaim & { example?: true };

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
    .map((c): PreviewClaim => ({
      kind: c.kind,
      namespace: c.metadata.namespace ?? "",
      name: c.metadata.name ?? "",
      plugins: (c.spec.plugins ?? []) as PluginSelection[],
      ...(c.kind === "CamundaClusterClaim" &&
        c.spec.releaseRef && { releaseRef: c.spec.releaseRef }),
    }))
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

/** emptyPreviewText says why previewClaims gives no claim in environment. */
export function emptyPreviewText(
  s: EnvironmentSources,
  environment: string,
): string {
  return labelledNamespaces(environment, s.namespaces).length === 0
    ? "Add a namespace first. The preview renders the template for a claim of the environment."
    : "No section of this option applies to a claim, so the preview has no claim to render for.";
}

/**
 * previewSection returns the section of registration that a preview of
 * claim renders for: the section of the selection of registration in the
 * claim, else a section whose policy in spec allows registration, else any
 * section of registration. Each one applies to the kind of the claim. It
 * returns undefined when no section of registration applies to that kind.
 */
export function previewSection(
  claim: PreviewClaim,
  registration: PluginRegistration,
  sections: readonly PluginSection[],
  spec: Environment["spec"],
): PluginSection | undefined {
  const name = registration.metadata.name ?? "";
  const applying = sections.filter(
    (s) =>
      registration.spec.sections.includes(s.metadata.name ?? "") &&
      s.spec.appliesTo.includes(claim.kind),
  );
  const selected = claim.plugins.find((p) => p.registration === name)?.section;
  return (
    applying.find((s) => s.metadata.name === selected) ??
    applying.find((s) =>
      (spec.sections?.[s.metadata.name ?? ""]?.allowed ?? []).includes(name),
    ) ??
    applying[0]
  );
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
