import type { CamundaPlatformConfig } from "@/generated/crds/camundaplatformconfig";
import type { Environment } from "@/generated/crds/environment";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import type { PluginRequest } from "@/generated/crds/pluginrequest";
import type { PluginSection } from "@/generated/crds/pluginsection";
import type { Resource } from "@/lib/k8s/kinds";
import { mailErrors, runningOfClaim } from "@/lib/planes/components";
import { mailDraftOf } from "@/lib/planes/draft";
import {
  externalKeycloakDraftOf,
  externalKeycloakErrors,
  identityModeOf,
  oidcDisabledReason,
  type IdentityModeId,
  type PlaneManagement,
} from "@/lib/planes/identity";
import {
  databaseConfigErrors,
  databaseSection,
  planeControllerParameters,
} from "@/lib/planes/storage";
import { planeUrlFields, urlErrors } from "@/lib/planes/urls";
import {
  runningFields,
  versionError,
  type PlaneVersionField,
} from "@/lib/planes/versions";
import { pathFrom, type Step } from "@/lib/fields";
import {
  checkAllowed,
  checkDeletionPolicy,
  checkSlotConflicts,
  deletionPolicyOf,
  equalJSON,
  type ResolvedPolicy,
} from "@/lib/policy";
import type { FieldError } from "./cluster";
import {
  changedParameterErrors,
  generated,
  messagesOf,
  requestsOf,
  selectionsOf,
} from "./parameters";
import { claimAfter, type Session } from "./session";
import { valueAt } from "./steps";

/**
 * PlaneClaim is a CamundaManagementClusterClaim as the cache reads it. The
 * create flow writes the narrower PlaneCreateClaim of @/lib/planes/draft.
 */
export type PlaneClaim = Resource<"CamundaManagementClusterClaim">;

/**
 * PlaneClaimContext is what validatePlaneClaim reads beside the session.
 * environment is the Environment of the namespace of the claim, and
 * environments are all Environments. requests are the live PluginRequests
 * of the claim.
 */
export type PlaneClaimContext = {
  environment?: Resource<"Environment">;
  environments: readonly Resource<"Environment">[];
  platformConfigs: readonly Resource<"CamundaPlatformConfig">[];
  sections: readonly Resource<"PluginSection">[];
  registrations: readonly Resource<"PluginRegistration">[];
  requests: readonly Resource<"PluginRequest">[];
};

type Spec = Partial<PlaneClaim["spec"]>;

const PROVIDER = "spec.management.identityProvider";
const ADMIN = "spec.management.identity.admin";
const EXTERNAL = `${PROVIDER}.externalKeycloak`;
const EXTERNAL_PATHS: Record<string, string> = {
  url: `${EXTERNAL}.url`,
  realm: `${EXTERNAL}.realm`,
  adminSecret: `${EXTERNAL}.adminCredentialsSecretRef.name`,
  caBundle: `${EXTERNAL}.caBundleSecretRef`,
};

/**
 * validatePlaneClaim returns the errors of the plane claim after the change
 * of s. An Environment name, an app URL that the claim already needed, a set
 * version of an app that already ran, the mail, the DatabaseConfigs while
 * the running apps stay, and the schema check of a section parameter skip a
 * value that s does not change, so that a claim that already breaks one of
 * those rules does not block another change. Types and ranges of the CRD are
 * left to the dry run.
 */
