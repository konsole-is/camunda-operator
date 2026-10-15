// The merges of this module follow MergeSpec of the operator in
// pkg/components/{camundacluster,elasticsearchcluster,databaseserver}. Each
// field has one rule, so each path gets its source at one fixed level.
import { child, indexItem, keyedItem, type FieldPath } from "./path";

/** Layer names the input of a merge that set a value. */
export type Layer = "preset" | "release" | "overrides";

/**
 * FieldSourceMap gives the layer that set each value of a merged spec, by
 * field path. A value that one layer sets as a whole, such as a block that
 * replaces as a whole or one entry of extraEnv, has one entry at its own path
 * and none below it. The layer of a deeper path is the entry of its nearest
 * ancestor.
 */
export type FieldSourceMap = Record<FieldPath, Layer>;

/** Merged is the spec that a merge returns and the source of each value. */
export type Merged<S> = { spec: S; source: FieldSourceMap };

export type JsonObject = Record<string, unknown>;

export type MergeContext = { layer: Layer; source: FieldSourceMap };

/** CustomMerge merges the field name of over into target, for a rule that the other kinds do not cover. */
export type CustomMerge = (
  target: JsonObject,
  name: string,
  over: unknown,
  path: FieldPath,
  ctx: MergeContext,
) => void;

/**
 * Rule says how a field of a layer merges over the layers before it.
 *
 * - value: a Go value field. "", false, and 0 count as unset.
 * - pointer: a Go pointer field. Every value but null replaces.
 * - block: replaces as a whole. An empty list replaces and leaves the field unset.
 * - collection: a list or map that replaces as a whole. An empty one counts as unset.
 * - byKey: a map that merges key by key. The later layer wins.
 * - byName: a list of environment variables that merges by name. A new name goes to the end.
 * - concat: a list that each later layer appends to.
 * - instance: comes from the overrides only, by the inner rule.
 * - fields: an object that merges field by field.
 * - merge: a field with its own merge function.
 */
export type Rule =
  | "value"
  | "pointer"
  | "block"
  | "collection"
  | "byKey"
  | "byName"
  | "concat"
  | { readonly instance: "value" | "block" }
  | { readonly fields: Rules }
  | { readonly merge: CustomMerge };

export type Rules = Readonly<Record<string, Rule>>;

/** RulesOf needs one rule for each field of T, so a new CRD field fails tsc until it has a rule. */
export type RulesOf<T> = { readonly [K in keyof Required<T>]: Rule };

/**
 * mergeLayers merges the layers in order, each over the ones before it, by
 * rules. An undefined layer is empty. The source paths start at root. The
 * result shares no memory with the layers, and the layers do not change.
 */
export function mergeLayers(
  rules: Rules,
  layers: ReadonlyArray<readonly [Layer, JsonObject | undefined]>,
  root: FieldPath,
): { value: JsonObject; source: FieldSourceMap } {
  const value: JsonObject = {};
  const source: FieldSourceMap = {};
  for (const [layer, over] of layers) {
    if (over !== undefined)
      applyLayer(rules, value, over, root, { layer, source });
  }
  return { value, source };
}

/** applyLayer merges over into target by rules. path is the path of target. */
export function applyLayer(
  rules: Rules,
  target: JsonObject,
  over: JsonObject,
  path: FieldPath,
  ctx: MergeContext,
): void {
  for (const [name, rule] of Object.entries(rules)) {
    applyRule(rule, target, name, over[name], child(path, name), ctx);
  }
}

function applyRule(
  rule: Rule,
  target: JsonObject,
  name: string,
  over: unknown,
  path: FieldPath,
  ctx: MergeContext,
): void {
  if (typeof rule === "object") {
    if ("instance" in rule) {
      if (ctx.layer === "overrides")
        applyRule(rule.instance, target, name, over, path, ctx);
    } else if ("fields" in rule) {
      if (isObject(over))
        applyLayer(rule.fields, objectField(target, name), over, path, ctx);
    } else {
      rule.merge(target, name, over, path, ctx);
    }
    return;
  }

  switch (rule) {
    case "value":
      if (
        over !== undefined &&
        over !== null &&
        over !== "" &&
        over !== false &&
        over !== 0
      ) {
        set(target, name, over, path, ctx);
      }
      return;
    case "pointer":
      if (over !== undefined && over !== null)
        set(target, name, over, path, ctx);
      return;
    case "block":
      if (Array.isArray(over) && over.length === 0) {
        delete target[name];
        delete ctx.source[path];
      } else if (over !== undefined && over !== null) {
        set(target, name, over, path, ctx);
      }
      return;
    case "collection":
      if (
        Array.isArray(over)
          ? over.length > 0
          : isObject(over) && Object.keys(over).length > 0
      ) {
        set(target, name, over, path, ctx);
      }
      return;
    case "byKey":
      if (isObject(over)) {
        const map = objectField(target, name);
        for (const [key, value] of Object.entries(over))
          set(map, key, value, child(path, key), ctx);
        if (Object.keys(map).length === 0) delete target[name];
      }
      return;
    case "byName":
      if (Array.isArray(over) && over.length > 0) {
        const list = listField(target, name);
        for (const entry of over) {
          const key = isObject(entry) ? entry.name : undefined;
          const at = list.findIndex(
            (item) => isObject(item) && item.name === key,
          );
          if (at >= 0) list[at] = structuredClone(entry);
          else list.push(structuredClone(entry));
          ctx.source[keyedItem(path, { name: key })] = ctx.layer;
        }
      }
      return;
    case "concat":
      if (Array.isArray(over) && over.length > 0) {
        const list = listField(target, name);
        for (const entry of over) {
          ctx.source[indexItem(path, list.length)] = ctx.layer;
          list.push(structuredClone(entry));
        }
      }
      return;
  }
}

function set(
  target: JsonObject,
  name: string,
  value: unknown,
  path: FieldPath,
  ctx: MergeContext,
): void {
  target[name] = structuredClone(value);
  ctx.source[path] = ctx.layer;
}

function listField(target: JsonObject, name: string): unknown[] {
  const current = target[name];
  if (Array.isArray(current)) return current;
  const list: unknown[] = [];
  target[name] = list;
  return list;
}

/** objectField returns the object at target[name], and puts an empty one there first when there is none. */
export function objectField(target: JsonObject, name: string): JsonObject {
  const current = target[name];
  if (isObject(current)) return current;
  const created: JsonObject = {};
  target[name] = created;
  return created;
}

export function isObject(value: unknown): value is JsonObject {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

/** asObject views a generated spec type as the plain object that the engine reads. */
export function asObject(value: object | undefined): JsonObject | undefined {
  return value as JsonObject | undefined;
}
