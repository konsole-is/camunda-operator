import type { ConditionRow } from "@/components/fleet/conditions-table";

// The API server checks a stored object against the schema of its CRD, but
// a watch can still give an object of an older CRD. So the tabs check each
// value that they render before they use it.

/**
 * listOf returns value when it is a list, an empty list for no value
 * (undefined or null), and undefined for any other value.
 */
export function listOf(value: unknown): unknown[] | undefined {
  if (value === undefined || value === null) return [];
  return Array.isArray(value) ? value : undefined;
}

/** isRecord is true for a JSON object that is not a list. */
export function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

/** isText is true for a string, and for no value when optional is true. */
export function isText(value: unknown, optional = false): boolean {
  return typeof value === "string" || (optional && value === undefined);
}

const STATUSES: readonly unknown[] = ["True", "False", "Unknown"];

/**
 * conditionsOf returns the conditions of a status, or an error that names
 * the first item that is not a condition with a text type, a status of
 * True, False, or Unknown, and a text reason, message, and
 * lastTransitionTime where it has them. No value gives no conditions.
 */
export function conditionsOf(
  value: unknown,
): { conditions: ConditionRow[] } | { error: string } {
  const items = listOf(value);
  if (!items) return { error: "The conditions are not a list." };
  const bad = items.findIndex(
    (c) =>
      !isRecord(c) ||
      !isText(c.type) ||
      !STATUSES.includes(c.status) ||
      !isText(c.reason, true) ||
      !isText(c.message, true) ||
      !isText(c.lastTransitionTime, true),
  );
  if (bad >= 0)
    return { error: `Condition ${bad + 1} is not a valid condition.` };
  return { conditions: items as ConditionRow[] };
}
