import { LAST_APPLIED_ANNOTATION } from "@/lib/labels";
import {
  APP_FIELD_MANAGER,
  type FieldOwner,
  type FieldPath,
  type ManagedObject,
  type OwnedStep,
  type OwnerTree,
  isApplyOf,
  isKnownManager,
  itemOwners,
  managerKind,
  ownerTree,
  stepOf,
  stepPath,
} from "@/lib/merge";
import { equalJSON } from "@/lib/policy/json";
import { refKey, refOf } from "./refs";
import type {
  ApiError,
  Change,
  ForeignField,
  ForeignManager,
  KubeObject,
  ObjectOwnership,
  ObjectRef,
  RefKey,
} from "./types";

type JsonMap = Record<string, unknown>;

const PARTIAL_SAVE = "The fleet manager saved the ";

/**
 * partialSaveMessage is the message of an error of a save that applied ref
 * and did not remove the fields of paths. message is the error of the
 * removal.
 */
export function partialSaveMessage(
  ref: ObjectRef,
  paths: readonly FieldPath[],
  message: string,
): string {
  return `${PARTIAL_SAVE}${ref.kind} ${ref.name}, but did not remove ${paths.join(", ")}. ${message}`;
}

/** isPartialSave reports whether error has a message of partialSaveMessage. */
export function isPartialSave(error: ApiError): boolean {
  return error.message.startsWith(PARTIAL_SAVE);
}

/**
 * foreignFields returns the fields of spec, metadata.labels, and
 * metadata.annotations of live that a server-side apply of applied by the
 * fleet manager changes or clears while another field manager owns them.
 * Every managedFields entry other than the Apply entry of the fleet
 * manager is another manager, also an Update entry of the fleet manager.
 * An entry of a subresource is not another manager.
 *
 * changed holds each field that applied sets to another value than live.
 * The apply fails on such a field unless it forces. cleared holds the
 * topmost field that live holds and applied leaves out, under a field
 * that applied holds, with every other manager of it or of a field below
 * it. The apply keeps such a field. The annotation
 * LAST_APPLIED_ANNOTATION is in neither list. The fields are in the
 * code-unit order of their paths, and the managers of a field by name.
 */
export function foreignFields(
  live: KubeObject,
  applied: KubeObject,
): { changed: ForeignField[]; cleared: ForeignField[] } {
  const tree = ownerTree(
    live as ManagedObject,
    (entry) => !entry.subresource && !isApplyOf(entry, APP_FIELD_MANAGER),
  );
  const changed: ForeignField[] = [];
  const cleared: ForeignField[] = [];
  const managersOf = (owners: FieldOwner[]) => foreignManagers(owners, live);

  const visit = (
    node: OwnerTree,
    before: unknown,
    after: unknown,
    path: FieldPath,
    steps: OwnedStep[],
  ): void => {
    if (after === undefined) {
      const owners = ownersBelow(node);
      if (owners.length > 0)
        cleared.push({ path, steps, managers: managersOf(owners) });
      return;
    }
    if (node.whole.length > 0) {
      if (!equalJSON(before, after))
        changed.push({ path, steps, managers: managersOf(node.whole) });
      return;
    }
    visitChildren(node, before, after, path, steps);
  };

  const visitChildren = (
    node: OwnerTree,
    before: unknown,
    after: unknown,
    path: FieldPath,
    steps: OwnedStep[],
    skip?: string,
  ): void => {
    if (Array.isArray(before)) {
      const items = Array.isArray(after) ? after : [];
      for (const item of before) {
        const found = itemOwners(node, item);
        if (found === undefined) continue;
        visit(
          found.owners,
          item,
          items.find(found.matches),
          stepPath(path, found.step),
          [...steps, found.step],
        );
      }
      return;
    }
    if (!isRecord(before)) return;
    const fields = isRecord(after) ? after : {};
    for (const [key, value] of Object.entries(before)) {
      const below = node.children.get(`f:${key}`);
      if (below === undefined || key === skip) continue;
      const step = { field: key };
      visit(
        below,
        value,
        Object.hasOwn(fields, key) ? fields[key] : undefined,
        stepPath(path, step),
        [...steps, step],
      );
    }
  };

  // The roots of the scope are containers: their own "." owner is not a
  // field to take over.
  const spec = tree.children.get("f:spec");
  if (spec !== undefined)
    visitChildren(spec, live.spec, applied.spec ?? {}, "spec", [
      { field: "spec" },
    ]);
  const metadata = tree.children.get("f:metadata");
  for (const name of ["labels", "annotations"]) {
    const node = metadata?.children.get(`f:${name}`);
    if (node === undefined) continue;
    visitChildren(
      node,
      live.metadata[name],
      applied.metadata[name] ?? {},
      stepPath("metadata", { field: name }),
      [{ field: "metadata" }, { field: name }],
      name === "annotations" ? LAST_APPLIED_ANNOTATION : undefined,
    );
  }
  return { changed: byPath(changed), cleared: byPath(cleared) };
}

