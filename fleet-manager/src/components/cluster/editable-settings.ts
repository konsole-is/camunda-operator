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
 * map key by key.
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
    if (holder?.field.claimSteps)
      return {
        ...setting,
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
    return { ...setting, editor, steps: [...base, ...steps(names)] };
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
