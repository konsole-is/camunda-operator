// This module runs in the browser and on the server.
import type { NamespaceChoice } from "@/components/fleet/namespace-picker";
import type { SecretRefs } from "@/components/fleet/secret-ref-picker";
import type {
  CamundaManagementClusterClaim,
  CamundaManagementClusterClaimSpec,
} from "@/generated/crds/camundamanagementclusterclaim";
import type { CamundaPlatformConfig } from "@/generated/crds/camundaplatformconfig";
import type { CamundaRelease } from "@/generated/crds/camundarelease";
import type { Environment } from "@/generated/crds/environment";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import type { PluginRequest } from "@/generated/crds/pluginrequest";
import type { PluginSection } from "@/generated/crds/pluginsection";
import { claimNameErrors } from "@/lib/claims/names";
import { syncedNamespace } from "@/lib/clusters/draft";
import { environmentReady } from "@/lib/environments";
import type { LabelSelector } from "@/lib/labels";
import type { PluginSelection } from "@/lib/policy";
import type { ValidationError } from "@/lib/schema";
import type { ImagesTable } from "@/startup/bundled-data";
import { createChangeOf } from "@/lib/write/create";
import type { Change } from "@/lib/write/types";
import {
  type AdminDraft,
  EMPTY_ADMIN,
  EMPTY_MAIL,
  type MailDraft,
  adminErrors,
  mailErrors,
  mailSecretErrors,
  runningOf,
} from "./components";
import {
  type ExternalKeycloakDraft,
  type IdentityModeId,
  authMethod,
  externalKeycloakErrors,
} from "./identity";
import { accessSelection, planeSelections, runningSelection } from "./storage";
import { type PlaneUrlId, planeUrlFields } from "./urls";
import {
  type PlaneVersions,
  type VersionPrefill,
  defaultRelease,
  versionErrors,
  versionPrefill,
  versionsOf,
} from "./versions";

/**
 * PlaneDraft is the draft of the plane create flow. environment is the
 * object name of the Environment of step 1. platformConfig is the pick of
 * the admin; undefined keeps the default of the environment. served is the
 * pick of step 2; undefined serves the environment of step 1. versions
 * holds only the versions that the admin typed. storage holds the
 * selections by section name, once step 4 showed the section. urls are the
 * URLs typed for "No access from outside". contractName "" means the claim
 * name.
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
  admin: AdminDraft;
  console: boolean;
  webModeler: boolean;
  mail: MailDraft;
  versions: PlaneVersions;
  storage: Record<string, PluginSelection[]>;
  urls: Partial<Record<PlaneUrlId, string>>;
  contractName: string;
};

/** EMPTY_PLANE_DRAFT is the draft that the flow starts from. */
export const EMPTY_PLANE_DRAFT: PlaneDraft = {
  name: "",
  externalKeycloak: { url: "", realm: "camunda-platform", adminSecret: "" },
  clusterLabels: {},
  admin: EMPTY_ADMIN,
  console: false,
  webModeler: false,
  mail: EMPTY_MAIL,
  versions: {},
  storage: {},
  urls: {},
  contractName: "",
};

/**
 * PlaneContext is the data that the steps check a draft against.
 * takenNames are the names of the plane claims and CamundaManagementClusters
 * in the namespace of the draft. namespaces are the live namespaces, each
 * with the name of its Environment. images is the images table of the
 * fleet manager. accessRequests are the requests of the access section,
 * and planeClaims the plane claims, both of every namespace. authConfigs
 * are the names of the ManagementAuthConfigs.
 */
export type PlaneContext = {
  environments: readonly Environment[];
  platformConfigs: readonly CamundaPlatformConfig[];
  takenNames: readonly string[];
  namespaces: readonly { name: string; environment?: string }[];
  releases: readonly CamundaRelease[];
  images: ImagesTable;
  sections: readonly PluginSection[];
  registrations: readonly PluginRegistration[];
  accessRequests: readonly PluginRequest[];
  authConfigs: readonly string[];
  planeClaims: readonly CamundaManagementClusterClaim[];
};

