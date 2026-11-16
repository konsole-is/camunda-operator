import type { ConditionRow } from "@/components/fleet/conditions-table";
import type { ReadError } from "@/components/planes/planes-table";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import type { PluginSection } from "@/generated/crds/pluginsection";
import { SLOT_FIELDS, type SlotField } from "@/lib/fields/slots";
import { environmentOf } from "@/lib/policy/environment";
import { conditionTrue } from "@/lib/policy/options";
import type { RegistrationSources } from "./sources";

/** SectionHeader is what the group of one section says about it. */
export type SectionHeader = {
  name: string;
  label: string;
  selection: "One" | "Many";
  emptyOptionLabel?: string;
  fields: readonly SlotField[];
};

/** RegistrationRow is one registration in the group of one section. */
export type RegistrationRow = {
  name: string;
  label: string;
  controller?: { name: string; version?: string };
  mode: "Default" | "Optional";
  accepted?: ConditionRow;
  ready?: ConditionRow;
  /**
   * healthUnknown is true while PluginRegistration is not read. accepted,
   * ready, and healthySince are then unset.
   */
  healthUnknown?: boolean;
  /** healthySince is lastTransitionTime of Ready while Ready is True. */
  healthySince?: string;
  /**
   * requests is unset while PluginRequest is not read, and byEnvironment
   * while Namespace or Environment is not read.
   */
  requests?: {
    total: number;
    byEnvironment?: { environment: string; count: number }[];
  };
  operatorVersions?: string;
  orphan: boolean;
};

/**
 * section is undefined for the group of the registrations without an
 * existing section. unread is true while a list that the group needs is not
 * read: PluginRegistration for the group of a section, so that its rows can
 * be missing, and PluginSection for the group without a section, so that
 * its sections can exist.
 */
export type RegistrationGroup = {
  section?: SectionHeader;
  rows: RegistrationRow[];
  unread?: boolean;
};

/**
 * SourceReads says which lists of RegistrationSources are not read: errors
 * holds the error of each failed watch, and stale the kind of each list
 * whose watch has no snapshot on its current connection.
 */
export type SourceReads = {
  errors?: readonly ReadError[];
  stale?: readonly string[];
};

/** NO_ENVIRONMENT names the requests whose namespace belongs to no Environment. */
const NO_ENVIRONMENT = "No environment";

/**
 * sectionFields returns the fields that the slot of section fills, for each
 * claim kind that the section applies to. A section without a slot fills
 * none.
 */
export function sectionFields(section: PluginSection): SlotField[] {
  const { slot, appliesTo } = section.spec;
  if (!slot) return [];
  return SLOT_FIELDS.filter(
    (f) => f.slot === slot && appliesTo.includes(f.claimKind),
  );
}

/**
 * registrationRow returns the row of registration. The requests count by
 * the display name of the Environment of their namespace, else its name,
 * and "No environment" for a namespace without one or not in sources, in
 * the order of the names.
 */
export function registrationRow(
  registration: PluginRegistration,
  sources: RegistrationSources & SourceReads,
): RegistrationRow {
  const { spec } = registration;
  const unread = unreadKinds(sources);
  const healthUnknown = unread.includes("PluginRegistration");
  const accepted = healthUnknown
    ? undefined
    : conditionOf(registration, "Accepted");
  const ready = healthUnknown ? undefined : conditionOf(registration, "Ready");
  return {
    name: registration.metadata.name ?? "",
    label: spec.label,
    ...(spec.controller && { controller: spec.controller }),
    mode: spec.mode,
    ...(healthUnknown && { healthUnknown }),
    ...(accepted && { accepted }),
    ...(ready && { ready }),
    ...(ready?.status === "True" && {
      healthySince: String(ready.lastTransitionTime),
    }),
    ...(!unread.includes("PluginRequest") && {
      requests: requestCounts(registration, sources, unread),
    }),
    ...(spec.operatorVersions && { operatorVersions: spec.operatorVersions }),
    orphan: spec.deletionPolicies.includes("Orphan"),
  };
}

