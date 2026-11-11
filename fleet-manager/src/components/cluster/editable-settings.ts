import { fieldSchemas } from "@/generated/fieldschema";
import {
  type ClusterField,
  COMPONENTS,
  componentFieldPaths,
  type FieldLock,
  type FieldSchema,
  type FieldState,
  fieldSchemaOf,
  fullSchemaOf,
  optimizeClaimNames,
  type SchemaRoot,
  type Step,
} from "@/lib/fields";
import type { OpenAPISchema } from "@/lib/schema";
import { valueAt, without, withValue } from "@/lib/session";
import type { SettingsGroup } from "./settings";

/** SettingEditor is the editor of one setting and what it needs. */
export type SettingEditor =
  | { kind: "text" }
  | { kind: "number"; integer: boolean; minimum?: number; maximum?: number }
  | { kind: "toggle" }
  | { kind: "select"; options: readonly string[] }
  | { kind: "resources" }
  | { kind: "storageSize" }
  | { kind: "env" }
  | { kind: "keyValue"; format?: "label" }
  | { kind: "tolerations" }
  | { kind: "secretRef" }
  | { kind: "yaml"; schema?: OpenAPISchema };

/**
 * editorOf returns the editor of the field at path with schema, or
 * undefined when the field has no schema. The first rule that holds wins:
 *
 *  1. By the last name of path: resources, storageSize, extraEnv
 *     (environment variables), and tolerations.
 *  2. A field of fullSchemaOf: YAML checked against that schema.
 *  3. An enum: a select of its values. A boolean: a toggle. An integer or a
 *     number: a number in its range. A string or an int-or-string: text.
 *  4. An object: a map of strings, with label rules for podLabels.
 *  5. Anything else: YAML, checked against the type of the schema when it
 *     has one.
 */
export function editorOf(
  path: string,
  schema: FieldSchema | undefined,
): SettingEditor | undefined {
  const last = path.slice(path.lastIndexOf(".") + 1);
  const byName = BY_NAME[last];
  if (byName) return byName;
  const full = fullSchemaOf(path);
  if (full) return { kind: "yaml", schema: full };
  if (schema === undefined) return undefined;
  if (schema.enum) return { kind: "select", options: schema.enum };
  if (schema.type === "boolean") return { kind: "toggle" };
  if (schema.type === "integer" || schema.type === "number")
    return {
      kind: "number",
      integer: schema.type === "integer",
      ...(schema.minimum !== undefined && { minimum: schema.minimum }),
      ...(schema.maximum !== undefined && { maximum: schema.maximum }),
    };
  if (schema.type === "string" || schema.intOrString) return { kind: "text" };
  if (schema.type === "object")
    return last === "podLabels"
      ? { kind: "keyValue", format: "label" }
      : { kind: "keyValue" };
  if (schema.type === undefined) return { kind: "yaml" };
  return { kind: "yaml", schema: openAPIOf(schema) };
}

const BY_NAME: Partial<Record<string, SettingEditor>> = {
  resources: { kind: "resources" },
  storageSize: { kind: "storageSize" },
  extraEnv: { kind: "env" },
  tolerations: { kind: "tolerations" },
};

/**
 * fieldSchemaFromOpenAPI returns the FieldSchema of one property of a
 * registration schema, or undefined when schema is not an object. An object
 * is a map of strings only when its additionalProperties are strings. Any
 * other object gives an empty FieldSchema, which editorOf edits as YAML.
 */
export function fieldSchemaFromOpenAPI(
  schema: unknown,
): FieldSchema | undefined {
  if (!isObject(schema)) return undefined;
  const { type } = schema;
  if (type === "object")
    return isObject(schema.additionalProperties) &&
      schema.additionalProperties.type === "string"
      ? { type: "object" }
      : {};
  const result: FieldSchema = {};
  if (
    type === "string" ||
    type === "integer" ||
    type === "number" ||
    type === "boolean" ||
    type === "array"
  )
    result.type = type;
  if (Array.isArray(schema.enum) && schema.enum.every(isString))
    result.enum = schema.enum;
  if (typeof schema.minimum === "number") result.minimum = schema.minimum;
  if (typeof schema.maximum === "number") result.maximum = schema.maximum;
  if (typeof schema.format === "string") result.format = schema.format;
  if (schema["x-kubernetes-int-or-string"] === true) result.intOrString = true;
  const items = fieldSchemaFromOpenAPI(schema.items);
  if (items) result.items = items;
  return result;
}

/**
 * claimValueOf returns the claim value of an editor input, or undefined for
 * an empty input, which resets the field: an empty string, a number that is
 * not finite, an empty list or map, resources without a request or a limit,
 * and no Secret reference. A toggle keeps false. Resources drop an empty
 * requests or limits.
 */
