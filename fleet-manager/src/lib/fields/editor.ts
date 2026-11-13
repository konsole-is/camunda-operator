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
 * undefined when the field has neither a schema nor choices. choices are
 * names that the field takes in place of its values. openAPI is the
 * schema that schema comes from. The first rule that holds wins:
 *
 *  1. With choices: a pick of names for an array, else a select of one.
 *  2. By the last name of path: resources, storageSize, extraEnv
 *     (environment variables), and tolerations.
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
  if (Object.hasOwn(BY_NAME, last)) return BY_NAME[last];
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
 * registration, with schema its property schema, or undefined when it has
 * neither a schema nor choices. choices are the names that the claim
 * stores in place of the values.
 */
export function parameterEditorOf(
  name: string,
  schema: unknown,
  choices?: readonly string[],
): SchemaEditor | undefined {
  return editorOf(name, fieldSchemaFromOpenAPI(schema), {
    ...(choices && { choices }),
    ...(isObject(schema) && { openAPI: schema as OpenAPISchema }),
  });
}

const BY_NAME: Partial<Record<string, SchemaEditor>> = {
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
