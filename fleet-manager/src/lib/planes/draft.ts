// This module runs in the browser and on the server.
import type { NamespaceChoice } from "@/components/fleet/namespace-picker";
import type { CamundaManagementClusterClaimSpec } from "@/generated/crds/camundamanagementclusterclaim";
import type { CamundaPlatformConfig } from "@/generated/crds/camundaplatformconfig";
import type { Environment } from "@/generated/crds/environment";
import { claimNameErrors } from "@/lib/claims/names";
import { environmentReady } from "@/lib/environments";
import type { LabelSelector } from "@/lib/labels";
import type { ValidationError } from "@/lib/schema";
import {
  type ExternalKeycloakDraft,
  type IdentityModeId,
  authMethod,
  externalKeycloakErrors,
} from "./identity";

/**
 * PlaneDraft is the draft of the plane create flow. environment is the
 * object name of the Environment of step 1. platformConfig is the pick of
 * the admin; undefined keeps the default of the environment. served is the
 * pick of step 2; undefined serves the environment of step 1.
 */
export type PlaneDraft = {
  environment?: string;
  name: string;
  namespace?: NamespaceChoice;
  platformConfig?: string;
  mode?: IdentityModeId;
  externalKeycloak: ExternalKeycloakDraft;
  served?: string[];
  clusterLabels: Record<string, string>;
};

/** EMPTY_PLANE_DRAFT is the draft that the flow starts from. */
export const EMPTY_PLANE_DRAFT: PlaneDraft = {
  name: "",
  externalKeycloak: { url: "", realm: "camunda-platform", adminSecret: "" },
  clusterLabels: {},
};

/**
 * PlaneContext is the live data that the steps check a draft against.
 * takenNames are the names of the plane claims and CamundaManagementClusters
 * in the namespace of the draft. namespaces are the live namespaces, each
 * with the name of its Environment.
 */
export type PlaneContext = {
  environments: readonly Environment[];
  platformConfigs: readonly CamundaPlatformConfig[];
  takenNames: readonly string[];
  namespaces: readonly { name: string; environment?: string }[];
};

/** PlaneClaimBasics is the part of a CamundaManagementClusterClaim that steps 1 and 2 decide. */
export type PlaneClaimBasics = {
  apiVersion: "fleet.camunda.io/v1alpha1";
  kind: "CamundaManagementClusterClaim";
  metadata: { name: string; namespace: string };
  spec: {
    environments: string[];
    clusterSelector: LabelSelector;
    platformConfigRef?: string;
    management: {
      identityProvider: CamundaManagementClusterClaimSpec["management"]["identityProvider"];
    };
  };
};

/** NO_ENVIRONMENT is the message of step 1 while no Environment exists. A plane cannot be made then. */
export const NO_ENVIRONMENT =
  "No Environment exists. A management plane serves environments, so create an Environment first.";

/** OIDC_NEEDS_OIDC_CONFIG is the error of OIDC mode with a platform config that uses basic authentication. */
export const OIDC_NEEDS_OIDC_CONFIG =
  "OIDC needs a platform config with OIDC authentication.";

/** platformConfigOf is the platform config that the plane uses: the pick, else the Environment's, else undefined. */
export function platformConfigOf(
  draft: PlaneDraft,
  ctx: PlaneContext,
): string | undefined {
  return (
    draft.platformConfig ?? environmentOf(draft, ctx)?.spec.platformConfigRef
  );
}

/** servedOf is the environments that the plane serves: the pick, else the environment of step 1, else none. */
export function servedOf(draft: PlaneDraft): string[] {
  return draft.served ?? (draft.environment ? [draft.environment] : []);
}

/**
 * basicsErrors returns the errors of step 1 in screen order. Without any
 * Environment it returns only NO_ENVIRONMENT. A picked environment that is
 * gone counts as none, and one that turned Ready=False refuses the step
 * with its message. A namespace that the live data no longer allows is
 * refused.
 */
