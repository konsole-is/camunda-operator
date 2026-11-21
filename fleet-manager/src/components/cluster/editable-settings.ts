import { type EnvVar, envErrors } from "@/components/fleet/env-editor";
import { keyValueErrors } from "@/components/fleet/key-value-editor";
import {
  EDITED_RESOURCES,
  type LimitBelowRequest,
  limitsBelowRequests,
  type Resources,
} from "@/components/fleet/resources-editor";
import type { CamundaClusterSpec } from "@/generated/crds/camundacluster";
import { fieldSchemas } from "@/generated/fieldschema";
import {
  type ClusterField,
  claimValueOf,
  COMPONENTS,
  componentFieldPaths,
  editorOf,
  type FieldLock,
  type FieldState,
  fieldSchemaOf,
  itemStep,
  type KeyValueFormat,
  optimizeClaimNames,
  overrideEditorOf,
  parameterEditorOf,
  pathFrom,
  type SchemaRoot,
  type SettingEditor,
  type Step,
} from "@/lib/fields";
import type { OpenAPISchema } from "@/lib/schema";
import { valueAt, without, withValue } from "@/lib/session";
import { type SettingsGroup, settingsGroupsOf } from "./settings";
import { type Process, runsOwnWorkload } from "./topology";

/**
 * Setting is one editor of a settings group. steps is where a change goes,
 * from the root of the claim, and is undefined when the setting cannot
 * change. When a layer sets a whole field above the setting, steps is that
 * field and holder holds its value and the rest of the path to the
 * setting, so that a change keeps the rest of the field. claimSet shows the
 * reset link. current is the value before the pending changes. inherited
 * counts the entries that the preset and the release put before value.
 * replaces says that a claim value replaces the value below it as a
 * whole. Only then does a map of {} hide that value: the preset merges a
 * map key by key. mergedErrors are the faults of the merged value that the
 * editor does not own, and undefined when no check of the merged value
 * runs.
 */
export type Setting = {
  key: string;
  label: string;
  editor?: SettingEditor;
  value: unknown;
  current?: unknown;
  inherited?: number;
  state?: FieldState;
  lock?: FieldLock;
  steps?: Step[];
  holder?: { value: unknown; rest: Step[] };
  claimSet: boolean;
  resetLabel: string;
  resetNote?: string;
  replaces?: true;
  mergedErrors?: MergedError[];
};

/**
 * MergedError is a fault of the merged value of a setting: a limit below
 * its request, or an entry of a map or a list of variables that fails the
 * rule of its editor and that the claim does not hold. entry is that entry,
 * as the editor holds it. blocks is false when the value does not apply
 * now: the process runs embedded, or Connectors are not enabled.
 */
export type MergedError = {
  message: string;
  blocks: boolean;
  entry?: MapEntry | EnvVar;
};

/** MapEntry is one entry of a map of strings. */
export type MapEntry = { key: string; value: string };

/**
 * settingsOf returns the settings of group. spec holds the values by their
 * claim names: the merged CamundaCluster spec for a process or the
 * cluster, and spec.attachments.optimize of the claim for Optimize. live,
 * that spec before the pending changes, sets current. claim is the claim
 * spec at the same root: an extraEnvFrom setting holds the list of the
 * claim only.
 *
 * Each path of componentFieldPaths gives one setting, in that order, also
 * when no layer sets it. Its label is the path without the process name.
 * A Secret reference, an object whose name ends in Ref with exactly the
 * fields name and key, is one setting. A field of the group belongs to a
 * setting at or above it, or to the setting inside it when a layer sets the
 * field as a whole: then the setting has a holder. A setting takes the
 * state of its fields, a claim state first, and the lock of any of them. A
 * locked setting has no editor and no steps. A field that belongs to no
 * setting is a read-only setting of its own, after the others.
 *
 * With claim, spec is a merge of layers, and a setting of resources, of a
 * map of strings, or of variables gets mergedErrors. For resources, they
 * are each limit of spec below its request, for any resource. For a map or
 * variables, they are the entries of spec that the claim does not hold
 * and that fail the rule of the editor, one for each entry.
 */
