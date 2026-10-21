/** NOT_AN_OBJECT is the error text of a value that must be a JSON object. */
export const NOT_AN_OBJECT = "the value is not a JSON object";

/**
 * decodeObject returns value as an object: undefined and null give an empty
 * object, and any other value that is not a plain object gives undefined.
 */
export function decodeObject(
  value: unknown,
): Record<string, unknown> | undefined {
  if (value === undefined || value === null) return {};
  return isObject(value) ? value : undefined;
}

/** equalJSON says whether a and b are the same JSON value, with any key order. */
export function equalJSON(a: unknown, b: unknown): boolean {
  if (Array.isArray(a) || Array.isArray(b)) {
    return (
      Array.isArray(a) &&
      Array.isArray(b) &&
      a.length === b.length &&
      a.every((item, i) => equalJSON(item, b[i]))
    );
  }
  if (isObject(a) && isObject(b)) {
    const keys = Object.keys(a).filter((k) => a[k] !== undefined);
    return (
      keys.length === Object.keys(b).filter((k) => b[k] !== undefined).length &&
      keys.every((k) => equalJSON(a[k], b[k]))
    );
  }
  return a === b;
}

/** nameOf returns value when it is a string, else undefined. */
export function nameOf(value: unknown): string | undefined {
  return typeof value === "string" ? value : undefined;
}

/** listOfNames returns value when it is a list of strings only, else undefined. */
export function listOfNames(value: unknown): string[] | undefined {
  return Array.isArray(value) && value.every((v) => typeof v === "string")
    ? (value as string[])
    : undefined;
}

/**
 * byCodeUnit compares two strings by UTF-16 code unit, for sort. The
 * operator sorts Go strings by byte, which gives the same order for the
 * names that Kubernetes allows. localeCompare does not.
 */
export function byCodeUnit(a: string, b: string): number {
  return a < b ? -1 : a > b ? 1 : 0;
}

/** isObject says whether value is a plain JSON object: not null, not a list. */
export function isObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
