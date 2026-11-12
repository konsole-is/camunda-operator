// This module runs in the browser and on the server.
import type { Environment } from "@/generated/crds/environment";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import type { PluginSection } from "@/generated/crds/pluginsection";
import { accessPatterns, defaultAccessPatterns } from "@/lib/claims/hosts";
import { SLOT_FIELDS } from "@/lib/fields/slots";
import { ACCESS_SECTION } from "@/lib/labels";
import {
  type StorageDraft,
  draftErrors,
  picksOf,
  resolvedErrors,
  viewSelections,
} from "@/lib/drafts";
import {
  type BuiltinObjects,
  type DeletionChoice,
  type ParameterRule,
  type PluginSelection,
  type SectionView,
  type Slot,
  builtinEntry,
  builtinOption,
  byCodeUnit,
  deletionChoice,
  isObject,
  lockedSummary,
  offeredSection,
  resolve,
  slotOf,
  withMissingInputs,
} from "@/lib/policy";
import type { ValidationError } from "@/lib/schema";
import { type ClusterDraft, releaseOf } from "./draft";

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
  const storage = storageDraftOf(draft, data);
  const selections = viewSelections(storage, views);
  return views.map((view) =>
    withMissingInputs(view, selections, data.registrations, data.sections),
  );
}

/**
 * storageDraftOf returns the storage part of draft in the shape of the draft
 * checks, with the live data of data.
 */
export function storageDraftOf(
  draft: ClusterDraft,
  data: StorageData,
): StorageDraft {
  return {
    kind: "CamundaClusterClaim",
    picks: draft.storage ?? {},
    namespace: draft.namespace?.name ?? "",
    environment: data.environment,
    sections: data.sections,
    registrations: data.registrations,
    objects: data.objects,
  };
}

/**
 * storageErrors returns the errors of step 2, with the host conflicts of
 * the access pick in conflicts. cardErrors are the errors that SectionCards
 * last reported, by section name. A plugin value that the policy of the
 * environment or the schema of its registration refuses blocks the step
 * too, and so does a preset or a config that a request names and that does
 * not exist.
 */
export function storageErrors(
  draft: ClusterDraft,
  data: StorageData,
  cardErrors: Readonly<Record<string, readonly ValidationError[]>>,
  conflicts: readonly string[],
): string[] {
  const views = storageViews(draft, data);
  const storage = storageDraftOf(draft, data);
  const errors = [...draftErrors(storage, views, cardErrors), ...conflicts];
  if (errors.length > 0) return errors;
  // A later step picks the release when the Environment gives no default.
  const release = releaseOf(draft, data.environment);
  return resolvedErrors(
    storage,
    views,
    viewSelections(storage, views),
    (_, registration) =>
      declaresRelease(registration) ? { releaseRef: release } : {},
  );
}

