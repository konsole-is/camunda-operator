// This module runs in the browser and on the server.
import type { PluginSection } from "@/generated/crds/pluginsection";
import {
  accessPatterns,
  hostConflicts,
  renderHosts,
  takenHosts,
} from "@/lib/claims/hosts";
import { ACCESS_SECTION } from "@/lib/labels";
import {
  type BuiltinObjects,
  type OptionView,
  type PluginSelection,
  type SectionView,
  builtinOption,
  checkParameters,
  offeredSection,
  parameterPolicies,
  resolve,
  withMissingInputs,
} from "@/lib/policy";
import type { ValidationError } from "@/lib/schema";
import { listOf } from "@/lib/text";
import { runningOf } from "./components";
import type { PlaneContext, PlaneDraft } from "./draft";
import { planeUrlFields, urlErrors } from "./urls";
import { type Running, VERSION_LABEL } from "./versions";

/** EXISTING_DATABASE_CONFIGS is the built-in registration that names one DatabaseConfig for each app. */
export const EXISTING_DATABASE_CONFIGS = "existing-database-configs";

/** DATABASE_SUFFIX is NameSuffix of pkg/planedatabases: the suffix of each Database that management-database-server makes, by app. */
export const DATABASE_SUFFIX = {
  keycloak: "keycloak",
  identity: "identity",
  webModeler: "web-modeler",
} as const;

const MANAGEMENT_HOST = "managementHostnamePattern";
const CONTRACT_NAME = /^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/;
const DATABASE_APPS = ["keycloak", "identity", "webModeler"] as const;
const FIX_FIELDS = "Fix the fields of the databases and access options.";

/** databaseSection returns the section whose slot is managementDatabases. */
export function databaseSection(
  sections: readonly PluginSection[],
): PluginSection | undefined {
  return sections.find((s) => s.spec.slot === "managementDatabases");
}

/** planeSelections returns the selections of draft.storage, section after section. */
export function planeSelections(draft: PlaneDraft): PluginSelection[] {
  return Object.values(draft.storage).flat();
}

/** PlaneSectionViews are the sections of step 4. A section that no PluginSection has is undefined. */
export type PlaneSectionViews = {
  databases?: SectionView;
  access?: SectionView;
};

/**
 * planeSectionViews returns the database and access sections of step 4 as
 * a plane claim sees them: offeredSection with builtinOption in the
 * namespace of draft, then withMissingInputs over the picks of draft.
 * existing-database-configs has no parameter of an app that does not run.
 * objects are the source kinds of builtinOption, in every namespace.
 */
export function planeSectionViews(
  draft: PlaneDraft,
  ctx: PlaneContext,
  objects: BuiltinObjects,
): PlaneSectionViews {
  const environment = ctx.environments.find(
    (e) => e.metadata.name === draft.environment,
  );
  const context = {
    environment,
    ...(draft.namespace && { namespace: draft.namespace.name }),
    objects,
  };
  const running = runningOf(draft);
  const view = (section: PluginSection | undefined) => {
    if (!section) return undefined;
    const offered = offeredSection(
      section,
      ctx.registrations,
      environment,
      "CamundaManagementClusterClaim",
    );
    return {
      ...offered,
      options: offered.options.map((o) =>
        runningOption(builtinOption(o, ctx.registrations, context), running),
      ),
    };
  };
  const views = {
    databases: view(databaseSection(ctx.sections)),
    access: view(ctx.sections.find((s) => s.metadata.name === ACCESS_SECTION)),
  };
  const shown = [views.databases, views.access].filter((v) => v !== undefined);
  const selections = shown.flatMap((v) => picksOf(draft, v));
  const missing = (v: SectionView | undefined) =>
    v && withMissingInputs(v, selections, ctx.registrations, ctx.sections);
  return { databases: missing(views.databases), access: missing(views.access) };
}

// A section that draft.storage does not hold yet counts its enabled default
// option as picked, as SectionCards preselects it.
function picksOf(draft: PlaneDraft, view: SectionView): PluginSelection[] {
  if (Object.hasOwn(draft.storage, view.name)) return draft.storage[view.name];
  const preselected = view.options.find(
    (o) => o.isDefault && !o.disabledReason,
  );
  return preselected
    ? [{ section: view.name, registration: preselected.registration }]
    : [];
}