/** offered reports whether the review offers field for a take-over: no controller of the operator owns it. */
export function offered(field: ForeignField): boolean {
  return field.managers.every((m) => m.kind !== "operator");
}

// Two fields can have one path string, and a list names each path once.
function offeredPaths(fields: readonly ForeignField[]): FieldPath[] {
  return [...new Set(fields.filter(offered).map((f) => f.path))];
}

/**
 * withTakeOver returns change with takeOver and remove set to the offered
 * changed and cleared fields of each entry of ownership whose object
 * change applies. A list without a field is left out. It returns change
 * itself when ownership offers no field of an applied object.
 */
export function withTakeOver(
  change: Change,
  ownership: readonly ObjectOwnership[],
): Change {
  const applied = new Set(change.apply.map((o) => refKey(refOf(o))));
  const takeOver: Record<RefKey, FieldPath[]> = {};
  const remove: Record<RefKey, FieldPath[]> = {};
  for (const object of ownership) {
    const key = refKey(object.ref);
    if (!applied.has(key)) continue;
    const taken = offeredPaths(object.changed);
    const removed = offeredPaths(object.cleared);
    if (taken.length > 0) takeOver[key] = taken;
    if (removed.length > 0) remove[key] = removed;
  }
  const hasTakeOver = Object.keys(takeOver).length > 0;
  const hasRemove = Object.keys(remove).length > 0;
  if (!hasTakeOver && !hasRemove) return change;
  const rest = { ...change };
  delete rest.takeOver;
  delete rest.remove;
  return {
    ...rest,
    ...(hasTakeOver ? { takeOver } : {}),
    ...(hasRemove ? { remove } : {}),
  };
}

/**
 * takenFields returns each field that a managedFields entry of before
 * other than the Apply entry of the fleet manager owns, that no entry of
 * the same manager and operation owns in after, and whose value after
 * still holds: the fields that a write from before to after took from
 * their managers. Each field names the managers that lost it. The fields
 * are in the code-unit order of their paths.
 */
export function takenFields(
  before: KubeObject,
  after: KubeObject,
): ForeignField[] {
  const lost = ownerTree(
    before as ManagedObject,
    (entry) => !isApplyOf(entry, APP_FIELD_MANAGER),
  );
  const kept = ownerTree(after as ManagedObject, () => true);
  const taken: ForeignField[] = [];
  const walk = (
    node: OwnerTree,
    still: OwnerTree | undefined,
    path: FieldPath,
    steps: OwnedStep[],
  ) => {
    const gone = node.owners.filter(
      (o) => !still?.owners.some((k) => sameOwner(k, o)),
    );
    if (gone.length > 0 && valueAt(after, steps) !== undefined)
      taken.push({ path, steps, managers: foreignManagers(gone, before) });
    for (const [key, below] of node.children) {
      const step = stepOf(key);
      if (step === undefined) continue;
      walk(below, still?.children.get(key), stepPath(path, step), [
        ...steps,
        step,
      ]);
    }
  };
  walk(lost, kept, "", []);
  return byPath(taken);
}

/**
 * withoutFields returns a copy of object without each of fields, found by
 * its steps. A field that object does not hold is skipped. A list item
 * leaves the list, and the other items keep their order. object is not
 * changed, and a key __proto__ stays a key.
 */
