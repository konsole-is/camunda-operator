export { conditionsOf, readyOf } from "@/lib/policy";

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

/** Invalid stands for an item of a list that a tab cannot use, with the reason. */
export type Invalid = { error: string };

/**
 * itemsOf returns the items of value that valid accepts, each other item
 * as the error "<noun> <n> is not a valid <noun lower case>.", or one error
 * "The <plural> are not a list." when value is not a list. No value gives
 * no items.
 */
export function itemsOf<T>(
  value: unknown,
  valid: (item: unknown) => item is T,
  noun: string,
  plural: string,
): (T | Invalid)[] {
  const items = listOf(value);
  if (!items) return [{ error: `The ${plural} are not a list.` }];
  return items.map((item, i) =>
    valid(item)
      ? item
      : { error: `${noun} ${i + 1} is not a valid ${noun.toLowerCase()}.` },
  );
}

/** isInvalid is true for an item that itemsOf could not use. */
export function isInvalid(item: object): item is Invalid {
  return "error" in item;
}
