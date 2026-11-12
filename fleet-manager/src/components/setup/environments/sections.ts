import type { Environment } from "@/generated/crds/environment";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import type { PluginSection } from "@/generated/crds/pluginsection";
import {
  byCodeUnit,
  type ClaimKind,
  conditionTrue,
  missingInputs,
  type PluginSelection,
  setOwn,
} from "@/lib/policy";

type Spec = Environment["spec"];

/**
 * OfferedOption is a registration that the admin can allow in a section.
 * readyNote is set while the registration is not Ready=True. inputNotes
 * names each section that a required input of the registration reads and
 * that has no default in the edited spec.
 */
export type OfferedOption = {
  registration: string;
  label: string;
  readyNote?: string;
  inputNotes: string[];
};

/** SectionPolicyForm is the policy of one section as the table edits it. */
export type SectionPolicyForm = {
  required: boolean;
  default?: string;
  allowed: string[];
};

/** SectionRow is one PluginSection in the sections table. */
export type SectionRow = {
  name: string;
  label: string;
  description?: string;
  selection: "One" | "Many";
  /** emptyOptionLabel is unset for a required section or a section without an empty option. */
  emptyOptionLabel?: string;
  policy: SectionPolicyForm;
  options: OfferedOption[];
  /** missing are allowed names without an offered registration, in allowed order. */
  missing: string[];
};

/** SectionGroup is the rows of the sections that apply to one claim kind. */
export type SectionGroup = {
  label: "Clusters" | "Management planes";
  rows: SectionRow[];
};

const GROUPS: readonly [ClaimKind, SectionGroup["label"]][] = [
  ["CamundaClusterClaim", "Clusters"],
  ["CamundaManagementClusterClaim", "Management planes"],
];

/**
 * sectionGroups returns the sections that apply to each claim kind, by
 * order and then by name, with the policy of spec. A section of both kinds
 * is in both groups. A kind without a section has no group. An option is a
 * registration that serves the section and has Accepted=True, by name.
 */
export function sectionGroups(
  sections: readonly PluginSection[],
  registrations: readonly PluginRegistration[],
  spec: Spec,
): SectionGroup[] {
  const ordered = [...sections].sort(
    (a, b) =>
      (a.spec.order ?? 0) - (b.spec.order ?? 0) ||
      byCodeUnit(a.metadata.name ?? "", b.metadata.name ?? ""),
  );
  const defaults: PluginSelection[] = Object.entries(spec.sections ?? {})
    .filter(([, policy]) => policy.default)
    .map(([section, policy]) => ({
      section,
      registration: policy.default ?? "",
    }));
  return GROUPS.flatMap(([kind, label]) => {
    const rows = ordered
      .filter((s) => s.spec.appliesTo.includes(kind))
      .map((s) => rowOf(s, sections, registrations, spec, defaults));
    return rows.length > 0 ? [{ label, rows }] : [];
  });
}

function rowOf(
  section: PluginSection,
  sections: readonly PluginSection[],
  registrations: readonly PluginRegistration[],
  spec: Spec,
  defaults: readonly PluginSelection[],
): SectionRow {
  const name = section.metadata.name ?? "";
  const entry = Object.hasOwn(spec.sections ?? {}, name)
    ? spec.sections?.[name]
    : undefined;
  const policy: SectionPolicyForm = {
    required: entry?.required ?? false,
    ...(entry?.default && { default: entry.default }),
    allowed: entry?.allowed ?? [],
  };
  const options = registrations
    .filter(
      (r) => r.spec.sections.includes(name) && conditionTrue(r, "Accepted"),
    )
    .sort((a, b) => byCodeUnit(a.metadata.name ?? "", b.metadata.name ?? ""))
    .map((r): OfferedOption => {
      const registration = r.metadata.name ?? "";
      const readyNote = readyNoteOf(r);
      return {
        registration,
        label: r.spec.label,
        ...(readyNote && { readyNote }),
        inputNotes: inputNotes(
          { section: name, registration },
          defaults,
          registrations,
          sections,
        ),
      };
    });
  const offered = new Set(options.map((o) => o.registration));
  const { spec: s } = section;
  return {
    name,
    label: s.label,
    ...(s.description && { description: s.description }),
    selection: s.selection,
    ...(s.emptyOptionLabel &&
      !policy.required && { emptyOptionLabel: s.emptyOptionLabel }),
    policy,
    options,
    missing: policy.allowed.filter((n) => !offered.has(n)),
  };
}

function readyNoteOf(registration: PluginRegistration): string | undefined {
  if (conditionTrue(registration, "Ready")) return undefined;
  const message = registration.status?.conditions?.find(
    (c) => c.type === "Ready",
  )?.message;
  return message ? `Not ready: ${message}` : "Not ready yet.";
}

function inputNotes(
  selection: PluginSelection,
  defaults: readonly PluginSelection[],
  registrations: readonly PluginRegistration[],
  sections: readonly PluginSection[],
): string[] {
  const missing = missingInputs(selection, defaults, registrations);
  return [...new Set(missing.map((input) => input.section))].map((name) => {
    const label =
      sections.find((s) => s.metadata.name === name)?.spec.label ?? name;
    return `Needs an option of ${label}. Set a default in ${label}.`;
  });
}

/**
 * withSectionPolicy returns spec with the policy of section, without
 * required false, an empty default, or an empty allowed list. A policy that
 * holds none of the three removes the entry, and spec.sections without
 * entries goes too. spec does not change.
 */
export function withSectionPolicy(
  spec: Spec,
  section: string,
  policy: SectionPolicyForm,
): Spec {
  const entry = {
    ...(policy.required && { required: true }),
    ...(policy.default && { default: policy.default }),
    ...(policy.allowed.length > 0 && { allowed: [...policy.allowed] }),
  };
  const sections = Object.fromEntries(
    Object.entries(spec.sections ?? {}).filter(([name]) => name !== section),
  );
  if (Object.keys(entry).length > 0) setOwn(sections, section, entry);
  const next: Spec = { ...spec, sections };
  if (Object.keys(sections).length === 0) delete next.sections;
  return next;
}

/** unknownSections returns the names of spec.sections that no PluginSection of sections has, sorted. */
export function unknownSections(
  sections: readonly PluginSection[],
  spec: Spec,
): string[] {
  const known = new Set(sections.map((s) => s.metadata.name ?? ""));
  return Object.keys(spec.sections ?? {})
    .filter((name) => !known.has(name))
    .sort(byCodeUnit);
}