/**
 * registrationGroups returns one group for each section, by spec.order and
 * then by name, also for a section without a registration. A registration
 * is a row of each of its sections that exists. The registrations with no
 * existing section are the rows of a last group without a section, which
 * exists only when it has a row. Rows sort by label, then by name.
 */
export function registrationGroups(
  sources: RegistrationSources & SourceReads,
): RegistrationGroup[] {
  const unread = unreadKinds(sources);
  const sections = [...sources.sections].sort(
    (a, b) =>
      (a.spec.order ?? 0) - (b.spec.order ?? 0) ||
      compare(a.metadata.name ?? "", b.metadata.name ?? ""),
  );
  const names = new Set(sections.map((s) => s.metadata.name));
  const rowsOf = (registrations: PluginRegistration[]) =>
    registrations
      .map((r) => registrationRow(r, sources))
      .sort((a, b) => compare(a.label, b.label) || compare(a.name, b.name));
  const groups: RegistrationGroup[] = sections.map((section) => ({
    section: {
      name: section.metadata.name ?? "",
      label: section.spec.label,
      selection: section.spec.selection,
      ...(section.spec.emptyOptionLabel && {
        emptyOptionLabel: section.spec.emptyOptionLabel,
      }),
      fields: sectionFields(section),
    },
    ...(unread.includes("PluginRegistration") && { unread: true }),
    rows: rowsOf(
      sources.registrations.filter((r) =>
        r.spec.sections.includes(section.metadata.name ?? ""),
      ),
    ),
  }));
  const lost = sources.registrations.filter(
    (r) => !r.spec.sections.some((s) => names.has(s)),
  );
  if (lost.length === 0) return groups;
  return [
    ...groups,
    {
      rows: rowsOf(lost),
      ...(unread.includes("PluginSection") && { unread: true }),
    },
  ];
}

/**
 * registrationSummary counts each registration once: healthy while its
 * Accepted and Ready conditions are both True, else not healthy.
 */
export function registrationSummary(
  registrations: readonly PluginRegistration[],
): { healthy: number; notHealthy: number } {
  const healthy = registrations.filter(
    (r) => conditionTrue(r, "Accepted") && conditionTrue(r, "Ready"),
  ).length;
  return { healthy, notHealthy: registrations.length - healthy };
}

/** unreadKinds returns each kind of reads that failed or is stale, once. */
export function unreadKinds(reads: SourceReads): string[] {
  const kinds = [
    ...(reads.errors ?? []).map((e) => e.kind),
    ...(reads.stale ?? []),
  ];
  return [...new Set(kinds)];
}

function requestCounts(
  registration: PluginRegistration,
  sources: RegistrationSources,
  unread: readonly string[],
): NonNullable<RegistrationRow["requests"]> {
  const requests = sources.requests.filter(
    (r) => r.spec.registration === registration.metadata.name,
  );
  if (unread.includes("Namespace") || unread.includes("Environment"))
    return { total: requests.length };
  const counts = new Map<string, number>();
  for (const request of requests) {
    const environment = environmentLabel(
      request.metadata.namespace ?? "",
      sources,
    );
    counts.set(environment, (counts.get(environment) ?? 0) + 1);
  }
  return {
    total: requests.length,
    byEnvironment: [...counts]
      .map(([environment, count]) => ({ environment, count }))
      .sort((a, b) => compare(a.environment, b.environment)),
  };
}

function conditionOf(
  registration: PluginRegistration,
  type: string,
): ConditionRow | undefined {
  return registration.status?.conditions?.find((c) => c.type === type);
}

function environmentLabel(
  namespace: string,
  sources: RegistrationSources,
): string {
  const object = sources.namespaces.find((n) => n.metadata?.name === namespace);
  if (!object) return NO_ENVIRONMENT;
  const { environment } = environmentOf(object, sources.environments);
  if (!environment) return NO_ENVIRONMENT;
  return environment.spec.displayName || (environment.metadata.name ?? "");
}

// Sorting by code unit gives the same order in every locale.
function compare(a: string, b: string): number {
  if (a === b) return 0;
  return a < b ? -1 : 1;
}
