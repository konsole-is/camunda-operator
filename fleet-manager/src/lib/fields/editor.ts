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
 */
export function editorOf(
  path: string,
  schema: FieldSchema | undefined,
  {
    choices,
    openAPI,
  }: { choices?: readonly string[]; openAPI?: OpenAPISchema } = {},
): SchemaEditor | undefined {
  if (choices)
    return schema?.type === "array"
      ? { kind: "choices", options: choices }
      : { kind: "select", options: choices };
  const last = path.slice(path.lastIndexOf(".") + 1);
  const byName = Object.hasOwn(BY_NAME, last) ? BY_NAME[last] : undefined;
  if (byName && (!openAPI || byName.fits(openAPI))) return byName.editor;
  const full = openAPI ? undefined : fullSchemaOf(path);
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
  if (openAPI) return { kind: "yaml", schema: openAPI };
  if (schema.type === undefined) return { kind: "yaml" };
  return { kind: "yaml", schema: openAPIOf(schema) };
}

/**
 * parameterEditorOf returns the editor of the parameter name of a
 * registration, with schema its property schema, or undefined only when
 * schema is not an object and there are no choices. choices are the names
 * that the claim stores in place of the values. A value that the editor of
 * a cluster field or of a map of strings cannot read gives YAML.
 */
export function parameterEditorOf(
  name: string,
  schema: unknown,
  { choices, value }: { choices?: readonly string[]; value?: unknown } = {},
): SchemaEditor | undefined {
  const openAPI = isObject(schema) ? (schema as OpenAPISchema) : undefined;
  const editor = editorOf(name, fieldSchemaFromOpenAPI(schema), {
    ...(choices && { choices }),
    ...(openAPI && { openAPI }),
  });
  return editor && !reads(editor, value)
    ? { kind: "yaml", ...(openAPI && { schema: openAPI }) }
    : editor;
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
// checks only the editors that fail on a value of another shape.
function reads(editor: SchemaEditor, value: unknown): boolean {
  if (value === undefined || value === null) return true;
  switch (editor.kind) {
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
      return (
        Array.isArray(value) &&
        value.every(
          (e) =>
            isObject(e) &&
            isString(e.name) &&
            (e.value === undefined || isString(e.value)) &&
            (e.valueFrom === undefined || isObject(e.valueFrom)),
        )
      );
    case "tolerations":
      return (
        Array.isArray(value) &&
        value.every(
          (t) =>
            isObject(t) &&
            ["key", "operator", "value", "effect"].every(
              (f) => t[f] === undefined || isString(t[f]),
            ) &&
            (t.tolerationSeconds === undefined ||
              typeof t.tolerationSeconds === "number"),
        )
      );
    default:
      return true;
  }
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
 * not finite, an empty list, an empty YAML map, resources without a request
 * or a limit, and no Secret reference. A toggle keeps false, a list of
 * choices keeps an empty list, and a map of strings keeps {}. Resources
 * drop an empty requests or limits.
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
    case "keyValue":
      return value;
    default:
      return isEmpty(value) ? undefined : value;
  }
}

/**
 * editorValueOf returns the input of an editor for a claim value. An unset
 * value, undefined or null, is the empty input of the editor: an empty
 * string, false, an empty list, an empty map for resources, or undefined
 * for a number, a map of strings, and a Secret reference. For YAML, the
 * result keeps null.
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
