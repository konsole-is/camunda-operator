import type { Environment } from "@/generated/crds/environment";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import type { PluginSection } from "@/generated/crds/pluginsection";
import { isObject, own } from "./json";
import { hasValue, parameterPolicies } from "./resolve";
import type { ClaimKind } from "./types";

/**
 * declaresRelease reports whether the parameter schema of registration has
 * a top-level string property releaseRef. The claim controller of a cluster
 * claim sets that parameter to the release of the claim
 * (requestParameters of pkg/components/claimrequests).
 */
export function declaresRelease(registration: PluginRegistration): boolean {
  const schema = registration.spec.parameters;
  const properties = isObject(schema) ? schema.properties : undefined;
  return (
    isObject(properties) &&
    isObject(properties.releaseRef) &&
    properties.releaseRef.type === "string"
  );
}

/**
 * OptionPick is one way for a claim to pick an option: a claim of kind in
 * section. kind is absent for a section that sections does not hold.
 */
export type OptionPick = { section: string; kind?: ClaimKind };

/**
 * optionPicks returns one pick for each claim kind of each section that
 * allows registration: the sections of environment that list it as allowed,
 * or the sections of registration without an environment.
 */
export function optionPicks(
  registration: PluginRegistration,
  environment: Environment | undefined,
  sections: readonly PluginSection[],
): OptionPick[] {
  const name = registration.metadata.name ?? "";
  const allowing = environment
    ? Object.entries(environment.spec.sections ?? {})
        .filter(([, policy]) => policy.allowed?.includes(name))
        .map(([section]) => section)
    : registration.spec.sections;
  return allowing.flatMap((section) => {
    const kinds = sections.find((s) => s.metadata.name === section)?.spec
      .appliesTo;
    return kinds ? kinds.map((kind) => ({ section, kind })) : [{ section }];
  });
}

/** UnsetParameter is a required parameter and the picks of its option that give it no value. */
export type UnsetParameter = { name: string; picks: OptionPick[] };

/**
 * unsetRequired returns parameters that the parameter schema of
 * registration requires and that a claim of some optionPicks gets no value
 * for, in the order of the required list. Such a parameter gets no value
 * from environment or from the registration (where null is no value and ""
 * is a value), a claim cannot change it, and the claim controller does not
 * set it for that pick. A parameterValues that is not a JSON object counts
 * as no values.
 */
export function unsetRequired(
  registration: PluginRegistration,
  environment: Environment | undefined,
  sections: readonly PluginSection[],
): UnsetParameter[] {
  let result = parameterPolicies(registration, environment);
  if (!("value" in result)) {
    result = parameterPolicies(
      {
        ...registration,
        spec: { ...registration.spec, parameterValues: undefined },
      },
      environment,
    );
  }
  const policies = "value" in result ? result.value : {};

  const schema = registration.spec.parameters;
  const required =
    isObject(schema) && Array.isArray(schema.required)
      ? schema.required.filter((n): n is string => typeof n === "string")
      : [];
  const picks = optionPicks(registration, environment, sections);
  const release = declaresRelease(registration);
  // The claim controller sets databases in a section with the slot
  // managementDatabases (WithDatabases of camundamanagementclusterclaim), and
  // releaseRef to the release that a cluster claim names or gets from its
  // environment (requestParameters of claimrequests).
  const setByController = (name: string, pick: OptionPick) =>
    (name === "releaseRef" && release && pick.kind === "CamundaClusterClaim") ||
    (name === "databases" &&
      pick.kind === "CamundaManagementClusterClaim" &&
      sections.find((s) => s.metadata.name === pick.section)?.spec.slot ===
        "managementDatabases");
  return required.flatMap((name) => {
    const policy = own(policies, name) ?? {};
    if (policy.claimEditable || hasValue(policy)) return [];
    const unset = picks.filter((pick) => !setByController(name, pick));
    return unset.length > 0 ? [{ name, picks: unset }] : [];
  });
}
