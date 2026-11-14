import type { FieldSchema } from "@/generated/fieldschema";
import type { OpenAPISchema } from "@/lib/schema";
import { fullSchemaOf } from "./schema";

/** SettingEditor is the editor of one setting and what it needs. */
export type SettingEditor =
  | { kind: "text" }
  | { kind: "number"; integer: boolean; minimum?: number; maximum?: number }
  | { kind: "toggle" }
  | { kind: "select"; options: readonly string[] }
  | { kind: "choices"; options: readonly string[] }
  | { kind: "resources" }
  | { kind: "storageSize" }
  | { kind: "env" }
  | { kind: "keyValue"; format?: "label" }
  | { kind: "tolerations" }
  | { kind: "secretRef" }
  | { kind: "yaml"; schema?: OpenAPISchema };

/** SchemaEditor is an editor that a schema picks: any but a Secret reference. */
export type SchemaEditor = Exclude<SettingEditor, { kind: "secretRef" }>;

/**
 * ParameterEditor is an editor that a registration schema picks: any but a
 * Secret reference and the editors that only the name of a cluster field
 * picks.
 */
export type ParameterEditor = Exclude<
  SchemaEditor,
  { kind: "resources" | "storageSize" | "env" | "tolerations" }
>;

type YamlEditor = Extract<SchemaEditor, { kind: "yaml" }>;

/**
 * editorOf returns the editor of the field at path with schema, or
 * undefined only when the field has neither a schema nor choices. choices
 * are names that the field takes in place of its values. openAPI is the
 * schema that schema comes from. The first rule that holds wins:
 *
 *  1. With choices: a pick of names for an array, else a select of one.
 *  2. By the last name of path: resources, storageSize, extraEnv
 *     (environment variables), and tolerations, unless openAPI, its items,
 *     or its requests or limits do not take the shape that this editor
 *     writes.
 *  3. Without openAPI, a field of fullSchemaOf: YAML checked against that
 *     schema.
 *  4. An enum: a select of its values. A boolean: a toggle. An integer or a
 *     number: a number in its range. A string or an int-or-string: text.
 *  5. An object: a map of strings, with label rules for podLabels.
 *  6. Anything else: YAML, checked against openAPI, else against the type
 *     of the schema when it has one.
 *
 * value is the value that the editor opens with. When the editor of rule
 * 2, or the text or map editor of rules 4 and 5, cannot keep it, the
 * result is YAML, checked as in rule 6.
 */
export function editorOf(
  path: string,
  schema: FieldSchema | undefined,
  {
    choices,
    openAPI,
    value,
  }: {
    choices?: readonly string[];
    openAPI?: OpenAPISchema;
    value?: unknown;
  } = {},
): SchemaEditor | undefined {
  const last = path.slice(path.lastIndexOf(".") + 1);
  if (!choices) {
    const byName = Object.hasOwn(BY_NAME, last) ? BY_NAME[last] : undefined;
    if (byName && (!openAPI || byName.fits(openAPI)))
      return kept(byName.editor, value, openAPI ?? openAPIOf(schema));
    const full = openAPI ? undefined : fullSchemaOf(path);
    if (full) return { kind: "yaml", schema: full };
  }
  return schemaEditorOf(schema, {
    choices,
    openAPI,
    value,
    labels: last === "podLabels",
  });
}

/**
 * parameterEditorOf returns the editor of a registration parameter with
 * schema its property schema, or undefined only when schema is not an
 * object and there are no choices. choices are the names that the claim
 * stores in place of the values. The editor does not depend on the name of
 * the parameter: rules 1 and 4 to 6 of editorOf. A value that the text or
 * map editor cannot keep gives YAML.
 */
export function parameterEditorOf(
  schema: unknown,
  { choices, value }: { choices?: readonly string[]; value?: unknown } = {},
): ParameterEditor | undefined {
  return schemaEditorOf(fieldSchemaFromOpenAPI(schema), {
    choices,
    openAPI: isObject(schema) ? (schema as OpenAPISchema) : undefined,
    value,
  });
}

/**
 * overrideEditorOf returns the editor of the part name of the parameter
 * overrides of a registration, with schema its property schema, or
 * undefined as editorOf. A part named resources, storageSize, extraEnv, or
 * tolerations gets the editor of that cluster field when schema fits it and
 * the editor can keep value, else YAML.
 */
