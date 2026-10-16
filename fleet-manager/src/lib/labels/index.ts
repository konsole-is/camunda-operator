// This module runs in the browser and on the server, so it imports nothing.

/**
 * ENVIRONMENT_LABEL on a namespace names the Environment that the namespace
 * belongs to. It is LabelEnvironment of api/fleet/v1alpha1.
 */
export const ENVIRONMENT_LABEL = "fleet.camunda.io/environment";

// The label rules of Kubernetes: a name of at most 63 characters, and a key
// with an optional DNS subdomain prefix of at most 253 characters.
const NAME = /^[A-Za-z0-9]([-A-Za-z0-9_.]{0,61}[A-Za-z0-9])?$/;
const PREFIX =
  /^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$/;

/** isLabelKey says whether key is a valid Kubernetes label key: name or prefix/name. */
export function isLabelKey(key: string): boolean {
  const parts = key.split("/");
  if (parts.length > 2) return false;
  const name = parts.at(-1) ?? "";
  const prefix = parts.length === 2 ? parts[0] : undefined;
  return (
    NAME.test(name) &&
    (prefix === undefined || (prefix.length <= 253 && PREFIX.test(prefix)))
  );
}

/** isLabelValue says whether value is a valid Kubernetes label value. The empty value is valid. */
export function isLabelValue(value: string): boolean {
  return value === "" || NAME.test(value);
}