export function withoutFields<T extends object>(
  object: T,
  fields: readonly ForeignField[],
): T {
  const copy = copyJSON(object) as T;
  // Find each item before the first splice, which moves the items after it.
  const items = new Map<unknown[], Set<number>>();
  const keys: [JsonMap, string][] = [];
  for (const field of fields) {
    const last = field.steps.at(-1);
    if (last === undefined) continue;
    const parent = valueAt(copy, field.steps.slice(0, -1));
    if (Array.isArray(parent)) {
      const index = indexIn(parent, last);
      if (index < 0) continue;
      const found = items.get(parent) ?? new Set<number>();
      items.set(parent, found.add(index));
    } else if (isRecord(parent) && "field" in last) {
      keys.push([parent, last.field]);
    }
  }
  for (const [parent, key] of keys)
    if (Object.hasOwn(parent, key)) delete parent[key];
  for (const [list, indexes] of items)
    for (const index of [...indexes].sort((a, b) => b - a))
      list.splice(index, 1);
  return copy;
}

/** fieldValue returns the value of field in object, or undefined when object does not hold it. */
export function fieldValue(object: unknown, field: ForeignField): unknown {
  return valueAt(object, field.steps);
}

/**
 * withValuesOf returns a copy of object in which each field of fields that
 * from holds has the value of from. It sets only a field whose last step
 * names a key, under a JSON object that object holds.
 */
export function withValuesOf<T extends object>(
  object: T,
  fields: readonly ForeignField[],
  from: unknown,
): T {
  const copy = copyJSON(object) as T;
  for (const field of fields) {
    const last = field.steps.at(-1);
    const value = valueAt(from, field.steps);
    if (last === undefined || !("field" in last) || value === undefined)
      continue;
    const parent = valueAt(copy, field.steps.slice(0, -1));
    if (isRecord(parent))
      Object.defineProperty(parent, last.field, {
        value: copyJSON(value),
        enumerable: true,
        writable: true,
        configurable: true,
      });
  }
  return copy;
}

function valueAt(object: unknown, steps: readonly OwnedStep[]): unknown {
  let at: unknown = object;
  for (const step of steps) {
    if ("field" in step) {
      at =
        isRecord(at) && Object.hasOwn(at, step.field)
          ? at[step.field]
          : undefined;
    } else if (Array.isArray(at)) {
      const index = indexIn(at, step);
      at = index < 0 ? undefined : at[index];
    } else return undefined;
    if (at === undefined) return undefined;
  }
  return at;
}

function indexIn(list: unknown[], step: OwnedStep): number {
  if ("item" in step) {
    const keys = Object.entries(step.item);
    return list.findIndex(
      (item) =>
        isRecord(item) &&
        keys.every(([key, value]) => equalJSON(item[key], value)),
    );
  }
  if ("value" in step)
    return list.findIndex((item) => equalJSON(item, step.value));
  if ("index" in step) return step.index < list.length ? step.index : -1;
  return -1;
}

function ownersBelow(node: OwnerTree): FieldOwner[] {
  const owners: FieldOwner[] = [];
  const collect = (at: OwnerTree) => {
    for (const owner of at.owners)
      if (!owners.some((o) => sameOwner(o, owner))) owners.push(owner);
    for (const below of at.children.values()) collect(below);
  };
  collect(node);
  return owners;
}

function foreignManagers(
  owners: FieldOwner[],
  object: KubeObject,
): ForeignManager[] {
  return owners
    .map((o) => {
      const kind = managerKind(
        o.manager,
        object as Parameters<typeof managerKind>[1],
      );
      return {
        name: o.manager,
        operation: o.operation,
        kind,
        known: kind !== "other" || isKnownManager(o.manager),
      };
    })
    .sort((a, b) =>
      a.name === b.name
        ? compare(a.operation, b.operation)
        : compare(a.name, b.name),
    );
}

function byPath(fields: ForeignField[]): ForeignField[] {
  return fields.sort((a, b) => compare(a.path, b.path));
}

function compare(a: string, b: string): number {
  return a < b ? -1 : a > b ? 1 : 0;
}

function sameOwner(a: FieldOwner, b: FieldOwner): boolean {
  return a.manager === b.manager && a.operation === b.operation;
}

function copyJSON(value: unknown): unknown {
  if (Array.isArray(value)) return value.map(copyJSON);
  if (!isRecord(value)) return value;
  const copy: JsonMap = {};
  for (const [key, inner] of Object.entries(value)) {
    // A plain assignment of the key __proto__ sets the prototype, not a field.
    Object.defineProperty(copy, key, {
      value: copyJSON(inner),
      enumerable: true,
      writable: true,
      configurable: true,
    });
  }
  return copy;
}

function isRecord(value: unknown): value is JsonMap {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
