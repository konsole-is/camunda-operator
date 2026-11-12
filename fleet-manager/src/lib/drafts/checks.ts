// This module runs in the browser and on the server.
import type { Environment } from "@/generated/crds/environment";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import type { PluginSection } from "@/generated/crds/pluginsection";
import {
  type BuiltinObjects,
  type ClaimKind,
  type PluginSelection,
  type RequestParameters,
  type SectionView,
  builtinEntry,
  checkAllowed,
  claimEditable,
  isObject,
  resolve,
} from "@/lib/policy";
import { type ValidationError, validate } from "@/lib/schema";

/**
 * StorageDraft is the storage part of a create draft, in the shape that
 * the checks of this module read. picks holds the selections of each
 * section by section name, as the storage step last reported them: a
 * section that has a key was shown, and an empty list is a picked empty
 * option. namespace is the namespace of the claim. objects are the source
 * kinds of builtinOption, in every namespace. A kind that objects does not
 * hold is not checked against the names of a request.
 */
export type StorageDraft = {
  kind: ClaimKind;
  picks: Readonly<Record<string, readonly PluginSelection[]>>;
  namespace: string;
  environment: Environment | undefined;
  sections: readonly PluginSection[];
  registrations: readonly PluginRegistration[];
  objects?: BuiltinObjects;
};

/**
 * ControllerParameters returns the parameters that the claim controller
 * adds to request, for the schema check of resolvedErrors. A value that the
 * draft does not know yet is undefined, and the check leaves it out.
 */
export type ControllerParameters = (
  request: RequestParameters,
  registration: PluginRegistration,
) => Record<string, unknown | undefined>;

/**
 * picksOf returns the selections of view in picks. A section that picks
 * does not hold yet counts its enabled default option as picked, as
 * SectionCards preselects it.
 */
export function picksOf(
  picks: StorageDraft["picks"],
  view: SectionView,
): PluginSelection[] {
  if (Object.hasOwn(picks, view.name)) return [...picks[view.name]];
  const preselected = view.options.find(
    (o) => o.isDefault && !o.disabledReason,
  );
  return preselected
    ? [{ section: view.name, registration: preselected.registration }]
    : [];
}

/** viewSelections returns picksOf of each view, view after view. */
export function viewSelections(
  draft: StorageDraft,
  views: readonly SectionView[],
): PluginSelection[] {
  return views.flatMap((view) => picksOf(draft.picks, view));
}

/**
 * goneSections returns the name of each section that holds a pick in
 * draft.picks and that draft.sections do not offer to draft.kind: it is
 * missing, or its appliesTo does not list the kind.
 */
export function goneSections(draft: StorageDraft): string[] {
  return Object.entries(draft.picks)
    .filter(
      ([name, picked]) =>
        picked.length > 0 &&
        !draft.sections.some(
          (s) =>
            s.metadata.name === name && s.spec.appliesTo.includes(draft.kind),
        ),
    )
    .map(([name]) => name);
}

/**
 * draftErrors returns the errors of the picks of draft against views, the
 * live sections of the storage step. cardErrors are the errors that
 * SectionCards last reported, by section name. The card errors of a section
 * that views do not hold do not count. asked names the sections whose
 * missing pick the flow reports in its own words.
 */
export function draftErrors(
  draft: StorageDraft,
  views: readonly SectionView[],
  cardErrors: Readonly<Record<string, readonly ValidationError[]>>,
  asked: readonly string[] = [],
): string[] {
  const errors = views.flatMap((view) => {
    const values = valueErrors(draft, [view]);
    const reported = Object.hasOwn(cardErrors, view.name)
      ? cardErrors[view.name]
      : [];
    // SectionCards reports the missing pick of a required section with the
    // section name as the path, and selectionError has it.
    const settings =
      values.length === 0 && reported.some((e) => e.path !== view.name)
        ? [`Fix the ${view.label} settings.`]
        : [];
    return [...selectionError(draft, view, asked), ...values, ...settings];
  });
  return [
    ...errors,
    ...goneSections(draft).map((name) => `Choose another option for ${name}.`),
  ];
}

