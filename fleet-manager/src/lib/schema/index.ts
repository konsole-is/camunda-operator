import Ajv, { type ErrorObject, type ValidateFunction } from "ajv";

/** OpenAPISchema is an OpenAPI v3 schema, as a CRD or a registration holds it. */
export type OpenAPISchema = Record<string, unknown>;

/** ValidationError names a value by its path and says what is wrong with it. */
export type ValidationError = { path: string; message: string };

export type ValidationResult =
  { ok: true } | { ok: false; errors: ValidationError[] };

const ajv = new Ajv({ allErrors: true, strict: false, validateFormats: false });
const compiled = new WeakMap<object, ValidateFunction | Error>();

/**
 * validate checks value against an OpenAPI v3 schema and returns every error.
 * A path joins property names and array indexes with dots, for example
 * "buckets.1". The empty path is the value itself. A missing required
 * property has its own path. The check ignores formats and the keywords that
 * only Kubernetes knows, such as x-kubernetes-int-or-string. A schema that
 * cannot compile gives one error at the empty path.
 */
export function validate(
  schema: OpenAPISchema,
  value: unknown,
): ValidationResult {
  const check = compile(schema);
  if (check instanceof Error) {
    return {
      ok: false,
      errors: [
        { path: "", message: `The schema is not valid: ${check.message}` },
      ],
    };
  }
  if (check(value)) return { ok: true };
  return { ok: false, errors: (check.errors ?? []).map(toError) };
}

function compile(schema: OpenAPISchema): ValidateFunction | Error {
  let check = compiled.get(schema);
  if (!check) {
    try {
      check = ajv.compile(withNumericBounds(schema) as OpenAPISchema);
    } catch (error) {
      check = error instanceof Error ? error : new Error(String(error));
    }
    compiled.set(schema, check);
  }
  return check;
}

// Only these keywords hold schemas. enum, default, and example hold data,
// which must stay as it is.
const SCHEMA_KEYWORDS = [
  "items",
  "additionalProperties",
  "additionalItems",
  "not",
  "allOf",
  "anyOf",
  "oneOf",
];
const SCHEMA_MAP_KEYWORDS = ["properties", "patternProperties", "definitions"];

// OpenAPI 3.0, as a CRD writes it, marks an exclusive bound with a boolean
// beside minimum or maximum. Ajv reads only the numeric form of JSON Schema.
function withNumericBounds(schema: unknown): unknown {
  if (Array.isArray(schema)) return schema.map(withNumericBounds);
  if (schema === null || typeof schema !== "object") return schema;
  const result: Record<string, unknown> = { ...schema };
  for (const key of SCHEMA_KEYWORDS) {
    if (Object.hasOwn(result, key))
      result[key] = withNumericBounds(result[key]);
  }
  for (const key of SCHEMA_MAP_KEYWORDS) {
    const map = result[key];
    if (map !== null && typeof map === "object" && !Array.isArray(map))
      result[key] = Object.fromEntries(
        Object.entries(map).map(([name, value]) => [
          name,
          withNumericBounds(value),
        ]),
      );
  }
  for (const [exclusive, bound] of [
    ["exclusiveMinimum", "minimum"],
    ["exclusiveMaximum", "maximum"],
  ]) {
    if (typeof result[exclusive] !== "boolean") continue;
    if (result[exclusive] && typeof result[bound] === "number") {
      result[exclusive] = result[bound];
      delete result[bound];
    } else {
      delete result[exclusive];
    }
  }
  return result;
}

function toError(error: ErrorObject): ValidationError {
  const segments = error.instancePath
    .split("/")
    .slice(1)
    .map((s) => s.replaceAll("~1", "/").replaceAll("~0", "~"));
  const path = (extra?: unknown) =>
    [...segments, ...(extra === undefined ? [] : [String(extra)])].join(".");
  switch (error.keyword) {
    case "required":
      return {
        path: path(error.params.missingProperty),
        message: "is required",
      };
    case "additionalProperties":
      return {
        path: path(error.params.additionalProperty),
        message: "is not a known field",
      };
    case "enum":
      return {
        path: path(),
        message: `must be one of ${(error.params.allowedValues as unknown[])
          .map((v) => JSON.stringify(v))
          .join(", ")}`,
      };
    default:
      return { path: path(), message: error.message ?? "is not valid" };
  }
}

export { type SchemaRule, schemaRules } from "./rules";