export function overrideEditorOf(
  name: string,
  schema: unknown,
  { value }: { value?: unknown } = {},
): SchemaEditor | undefined {
  return editorOf(name, fieldSchemaFromOpenAPI(schema), {
    ...(isObject(schema) && { openAPI: schema as OpenAPISchema }),
    value,
  });
}

// schemaEditorOf picks the editor by rules 1 and 4 to 6 of editorOf, with
// label rules for a map of strings when labels is set.
function schemaEditorOf(
  schema: FieldSchema | undefined,
  {
    choices,
    openAPI,
    value,
    labels,
  }: {
    choices?: readonly string[];
    openAPI?: OpenAPISchema;
    value?: unknown;
    labels?: boolean;
  },
): ParameterEditor | undefined {
  if (choices)
    return schema?.type === "array"
      ? { kind: "choices", options: choices }
      : { kind: "select", options: choices };
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
  const yaml = openAPI ?? openAPIOf(schema);
  if (schema.type === "string" || schema.intOrString)
    return kept({ kind: "text" }, value, yaml);
  if (schema.type === "object")
    return kept(
      labels ? { kind: "keyValue", format: "label" } : { kind: "keyValue" },
      value,
      yaml,
    );
  return yamlOf(yaml);
}

// kept returns editor when it can keep value as it is, else YAML checked
// against schema.
function kept<E extends SchemaEditor>(
  editor: E,
  value: unknown,
  schema: OpenAPISchema | undefined,
): E | YamlEditor {
  return reads(editor, value) ? editor : yamlOf(schema);
}

function yamlOf(schema: OpenAPISchema | undefined): YamlEditor {
  return { kind: "yaml", ...(schema && { schema }) };
}

const BY_NAME: Partial<
  Record<
    string,
    { editor: SchemaEditor; fits: (schema: Record<string, unknown>) => boolean }
  >
> = {
  resources: {
    editor: { kind: "resources" },
    fits: (s) =>
      takes(s, "object") &&
      takesObjectAt(s, "requests") &&
      takesObjectAt(s, "limits"),
  },
  storageSize: {
    editor: { kind: "storageSize" },
    fits: (s) => takes(s, "string"),
  },
  extraEnv: { editor: { kind: "env" }, fits: isListOfObjects },
  tolerations: { editor: { kind: "tolerations" }, fits: isListOfObjects },
};

// takes tells whether schema has the type, or no type.
function takes(schema: unknown, type: string): boolean {
  return (
    isObject(schema) && (schema.type === undefined || schema.type === type)
  );
}

function takesObjectAt(schema: Record<string, unknown>, key: string): boolean {
  const properties = isObject(schema.properties) ? schema.properties : {};
  const at = Object.hasOwn(properties, key)
    ? properties[key]
    : (schema.additionalProperties ?? true);
  return at === true || takes(at, "object");
}

function isListOfObjects(schema: Record<string, unknown>): boolean {
  return (
    takes(schema, "array") &&
    (schema.items === undefined || takes(schema.items, "object"))
  );
}

// reads tells whether editor can show value and keep it in an edit. It
// returns true for any value with a number, toggle, select, choices, or
// YAML editor.
function reads(editor: SchemaEditor, value: unknown): boolean {
  if (value === undefined || value === null) return true;
  switch (editor.kind) {
    case "text":
      return isString(value);
    case "keyValue":
      return isObject(value) && Object.values(value).every(isString);
    case "resources":
      return (
        isObject(value) &&
        ["requests", "limits"].every(
          (side) =>
            value[side] === undefined ||
            (isObject(value[side]) &&
              Object.values(value[side]).every(
                (q) => isString(q) || typeof q === "number",
              )),
        )
      );
    case "storageSize":
      return isString(value) || typeof value === "number";
    case "env":
      return Array.isArray(value) && value.every(keepsEnv);
    case "tolerations":
      return Array.isArray(value) && value.every(keepsToleration);
    default:
      return true;
  }
}

const ENV_FIELDS = ["name", "value", "valueFrom"];

