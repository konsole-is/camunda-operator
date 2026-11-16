// This module runs in the browser and on the server.
import type { ConditionRow } from "@/components/fleet/conditions-table";
import { isObject } from "./json";

const STATUSES: readonly unknown[] = ["True", "False", "Unknown"];

const isText = (value: unknown, optional = false) =>
  typeof value === "string" || (optional && value === undefined);

/**
 * conditionsOf returns the conditions of a status, or an error that names
 * the first item that is not a condition with a text type, a status of
 * True, False, or Unknown, and a text reason, message, and
 * lastTransitionTime where it has them. No value gives no conditions.
 */
export function conditionsOf(
  value: unknown,
): { conditions: ConditionRow[] } | { error: string } {
  if (value === undefined || value === null) return { conditions: [] };
  if (!Array.isArray(value)) return { error: "The conditions are not a list." };
  const bad = value.findIndex(
    (c) =>
      !isObject(c) ||
      !isText(c.type) ||
      !STATUSES.includes(c.status) ||
      !isText(c.reason, true) ||
      !isText(c.message, true) ||
      !isText(c.lastTransitionTime, true),
  );
  if (bad >= 0)
    return { error: `Condition ${bad + 1} is not a valid condition.` };
  return { conditions: value as ConditionRow[] };
}

/**
 * readyOf returns the Ready condition of a status, or undefined when it has
 * none or when conditionsOf gives an error for its conditions.
 */
export function readyOf(conditions: unknown): ConditionRow | undefined {
  const read = conditionsOf(conditions);
  return "conditions" in read
    ? read.conditions.find((c) => c.type === "Ready")
    : undefined;
}
