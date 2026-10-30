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

/** StepChange is a value to set at steps, or a reset that removes the value at steps. */
export type StepChange =
  | { steps: readonly Step[]; value: unknown }
  | { steps: readonly Step[]; reset: true };

/**
 * withChanges returns a copy of object with each change applied in order,
 * as withValue and without do. An index of a step names the item of the
 * list in object, also after a change before it removed an item of that
 * list. It does not change object.
 */
export function withChanges<T>(object: T, changes: readonly StepChange[]): T {
  let at: unknown = structuredClone(object);
  for (const change of changes) {
    if (!("reset" in change)) at = put(at, change.steps, change.value);
    else if (valueAt(at, change.steps) !== undefined)
      at = drop(at, change.steps, true) ?? {};
  }
  const compacted = compact(at);
  return (compacted === REMOVED ? {} : compacted) as T;
}

// REMOVED holds the place of a list item that withChanges removes, so that
// the indexes of the later changes still name the items of the input.
const REMOVED = Symbol("removed");

function childOf(at: unknown, step: Step): unknown {
  if ("field" in step)
    return isObject(at) && Object.hasOwn(at, step.field)
      ? at[step.field]
      : undefined;
  if (!Array.isArray(at)) return undefined;
  if ("index" in step) return at[step.index];
  return at.find((item) => matches(item, step.item));
}

function put(at: unknown, steps: readonly Step[], value: unknown): unknown {
  if (steps.length === 0) return structuredClone(value);
  const [step, ...rest] = steps;
  if ("field" in step) {
    const copy: Json = isObject(at) ? { ...at } : {};
    setOwn(copy, step.field, put(childOf(copy, step), rest, value));
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
// removes it too. With hold, a removed list item stays as REMOVED.
function drop(at: unknown, steps: readonly Step[], hold = false): unknown {
  const [step, ...rest] = steps;
  if ("field" in step) {
    const copy: Json = { ...(at as Json) };
    const next =
      rest.length === 0 ? undefined : drop(childOf(copy, step), rest, hold);
    if (next === undefined) delete copy[step.field];
    else setOwn(copy, step.field, next);
    return Object.keys(copy).length === 0 ? undefined : copy;
  }
  const list = [...(at as unknown[])];
  const index =
    "item" in step
      ? list.findIndex((item) => matches(item, step.item))
      : step.index;
  const next = rest.length === 0 ? undefined : drop(list[index], rest, hold);
  if (next !== undefined) list[index] = next;
  else if (hold) list[index] = REMOVED;
  else list.splice(index, 1);
  // With hold, compact removes a list that holds only REMOVED.
  if (hold) return list;
  return list.length === 0 ? undefined : list;
}

// compact takes each REMOVED item out of its list. It returns REMOVED for a
// list or an object that holds nothing else.
function compact(at: unknown): unknown {
  if (Array.isArray(at)) {
    if (!at.some(holds)) return at;
    const items = at.map(compact).filter((item) => item !== REMOVED);
    return items.length === 0 ? REMOVED : items;
  }
  if (!isObject(at) || !holds(at)) return at;
  const copy: Json = {};
  for (const [key, value] of Object.entries(at)) {
    const next = compact(value);
    if (next !== REMOVED) setOwn(copy, key, next);
  }
  return Object.keys(copy).length === 0 ? REMOVED : copy;
}

function holds(at: unknown): boolean {
  if (at === REMOVED) return true;
  if (Array.isArray(at)) return at.some(holds);
  return isObject(at) && Object.values(at).some(holds);
}

// setOwn creates an own field, also for the name __proto__, which an
// assignment would take as the prototype.
function setOwn(object: Json, name: string, value: unknown): void {
  Object.defineProperty(object, name, {
    value,
    enumerable: true,
    writable: true,
    configurable: true,
  });
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