function runningOption(option: OptionView, running: Running): OptionView {
  if (option.registration !== EXISTING_DATABASE_CONFIGS) return option;
  return {
    ...option,
    parameters: option.parameters.filter((p) => runs(p.name, running)),
  };
}

// runs says whether the app that a parameter of existing-database-configs
// names runs. Management Identity always runs.
function runs(app: string, running: Running): boolean {
  if (app === "keycloak") return running.keycloak;
  if (app === "webModeler") return running.webModeler;
  return true;
}

/** accessSelection returns the selection of the access section in draft. */
export function accessSelection(
  draft: PlaneDraft,
): PluginSelection | undefined {
  return planeSelections(draft).find((p) => p.section === ACCESS_SECTION);
}

/**
 * managementHost returns the host that managementHostnamePattern of the
 * access selection gives the plane of draft, with that pattern. It is
 * undefined without an access selection, without its registration, and
 * while the pattern, the name, or the namespace is empty.
 */
export function managementHost(
  draft: PlaneDraft,
  ctx: PlaneContext,
): { host: string; pattern: string } | undefined {
  const selection = accessSelection(draft);
  const registration = ctx.registrations.find(
    (r) => r.metadata.name === selection?.registration,
  );
  if (!registration) return undefined;
  const environment = ctx.environments.find(
    (e) => e.metadata.name === draft.environment,
  );
  const patterns = accessPatterns(registration, environment, selection) ?? {};
  const pattern = patterns[MANAGEMENT_HOST];
  const rendered = renderHosts(
    { [MANAGEMENT_HOST]: pattern },
    draft.name,
    draft.namespace?.name ?? "",
  )[0];
  return rendered && typeof pattern === "string"
    ? { host: rendered.host, pattern }
    : undefined;
}

/**
 * runningSelection returns selection without the parameters of
 * existing-database-configs that name the DatabaseConfig of an app that
 * does not run.
 */
export function runningSelection(
  selection: PluginSelection,
  running: Running,
): PluginSelection {
  if (selection.registration !== EXISTING_DATABASE_CONFIGS) return selection;
  const parameters = Object.fromEntries(
    Object.entries(selection.parameters ?? {}).filter(([app]) =>
      runs(app, running),
    ),
  );
  return {
    section: selection.section,
    registration: selection.registration,
    ...(Object.keys(parameters).length > 0 && { parameters }),
  };
}

/**
 * databaseConfigErrors returns the errors of an existing-database-configs
 * selection: one for each running app without a DatabaseConfig, and one
 * for each DatabaseConfig that two running apps share. selection holds the
 * names that the claim gets, policy values included.
 */
export function databaseConfigErrors(
  selection: PluginSelection | undefined,
  running: Running,
): string[] {
  if (selection?.registration !== EXISTING_DATABASE_CONFIGS) return [];
  const parameters = runningSelection(selection, running).parameters ?? {};
  const apps = new Map<string, string[]>();
  const missing: string[] = [];
  for (const app of DATABASE_APPS) {
    if (!runs(app, running)) continue;
    const name = parameters[app];
    if (typeof name !== "string" || name === "") {
      missing.push(`Pick a DatabaseConfig for ${VERSION_LABEL[app]}.`);
      continue;
    }
    apps.set(name, [...(apps.get(name) ?? []), VERSION_LABEL[app]]);
  }
  return [
    ...missing,
    ...[...apps]
      .filter(([, labels]) => labels.length > 1)
      .map(
        ([name, labels]) =>
          `Pick a different DatabaseConfig for each app. ${listOf(labels)} ${labels.length > 2 ? "all" : "both"} use ${name}.`,
      ),
  ];
}

// effectiveSelection returns selection with the parameters that the claim
// gets: SectionCards leaves out a value that equals the policy value.
function effectiveSelection(
  selection: PluginSelection,
  draft: PlaneDraft,
  ctx: PlaneContext,
): PluginSelection {
  const registration = ctx.registrations.find(
    (r) => r.metadata.name === selection.registration,
  );
  if (!registration) return selection;
  const result = resolve({
    environment: ctx.environments.find(
      (e) => e.metadata.name === draft.environment,
    ),
    registrations: [registration],
    selections: [selection],
  });
  const parameters =
    "value" in result ? result.value.requests?.[0]?.parameters : undefined;
  return parameters ? { ...selection, parameters } : selection;
}

