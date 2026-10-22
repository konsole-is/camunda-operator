import { describe, expect, it } from "vitest";
import { schemaRules } from "./index";

describe("schemaRules", () => {
  it("lists the rules of the root and of each property with their path", () => {
    expect(
      schemaRules({
        type: "object",
        "x-kubernetes-validations": [
          {
            rule: "self.min <= self.max",
            message: "min must not be above max",
          },
        ],
        properties: {
          name: {
            type: "string",
            "x-kubernetes-validations": [{ rule: "self.startsWith('team-')" }],
          },
          limits: {
            type: "object",
            properties: {
              cpu: {
                type: "integer",
                "x-kubernetes-validations": [
                  { rule: "self > 0", message: "cpu must be positive" },
                ],
              },
            },
          },
        },
      }),
    ).toEqual([
      {
        path: "",
        rule: "self.min <= self.max",
        message: "min must not be above max",
      },
      { path: "name", rule: "self.startsWith('team-')" },
      { path: "limits.cpu", rule: "self > 0", message: "cpu must be positive" },
    ]);
  });

  it("names a rule of each item or map value by the property that holds it", () => {
    expect(
      schemaRules({
        type: "object",
        properties: {
          tags: {
            type: "array",
            items: {
              type: "string",
              "x-kubernetes-validations": [{ rule: "self != ''" }],
            },
          },
          labels: {
            type: "object",
            additionalProperties: {
              type: "string",
              "x-kubernetes-validations": [{ rule: "self.size() < 64" }],
            },
          },
        },
      }),
    ).toEqual([
      { path: "tags", rule: "self != ''" },
      { path: "labels", rule: "self.size() < 64" },
    ]);
  });

  it("keeps a property that is named like a keyword", () => {
    expect(
      schemaRules({
        type: "object",
        properties: {
          items: {
            type: "integer",
            "x-kubernetes-validations": [{ rule: "self > 1" }],
          },
          properties: {
            type: "object",
            properties: {
              constructor: {
                type: "integer",
                "x-kubernetes-validations": [{ rule: "self > 2" }],
              },
            },
          },
        },
      }),
    ).toEqual([
      { path: "items", rule: "self > 1" },
      { path: "properties.constructor", rule: "self > 2" },
    ]);
  });

  it("gives a rule with only a message expression no message", () => {
    expect(
      schemaRules({
        type: "integer",
        "x-kubernetes-validations": [
          { rule: "self > 1", messageExpression: "'too small'" },
        ],
      }),
    ).toEqual([{ path: "", rule: "self > 1" }]);
  });

  it("skips an entry without a rule, and a schema without rules has none", () => {
    expect(
      schemaRules({
        type: "object",
        "x-kubernetes-validations": [{ message: "x" }, "self > 1", null],
      }),
    ).toEqual([]);
    expect(
      schemaRules({ type: "object", properties: { a: { type: "string" } } }),
    ).toEqual([]);
  });

  it("skips rules in parts of a schema that do not hold values, and malformed parts", () => {
    expect(
      schemaRules({
        type: "object",
        "x-kubernetes-validations": "self > 1",
        properties: { a: null, b: "x" },
        items: [{ "x-kubernetes-validations": [{ rule: "self > 1" }] }],
        additionalProperties: true,
      }),
    ).toEqual([]);
  });
});
