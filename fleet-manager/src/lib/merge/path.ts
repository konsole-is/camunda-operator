// The field paths of the merge source map and of fieldManagers share one
// grammar, so that a page can look up both for the same field. Build a path
// only with these functions. Never parse one: a map key can hold a dot.

/**
 * FieldPath names one value of an object, from its root, for example
 * spec.zeebe.replicas, spec.podLabels.team, spec.extraEnv[name=TZ], or
 * spec.extraEnvFrom[0].
 */
export type FieldPath = string;

/** child returns the path of the field or map key name under parent. */
export function child(parent: FieldPath, name: string): FieldPath {
  return parent === "" ? name : `${parent}.${name}`;
}

/**
 * keyedItem returns the path of the item of a list that its keys name, in
 * the order of keys. A string key value stays as it is, and any other value
 * is written as JSON.
 */
export function keyedItem(
  list: FieldPath,
  keys: Record<string, unknown>,
): FieldPath {
  const parts = Object.entries(keys).map(
    ([key, value]) =>
      `${key}=${typeof value === "string" ? value : JSON.stringify(value)}`,
  );
  return `${list}[${parts.join(",")}]`;
}

/** indexItem returns the path of the item at index of a list. */
export function indexItem(list: FieldPath, index: number): FieldPath {
  return `${list}[${index}]`;
}

/** valueItem returns the path of the item of a set list that equals value. */
export function valueItem(list: FieldPath, value: unknown): FieldPath {
  return `${list}[=${JSON.stringify(value)}]`;
}
