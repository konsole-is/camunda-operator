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

// OpenAPI 3.0, as a CRD writes it, marks an exclusive bound with a boolean
// beside minimum or maximum. Ajv reads only the numeric form of JSON Schema.
function withNumericBounds(node: unknown): unknown {
  if (Array.isArray(node)) return node.map(withNumericBounds);
  if (node === null || typeof node !== "object") return node;
  const result: Record<string, unknown> = Object.fromEntries(
    Object.entries(node).map(([key, value]) => [key, withNumericBounds(value)]),
  );
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