export function settingsOf(input: {
  group: SettingsGroup;
  spec: unknown;
  live?: unknown;
  claim?: unknown;
}): Setting[] {
  const { group, spec, live, claim } = input;
  const optimize = group.component === "optimize";
  const root: SchemaRoot = optimize ? "optimize" : "cluster";
  const base: Step[] = optimize
    ? steps(["spec", "attachments", "optimize"])
    : steps(["spec", "cluster"]);
  const prefix = COMPONENTS.includes(group.component) ? 1 : 0;
  const fields = group.fields.map((field) => ({
    field,
    names: (optimize ? optimizeClaimNames : (n: string[]) => n)(
      (field.docNames ?? []).slice(1),
    ),
  }));
  const used = new Set<string>();

  const settings = secretRefPaths(
    root,
    componentFieldPaths(group.component),
  ).map(({ path, secretRef }): Setting => {
    const names = path.split(".");
    const mine = fields.filter(
      ({ names: f }) => startsWith(f, names) || startsWith(names, f),
    );
    for (const { field } of mine) used.add(field.key);
    const holder = mine.find(({ names: f }) => f.length < names.length);
    const state =
      mine.find(({ field }) => field.state.kind === "claim")?.field.state ??
      mine[0]?.field.state;
    const lock = mine.find(({ field }) => field.lock)?.field.lock;
    const value = valueAt(spec, steps(names));
    // The merge appends the extraEnvFrom of the claim to that of the preset
    // and the release, so a merged value written to the claim repeats theirs.
    const own =
      "claim" in input && names.at(-1) === "extraEnvFrom"
        ? valueAt(claim, steps(names))
        : value;
    const inherited =
      own !== value ? lengthOf(value) - lengthOf(own) : undefined;
    const setting: Setting = {
      key: `${group.component}:${path}`,
      label: names.slice(prefix).join("."),
      value: own,
      ...(inherited ? { inherited } : {}),
      ...("live" in input &&
        own === value && { current: valueAt(live, steps(names)) }),
      ...(state && { state }),
      claimSet: mine.some(({ field }) => field.state.kind === "claim"),
      resetLabel: "Reset to preset",
    };
    if (lock) return { ...setting, lock };
    const editor = secretRef
      ? ({ kind: "secretRef" } as const)
      : editorOf(path, fieldSchemaOf(root, path), { value: own });
    if (!editor) return setting;
    const checked: Setting =
      "claim" in input
        ? {
            ...setting,
            ...mergedCheckOf({
              editor,
              group,
              spec,
              value,
              claimValue: valueAt(claim, steps(names)),
              stateAt: (part) =>
                mine.find(({ names: f }) => sameNames(f, [...names, ...part]))
                  ?.field.state,
              // A map below a lock path is one field, so its entries
              // take the state of the setting.
              entryStateAt: (step) =>
                mine.find(
                  ({ field }) =>
                    field.path === pathFrom("spec", [...steps(names), step]),
                )?.field.state ?? (state?.kind === "claim" ? undefined : state),
            }),
          }
        : setting;
    if (holder?.field.claimSteps)
      return {
        ...checked,
        ...blockReset(
          names.slice(prefix, holder.names.length).join(".") || group.label,
          "preset",
        ),
        editor,
        steps: holder.field.claimSteps,
        holder: {
          value: valueAt(spec, steps(holder.names)),
          rest: steps(names.slice(holder.names.length)),
        },
      };
    return { ...checked, editor, steps: [...base, ...steps(names)] };
  });
  const rest = group.fields
    .filter((field) => !used.has(field.key))
    .map((field): Setting => ({
      key: field.key,
      label: field.label,
      value: field.value,
      state: field.state,
      ...(field.lock && { lock: field.lock }),
      claimSet: field.state.kind === "claim",
      resetLabel: "Reset to preset",
    }));
  return [...settings, ...rest];
}