// keepsEnv returns true only when the env editor keeps entry, apart from an
// empty value that it can add.
function keepsEnv(entry: unknown): boolean {
  if (
    !isObject(entry) ||
    !isString(entry.name) ||
    !Object.keys(entry).every((k) => ENV_FIELDS.includes(k))
  )
    return false;
  const from = entry.valueFrom;
  if (from === undefined)
    return entry.value === undefined || isString(entry.value);
  if (!isObject(from)) return false;
  // EnvEditor writes an entry with only a Secret key that is not optional
  // from the name and the key of the reference. It keeps any other entry
  // whole.
  const ref = from.secretKeyRef;
  if (
    !isObject(ref) ||
    Object.keys(from).length !== 1 ||
    !ref.name ||
    ref.optional
  )
    return true;
  return (
    entry.value === undefined &&
    Object.keys(ref).every((k) => k === "name" || k === "key")
  );
}

const TOLERATION_FIELDS = [
  "key",
  "operator",
  "value",
  "effect",
  "tolerationSeconds",
];

// keepsToleration returns true only when the tolerations editor keeps
// toleration, apart from a default operator that it adds and empty fields
// that it drops.
function keepsToleration(toleration: unknown): boolean {
  return (
    isObject(toleration) &&
    Object.keys(toleration).every((k) => TOLERATION_FIELDS.includes(k)) &&
    ["key", "operator", "value", "effect"].every(
      (f) => toleration[f] === undefined || isString(toleration[f]),
    ) &&
    // The editor writes a value only for an operator other than Exists, and
    // seconds only for the effect NoExecute.
    !(toleration.operator === "Exists" && toleration.value) &&
    (toleration.tolerationSeconds === undefined ||
      (Number.isInteger(toleration.tolerationSeconds) &&
        toleration.effect === "NoExecute"))
  );
}

/**
 * fieldSchemaFromOpenAPI returns the FieldSchema of one property of a
 * registration schema, or undefined when schema is not an object. An object
 * is a map of strings only when its additionalProperties are strings. Any
 * other object gives an empty FieldSchema.
 */
export function fieldSchemaFromOpenAPI(
  schema: unknown,
): FieldSchema | undefined {
  if (!isObject(schema)) return undefined;
  const { type } = schema;
  if (type === "object")
    return isObject(schema.additionalProperties) &&
      schema.additionalProperties.type === "string" &&
      [schema.properties, schema.patternProperties].every(takesStrings)
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
 * and no Secret reference. A toggle keeps false, and a list of choices keeps
 * an empty list. Resources drop an empty requests or limits.
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
    case "choices":
      return value;
    default:
      return isEmpty(value) ? undefined : value;
  }
}

/**
 * editorValueOf returns the input of an editor for a claim value. An unset
 * value, undefined or null, is the empty input of the editor: an empty
 * string, false, an empty list or map, or undefined for a number and a
 * Secret reference. For YAML, the result keeps null.
 */
export function editorValueOf(
  editor: SettingEditor,
  claimValue: unknown,
): unknown {
  if (
    claimValue !== undefined &&
    (claimValue !== null || editor.kind === "yaml")
  )
    return claimValue;
  switch (editor.kind) {
    case "text":
    case "select":
    case "storageSize":
      return "";
    case "toggle":
      return false;
    case "env":
    case "tolerations":
    case "choices":
      return [];
    case "keyValue":
    case "resources":
      return {};
    default:
      return undefined;
  }
}

function resourcesOf(value: unknown): unknown {
  if (!isObject(value)) return undefined;
  const result: Record<string, unknown> = {};
  for (const [side, map] of Object.entries(value))
    if (!(isObject(map) && Object.keys(map).length === 0)) result[side] = map;
  return Object.keys(result).length === 0 ? undefined : result;
}

// openAPIOf returns the schema of the type of schema, or undefined when it
// has no type.
function openAPIOf(schema: FieldSchema | undefined): OpenAPISchema | undefined {
  if (schema?.type === undefined) return undefined;
  const items = openAPIOf(schema.items);
  return { type: schema.type, ...(items && { items }) };
}

function isEmpty(value: unknown): boolean {
  if (value === undefined || value === null) return true;
  if (Array.isArray(value)) return value.length === 0;
  return isObject(value) && Object.keys(value).length === 0;
}

function isObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function takesStrings(properties: unknown): boolean {
  return (
    properties === undefined ||
    (isObject(properties) &&
      Object.values(properties).every((p) => p === true || takes(p, "string")))
  );
}

function isString(value: unknown): value is string {
  return typeof value === "string";
}