export function validatePlaneClaim(
  s: Session<PlaneClaim>,
  ctx: PlaneClaimContext,
): FieldError[] {
  const after = claimAfter(s);
  // A reset of the last field of spec removes spec.
  const spec: Spec = after.spec ?? {};
  const live: Spec = s.claim.spec ?? {};
  const policy = after.status?.resolvedPolicy as ResolvedPolicy | undefined;
  const selections = selectionsOf(after);
  const mode = identityModeOf(spec.management?.identityProvider);

  const errors: FieldError[] = [
    ...environmentErrors(live, spec, ctx),
    ...modeErrors(live, spec, ctx),
    ...adminErrors(mode, spec.management),
    ...(mode === "externalKeycloak" ? externalErrors(spec.management) : []),
    ...urlErrorsOf(s.claim, after, ctx),
    ...versionErrorsOf(live, spec),
    ...mailErrorsOf(live, spec),
  ];

  // Without the snapshot of the claim, only the Environment can be
  // checked: the selections need the snapshot.
  errors.push(
    ...messagesOf(
      checkAllowed({
        kind: "CamundaManagementClusterClaim",
        sections: generated<PluginSection[]>(ctx.sections),
        environment: generated<Environment | undefined>(ctx.environment),
        policy: policy ?? {},
        selections: policy ? selections : [],
        requests: generated<PluginRequest[]>(ctx.requests),
      }),
    ),
  );

  errors.push(
    ...messagesOf(
      checkSlotConflicts(generated<PluginSection[]>(ctx.sections), selections, {
        management: spec.management,
      }),
    ),
  );

  errors.push(...databaseErrorsOf(s.claim, after, ctx, policy));

  errors.push(
    ...changedParameterErrors({
      claim: s.claim,
      after,
      environment: ctx.environment,
      registrations: ctx.registrations,
      policy,
      added: planeControllerParameters(
        generated<PluginSection[]>(ctx.sections),
        runningOfClaim(spec.management),
      ),
    }),
  );

  errors.push(
    ...messagesOf(
      checkDeletionPolicy(
        deletionPolicyOf(spec.deletionPolicy, policy),
        selections,
        generated<PluginRegistration[]>(ctx.registrations),
      ),
    ),
  );

  return errors;
}

function environmentErrors(
  live: Spec,
  spec: Spec,
  ctx: PlaneClaimContext,
): FieldError[] {
  const path = "spec.environments";
  const names = stringsOf(spec.environments);
  if (names.length === 0)
    return [{ path, message: "Pick at least one environment." }];
  const before = stringsOf(live.environments);
  return names
    .filter(
      (name) =>
        !before.includes(name) &&
        !ctx.environments.some((e) => e.metadata?.name === name),
    )
    .map((name) => ({ path, message: `Environment ${name} does not exist.` }));
}

function modeErrors(
  live: Spec,
  spec: Spec,
  ctx: PlaneClaimContext,
): FieldError[] {
  const mode = identityModeOf(spec.management?.identityProvider);
  if (mode === undefined)
    return [{ path: PROVIDER, message: "Pick where users sign in." }];
  if (
    mode !== "oidc" ||
    identityModeOf(live.management?.identityProvider) === "oidc"
  )
    return [];
  const ref = spec.platformConfigRef || ctx.environment?.spec.platformConfigRef;
  const config = ctx.platformConfigs.find((c) => c.metadata?.name === ref);
  const reason = oidcDisabledReason(
    generated<CamundaPlatformConfig | undefined>(config),
  );
  return reason === undefined ? [] : [{ path: PROVIDER, message: reason }];
}

// The CEL rules of ManagementOverrides and IdentityAdminSpec, which the API
// server checks on every update.
function adminErrors(
  mode: IdentityModeId | undefined,
  management: PlaneManagement | undefined,
): FieldError[] {
  if (mode === undefined) return [];
  const admin: PlaneManagement["identity"]["admin"] =
    management?.identity?.admin ?? {};
  const error = (field: string, message: string) => ({
    path: `${ADMIN}.${field}`,
    message,
  });
  if (mode === "oidc") {
    return [
      ...(admin.claimName ? [] : [error("claimName", "Enter the claim name.")]),
      ...(admin.claimName?.includes("=")
        ? [error("claimName", "The claim name cannot hold an equals sign.")]
        : []),
      ...(admin.claimValue
        ? []
        : [error("claimValue", "Enter the claim value.")]),
      ...(admin.username !== undefined
        ? [
            error(
              "username",
              "Remove the username. OIDC mode names the administrator by a claim.",
            ),
          ]
        : []),
      ...(admin.passwordSecretRef !== undefined
        ? [
            error(
              "passwordSecretRef",
              "Remove the password Secret. OIDC mode takes none.",
            ),
          ]
        : []),
    ];
  }
  return [
    ...(admin.username ? [] : [error("username", "Enter the username.")]),
    ...(admin.claimName !== undefined || admin.claimValue !== undefined
      ? [
          error(
            "claimName",
            "Remove the claim name and value. A Keycloak mode names the administrator by a username.",
          ),
        ]
      : []),
    ...(management?.webModeler !== undefined && !admin.email
      ? [
          error(
            "email",
            "Web Modeler needs the email of the administrator in a Keycloak mode.",
          ),
        ]
      : []),
  ];
}

