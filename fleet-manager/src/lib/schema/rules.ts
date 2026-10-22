import type { OpenAPISchema } from "./index";

/** SchemaRule is one x-kubernetes-validations rule of a schema. */
export type SchemaRule = { path: string; rule: string; message?: string };

/**
 * schemaRules lists the x-kubernetes-validations rules of a schema. It does
 * not run them. path joins the property names from the root to the schema
 * that holds the rule with dots, and the empty path is the root. A rule of
 * the items of an array has the path of the array. A rule of the values of a
 * map has the path of the map, then "*". message is absent when the rule
 * has none or an empty one, also when it has a messageExpression. A schema
 * lists its own rules before those of its properties, in property order.
 */
export function schemaRules(schema: OpenAPISchema): SchemaRule[] {
  const rules: SchemaRule[] = [];
  collect(schema, "", rules);
  return rules;
}

// A structural schema holds rules only on itself, its properties, its items,
// and its additionalProperties.
function collect(schema: unknown, path: string, rules: SchemaRule[]): void {
  if (!isObject(schema)) return;
  const own = ownValue(schema, "x-kubernetes-validations");
  if (Array.isArray(own)) {
    for (const entry of own) {
      if (!isObject(entry) || typeof entry.rule !== "string") continue;
      rules.push({
        path,
        rule: entry.rule,
        ...(typeof entry.message === "string" &&
          entry.message !== "" && { message: entry.message }),
      });
    }
  }
  const properties = ownValue(schema, "properties");
  if (isObject(properties)) {
    for (const [name, property] of Object.entries(properties))
      collect(property, path ? `${path}.${name}` : name, rules);
  }
  collect(ownValue(schema, "items"), path, rules);
  collect(
    ownValue(schema, "additionalProperties"),
    path ? `${path}.*` : "*",
    rules,
  );
}

function ownValue(record: Record<string, unknown>, key: string): unknown {
  return Object.hasOwn(record, key) ? record[key] : undefined;
}

function isObject(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}
