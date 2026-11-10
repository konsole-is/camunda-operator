// This module runs in the browser and on the server.
import type { Environment } from "@/generated/crds/environment";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import type { PluginSection } from "@/generated/crds/pluginsection";
import { accessPatterns, defaultAccessPatterns } from "@/lib/claims/hosts";
import { ACCESS_SECTION } from "@/lib/labels";
import {
  type BuiltinObjects,
  type DeletionPolicy,
  type PluginSelection,
  type SectionView,
  builtinOption,
  byCodeUnit,
  checkDeletionPolicy,
  offeredSection,
  withMissingInputs,
} from "@/lib/policy";
import type { ValidationError } from "@/lib/schema";
import type { ClusterDraft } from "./draft";

/** StorageData is the live data that step 2 reads. objects are the source kinds of builtinOption, in every namespace. */
export type StorageData = {
  environment: Environment | undefined;
  sections: readonly PluginSection[];
  registrations: readonly PluginRegistration[];
  objects: BuiltinObjects;
};

/**
 * storageViews returns the sections of step 2: those that apply to a
 * cluster claim, by spec.order and then by name, each from offeredSection,
 * with builtinOption in the namespace of draft, then withMissingInputs over
 * the picks of draft.
 */
export function storageViews(
  draft: ClusterDraft,
  data: StorageData,
): SectionView[] {
  const context = {
    environment: data.environment,
    ...(draft.namespace && { namespace: draft.namespace.name }),
    objects: data.objects,
  };
  const views = data.sections
    .filter((s) => s.spec.appliesTo.includes("CamundaClusterClaim"))
    .sort(
      (a, b) =>
        (a.spec.order ?? 0) - (b.spec.order ?? 0) ||
        byCodeUnit(a.metadata.name ?? "", b.metadata.name ?? ""),
    )
    .map((section) => {
      const view = offeredSection(
        section,
        data.registrations,
        data.environment,
        "CamundaClusterClaim",
      );
      return {
        ...view,
        options: view.options.map((o) =>
          builtinOption(o, data.registrations, context),
        ),
      };
    });
  const selections = views.flatMap((view) => picksOf(draft, view));
  return views.map((view) =>
    withMissingInputs(view, selections, data.registrations, data.sections),
  );
}

// A section that draft.storage does not hold yet counts its enabled default
// option as picked, as SectionCards preselects it.
function picksOf(draft: ClusterDraft, view: SectionView): PluginSelection[] {
  if (Object.hasOwn(draft.storage ?? {}, view.name)) {
    return draft.storage![view.name];
  }
  const preselected = view.options.find(
    (o) => o.isDefault && !o.disabledReason,
  );
  return preselected
    ? [{ section: view.name, registration: preselected.registration }]
    : [];
}

/**
 * storageErrors returns the errors of step 2, section after section: a
 * selection whose option is gone or disabled ("Choose another option for
 * <label>"), a required section without a selection ("Choose an option for
 * <label>"), and a card error other than the required one ("Fix the <label>
 * settings"), at most one of each per section. Then a pick of a section
 * that views no longer hold, then conflicts. cardErrors are the errors that
 * SectionCards reported, by section name.
 */
export function storageErrors(
  draft: ClusterDraft,
  views: readonly SectionView[],
  cardErrors: Readonly<Record<string, readonly ValidationError[]>>,
  conflicts: readonly string[],
): string[] {
  const errors: string[] = [];
  for (const view of views) {
    const selections = picksOf(draft, view);
    const unusable = selections.some((s) => {
      const option = view.options.find(
        (o) => o.registration === s.registration,
      );
      return !option || option.disabledReason !== undefined;
    });
    if (unusable) errors.push(`Choose another option for ${view.label}`);
    else if (view.required && selections.length === 0) {
      errors.push(`Choose an option for ${view.label}`);
    }
    const fieldErrors = Object.hasOwn(cardErrors, view.name)
      ? cardErrors[view.name]
      : [];
    if (fieldErrors.some((e) => e.path !== view.name)) {
      errors.push(`Fix the ${view.label} settings`);
    }
  }
  for (const [name, selections] of Object.entries(draft.storage ?? {})) {
    if (selections.length > 0 && !views.some((v) => v.name === name)) {
      errors.push(`Choose another option for ${name}`);
    }
  }
  return [...errors, ...conflicts];
}

/**
 * draftAccessPatterns returns the host patterns of draft: those of the
 * picked access option once step 2 has shown the access section (none for
 * the empty option or for an option whose registration is gone), else those
 * of the default option.
 */
export function draftAccessPatterns(
  draft: ClusterDraft,
  data: StorageData,
): Record<string, unknown> | undefined {
  if (!Object.hasOwn(draft.storage ?? {}, ACCESS_SECTION)) {
    return defaultAccessPatterns(
      data.sections,
      data.registrations,
      data.environment,
      "CamundaClusterClaim",
    );
  }
  const [picked] = draft.storage![ACCESS_SECTION];
  const registration = data.registrations.find(
    (r) => r.metadata.name === picked?.registration,
  );
  return registration && accessPatterns(registration, data.environment, picked);
}

/**
 * DeletionChoice is the deletion choice of step 2. defaultPolicy is
 * spec.deletionPolicy of the Environment, Delete when unset or without an
 * Environment. keepAllowed is false when checkDeletionPolicy refuses Orphan
 * for the selections. alwaysDeleted lists each selection whose registration
 * does not list Orphan, as "<option label> (<section label>)". policy is
 * the pick of the draft, else the default, and Delete when that is Orphan
 * and keepAllowed is false.
 */
export type DeletionChoice = {
  defaultPolicy: DeletionPolicy;
  keepAllowed: boolean;
  alwaysDeleted: { section: string; label: string }[];
  policy: DeletionPolicy;
};

/**
 * deletionChoice returns the deletion choice of draft. The selections are
 * those of the sections that storageViews shows, with the default of a
 * section that the step has not reported yet.
 */
export function deletionChoice(
  draft: ClusterDraft,
  data: StorageData,
): DeletionChoice {
  const defaultPolicy = data.environment?.spec.deletionPolicy || "Delete";
  const views = storageViews(draft, data);
  const selections = views.flatMap((view) => picksOf(draft, view));
  const keepAllowed =
    "value" in checkDeletionPolicy("Orphan", selections, data.registrations);
  const alwaysDeleted = selections.flatMap((s) => {
    const registration = data.registrations.find(
      (r) => r.metadata.name === s.registration,
    );
    if (!registration || registration.spec.deletionPolicies.includes("Orphan")) {
      return [];
    }
    const view = views.find((v) => v.name === s.section);
    const option =
      view?.options.find((o) => o.registration === s.registration)?.label ??
      registration.spec.label;
    return [
      { section: s.section, label: `${option} (${view?.label ?? s.section})` },
    ];
  });
  const picked = draft.claim.spec.deletionPolicy ?? defaultPolicy;
  return {
    defaultPolicy,
    keepAllowed,
    alwaysDeleted,
    policy: picked === "Orphan" && !keepAllowed ? "Delete" : picked,
  };
}