export function basicsErrors(draft: PlaneDraft, ctx: PlaneContext): string[] {
  if (ctx.environments.length === 0) return [NO_ENVIRONMENT];
  const errors: string[] = [];
  const environment = environmentOf(draft, ctx);
  if (!environment) {
    errors.push("Pick an environment.");
  } else {
    const { ready, message } = environmentReady(environment);
    if (!ready) {
      errors.push(
        `The environment ${environment.spec.displayName || draft.environment} is not ready${message ? `: ${message}` : ""}`,
      );
    }
  }
  errors.push(...claimNameErrors(draft.name, ctx.takenNames));
  errors.push(...namespaceErrors(draft, environment, ctx));
  const configName = platformConfigOf(draft, ctx);
  const config = ctx.platformConfigs.find(
    (c) => c.metadata.name === configName,
  );
  if (!configName) {
    errors.push("Pick a platform config.");
  } else if (!config) {
    errors.push(
      draft.platformConfig === undefined
        ? `Platform config ${configName} of the environment does not exist. Pick another one.`
        : `Platform config ${configName} does not exist. Pick another one.`,
    );
  }
  if (!draft.mode) errors.push("Pick where users sign in.");
  if (draft.mode === "oidc" && config && authMethod(config) === "basic") {
    errors.push(OIDC_NEEDS_OIDC_CONFIG);
  }
  if (draft.mode === "externalKeycloak") {
    errors.push(
      ...externalKeycloakErrors(draft.externalKeycloak).map((e) => e.message),
    );
  }
  return errors;
}

/**
 * servedErrors returns the errors of step 2: no environment, an
 * environment that does not exist, and the errors of the cluster labels
 * as one message.
 */
export function servedErrors(
  draft: PlaneDraft,
  ctx: PlaneContext,
  selectorErrors: readonly ValidationError[],
): string[] {
  const served = servedOf(draft);
  const errors: string[] = [];
  if (served.length === 0) errors.push("Pick at least one environment.");
  for (const name of served) {
    if (!ctx.environments.some((e) => e.metadata.name === name)) {
      errors.push(`Environment ${name} does not exist.`);
    }
  }
  if (selectorErrors.length > 0) errors.push("Fix the cluster labels.");
  return errors;
}

/**
 * planeClaimOf returns the claim of draft as far as steps 1 and 2 decide
 * it. clusterSelector is never unset: an unset selector selects no cluster,
 * so no cluster labels give {}. platformConfigRef is set only when the
 * pick differs from the platform config of the environment.
 */
export function planeClaimOf(
  draft: PlaneDraft,
  ctx: PlaneContext,
): PlaneClaimBasics {
  const own = environmentOf(draft, ctx)?.spec.platformConfigRef;
  const ref =
    draft.platformConfig !== undefined && draft.platformConfig !== own
      ? draft.platformConfig
      : undefined;
  return {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "CamundaManagementClusterClaim",
    metadata: { name: draft.name, namespace: draft.namespace?.name ?? "" },
    spec: {
      environments: servedOf(draft),
      clusterSelector:
        Object.keys(draft.clusterLabels).length === 0
          ? {}
          : { matchLabels: { ...draft.clusterLabels } },
      ...(ref !== undefined && { platformConfigRef: ref }),
      management: { identityProvider: identityProviderOf(draft) },
    },
  };
}

function identityProviderOf(
  draft: PlaneDraft,
): PlaneClaimBasics["spec"]["management"]["identityProvider"] {
  switch (draft.mode) {
    case "keycloak":
      return { keycloak: {} };
    case "oidc":
      return { oidc: {} };
    case "externalKeycloak": {
      const { url, realm, adminSecret, caBundle } = draft.externalKeycloak;
      // No usernameKey or passwordKey: the CRD defaults apply.
      return {
        externalKeycloak: {
          url,
          ...(realm !== "" && { realm }),
          adminCredentialsSecretRef: { name: adminSecret },
          ...(caBundle && { caBundleSecretRef: { ...caBundle } }),
        },
      };
    }
    default:
      return {};
  }
}

function namespaceErrors(
  draft: PlaneDraft,
  environment: Environment | undefined,
  ctx: PlaneContext,
): string[] {
  const choice = draft.namespace;
  const pick = "Pick or create a namespace.";
  if (!choice) return [pick];
  const live = ctx.namespaces.find((ns) => ns.name === choice.name);
  if (choice.kind === "existing") {
    return live && live.environment === draft.environment ? [] : [pick];
  }
  const creation = environment?.spec.namespaceCreation;
  if (!creation?.allowed) return [pick];
  const missing = (creation.requiredLabelKeys ?? []).filter(
    (key) => !Object.hasOwn(creation.labels ?? {}, key) && !choice.labels[key],
  );
  return [
    ...(live ? [`The namespace ${choice.name} already exists.`] : []),
    ...missing.map((key) => `Enter a value for ${key}.`),
  ];
}

function environmentOf(
  draft: PlaneDraft,
  ctx: PlaneContext,
): Environment | undefined {
  return ctx.environments.find((e) => e.metadata.name === draft.environment);
}