function selectionError(
  draft: StorageDraft,
  view: SectionView,
  asked: readonly string[],
): string[] {
  const picked = picksOf(draft.picks, view);
  const unusable = picked.some((s) => {
    const option = view.options.find((o) => o.registration === s.registration);
    return !option || option.disabledReason !== undefined;
  });
  if (unusable) return [`Choose another option for ${view.label}.`];
  if (view.selection === "One" && picked.length > 1) {
    return [`Choose one option for ${view.label}.`];
  }
  return view.required && picked.length === 0 && !asked.includes(view.name)
    ? [`Choose an option for ${view.label}.`]
    : [];
}

/**
 * valueErrors returns an error for each claim-editable parameter of a pick
 * whose value the live choices of views refuse: a required parameter
 * without a value, a name that is not among the choices or the allowed
 * names (the message names it), a list whose count the schema refuses, or
 * a free value that the schema refuses. A pick takes the policy value of a
 * parameter that it does not set. Of a disabled option only the names are
 * checked, so a config that is gone is named also with the last one.
 */
export function valueErrors(
  draft: StorageDraft,
  views: readonly SectionView[],
): string[] {
  return views.flatMap((view) =>
    picksOf(draft.picks, view).flatMap((selection) => {
      const option = view.options.find(
        (o) => o.registration === selection.registration,
      );
      if (!option) return [];
      const disabled = option.disabledReason !== undefined;
      const set = selection.parameters ?? {};
      return option.parameters.flatMap((rule) => {
        if (!rule.claimEditable) return [];
        const value = Object.hasOwn(set, rule.name)
          ? set[rule.name]
          : rule.value;
        const empty =
          value === "" || (Array.isArray(value) && value.length === 0);
        if (value === undefined || (empty && rule.required)) {
          return rule.required && !disabled
            ? [`Choose a ${rule.label} for ${option.label}.`]
            : [];
        }
        const names =
          option.choices?.parameter === rule.name
            ? option.choices.items.map((i) => i.name)
            : rule.allowed?.map((a) => a.name);
        const fix = disabled
          ? []
          : [`Fix the ${rule.label} of ${option.label}.`];
        if (!names) {
          return rule.schema && !validate(rule.schema, value).ok ? fix : [];
        }
        if (empty) return [];
        const list = Array.isArray(value) && !rule.allowedLists;
        const picked = list ? value : [value];
        const gone = picked
          .filter((v) => !names.includes(v as string))
          .map(
            (v) =>
              `${String(v)} is not available for ${rule.label} of ${option.label}. Choose another.`,
          );
        if (gone.length > 0 || !list) return gone;
        // The schema describes the values of the choices, not their names,
        // so only its count applies to a list of names.
        const { minItems, maxItems } = rule.schema ?? {};
        return (typeof minItems === "number" && value.length < minItems) ||
          (typeof maxItems === "number" && value.length > maxItems)
          ? fix
          : [];
      });
    }),
  );
}

/**
 * resolvedErrors returns the errors of the requests that selections, the
 * plugins of the claim, resolve to under draft.environment, as the claim
 * controller finds them: each preset or config that a built-in request
 * names and that is not there (a config must be in draft.namespace), then
 * the first refusal of checkAllowed, then each request whose parameters,
 * with those of controllerParameters, the schema of its registration
 * refuses. views give the words of a missing object: one that a claim can
 * pick in views asks for another pick, any other asks for the admin. The messages of checkAllowed are its own, so
 * a flow runs this after draftErrors passes.
 */
export function resolvedErrors(
  draft: StorageDraft,
  views: readonly SectionView[],
  selections: readonly PluginSelection[],
  controllerParameters: ControllerParameters = () => ({}),
): string[] {
  const { environment, registrations } = draft;
  const resolved = resolve({
    environment,
    registrations: [...registrations],
    selections: [...selections],
  });
  if ("refusal" in resolved) return [resolved.refusal.message];
  if ("error" in resolved) return [resolved.error];
  const requests = resolved.value.requests ?? [];
  const allowed = checkAllowed({
    kind: draft.kind,
    sections: draft.sections,
    environment,
    policy: resolved.value.policy,
    selections: [...selections],
  });
  const policy =
    "refusal" in allowed
      ? [allowed.refusal.message]
      : "error" in allowed
        ? [allowed.error]
        : schemaErrors(draft, requests, controllerParameters);
  return [...missingObjectErrors(draft, views, requests), ...policy];
}