// mergedCheckOf returns the mergedErrors of a setting with editor in group,
// or nothing for an editor without a check of the merged value. value is
// the merged value of the setting and claimValue that of the claim.
// stateAt gives the state of the field at a path below the setting, and
// entryStateAt that of the entry at one step below it.
function mergedCheckOf(input: {
  editor: SettingEditor;
  group: SettingsGroup;
  spec: unknown;
  value: unknown;
  claimValue: unknown;
  stateAt: (part: string[]) => FieldState | undefined;
  entryStateAt: (step: Step) => FieldState | undefined;
}): Pick<Setting, "mergedErrors"> {
  const { editor, group, spec, value, claimValue, stateAt, entryStateAt } =
    input;
  if (editor.kind === "resources") {
    const applies = appliesNow(editor, group, spec);
    return {
      mergedErrors: limitsBelowRequests((value ?? {}) as Resources).map(
        (below) => ({
          message: mergedLimitError(
            below,
            group,
            applies,
            stateAt(["limits", below.resource]),
            stateAt(["requests", below.resource]),
          ),
          blocks: applies,
        }),
      ),
    };
  }
  if (editor.kind === "keyValue") {
    const applies = appliesNow(editor, group, spec);
    const held = isObject(claimValue) ? claimValue : {};
    const entries = Object.entries(isObject(value) ? value : {}).map(
      ([key, v]): MapEntry => ({ key, value: String(v) }),
    );
    const faults = entryFaults(keyValueErrors(entries, editor.format));
    // The editor ignores a row with an empty key and an empty value, as a
    // row that the user did not fill yet. Kubernetes refuses the empty key.
    const blank = entries.findIndex((e) => e.key === "" && e.value === "");
    if (blank >= 0) faults.push({ index: blank, field: "key" });
    return {
      mergedErrors: faults
        .filter(({ index }) => !Object.hasOwn(held, entries[index].key))
        .map(({ index, field }) => {
          const entry = entries[index];
          const state = entryStateAt({ field: entry.key });
          return {
            message:
              mapEntryFault(entry, field, editor.format, state) +
              remedyOf(state, field === "value" ? entry.key : undefined) +
              laterOf(group, applies),
            blocks: applies,
            entry,
          };
        }),
    };
  }
  if (editor.kind === "env") {
    const applies = appliesNow(editor, group, spec);
    const held = Array.isArray(claimValue) ? (claimValue as EnvVar[]) : [];
    const env = Array.isArray(value) ? (value as EnvVar[]) : [];
    return {
      mergedErrors: entryFaults(envErrors(env))
        .filter(({ index }) => !held.some((e) => e.name === env[index].name))
        // The merge keeps one entry for each name, and the editor reads an
        // entry from a Secret only with its key, so only a missing name
        // fails here.
        .map(({ index }) => {
          const entry = env[index];
          const state = entryStateAt(itemStep(entry, index));
          return {
            message:
              `A variable without a name ${entryOrigin(state)}. The claim cannot remove it.` +
              remedyOf(state) +
              laterOf(group, applies),
            blocks: applies,
            entry,
          };
        }),
    };
  }
  return {};
}

// entryOrigin returns the words that say where an entry of state comes
// from. The claim does not hold the entry, so without a known layer it is
// only known to be on the cluster.
function entryOrigin(state: FieldState | undefined): string {
  return (originOf(state) ?? ON_CLUSTER).one;
}

// The operator gives the resources and the pod labels and annotations of a
// process only to its own workload. It gives the variables of an embedded
// process to its host, and those of Connectors to their own workload only.
function appliesNow(
  editor: SettingEditor,
  group: SettingsGroup,
  spec: unknown,
): boolean {
  if (!COMPONENTS.includes(group.component)) return true;
  if (editor.kind === "env" && group.component !== "connectors") return true;
  return runsOwnWorkload(
    spec as CamundaClusterSpec,
    group.component as Process,
  );
}

// entryFaults gives the first error of each entry of errors, whose paths
// are "<entry>.<field>", with the entry and the field. An error of the key
// or the name comes first, because the claim cannot change it.
function entryFaults(
  errors: readonly { path: string }[],
): { index: number; field: string }[] {
  const faults = new Map<number, string>();
  for (const { path } of errors) {
    const dot = path.indexOf(".");
    const index = Number(path.slice(0, dot));
    if (!faults.has(index)) faults.set(index, path.slice(dot + 1));
  }
  return [...faults].map(([index, field]) => ({ index, field }));
}

// mapEntryFault returns the sentence of an entry of a map of strings whose
// field fails the rule of the editor.
function mapEntryFault(
  entry: MapEntry,
  field: string,
  format: KeyValueFormat | undefined,
  state: FieldState | undefined,
): string {
  const noun = format ?? "map";
  const from = entryOrigin(state);
  if (field === "value")
    return `The value of the ${noun} "${entry.key}" ${from} and is not valid.`;
  const key =
    entry.key === ""
      ? `An empty ${noun} key`
      : `The ${noun} key "${entry.key}"`;
  return `${key} ${from}. The claim cannot remove it.`;
}

