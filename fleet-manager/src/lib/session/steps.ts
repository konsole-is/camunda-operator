import type { Step } from "@/lib/fields";
import { equalJSON } from "@/lib/policy";

type Json = Record<string, unknown>;

/** valueAt returns the value at steps of object, or undefined when it is missing. */
export function valueAt(object: unknown, steps: readonly Step[]): unknown {
  let at = object;
  for (const step of steps) {
    if (at === undefined) return undefined;
    at = childOf(at, step);
  }
  return at;
}

/**
 * withValue returns a copy of object with value at steps. It creates
 * missing objects, and a missing keyed list item with its keys at the end
 * of its list. It does not change object.
 */
export function withValue<T>(
  object: T,
  steps: readonly Step[],
  value: unknown,
): T {
  return put(object, steps, value) as T;
}

/**
 * without returns a copy of object without the value at steps. It removes
 * an object or a list that becomes empty, up to the parent of the first
 * step. It does not change object.
 */
export function without<T>(object: T, steps: readonly Step[]): T {
  if (valueAt(object, steps) === undefined) return structuredClone(object);
  return (drop(object, steps) ?? {}) as T;
}

function childOf(at: unknown, step: Step): unknown {
  if ("field" in step) return isObject(at) ? at[step.field] : undefined;
  if (!Array.isArray(at)) return undefined;
  if ("index" in step) return at[step.index];
  return at.find((item) => matches(item, step.item));
}

function put(at: unknown, steps: readonly Step[], value: unknown): unknown {
  if (steps.length === 0) return structuredClone(value);
  const [step, ...rest] = steps;
  if ("field" in step) {
    const copy: Json = isObject(at) ? { ...at } : {};
    copy[step.field] = put(copy[step.field], rest, value);
    return copy;
  }
  const list = Array.isArray(at) ? [...at] : [];
  const index =
    "item" in step
      ? list.findIndex((item) => matches(item, step.item))
      : step.index;
  if (index >= 0) list[index] = put(list[index], rest, value);
  else if ("item" in step)
    list.push(put(structuredClone(step.item), rest, value));
  return list;
}

// drop returns undefined for a value that becomes empty, so that its parent
// removes it too.
function drop(at: unknown, steps: readonly Step[]): unknown {
  const [step, ...rest] = steps;
  if ("field" in step) {
    const copy: Json = { ...(at as Json) };
    const next = rest.length === 0 ? undefined : drop(copy[step.field], rest);
    if (next === undefined) delete copy[step.field];
    else copy[step.field] = next;
    return Object.keys(copy).length === 0 ? undefined : copy;
  }
  const list = [...(at as unknown[])];
  const index =
    "item" in step
      ? list.findIndex((item) => matches(item, step.item))
      : step.index;
  const next = rest.length === 0 ? undefined : drop(list[index], rest);
  if (next === undefined) list.splice(index, 1);
  else list[index] = next;
  return list.length === 0 ? undefined : list;
}

function matches(item: unknown, keys: Record<string, unknown>): boolean {
  return (
    isObject(item) &&
    Object.entries(keys).every(([key, value]) => equalJSON(item[key], value))
  );
}

function isObject(value: unknown): value is Json {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