// The claim controller sets releaseRef to the release of the claim on the
// request of a registration whose schema has a top-level string property
// releaseRef (requestParameters of pkg/components/claimrequests).
function declaresRelease(registration: PluginRegistration): boolean {
  const schema = registration.spec.parameters;
  const properties = isObject(schema) ? schema.properties : undefined;
  return (
    isObject(properties) &&
    isObject(properties.releaseRef) &&
    properties.releaseRef.type === "string"
  );
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
 * clusterDeletion returns the deletion choice of draft. The selections are
 * those of the sections that storageViews shows, with the default of a
 * section that the step has not reported yet.
 */
export function clusterDeletion(
  draft: ClusterDraft,
  data: StorageData,
): DeletionChoice {
  const views = storageViews(draft, data);
  return deletionChoice(
    draft.claim.spec.deletionPolicy,
    viewSelections(storageDraftOf(draft, data), views),
    data.environment,
    data.registrations,
    views,
  );
}

/**
 * StorageSummary is what the step 2 picks give the later steps.
 * backupStorage is the selection of the section with slot backupStorage.
 * secondaryStorageType is the type of the secondary storage selection when
 * it is known before creation, else undefined. optimizeUrl says that
 * the access selection gives the Optimize URL. slots are the slots that the
 * selections fill. picks are the review lines, one for each selection.
 */
export type StorageSummary = {
  backupStorage?: PluginSelection;
  secondaryStorageType?: "elasticsearch" | "rdbms";
  optimizeUrl: boolean;
  slots: Slot[];
  picks: StoragePick[];
};

/**
 * StoragePick is one selection as the review shows it, with no ARN. values
 * holds each claim-editable parameter that has a value, from the
 * selection, else the policy value.
 */
export type StoragePick = {
  section: string;
  sectionLabel: string;
  registration: string;
  optionLabel: string;
  values: { label: string; value: string }[];
};

// The output key of the access section that fills the Optimize URL.
const OPTIMIZE_URL_OUTPUT = SLOT_FIELDS.find(
  (row) =>
    row.claimKind === "CamundaClusterClaim" &&
    row.slot === "externalAccess" &&
    row.kind === "CamundaOptimize",
)?.output;

/**
 * storageSummary returns the summary of the step 2 picks of draft, section
 * after section in the order of storageViews, with the default of a section
 * that the step has not reported yet. The type of a selection that names a
 * SecondaryStorageConfig is the type of that object in the namespace of
 * draft.
 */
export function storageSummary(
  draft: ClusterDraft,
  data: StorageData,
): StorageSummary {
  const summary: StorageSummary = { optimizeUrl: false, slots: [], picks: [] };
  for (const view of storageViews(draft, data)) {
    const selections = picksOf(draft.storage ?? {}, view);
    const section = data.sections.find((s) => s.metadata.name === view.name);
    const slot = section && slotOf(section);
    if (slot && selections.length > 0) summary.slots.push(slot);
    for (const selection of selections) {
      const registration = data.registrations.find(
        (r) => r.metadata.name === selection.registration,
      );
      const option = view.options.find(
        (o) => o.registration === selection.registration,
      );
      summary.picks.push({
        section: view.name,
        sectionLabel: view.label,
        registration: selection.registration,
        optionLabel:
          option?.label ?? registration?.spec.label ?? selection.registration,
        values: pickValues(option?.parameters ?? [], selection),
      });
      if (!registration) continue;
      if (slot === "backupStorage") summary.backupStorage = selection;
      if (slot === "secondaryStorage") {
        summary.secondaryStorageType = storageType(
          registration,
          selection,
          draft,
          data,
        );
      }
      if (slot === "externalAccess") {
        summary.optimizeUrl = (registration.spec.outputs ?? []).some(
          (o) => o.key === OPTIMIZE_URL_OUTPUT,
        );
      }
    }
  }
  return summary;
}

// SectionCards leaves out of a selection each parameter that equals its
// policy value, so the policy value of an editable parameter fills the gap.
function pickValues(
  rules: readonly ParameterRule[],
  selection: PluginSelection,
): StoragePick["values"] {
  const set = selection.parameters ?? {};
  const names = [
    ...rules
      .filter(
        (r) =>
          r.claimEditable &&
          (Object.hasOwn(set, r.name) || r.value !== undefined),
      )
      .map((r) => r.name),
    ...Object.keys(set).filter((name) => !rules.some((r) => r.name === name)),
  ];
  return names.map((name) => {
    const rule = rules.find((r) => r.name === name);
    const value = Object.hasOwn(set, name)
      ? set[name]
      : rule?.claimEditable
        ? rule.value
        : undefined;
    return pickValue(rule, name, value, Object.hasOwn(set, name));
  });
}

function pickValue(
  rule: ParameterRule | undefined,
  name: string,
  value: unknown,
  entered: boolean,
): { label: string; value: string } {
  const allowed = (rule?.claimEditable && rule.allowed) || [];
  const text = (v: unknown) =>
    allowed.find((c) => c.name === v)?.label ??
    (typeof v === "string" && v.startsWith("arn:")
      ? entered
        ? "An ARN that you entered"
        : "Set by your admin"
      : lockedSummary(v));
  return {
    label: rule?.label ?? name,
    value:
      allowed.length > 0 && Array.isArray(value)
        ? value.map(text).join(", ")
        : text(value),
  };
}

function storageType(
  registration: PluginRegistration,
  selection: PluginSelection,
  draft: ClusterDraft,
  data: StorageData,
): StorageSummary["secondaryStorageType"] {
  const entry = builtinEntry(registration);
  if (!entry) return undefined;
  if (entry.storageType) return entry.storageType;
  const parameter = Object.entries(entry.lists).find(
    ([, kind]) => kind === "SecondaryStorageConfig",
  )?.[0];
  if (!parameter) return undefined;
  const result = resolve({
    environment: data.environment,
    registrations: [registration],
    selections: [selection],
  });
  if (!("value" in result)) return undefined;
  const name = result.value.requests?.[0]?.parameters?.[parameter];
  return data.objects.SecondaryStorageConfig?.find(
    (c) =>
      c.metadata?.namespace === draft.namespace?.name &&
      c.metadata?.name === name,
  )?.spec.type;
}