function externalErrors(management: PlaneManagement | undefined): FieldError[] {
  return externalKeycloakErrors(
    externalKeycloakDraftOf(management?.identityProvider?.externalKeycloak),
  ).map((e) => ({
    path: EXTERNAL_PATHS[e.path] ?? EXTERNAL,
    message: e.message,
  }));
}

// The claim controller refuses a claim without an access selection that
// leaves a URL of the plane empty (checkPlaneInputs).
function urlErrorsOf(
  claim: PlaneClaim,
  after: PlaneClaim,
  ctx: PlaneClaimContext,
): FieldError[] {
  if (hasAccess(after, ctx)) return [];
  const management = after.spec?.management;
  const before = hasAccess(claim, ctx)
    ? []
    : planeUrlFields(runningOfClaim(claim.spec?.management));
  return planeUrlFields(runningOfClaim(management)).flatMap((field) => {
    const value = stringAt(management, field.names);
    const needed = before.some((f) => f.id === field.id);
    if (needed && value === stringAt(claim.spec?.management, field.names))
      return [];
    const path = ["spec", "management", ...field.names].join(".");
    return urlErrors({ [field.id]: value }, [field]).map((message) => ({
      path,
      message,
    }));
  });
}

// The plugin refuses one DatabaseConfig for two apps, and a plane waits for
// the database of an app without one.
function databaseErrorsOf(
  claim: PlaneClaim,
  after: PlaneClaim,
  ctx: PlaneClaimContext,
  policy: ResolvedPolicy | undefined,
): FieldError[] {
  const section = databaseSection(generated<PluginSection[]>(ctx.sections));
  const selections = selectionsOf(after);
  const i = selections.findIndex((p) => p.section === section?.metadata.name);
  if (i < 0) return [];
  const { section: name, registration } = selections[i];
  const steps: Step[] = [
    { field: "spec" },
    { field: "plugins" },
    { item: { section: name, registration } },
    { field: "parameters" },
  ];
  const running = runningOfClaim(after.spec?.management);
  if (
    equalJSON(valueAt(claim, steps), selections[i].parameters) &&
    equalJSON(runningOfClaim(claim.spec?.management), running)
  )
    return [];
  const parameters = requestsOf(selections, {
    environment: ctx.environment,
    registrations: ctx.registrations,
    policy,
  })[i];
  return databaseConfigErrors({ ...selections[i], parameters }, running).map(
    (message) => ({ path: pathFrom("", steps), message }),
  );
}

function hasAccess(claim: PlaneClaim, ctx: PlaneClaimContext): boolean {
  return selectionsOf(claim).some(
    (p) =>
      ctx.sections.find((s) => s.metadata?.name === p.section)?.spec.slot ===
      "externalAccess",
  );
}

function stringAt(value: unknown, names: readonly string[]): string {
  const found = names.reduce<unknown>(
    (at, name) =>
      typeof at === "object" && at !== null
        ? (at as Record<string, unknown>)[name]
        : undefined,
    value,
  );
  return typeof found === "string" ? found : "";
}

// A version that the change leaves as it is stays unchecked: the floor of
// the operator can rise after the claim was written.
function versionErrorsOf(live: Spec, spec: Spec): FieldError[] {
  const before = runningOfClaim(live.management);
  const versions = (spec.versions ?? {}) as Record<string, unknown>;
  const old = (live.versions ?? {}) as Record<string, unknown>;
  return runningFields(runningOfClaim(spec.management)).flatMap(
    (field: PlaneVersionField) => {
      const value = versions[field];
      const turnedOn = field !== "identity" && !before[field];
      if (value === old[field] && !turnedOn && value) return [];
      const message = versionError(field, value as string | undefined);
      return message === undefined
        ? []
        : [{ path: `spec.versions.${field}`, message }];
    },
  );
}

function mailErrorsOf(live: Spec, spec: Spec): FieldError[] {
  const webModeler = spec.management?.webModeler;
  if (
    webModeler === undefined ||
    equalJSON(webModeler.mail, live.management?.webModeler?.mail)
  )
    return [];
  const path = "spec.management.webModeler.mail";
  return mailErrors(mailDraftOf(webModeler.mail)).map((message) => ({
    path,
    message,
  }));
}

function stringsOf(value: unknown): string[] {
  return Array.isArray(value)
    ? value.filter((v): v is string => typeof v === "string")
    : [];
}