// remedyOf returns what the user can do about an entry of the layer of
// state. With key, the claim can set a valid value for that key.
function remedyOf(state: FieldState | undefined, key?: string): string {
  const own =
    key === undefined ? [] : [`Set a valid value for "${key}" in the claim`];
  const others =
    state?.kind === "preset"
      ? ["pick another preset", "ask your admin to correct the preset"]
      : state?.kind === "release"
        ? ["ask your admin to correct the release"]
        : [];
  const all = [...own, ...others];
  if (all.length === 0) return "";
  const text =
    all.length === 1
      ? all[0]
      : `${all.slice(0, -1).join(", ")}, or ${all.at(-1)}`;
  return ` ${text.charAt(0).toUpperCase()}${text.slice(1)}.`;
}

// laterOf returns the sentence that says when a fault of group must be
// corrected, or nothing when the value of group applies now.
function laterOf(group: SettingsGroup, applies: boolean): string {
  if (applies) return "";
  return group.embedded
    ? ` Correct it before ${group.label} runs standalone.`
    : ` Correct it before ${group.label} are turned on.`;
}

// mergedLimitError returns the message of a limit below its request in
// group. applies says whether the resources of group apply now.
function mergedLimitError(
  below: LimitBelowRequest,
  group: SettingsGroup,
  applies: boolean,
  limitState: FieldState | undefined,
  requestState: FieldState | undefined,
): string {
  const states = [limitState, requestState];
  // The editor has no field for this resource. An edit of another amount
  // writes the whole block, so the claim can hold copies of these values.
  const claimed = states.filter((state) => state?.kind === "claim").length;
  const unedited = !EDITED_RESOURCES.includes(below.resource);
  const fixed =
    (unedited && claimed > 0
      ? ` Select Reset to preset to remove ${claimed > 1 ? "these values" : "this value"} from the claim.`
      : "") +
    (unedited && states.some((state) => state?.kind === "preset")
      ? " Pick another preset, or ask your admin to correct the preset."
      : "");
  return (
    faultOf(below, limitState, requestState) + fixed + laterOf(group, applies)
  );
}

// faultOf returns the sentence of a limit below its request. It names
// where the limit and the request come from when that is not the claim.
function faultOf(
  below: LimitBelowRequest,
  limitState: FieldState | undefined,
  requestState: FieldState | undefined,
): string {
  const name = RESOURCE_NAMES[below.resource] ?? below.resource;
  const limit = `${name} limit of ${below.limit}`;
  const request = `${name} request of ${below.request}`;
  const limitFrom = originOf(limitState);
  const requestFrom = originOf(requestState);
  if (limitFrom === undefined)
    return requestFrom === undefined
      ? `The ${limit} is below the ${request}.`
      : `The ${request} ${requestFrom.one} and is above the ${limit}.`;
  if (requestFrom === undefined)
    return `The ${limit} ${limitFrom.one} and is below the ${request}.`;
  if (limitFrom.one === requestFrom.one)
    return `The ${limit} and the ${request} ${limitFrom.many}. The limit is below the request.`;
  return `The ${limit} ${limitFrom.one}. It is below the ${request}, which ${requestFrom.one}.`;
}

const RESOURCE_NAMES: Partial<Record<string, string>> = {
  cpu: "CPU",
  memory: "memory",
};

// originOf gives the words that say where a value of state comes from, for
// one value and for two, or undefined for a value of the claim. A value
// that no manager owns, or one of an unknown writer, is only known to be on
// the cluster.
function originOf(
  state: FieldState | undefined,
): { one: string; many: string } | undefined {
  const from = (layer: string) => ({
    one: `comes from ${layer}`,
    many: `come from ${layer}`,
  });
  const by = (writer: string) => ({
    one: `is set by ${writer}`,
    many: `are set by ${writer}`,
  });
  switch (state?.kind) {
    case undefined:
    case "claim":
      return undefined;
    case "preset":
      return from("the preset");
    case "release":
      return from("the release");
    case "environment":
      return from("the environment");
    case "admin":
      return by("your admin");
    case "filled":
      return {
        one: `is filled by ${state.sectionLabel}`,
        many: `are filled by ${state.sectionLabel}`,
      };
    case "setBy":
      if (state.manager !== undefined) return by(state.manager);
      break;
    case "default":
      break;
  }
  return ON_CLUSTER;
}

