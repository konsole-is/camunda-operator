import { child, indexItem, keyedItem, type FieldPath } from "@/lib/merge";

/** Step is one segment of a path: a field or map key, a list item by name, or a list item by index. */
export type Step =
  { field: string } | { item: Record<string, unknown> } | { index: number };

/** pathFrom returns the path that steps name under root. */
export function pathFrom(root: FieldPath, steps: readonly Step[]): FieldPath {
  return steps.reduce(
    (path, step) =>
      "field" in step
        ? child(path, step.field)
        : "item" in step
          ? keyedItem(path, step.item)
          : indexItem(path, step.index),
    root,
  );
}

/**
 * WalkNode is one value of an object. steps lead from the root to it.
 * parent is undefined for the root. leaf marks a scalar, null, or an empty
 * object or list.
 */
export type WalkNode = {
  path: FieldPath;
  steps: Step[];
  value: unknown;
  parent?: FieldPath;
  leaf: boolean;
};

/**
 * walk returns every node of value under root, the root included, by path,
 * parents before children. A list item with a string name has the path
 * [name=<name>], any other item its index. A field whose value is undefined
 * has no node.
 */
export function walk(
  value: unknown,
  root: FieldPath,
): Map<FieldPath, WalkNode> {
  const nodes = new Map<FieldPath, WalkNode>();
  const visit = (
    at: unknown,
    path: FieldPath,
    steps: Step[],
    parent: FieldPath | undefined,
  ): void => {
    const entries: [Step, unknown][] = Array.isArray(at)
      ? at.map((item, index) => [itemStep(item, index), item])
      : isObject(at)
        ? Object.entries(at)
            .filter(([, v]) => v !== undefined)
            .map(([field, v]) => [{ field }, v])
        : [];
    nodes.set(path, {
      path,
      steps,
      value: at,
      parent,
      leaf: entries.length === 0,
    });
    for (const [step, sub] of entries)
      visit(sub, pathFrom(path, [step]), [...steps, step], path);
  };
  visit(value, root, [], undefined);
  return nodes;
}

function itemStep(item: unknown, index: number): Step {
  return isObject(item) && typeof item.name === "string"
    ? { item: { name: item.name } }
    : { index };
}

function isObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
