import type { V1Namespace } from "@kubernetes/client-node";
import type { Environment } from "@/generated/crds/environment";
import { ENVIRONMENT_LABEL } from "@/lib/labels";

/** EnvironmentOf is the environment of a namespace, or the reason that the claim controller refuses a claim in it. */
export type EnvironmentOf = {
  environment?: Environment;
  reason?: "NoEnvironment" | "UnknownEnvironment";
  message?: string;
};

/**
 * environmentOf returns the Environment that the label ENVIRONMENT_LABEL of
 * namespace names. An empty label counts as no label. It returns neither an
 * environment nor a reason when the namespace has no label and no
 * Environment exists: the registrations then give the policy. The reason is
 * NoEnvironment for a namespace without the label while an Environment
 * exists, and UnknownEnvironment for a label that names no Environment.
 */
export function environmentOf(
  namespace: V1Namespace,
  environments: readonly Environment[],
): EnvironmentOf {
  const namespaceName = namespace.metadata?.name ?? "";
  const name = namespace.metadata?.labels?.[ENVIRONMENT_LABEL] ?? "";
  if (name === "") {
    if (environments.length === 0) return {};
    return {
      reason: "NoEnvironment",
      message: `namespace "${namespaceName}" has no label ${ENVIRONMENT_LABEL}, and an Environment exists`,
    };
  }

  const environment = environments.find((e) => e.metadata.name === name);
  if (environment) return { environment };
  return {
    reason: "UnknownEnvironment",
    message: `namespace "${namespaceName}" has the label ${ENVIRONMENT_LABEL}=${name}, and no Environment "${name}" exists`,
  };
}
