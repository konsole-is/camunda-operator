import {
  type LimitBelowRequest,
  limitsBelowRequests,
  type Resources,
} from "@/components/fleet/resources-editor";
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
  optimizeClaimNames,
  overrideEditorOf,
  parameterEditorOf,
  type SchemaRoot,
  type SettingEditor,
  type Step,
} from "@/lib/fields";
import type { OpenAPISchema } from "@/lib/schema";
import { valueAt, without, withValue } from "@/lib/session";
import type { SettingsGroup } from "./settings";

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
 * map key by key. mergedErrors are the limits of the merged value that are
 * below their request.
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
 * MergedError is a limit below its request in the merged value of a
 * setting. blocks is false when the value does not apply now, as the
 * resources of an embedded app.
 */
export type MergedError = {
  message: string;
  belowRequest: LimitBelowRequest;
  blocks: boolean;
};

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
 * With claim, spec is a merge of layers, and a resources setting gets
 * mergedErrors: each limit of spec below its request, for any resource.
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
    const merged: MergedError[] =
      "claim" in input && editor.kind === "resources"
        ? limitsBelowRequests((value ?? {}) as Resources).map((below) => ({
            message: mergedLimitError(below, (side) => {
              const part = [...names, side, below.resource];
              return mine.find(({ names: f }) => sameNames(f, part))?.field
                .state;
            }),
            belowRequest: below,
            blocks: !group.embedded,
          }))
        : [];
    const checked: Setting =
      merged.length > 0 ? { ...setting, mergedErrors: merged } : setting;
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

// mergedLimitError returns the message of a limit below its request. It
// names where the limit and the request come from when that is not the
// claim. stateOf gives the state of the requests or the limits of the
// resource.
function mergedLimitError(
  below: LimitBelowRequest,
  stateOf: (side: "requests" | "limits") => FieldState | undefined,
): string {
  const name = RESOURCE_NAMES[below.resource] ?? below.resource;
  const limit = `${name} limit of ${below.limit}`;
  const request = `${name} request of ${below.request}`;
  const limitFrom = originOf(stateOf("limits"));
  const requestFrom = originOf(stateOf("requests"));
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
  return {
    one: "is already set on the cluster",
    many: "are already set on the cluster",
  };
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