export function claimValueOf(editor: SettingEditor, value: unknown): unknown {
  switch (editor.kind) {
    case "text":
    case "select":
    case "storageSize":
      return value === "" ? undefined : value;
    case "number":
      return typeof value === "number" && Number.isFinite(value)
        ? value
        : undefined;
    case "toggle":
      return value;
    case "resources":
      return resourcesOf(value);
    default:
      return isEmpty(value) ? undefined : value;
  }
}

/**
 * editorValueOf returns the input of an editor for a claim value. An unset
 * value is the empty input of the editor: an empty string, false, an empty
 * list or map, or undefined for a number, YAML, and a Secret reference.
 */
export function editorValueOf(
  editor: SettingEditor,
  claimValue: unknown,
): unknown {
  if (claimValue !== undefined) return claimValue;
  switch (editor.kind) {
    case "text":
    case "select":
    case "storageSize":
      return "";
    case "toggle":
      return false;
    case "env":
    case "tolerations":
      return [];
    case "keyValue":
    case "resources":
      return {};
    default:
      return undefined;
  }
}

/**
 * Setting is one editor of a settings group. steps is where a change goes,
 * from the root of the claim, and is undefined when the setting cannot
 * change. When a layer sets a whole field above the setting, steps is that
 * field and holder holds its value and the rest of the path to the
 * setting, so that a change keeps the rest of the field. claimSet shows the
 * reset link.
 */
export type Setting = {
  key: string;
  label: string;
  editor?: SettingEditor;
  value: unknown;
  state?: FieldState;
  lock?: FieldLock;
  steps?: Step[];
  holder?: { value: unknown; rest: Step[] };
  claimSet: boolean;
  resetLabel: "Reset to preset" | "Reset to environment";
};

/**
 * settingsOf returns the settings of group. spec holds the values by their
 * claim names: the merged CamundaCluster spec for a process or the
 * cluster, and spec.attachments.optimize of the claim for Optimize.
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
}): Setting[] {
  const { group, spec } = input;
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
    const setting: Setting = {
      key: `${group.component}:${path}`,
      label: names.slice(prefix).join("."),
      value: valueAt(spec, steps(names)),
      ...(state && { state }),
      claimSet: mine.some(({ field }) => field.state.kind === "claim"),
      resetLabel: "Reset to preset",
    };
    if (lock) return { ...setting, lock };
    const editor = secretRef
      ? ({ kind: "secretRef" } as const)
      : editorOf(path, fieldSchemaOf(root, path));
    if (!editor) return setting;
    if (holder?.field.claimSteps)
      return {
        ...setting,
        editor,
        steps: holder.field.claimSteps,
        holder: {
          value: holder.field.value,
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
 * one setting. choices make a select of a string parameter. A parameter
 * resets to the environment.
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
    };
    if (field.lock || !field.claimSteps)
      return [{ ...base, ...(field.lock && { lock: field.lock }) }];
    const property = Object.hasOwn(properties, name)
      ? properties[name]
      : undefined;
    const inner = propertiesOf(property);
    if (Object.keys(inner).length > 0)
      return Object.entries(inner).map(([part, partSchema]) => {
        const editor = editorOf(part, fieldSchemaFromOpenAPI(partSchema));
        return {
          ...base,
          key: `${field.key}.${part}`,
          label: `${name}.${part}`,
          value: valueAt(field.value, [{ field: part }]),
          ...(editor && {
            editor,
            steps: field.claimSteps,
            holder: { value: field.value, rest: [{ field: part }] },
          }),
        };
      });
    const own = fieldSchemaFromOpenAPI(property);
    const options = Object.hasOwn(choices ?? {}, name)
      ? choices?.[name]
      : undefined;
    const editor =
      options && (own === undefined || own.type === "string")
        ? ({ kind: "select", options } as const)
        : editorOf(name, own);
    return [{ ...base, ...(editor && { editor, steps: field.claimSteps }) }];
  });
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

function resourcesOf(value: unknown): unknown {
  if (!isObject(value)) return undefined;
  const result: Record<string, unknown> = {};
  for (const [side, map] of Object.entries(value))
    if (!(isObject(map) && Object.keys(map).length === 0)) result[side] = map;
  return Object.keys(result).length === 0 ? undefined : result;
}

function openAPIOf(schema: FieldSchema): OpenAPISchema {
  return {
    type: schema.type,
    ...(schema.items && { items: openAPIOf(schema.items) }),
  };
}

function isEmpty(value: unknown): boolean {
  if (value === undefined || value === null) return true;
  if (Array.isArray(value)) return value.length === 0;
  return isObject(value) && Object.keys(value).length === 0;
}

function isObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function isString(value: unknown): value is string {
  return typeof value === "string";
}