// The database section is never required here: storageErrors asks for its
// option in its own words.
function selectionErrors(
  draft: PlaneDraft,
  views: PlaneSectionViews,
): string[] {
  return [views.databases, views.access].flatMap((view) => {
    if (!view) return [];
    const picked = draft.storage[view.name] ?? [];
    const unusable = picked.some((s) => {
      const option = view.options.find(
        (o) => o.registration === s.registration,
      );
      return !option || option.disabledReason !== undefined;
    });
    if (unusable) return [`Choose another option for ${view.label}.`];
    return view.required && picked.length === 0 && view !== views.databases
      ? [`Choose an option for ${view.label}.`]
      : [];
  });
}

// SectionCards checks the parameters against the policy only while step 4
// shows, and the Environment can change after that.
function parameterErrors(draft: PlaneDraft, ctx: PlaneContext): string[] {
  const environment = ctx.environments.find(
    (e) => e.metadata.name === draft.environment,
  );
  const running = runningOf(draft);
  const refused = planeSelections(draft).some((picked) => {
    const selection = runningSelection(picked, running);
    const registration = ctx.registrations.find(
      (r) => r.metadata.name === selection.registration,
    );
    if (!registration) return false;
    const policies = parameterPolicies(registration, environment);
    return (
      !("value" in policies) ||
      checkParameters(selection, policies.value) !== undefined
    );
  });
  return refused ? [FIX_FIELDS] : [];
}

/** contractNameOf is the name of the ManagementAuthConfig of the draft: the typed name, else the claim name. */
export function contractNameOf(draft: PlaneDraft): string {
  return draft.contractName || draft.name;
}

/**
 * contractNameErrors returns the errors of contractNameOf(draft): a name
 * that a ManagementAuthConfig cannot have, a name that another plane
 * claim writes, else a name that a ManagementAuthConfig has. It returns
 * none while the name is empty.
 */
export function contractNameErrors(
  draft: PlaneDraft,
  ctx: PlaneContext,
): string[] {
  const name = contractNameOf(draft);
  if (name === "") return [];
  if (name.length > 253 || !CONTRACT_NAME.test(name)) {
    return [
      "The contract name must be lower case letters, digits, and dashes.",
    ];
  }
  const holder = ctx.planeClaims.find(
    (c) =>
      (c.spec.managementAuthConfigName || c.metadata.name) === name &&
      !(
        c.metadata.namespace === draft.namespace?.name &&
        c.metadata.name === draft.name
      ),
  );
  if (holder) {
    return [
      `The plane ${holder.metadata.namespace}/${holder.metadata.name} writes the contract ${name}. Enter another name.`,
    ];
  }
  return ctx.authConfigs.includes(name)
    ? [`A contract ${name} exists already. Enter another name.`]
    : [];
}

/**
 * storageErrors returns the errors of step 4. sectionErrors are the errors
 * that SectionCards reported, and views the sections of planeSectionViews.
 * A section of views whose selected option is missing or disabled, or an
 * access section that is required and has no selection, blocks the step.
 */
export function storageErrors(
  draft: PlaneDraft,
  ctx: PlaneContext,
  sectionErrors: readonly ValidationError[],
  views: PlaneSectionViews,
): string[] {
  const running = runningOf(draft);
  const databases = databaseSection(ctx.sections);
  const sectionNames = ctx.sections.map((s) => s.metadata.name);
  const errors: string[] = selectionErrors(draft, views);
  for (const [name, picked] of Object.entries(draft.storage)) {
    if (picked.length > 0 && !sectionNames.includes(name)) {
      errors.push(`Choose another option for ${name}.`);
    }
  }
  if (sectionErrors.some((e) => !sectionNames.includes(e.path))) {
    errors.push(FIX_FIELDS);
  } else {
    errors.push(...parameterErrors(draft, ctx));
  }
  const database = planeSelections(draft).find(
    (p) => p.section === databases?.metadata.name,
  );
  if (!database) errors.push("Pick a database option.");
  if (accessSelection(draft)) {
    const host = managementHost(draft, ctx);
    errors.push(
      ...hostConflicts(
        host ? [{ parameter: MANAGEMENT_HOST, host: host.host }] : [],
        takenHosts(ctx.accessRequests),
      ),
    );
  } else {
    errors.push(...urlErrors(draft.urls, planeUrlFields(running)));
  }
  errors.push(
    ...databaseConfigErrors(
      database && effectiveSelection(database, draft, ctx),
      running,
    ),
  );
  errors.push(...contractNameErrors(draft, ctx));
  return errors;
}
