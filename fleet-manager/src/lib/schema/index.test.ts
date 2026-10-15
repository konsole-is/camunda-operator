import { describe, expect, it } from "vitest";
import { validate } from "./index";

const retention = {
  type: "object",
  required: ["days"],
  properties: {
    days: { type: "integer", minimum: 1, maximum: 365 },
    buckets: { type: "array", items: { type: "string", maxLength: 5 } },
  },
};

describe("validate", () => {
  it("accepts a value that matches the schema", () => {
    expect(validate(retention, { days: 30, buckets: ["a"] })).toEqual({
      ok: true,
    });
  });

  it("names an array item by its index", () => {
    const result = validate(retention, {
      days: 30,
      buckets: ["ok", "toolong"],
    });
    expect(result).toEqual({
      ok: false,
      errors: [
        { path: "buckets.1", message: "must NOT have more than 5 characters" },
      ],
    });
  });

  it("names a missing required property by its own path", () => {
    expect(validate(retention, {})).toEqual({
      ok: false,
      errors: [{ path: "days", message: "is required" }],
    });
  });

  it("returns every error, not only the first", () => {
    const result = validate(retention, { days: 0, buckets: [1] });
    expect(result.ok).toBe(false);
    expect(result.ok ? [] : result.errors.map((e) => e.path)).toEqual([
      "days",
      "buckets.0",
    ]);
  });

  it("validates the value itself at the empty path", () => {
    expect(validate({ type: "integer", maximum: 365 }, 500)).toEqual({
      ok: false,
      errors: [{ path: "", message: "must be <= 365" }],
    });
  });

  it("lists the allowed values of an enum", () => {
    expect(validate({ type: "string", enum: ["One", "Many"] }, "Two")).toEqual({
      ok: false,
      errors: [{ path: "", message: 'must be one of "One", "Many"' }],
    });
  });

  it("accepts null for a nullable property", () => {
    expect(validate({ type: "string", nullable: true }, null)).toEqual({
      ok: true,
    });
  });

  it("ignores Kubernetes keywords and formats that it does not know", () => {
    const quantity = {
      "x-kubernetes-int-or-string": true,
      anyOf: [{ type: "integer" }, { type: "string" }],
      format: "quantity",
    };
    expect(validate(quantity, "500m")).toEqual({ ok: true });
    expect(validate(quantity, 2)).toEqual({ ok: true });
  });

  it("reads the boolean exclusive bounds of OpenAPI 3.0, also when nested", () => {
    const schema = {
      type: "object",
      properties: {
        ratio: {
          type: "number",
          minimum: 0,
          exclusiveMinimum: true,
          maximum: 1,
          exclusiveMaximum: false,
        },
      },
    };
    expect(validate(schema, { ratio: 1 })).toEqual({ ok: true });
    expect(validate(schema, { ratio: 0.5 })).toEqual({ ok: true });
    expect(validate(schema, { ratio: 0 })).toEqual({
      ok: false,
      errors: [{ path: "ratio", message: "must be > 0" }],
    });
  });

  it("leaves literal values in a schema as they are", () => {
    const schema = {
      type: "object",
      enum: [{ exclusiveMinimum: false }],
      default: { exclusiveMaximum: true, maximum: 3 },
    };
    expect(validate(schema, { exclusiveMinimum: false })).toEqual({ ok: true });
    expect(validate(schema, {}).ok).toBe(false);
  });

  it("reads a property that has the name of a bound keyword", () => {
    const schema = {
      type: "object",
      properties: {
        exclusiveMinimum: {
          type: "integer",
          minimum: 1,
          exclusiveMinimum: true,
        },
      },
    };
    expect(validate(schema, { exclusiveMinimum: 2 })).toEqual({ ok: true });
    expect(validate(schema, { exclusiveMinimum: 1 }).ok).toBe(false);
  });

  it("reports a schema that cannot compile as one error at the empty path", () => {
    const result = validate({ type: "nope" }, "x");
    expect(result.ok).toBe(false);
    expect(result.ok ? [] : result.errors).toEqual([
      { path: "", message: expect.stringMatching(/^The schema is not valid/) },
    ]);
  });
});
