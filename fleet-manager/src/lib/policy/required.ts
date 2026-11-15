import type { Environment } from "@/generated/crds/environment";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import { builtinEntry } from "./builtins";
import { isObject, own } from "./json";
import { hasValue, parameterPolicies } from "./resolve";

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
 * unsetRequired returns the names of the parameters that the parameter
 * schema of registration requires and that no claim can get a value for,
 * in the order of the required list. Such a parameter gets no value from
 * environment or from the registration (parameterPolicies, where null is no
 * value and "" is a value), and a claim cannot change it. It leaves out the
 * parameters that the claim controller sets: the hidden parameters of a
 * built-in registration, and releaseRef when declaresRelease. A
 * parameterValues that is not a JSON object counts as no values.
 */
export function unsetRequired(
  registration: PluginRegistration,
  environment: Environment | undefined,
): string[] {
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
  const hidden = builtinEntry(registration)?.hidden ?? [];
  const release = declaresRelease(registration);
  return required.filter((name) => {
    if (hidden.includes(name)) return false;
    if (release && name === "releaseRef") return false;
    const policy = own(policies, name) ?? {};
    return !policy.claimEditable && !hasValue(policy);
  });
}
