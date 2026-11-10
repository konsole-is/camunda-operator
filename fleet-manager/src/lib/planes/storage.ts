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
  offeredSection,
} from "@/lib/policy";
import type { ValidationError } from "@/lib/schema";
import { runningOf } from "./components";
import type { PlaneContext, PlaneDraft } from "./draft";
import { planeUrlFields, urlErrors } from "./urls";
import { type Running, VERSION_LABEL } from "./versions";

/** EXISTING_DATABASE_CONFIGS is the built-in registration that names one DatabaseConfig for each app. */
export const EXISTING_DATABASE_CONFIGS = "existing-database-configs";

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

/** PlaneSectionViews are the sections of step 4. A section that no PluginSection has is undefined. */
export type PlaneSectionViews = {
  databases?: SectionView;
  access?: SectionView;
};

/**
 * planeSectionViews returns the database and access sections of step 4 as
 * a plane claim sees them: offeredSection with builtinOption in the
 * namespace of draft. existing-database-configs has no parameter of an app
 * that does not run. objects are the source kinds of builtinOption, in
 * every namespace.
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
  return {
    databases: view(databaseSection(ctx.sections)),
    access: view(ctx.sections.find((s) => s.metadata.name === ACCESS_SECTION)),
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
 * databaseConfigErrors returns one error for each DatabaseConfig that two
 * running apps share in an existing-database-configs selection.
 */
export function databaseConfigErrors(
  selection: PluginSelection | undefined,
  running: Running,
): string[] {
  if (selection?.registration !== EXISTING_DATABASE_CONFIGS) return [];
  const parameters = runningSelection(selection, running).parameters ?? {};
  const apps = new Map<string, string[]>();
  for (const app of DATABASE_APPS) {
    const name = parameters[app];
    if (typeof name !== "string" || name === "") continue;
    apps.set(name, [...(apps.get(name) ?? []), VERSION_LABEL[app]]);
  }
  return [...apps]
    .filter(([, labels]) => labels.length > 1)
    .map(
      ([name, labels]) =>
        `Pick a different DatabaseConfig for each app. ${listOf(labels)} ${labels.length > 2 ? "all" : "both"} use ${name}.`,
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
 * storageErrors returns the errors of step 4. sectionErrors are the errors
 * that SectionCards reported. Its error for a required section without a
 * selection counts as no database option.
 */
export function storageErrors(
  draft: PlaneDraft,
  ctx: PlaneContext,
  sectionErrors: readonly ValidationError[],
): string[] {
  const running = runningOf(draft);
  const databases = databaseSection(ctx.sections);
  const sectionNames = ctx.sections.map((s) => s.metadata.name);
  const errors: string[] = [];
  if (sectionErrors.some((e) => !sectionNames.includes(e.path))) {
    errors.push("Fix the fields of the databases and access options.");
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
  errors.push(...databaseConfigErrors(database, running));
  errors.push(...contractNameErrors(draft, ctx));
  return errors;
}

function listOf(labels: readonly string[]): string {
  if (labels.length <= 2) return labels.join(" and ");
  return `${labels.slice(0, -1).join(", ")}, and ${labels.at(-1)}`;
}
