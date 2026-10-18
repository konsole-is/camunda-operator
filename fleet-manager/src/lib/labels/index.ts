// This module runs in the browser and on the server, so it imports nothing.

/**
 * ENVIRONMENT_LABEL on a namespace names the Environment that the namespace
 * belongs to. It is LabelEnvironment of api/fleet/v1alpha1.
 */
export const ENVIRONMENT_LABEL = "fleet.camunda.io/environment";

/** CLAIM_LABEL names the claim that made an object. It is LabelClaim of api/fleet/v1alpha1. */
export const CLAIM_LABEL = "fleet.camunda.io/claim";

/** REVISION_ANNOTATION is AnnotationRevision of api/fleet/v1alpha1. */
export const REVISION_ANNOTATION = "fleet.camunda.io/revision";

/** SECTION_LABEL names the section of a PluginRequest. It is LabelSection of api/plugins/v1alpha1. */
export const SECTION_LABEL = "plugins.camunda.io/section";

/**
 * CLUSTER_LABEL on a pod holds labels.OwnerName of its CamundaCluster. It is
 * ClusterKey of pkg/labels.
 */
export const CLUSTER_LABEL = "camunda.io/cluster";

/** COMPONENT_LABEL on a pod names its component. It is ComponentKey of pkg/labels. */
export const COMPONENT_LABEL = "camunda.io/component";

/** isManaged says whether a claim made object: it carries CLAIM_LABEL with a value. */
export function isManaged(object: {
  metadata?: { labels?: Record<string, string> };
}): boolean {
  return Boolean(object.metadata?.labels?.[CLAIM_LABEL]);
}

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