const ON_CLUSTER = {
  one: "is already set on the cluster",
  many: "are already set on the cluster",
};

/**
 * blockingMergedErrors returns each merged error that blocks of a cluster,
 * as "<path>: <message>", where path names the setting in spec.cluster of
 * the claim, such as zeebe.resources. fields are clusterFields of the
 * cluster, spec the merged spec, and claim spec.cluster of the claim.
 */
export function blockingMergedErrors(
  fields: readonly ClusterField[],
  spec: CamundaClusterSpec,
  claim: unknown,
): string[] {
  return settingsGroupsOf(fields, spec).flatMap((group) =>
    settingsOf({ group, spec, claim }).flatMap((setting) =>
      (setting.mergedErrors ?? [])
        .filter((e) => e.blocks)
        .map(
          (e) =>
            `${group.component === "cluster" ? "" : `${group.component}.`}${setting.label}: ${e.message}`,
        ),
    ),
  );
}

/**
 * settingsSummary returns the line that says how many settings there are
 * and where their values come from, as in "12 settings: 8 from the preset,
 * 1 changed in the claim". A source that no setting has is left out, and
 * the changes in the claim come last.
 */
export function settingsSummary(settings: readonly Setting[]): string {
  if (settings.length === 0) return "No settings";
  const counts = new Map<string, { rank: number; n: number }>();
  for (const { state } of settings) {
    const source = sourceOf(state);
    const rank = SOURCE_ORDER.indexOf(state?.kind ?? "unset");
    const count = counts.get(source) ?? { rank, n: 0 };
    counts.set(source, { rank, n: count.n + 1 });
  }
  const parts = [...counts]
    .sort(([, a], [, b]) => a.rank - b.rank)
    .map(([source, { n }]) => `${n} ${source}${n === 1 ? "" : plural(source)}`);
  const total = `${settings.length} setting${settings.length === 1 ? "" : "s"}`;
  return `${total}: ${parts.join(", ")}`;
}

const SOURCE_ORDER: readonly (FieldState["kind"] | "unset")[] = [
  "preset",
  "release",
  "environment",
  "admin",
  "filled",
  "setBy",
  "default",
  "unset",
  "claim",
];

function sourceOf(state: FieldState | undefined): string {
  switch (state?.kind) {
    case undefined:
      return "not set";
    case "preset":
      return "from the preset";
    case "release":
      return "from the release";
    case "environment":
      return "from the environment";
    case "admin":
      return "set by your admin";
    case "filled":
      return `filled by ${state.sectionLabel}`;
    case "setBy":
      return `set by ${state.manager ?? "an unknown writer"}`;
    case "default":
      return "default value";
    case "claim":
      return "changed in the claim";
  }
}

// plural returns the ending of source for more than one setting: only a
// noun takes one.
function plural(source: string): string {
  return source === "default value" ? "s" : "";
}

/**
 * settingChange returns where an editor value goes and its claim value. A
 * value of undefined resets the field at steps. A setting with a holder
 * writes the whole held field with the setting changed, and resets it when
 * that leaves it empty. The setting must have an editor and steps.
 */
export function settingChange(
  setting: Setting,
  editorValue: unknown,
): { steps: Step[]; value: unknown } {
  const claim = claimValueOf(setting.editor!, editorValue);
  const at = setting.steps!;
  const { holder } = setting;
  if (!holder) return { steps: at, value: claim };
  const value =
    claim === undefined
      ? without(holder.value ?? {}, holder.rest)
      : withValue(holder.value ?? {}, holder.rest, claim);
  return { steps: at, value: isEmpty(value) ? undefined : value };
}