// The kinds of BUILTIN_OPTIONS that are cluster-scoped. Any other kind lives
// in the namespace of the claim.
const CLUSTER_KINDS: readonly string[] = [
  "ElasticsearchClusterPreset",
  "DatabaseServerPreset",
];

// The built-in controller waits for, or fails on, each object that a
// request names and that does not exist.
function missingObjectErrors(
  draft: StorageDraft,
  views: readonly SectionView[],
  requests: readonly RequestParameters[],
): string[] {
  const { objects, namespace } = draft;
  if (!objects) return [];
  return requests.flatMap((request) => {
    const registration = draft.registrations.find(
      (r) => r.metadata.name === request.registration,
    );
    const entry = registration && builtinEntry(registration);
    if (!entry) return [];
    return Object.entries(entry.lists).flatMap(([parameter, kind]) => {
      const name = request.parameters?.[parameter];
      const clustered = CLUSTER_KINDS.includes(kind);
      if (
        !Object.hasOwn(objects, kind) ||
        typeof name !== "string" ||
        name === "" ||
        (objects[kind] ?? []).some(
          (o) =>
            o.metadata?.name === name &&
            (clustered || o.metadata?.namespace === namespace),
        )
      ) {
        return [];
      }
      const where = clustered ? "does not exist" : `is not in ${namespace}`;
      const shown = views
        .find((v) => v.name === request.section)
        ?.options.find((o) => o.registration === request.registration)
        ?.parameters.find((p) => p.name === parameter);
      if (shown?.claimEditable) {
        return [`Pick another ${kind} for ${shown.label}. ${name} ${where}.`];
      }
      const label =
        claimEditable(registration, draft.environment).find(
          (p) => p.name === parameter,
        )?.label ?? parameter;
      return [
        `The ${kind} ${name} that the policy gives ${label} ${where}. Ask your admin to change the policy of the Environment.`,
      ];
    });
  });
}

// checkAllowed does not check a locked value against the schema of its
// registration, and the built-in controller refuses a request of a built-in
// registration whose parameters the schema refuses.
function schemaErrors(
  draft: StorageDraft,
  requests: readonly RequestParameters[],
  controllerParameters: ControllerParameters,
): string[] {
  return requests.flatMap((request) => {
    const registration = draft.registrations.find(
      (r) => r.metadata.name === request.registration,
    );
    const schema = registration?.spec.parameters;
    if (!registration || !isObject(schema)) return [];
    const added = controllerParameters(request, registration);
    const later = Object.keys(added).filter((k) => added[k] === undefined);
    const parameters = Object.fromEntries(
      Object.entries({ ...request.parameters, ...added }).filter(
        ([k]) => !later.includes(k),
      ),
    );
    const checked = validate(withoutProperties(schema, later), parameters);
    if (checked.ok) return [];
    const [first] = checked.errors;
    return [
      `The policy gives ${registration.spec.label} a value that its schema refuses: ${first.path} ${first.message}. Pick another option, or ask your admin to change the policy of the Environment.`,
    ];
  });
}

// lib/schema compiles each schema object once, and the check runs on each
// render, so each schema and its left-out names keep one derived object.
const derived = new WeakMap<object, Map<string, Record<string, unknown>>>();

function withoutProperties(
  schema: Record<string, unknown>,
  names: readonly string[],
): Record<string, unknown> {
  if (names.length === 0) return schema;
  const key = JSON.stringify(names);
  const known = derived.get(schema) ?? new Map();
  derived.set(schema, known);
  const cached = known.get(key);
  if (cached) return cached;
  const properties = isObject(schema.properties) ? schema.properties : {};
  const result = {
    ...schema,
    properties: Object.fromEntries(
      Object.entries(properties).filter(([k]) => !names.includes(k)),
    ),
    ...(Array.isArray(schema.required) && {
      required: schema.required.filter((k) => !names.includes(k)),
    }),
  };
  known.set(key, result);
  return result;
}
