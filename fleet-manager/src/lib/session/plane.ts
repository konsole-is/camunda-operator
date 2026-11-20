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
  databaseApps,
  databaseConfigErrors,
  isDatabaseSection,
  planeControllerParameters,
} from "@/lib/planes/storage";
import { planeUrlFields, urlErrors } from "@/lib/planes/urls";
import {
  runningFields,
  versionError,
  type PlaneVersionField,
} from "@/lib/planes/versions";
import {
  IDENTITY_INITIAL_CLAIM_ANNOTATION,
  pathFrom,
  type Step,
} from "@/lib/fields";
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
  toOrphan,
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
 * of the claim. plane is the live CamundaManagementCluster of the claim,
 * unset while the claim has made none. registrationsRead is false while the
 * page has not read registrations: a selected registration that they lack
 * is then not known, not missing.
 */
export type PlaneClaimContext = {
  plane?: Resource<"CamundaManagementCluster">;
  environment?: Resource<"Environment">;
  environments: readonly Resource<"Environment">[];
  platformConfigs: readonly Resource<"CamundaPlatformConfig">[];
  sections: readonly Resource<"PluginSection">[];
  registrations: readonly Resource<"PluginRegistration">[];
  requests: readonly Resource<"PluginRequest">[];
  registrationsRead: boolean;
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
 * the apps with a database stay, and the schema check of a section
 * parameter skip a value that s does not change, so that a claim that
 * already breaks one of those rules does not block another change. Types
 * and ranges of the CRD are left to the dry run.
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
    ...(mode === "oidc" && pairMoves(live, spec)
      ? recordedClaimErrors(spec.management, ctx.plane)
      : []),
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
      addedBefore: planeControllerParameters(
        generated<PluginSection[]>(ctx.sections),
        runningOfClaim(live.management),
      ),
    }),
  );

  errors.push(
    ...messagesOf(
      checkDeletionPolicy(
        deletionPolicyOf(spec.deletionPolicy, policy),
        selections,
        generated<PluginRegistration[]>(ctx.registrations),
        policy &&
          ctx.registrationsRead &&
          toOrphan(live.deletionPolicy, spec.deletionPolicy, policy)
          ? "Refuses"
          : "Passes",
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
  const refOf = (s: Spec) =>
    s.platformConfigRef || ctx.environment?.spec.platformConfigRef;
  const ref = refOf(spec);
  const modeMoved =
    identityModeOf(live.management?.identityProvider) !== "oidc";
  if (mode !== "oidc" || (!modeMoved && refOf(live) === ref)) return [];
  const config = ctx.platformConfigs.find((c) => c.metadata?.name === ref);
  const reason = oidcDisabledReason(
    generated<CamundaPlatformConfig | undefined>(config),
  );
  if (reason === undefined) return [];
  return [
    {
      path: modeMoved ? PROVIDER : "spec.platformConfigRef",
      message: reason,
    },
  ];
}

// The CEL rules of ManagementOverrides and IdentityAdminSpec, which the API
// server checks on every update.
function adminErrors(
  mode: IdentityModeId | undefined,
  management: PlaneManagement | undefined,
): FieldError[] {
  if (mode === undefined) return [];
  const admin: Partial<PlaneManagement["identity"]["admin"]> =
    management?.identity?.admin ?? {};
  const error = (field: string, message: string) => ({
    path: `${ADMIN}.${field}`,
    message,
  });
  if (mode === "oidc") {
    return [
      ...(admin.claimName ? [] : [error("claimName", "Enter the claim name.")]),
      ...(typeof admin.claimName === "string" && admin.claimName.includes("=")
        ? [error("claimName", "The claim name cannot hold an equals sign.")]
        : []),
      ...(admin.claimValue
        ? []
        : [error("claimValue", "Enter the claim value.")]),
      ...(admin.username != null
        ? [
            error(
              "username",
              "Remove the username. OIDC mode names the administrator by a claim.",
            ),
          ]
        : []),
      ...(admin.passwordSecretRef != null
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
    ...(admin.claimName != null || admin.claimValue != null
      ? [
          error(
            "claimName",
            "Remove the claim name and value. A Keycloak mode names the administrator by a username.",
          ),
        ]
      : []),
    ...(management?.webModeler != null && !admin.email
      ? [
          error(
            "email",
            "Web Modeler needs the email of the administrator in a Keycloak mode.",
          ),
        ]
      : []),
  ];
}

// Management Identity keeps the administrator that it started with, so the
// operator refuses another claim pair while the plane stays in OIDC mode
// (recordInitialClaim of camundamanagementcluster).
function recordedClaimErrors(
  management: PlaneManagement | undefined,
  plane: PlaneClaimContext["plane"],
): FieldError[] {
  const recorded =
    plane?.metadata?.annotations?.[IDENTITY_INITIAL_CLAIM_ANNOTATION];
  if (!recorded) return [];
  const admin = management?.identity?.admin;
  if (`${admin?.claimName ?? ""}=${admin?.claimValue ?? ""}` === recorded)
    return [];
  return [
    {
      path: `${ADMIN}.claimName`,
      message: `Management Identity started with the administrator claim ${recorded}. Fixed after Identity started.`,
    },
  ];
}

// pairMoves reports whether the change moves the identity mode or the
// claim pair, so that a pair that was already off the record does not block
// another change.
function pairMoves(live: Spec, spec: Spec): boolean {
  const pair = (s: Spec) => {
    const admin = s.management?.identity?.admin;
    return [admin?.claimName ?? "", admin?.claimValue ?? ""];
  };
  return (
    identityModeOf(live.management?.identityProvider) !==
      identityModeOf(spec.management?.identityProvider) ||
    !equalJSON(pair(live), pair(spec))
  );
}

function externalErrors(management: PlaneManagement | undefined): FieldError[] {
  const spec = management?.identityProvider?.externalKeycloak;
  const typed = spec === undefined || typeof spec.url === "string";
  return externalKeycloakErrors(externalKeycloakDraftOf(spec))
    .filter((e) => typed || e.path !== "url")
    .map((e) => ({
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
    const steps = field.names.map((name) => ({ field: name }));
    const text = (at: unknown) => {
      const found = valueAt(at, steps);
      return typeof found === "string" ? found : "";
    };
    const value = text(management);
    const needed = before.some((f) => f.id === field.id);
    if (needed && value === text(claim.spec?.management)) return [];
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
  const sections = generated<PluginSection[]>(ctx.sections);
  const selections = selectionsOf(after);
  const running = runningOfClaim(after.spec?.management);
  const appsMoved = !equalJSON(
    databaseApps(runningOfClaim(claim.spec?.management)),
    databaseApps(running),
  );
  const requests = requestsOf(selections, {
    environment: ctx.environment,
    registrations: ctx.registrations,
    policy,
  });
  return selections.flatMap((selection, i) => {
    if (!isDatabaseSection(sections, selection.section)) return [];
    const { section, registration } = selection;
    const steps: Step[] = [
      { field: "spec" },
      { field: "plugins" },
      { item: { section, registration } },
      { field: "parameters" },
    ];
    if (!appsMoved && equalJSON(valueAt(claim, steps), selection.parameters))
      return [];
    return databaseConfigErrors(
      { ...selection, parameters: requests[i] },
      running,
    ).map((message) => ({ path: pathFrom("", steps), message }));
  });
}

function hasAccess(claim: PlaneClaim, ctx: PlaneClaimContext): boolean {
  return selectionsOf(claim).some(
    (p) =>
      ctx.sections.find((s) => s.metadata?.name === p.section)?.spec.slot ===
      "externalAccess",
  );
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
    webModeler == null ||
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