/**
 * selectionSettingsOf returns the settings of the parameters of one
 * selection. fields are the parameterFields of that selection and schema is
 * spec.parameters of its registration. A locked parameter is read only. An
 * object parameter with properties gives one setting for each property,
 * and a change of one writes the whole parameter. Any other parameter is
 * one setting. A property of overrides gets its editor from
 * overrideEditorOf, and any other parameter from parameterEditorOf. A
 * parameter in choices picks the names of its choices, whatever its schema,
 * because the claim stores the names: one name, or a list of names for a
 * list parameter. A parameter resets to the environment, and its value
 * replaces the value of the environment.
 */
export function selectionSettingsOf(
  fields: readonly ClusterField[],
  schema: OpenAPISchema | undefined,
  choices?: Partial<Record<string, readonly string[]>>,
): Setting[] {
  const properties = propertiesOf(schema);
  return fields.flatMap((field): Setting[] => {
    const name = field.label;
    const base: Setting = {
      key: field.key,
      label: name,
      value: field.value,
      state: field.state,
      claimSet: field.state.kind === "claim",
      resetLabel: "Reset to environment",
      replaces: true,
    };
    if (field.lock || !field.claimSteps)
      return [{ ...base, ...(field.lock && { lock: field.lock }) }];
    const options = Object.hasOwn(choices ?? {}, name)
      ? choices?.[name]
      : undefined;
    const property = Object.hasOwn(properties, name)
      ? properties[name]
      : undefined;
    if (options) {
      // Without a schema, the value tells whether the parameter is a list.
      const editor = parameterEditorOf(
        property ?? (Array.isArray(field.value) ? { type: "array" } : {}),
        { choices: options },
      );
      return [{ ...base, editor, steps: field.claimSteps }];
    }
    const inner = propertiesOf(property);
    if (Object.keys(inner).length > 0)
      return Object.entries(inner).map(([part, partSchema]) => {
        const value = valueAt(field.value, [{ field: part }]);
        const editor =
          name === "overrides"
            ? overrideEditorOf(part, partSchema, { value })
            : parameterEditorOf(partSchema, { value });
        return {
          ...base,
          key: `${field.key}.${part}`,
          label: `${name}.${part}`,
          value,
          ...(editor && {
            ...blockReset(name, "environment"),
            editor,
            steps: field.claimSteps,
            holder: { value: field.value, rest: [{ field: part }] },
          }),
        };
      });
    const editor = parameterEditorOf(property, { value: field.value });
    return [{ ...base, ...(editor && { editor, steps: field.claimSteps }) }];
  });
}

function lengthOf(value: unknown): number {
  return Array.isArray(value) ? value.length : 0;
}

function blockReset(block: string, layer: "preset" | "environment") {
  return {
    resetLabel: `Reset ${block} to ${layer}`,
    resetNote: `Every ${block} field of the claim goes back to the ${layer}.`,
  };
}

function propertiesOf(schema: unknown): Record<string, unknown> {
  return isObject(schema) && isObject(schema.properties)
    ? schema.properties
    : {};
}

// secretRefPaths folds the name and key leaves of each Secret reference of
// paths into the path of the reference.
function secretRefPaths(
  root: SchemaRoot,
  paths: readonly string[],
): { path: string; secretRef: boolean }[] {
  const result: { path: string; secretRef: boolean }[] = [];
  for (const path of paths) {
    const parent = path.slice(0, path.lastIndexOf("."));
    if (parent && isSecretRef(root, parent)) {
      if (!result.some((r) => r.path === parent))
        result.push({ path: parent, secretRef: true });
    } else result.push({ path, secretRef: false });
  }
  return result;
}

function isSecretRef(root: SchemaRoot, path: string): boolean {
  const below = Object.keys(fieldSchemas[root])
    .filter((p) => p.startsWith(`${path}.`))
    .sort();
  return (
    path.endsWith("Ref") &&
    below.length === 2 &&
    below[0] === `${path}.key` &&
    below[1] === `${path}.name`
  );
}

function steps(names: readonly string[]): Step[] {
  return names.map((field) => ({ field }));
}

function sameNames(a: readonly string[], b: readonly string[]): boolean {
  return a.length === b.length && startsWith(a, b);
}

function startsWith(names: readonly string[], prefix: readonly string[]) {
  return prefix.every((name, i) => names[i] === name);
}

function isEmpty(value: unknown): boolean {
  if (value === undefined || value === null) return true;
  if (Array.isArray(value)) return value.length === 0;
  return isObject(value) && Object.keys(value).length === 0;
}

function isObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