/** PlaneClaim is the CamundaManagementClusterClaim that the plane create flow writes. It always has a clusterSelector. */
export type PlaneClaim = {
  apiVersion: "fleet.camunda.io/v1alpha1";
  kind: "CamundaManagementClusterClaim";
  metadata: { name: string; namespace: string };
  spec: CamundaManagementClusterClaimSpec & { clusterSelector: LabelSelector };
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
 * planePrefill is the prefill of the versions of draft: versionPrefill of
 * the default release of the first environment of servedOf(draft).
 */
export function planePrefill(
  draft: PlaneDraft,
  ctx: PlaneContext,
): VersionPrefill | undefined {
  return versionPrefill(ctx.images, planeRelease(draft, ctx));
}

/** planeRelease is the default release of the first environment that draft serves. */
export function planeRelease(
  draft: PlaneDraft,
  ctx: PlaneContext,
): CamundaRelease | undefined {
  return defaultRelease(servedOf(draft), ctx.environments, ctx.releases);
}

/** planeVersions returns the version of each app that the plane of draft runs: the typed one, else the prefill. */
export function planeVersions(
  draft: PlaneDraft,
  ctx: PlaneContext,
): PlaneVersions {
  return versionsOf(draft.versions, planePrefill(draft, ctx), runningOf(draft));
}

/**
 * componentsErrors returns the errors of step 3: the admin fields of the
 * mode, the mail while Web Modeler is on, then the errors of planeVersions.
 * With secrets, the Secrets of the namespace of draft, a credentials Secret
 * that the mail names while Web Modeler is on must be among smtpSecrets of
 * them.
 */
export function componentsErrors(
  draft: PlaneDraft,
  ctx: PlaneContext,
  secrets?: SecretRefs,
): string[] {
  return [
    ...adminErrors(draft),
    ...(draft.webModeler ? mailErrors(draft.mail) : []),
    ...(draft.webModeler && secrets
      ? mailSecretErrors(draft.mail, secrets)
      : []),
    ...versionErrors(planeVersions(draft, ctx), runningOf(draft)),
  ];
}

/**
 * planeClaimOf returns the claim of draft. clusterSelector is never unset:
 * an unset selector selects no cluster, so no cluster labels give {}.
 * platformConfigRef is set only when the pick differs from the platform
 * config of the environment. The admin holds only the fields of the mode.
 * Console, Web Modeler, and their versions are set only while they run,
 * and versions.keycloak only in Keycloak mode. Without an access
 * selection, each typed URL of a running app is set. A selection of
 * existing-database-configs names no DatabaseConfig of an app that does
 * not run. managementAuthConfigName is set only when it differs from the
 * claim name.
 */
export function planeClaimOf(draft: PlaneDraft, ctx: PlaneContext): PlaneClaim {
  const own = environmentOf(draft, ctx)?.spec.platformConfigRef;
  const ref =
    draft.platformConfig !== undefined && draft.platformConfig !== own
      ? draft.platformConfig
      : undefined;
  const versions = planeVersions(draft, ctx);
  const running = runningOf(draft);
  const plugins = planeSelections(draft).map((p) =>
    runningSelection(p, running),
  );
  const contract =
    draft.contractName !== "" && draft.contractName !== draft.name
      ? draft.contractName
      : undefined;
  const management: Management = {
    identityProvider: identityProviderOf(draft),
    identity: { admin: adminOf(draft) },
    ...(draft.console && { console: {} }),
    ...(draft.webModeler && { webModeler: { mail: mailOf(draft.mail) } }),
  };
  if (!accessSelection(draft)) {
    for (const field of planeUrlFields(running)) {
      const url = draft.urls[field.id];
      if (url) setIn(management, field.names, url);
    }
  }
  return {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "CamundaManagementClusterClaim",
    metadata: { name: draft.name, namespace: draft.namespace?.name ?? "" },
    spec: {
      // The CRD needs one environment or more; the served step checks it.
      environments: servedOf(draft) as [string, ...string[]],
      clusterSelector:
        Object.keys(draft.clusterLabels).length === 0
          ? {}
          : { matchLabels: { ...draft.clusterLabels } },
      ...(ref !== undefined && { platformConfigRef: ref }),
      management,
      versions: { ...versions, identity: versions.identity ?? "" },
      ...(plugins.length > 0 && { plugins }),
      ...(contract !== undefined && { managementAuthConfigName: contract }),
    },
  };
}

/**
 * planeCreateChange returns the create change of draft: the new namespace
 * with the labels of syncedNamespace under its environment when the draft
 * makes one, then the claim of planeClaimOf.
 */
export function planeCreateChange(
  draft: PlaneDraft,
  ctx: PlaneContext,
): Change {
  const claim = planeClaimOf(draft, ctx);
  const { namespace } = claim.metadata;
  const choice = syncedNamespace(draft.namespace, environmentOf(draft, ctx));
  return createChangeOf(
    `Create the management plane ${claim.metadata.name} in ${namespace}`,
    claim,
    choice?.kind === "new"
      ? { name: choice.name, labels: { ...choice.labels } }
      : undefined,
  );
}

// setIn sets value at names under target. Each object on the way exists:
// planeUrlFields gives only the fields of running apps, whose objects
// planeClaimOf writes.
function setIn(target: object, names: readonly string[], value: string) {
  let at = target as Record<string, unknown>;
  for (const name of names.slice(0, -1)) {
    at = at[name] as Record<string, unknown>;
  }
  at[names.at(-1)!] = value;
}

type Management = CamundaManagementClusterClaimSpec["management"];

function adminOf(draft: PlaneDraft): Management["identity"]["admin"] {
  const { admin } = draft;
  if (draft.mode === "oidc") {
    return { claimName: admin.claimName, claimValue: admin.claimValue };
  }
  return {
    username: admin.username,
    ...(admin.email !== "" && { email: admin.email }),
    ...(admin.password && { passwordSecretRef: { ...admin.password } }),
  };
}

function mailOf(
  mail: MailDraft,
): NonNullable<Management["webModeler"]>["mail"] {
  return {
    smtpHost: mail.smtpHost,
    smtpPort: mail.smtpPort,
    fromAddress: mail.fromAddress,
    ...(mail.fromName !== "" && { fromName: mail.fromName }),
    tls: mail.tls,
    ...(mail.credentialsSecret !== "" && {
      credentialsSecretRef: { name: mail.credentialsSecret },
    }),
  };
}

function identityProviderOf(draft: PlaneDraft): Management["identityProvider"] {
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
