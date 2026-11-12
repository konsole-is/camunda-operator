// This module runs in the browser and on the server.
import type { PluginSection } from "@/generated/crds/pluginsection";
import {
  accessPatterns,
  hostConflicts,
  renderHosts,
  takenHosts,
} from "@/lib/claims/hosts";
import {
  type StorageDraft,
  draftErrors,
  picksOf,
  resolvedErrors,
  viewSelections,
} from "@/lib/drafts";
import { ACCESS_SECTION } from "@/lib/labels";
import {
  type BuiltinObjects,
  type DeletionChoice,
  type OptionView,
  type PluginSelection,
  type SectionView,
  builtinOption,
  byCodeUnit,
  deletionChoice,
  offeredSection,
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

/**
 * PlaneSectionViews are the sections of step 4. A section that no
 * PluginSection has is undefined. others are the other sections that apply
 * to a plane claim, by spec.order and then by name.
 */
export type PlaneSectionViews = {
  databases?: SectionView;
  access?: SectionView;
  others?: SectionView[];
};

/** shownViews returns the sections that views holds. */
export function shownViews(views: PlaneSectionViews): SectionView[] {
  return [views.databases, views.access, ...(views.others ?? [])].filter(
    (v) => v !== undefined,
  );
}

/**
 * planeSectionViews returns the sections of step 4 as a plane claim sees
 * them: offeredSection with builtinOption in the namespace of draft, then
 * withMissingInputs over the picks of draft.
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
  const databases = databaseSection(ctx.sections);
  const access = ctx.sections.find((s) => s.metadata.name === ACCESS_SECTION);
  const others = ctx.sections
    .filter(
      (s) =>
        s !== databases &&
        s !== access &&
        s.spec.appliesTo.includes("CamundaManagementClusterClaim"),
    )
    .sort(
      (a, b) =>
        (a.spec.order ?? 0) - (b.spec.order ?? 0) ||
        byCodeUnit(a.metadata.name ?? "", b.metadata.name ?? ""),
    );
  const views = {
    databases: view(databases),
    access: view(access),
    others: others.map((s) => view(s)!),
  };
  const selections = shownViews(views).flatMap((v) =>
    picksOf(draft.storage, v),
  );
  const missing = (v: SectionView) =>
    withMissingInputs(v, selections, ctx.registrations, ctx.sections);
  return {
    databases: views.databases && missing(views.databases),
    access: views.access && missing(views.access),
    others: views.others.map(missing),
  };
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
 * for each DatabaseConfig that two apps share. selection holds the names
 * of the request, policy values included.
 */
export function databaseConfigErrors(
  selection: PluginSelection | undefined,
  running: Running,
): string[] {
  if (selection?.registration !== EXISTING_DATABASE_CONFIGS) return [];
  const parameters = selection.parameters ?? {};
  const apps = new Map<string, string[]>();
  const missing: string[] = [];
  for (const app of DATABASE_APPS) {
    const name = parameters[app];
    if (typeof name !== "string" || name === "") {
      if (runs(app, running)) {
        missing.push(`Pick a DatabaseConfig for ${VERSION_LABEL[app]}.`);
      }
      continue;
    }
    // The handler refuses two equal names also for an app that does not run.
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

/**
 * planeStorageDraft returns the storage part of draft in the shape of the
 * draft checks. objects are the source kinds of builtinOption, in every
 * namespace. Without them, the config names are not checked.
 */
export function planeStorageDraft(
  draft: PlaneDraft,
  ctx: PlaneContext,
  objects?: BuiltinObjects,
): StorageDraft {
  return {
    kind: "CamundaManagementClusterClaim",
    picks: draft.storage,
    namespace: draft.namespace?.name ?? "",
    environment: ctx.environments.find(
      (e) => e.metadata.name === draft.environment,
    ),
    sections: ctx.sections,
    registrations: ctx.registrations,
    ...(objects && { objects }),
  };
}

/**
 * planeDeletion returns the deletion choice of draft. The selections are
 * those of the sections of planeSectionViews, with the default of a section
 * that the step has not reported yet. objects are as for planeSectionViews.
 */
export function planeDeletion(
  draft: PlaneDraft,
  ctx: PlaneContext,
  objects: BuiltinObjects,
): DeletionChoice {
  const views = shownViews(planeSectionViews(draft, ctx, objects));
  const storage = planeStorageDraft(draft, ctx);
  return deletionChoice(
    draft.deletionPolicy,
    viewSelections(storage, views),
    storage.environment,
    ctx.registrations,
    views,
  );
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
 * storageErrors returns the errors of step 4 against the newest data.
 * sectionErrors are the errors that SectionCards last reported, by section
 * name, and views the sections of planeSectionViews. It returns the
 * errors of draftErrors over views, a missing database pick, the host or
 * URL errors, the DatabaseConfig errors, and the contract name errors.
 * When there are none, it returns resolvedErrors of the plugins of the
 * claim, so a value that the claim controller would refuse blocks the
 * step too. With objects, the source kinds of builtinOption in every
 * namespace, each config name of a request must be in the namespace of
 * draft.
 */
export function storageErrors(
  draft: PlaneDraft,
  ctx: PlaneContext,
  sectionErrors: Readonly<Record<string, readonly ValidationError[]>>,
  views: PlaneSectionViews,
  objects?: BuiltinObjects,
): string[] {
  const running = runningOf(draft);
  const databases = databaseSection(ctx.sections);
  const storage = planeStorageDraft(draft, ctx, objects);
  const shown = shownViews(views);
  const errors: string[] = draftErrors(
    storage,
    shown,
    sectionErrors,
    // The pick of the database section is asked for below, in its own words.
    views.databases ? [views.databases.name] : [],
  );
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
  const request =
    database &&
    effectiveSelection(runningSelection(database, running), draft, ctx);
  errors.push(...databaseConfigErrors(request, running));
  errors.push(...contractNameErrors(draft, ctx));
  if (errors.length > 0) return errors;
  return resolvedErrors(
    storage,
    shown,
    planeSelections(draft).map((p) => runningSelection(p, running)),
    // The claim controller sets databases on the request of the database
    // section (WithDatabases of camundamanagementclusterclaim).
    (request) =>
      request.section === databases?.metadata.name
        ? { databases: DATABASE_APPS.filter((app) => runs(app, running)) }
        : {},
  );
}
