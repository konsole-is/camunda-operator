import { REVISION_ANNOTATION } from "@/lib/labels";
import { child, indexItem, keyedItem, valueItem, type FieldPath } from "./path";

/** APP_FIELD_MANAGER is the field manager of every write of the app. */
export const APP_FIELD_MANAGER = "fleet-manager";

/** CLUSTER_CLAIM_MANAGER_PREFIX starts the field manager of the cluster claim controller. */
export const CLUSTER_CLAIM_MANAGER_PREFIX = "CamundaClusterClaim/";

/** PLANE_CLAIM_MANAGER_PREFIX starts the field manager of the plane claim controller. */
export const PLANE_CLAIM_MANAGER_PREFIX = "CamundaManagementClusterClaim/";

/** isClaimManager reports whether manager is the field manager of a claim controller. */
export function isClaimManager(manager: string): boolean {
  return (
    manager.startsWith(CLUSTER_CLAIM_MANAGER_PREFIX) ||
    manager.startsWith(PLANE_CLAIM_MANAGER_PREFIX)
  );
}

/**
 * OPERATOR_MANAGER_PREFIXES start the field managers of the operator
 * controllers. A controller writes its field back on its next pass, so the
 * fleet manager never takes such a field over.
 */
export const OPERATOR_MANAGER_PREFIXES: readonly string[] = [
  "camunda-operator/",
  CLUSTER_CLAIM_MANAGER_PREFIX,
  PLANE_CLAIM_MANAGER_PREFIX,
];

/** OPERATOR_MANAGERS are the field managers of the operator without a prefix of OPERATOR_MANAGER_PREFIXES. */
export const OPERATOR_MANAGERS: readonly string[] = [
  "camunda-operator-builtins",
];

// An ocf component applies as <Kind>/<component>/<owner uid>.
const OCF_MANAGER = /^[A-Z][A-Za-z0-9]*\/[^/]+\/[0-9a-f-]{36}$/;

/**
 * GITOPS_MANAGER_PATTERNS match the field managers of Argo CD and Flux,
 * which set their own value again on the next sync.
 */
export const GITOPS_MANAGER_PATTERNS: readonly RegExp[] = [
  /^argocd-/,
  /^kustomize-controller$/,
  /^helm-controller$/,
  /^flux/,
];

/**
 * GITOPS_MARKERS are the annotations and labels that Argo CD and Flux put
 * on an object that they sync. Argo CD with client-side apply leaves no
 * argocd name in managedFields, so only the marker shows it.
 */
export const GITOPS_MARKERS: {
  readonly annotations: readonly string[];
  readonly labels: readonly string[];
} = {
  annotations: ["argocd.argoproj.io/tracking-id"],
  labels: ["kustomize.toolkit.fluxcd.io/name", "helm.toolkit.fluxcd.io/name"],
};

/** ManagerKind is who a field manager is: a controller of the operator, a GitOps tool, or any other tool. */
export type ManagerKind = "operator" | "gitops" | "other";

/**
 * managerKind returns the kind of the field manager name on object. Every
 * manager of an object with a GITOPS_MARKERS marker is a GitOps tool,
 * unless it is the operator. Without object, only the name decides. An
 * unknown name is "other".
 */
export function managerKind(
  name: string,
  object?: ManagedObject & {
    metadata?: {
      labels?: Record<string, string>;
      annotations?: Record<string, string>;
    };
  },
): ManagerKind {
  if (
    OPERATOR_MANAGERS.includes(name) ||
    OPERATOR_MANAGER_PREFIXES.some((prefix) => name.startsWith(prefix)) ||
    OCF_MANAGER.test(name)
  )
    return "operator";
  const annotations = object?.metadata?.annotations ?? {};
  const labels = object?.metadata?.labels ?? {};
  if (
    GITOPS_MARKERS.annotations.some((key) => Object.hasOwn(annotations, key)) ||
    GITOPS_MARKERS.labels.some((key) => Object.hasOwn(labels, key)) ||
    GITOPS_MANAGER_PATTERNS.some((pattern) => pattern.test(name)) ||
    (object !== undefined && revisionManager(object) === name)
  )
    return "gitops";
  return "other";
}

/**
 * revisionManager returns the field manager that applied the annotation
 * REVISION_ANNOTATION of object with server-side apply: the GitOps tool of
 * the repository of the fleet manager. It is never APP_FIELD_MANAGER. It is
 * undefined when no such manager exists, for example when the GitOps tool
 * applies with client-side apply.
 */
export function revisionManager(object: ManagedObject): string | undefined {
  const applied = (object.metadata?.managedFields ?? []).filter(
    (e) => e.operation === "Apply" && e.manager !== APP_FIELD_MANAGER,
  );
  return fieldManagers({ metadata: { managedFields: applied } })[
    child(child("metadata", "annotations"), REVISION_ANNOTATION)
  ];
}

/** ManagedFieldsEntry holds the fields of a managedFields entry that fieldManagers reads. */
export type ManagedFieldsEntry = {
  manager?: string;
  operation?: string;
  subresource?: string;
  time?: Date | string;
  fieldsV1?: object;
};

/** ManagedObject is any Kubernetes object, from the API or from the cache. */
export type ManagedObject = {
  metadata?: { managedFields?: ManagedFieldsEntry[] };
};

/**
 * fieldManagers returns the manager of each field path that
 * metadata.managedFields of live names. A list item that a manager owns as
 * a whole has an entry at the path of the item. When two managers own one
 * path, the entry with the latest time wins, and on the same time the later
 * entry of the list.
 */
export function fieldManagers(live: ManagedObject): Record<FieldPath, string> {
  return owners(live, () => true);
}

/**
 * wholeFieldManagers returns, for each field path that a managedFields entry
 * owns with no child keys, the manager of the latest such entry. An entry
 * that owns a path only through the key "." does not put the path in the
 * result.
 */
export function wholeFieldManagers(
  live: ManagedObject,
): Record<FieldPath, string> {
  return owners(live, (whole) => whole);
}

/**
 * lastChanged returns the newest time of metadata.managedFields as an ISO
 * time, or undefined when no entry has a time.
 */
export function lastChanged(object: ManagedObject): string | undefined {
  let newest: number | undefined;
  for (const entry of object.metadata?.managedFields ?? []) {
    if (entry.time === undefined) continue;
    const time = new Date(entry.time).getTime();
    if (!Number.isNaN(time) && (newest === undefined || time > newest))
      newest = time;
  }
  return newest === undefined ? undefined : new Date(newest).toISOString();
}

/** KeptField is a field that an apply leaves out and that managers keep. */
export type KeptField = { path: FieldPath; managers: string[] };

/**
 * keptRemovals returns each field of before that after leaves out while a
 * manager other than the Apply entry of applier owns it in
 * live.metadata.managedFields, with every such manager. An apply of after without force keeps these fields,
 * so result is after with each of them back. An item of a list is one
 * field. A field that a manager owns as a whole names no field below it.
 * The fields are in the order of before.
 */
export function keptRemovals(
  live: ManagedObject,
  before: object,
  after: object,
  applier: string = APP_FIELD_MANAGER,
): { kept: KeptField[]; result: object } {
  const tree = ownerTree(live, (entry) => !isApplyOf(entry, applier));
  const kept: KeptField[] = [];
  const result = restore(tree, before, after, "", kept) as object;
  return { kept, result };
}

/** FieldOwner is one managedFields entry as an owner of a field. */
export type FieldOwner = { manager: string; operation: string };

/**
 * OwnerTree is the FieldsV1 trees of several managedFields entries merged
 * into one. owners own the node itself, with no child keys or by the key
 * ".". whole are the owners with no child keys: they own a leaf, an atomic
 * list, or an atomic map as one value. children holds the node of each
 * FieldsV1 key below.
 */
export type OwnerTree = {
  owners: FieldOwner[];
  whole: FieldOwner[];
  children: Map<string, OwnerTree>;
};

/**
 * ownerTree merges the FieldsV1 tree of each entry of
 * object.metadata.managedFields that include accepts. An entry without a
 * manager or without fieldsV1, and an entry of a subresource, owns no field
 * of the object and is never in the tree.
 */
export function ownerTree(
  object: ManagedObject,
  include: (entry: ManagedFieldsEntry) => boolean,
): OwnerTree {
  const tree = newOwners();
  for (const entry of object.metadata?.managedFields ?? []) {
    if (!entry.manager || entry.subresource || !entry.fieldsV1) continue;
    if (!include(entry)) continue;
    addOwner(tree, entry.fieldsV1 as Record<string, unknown>, {
      manager: entry.manager,
      operation: entry.operation ?? "",
    });
  }
  return tree;
}

/**
 * isApplyOf reports whether entry is the server-side apply entry of
 * manager. An Update entry of the same name is another manager to the API
 * server.
 */
export function isApplyOf(entry: ManagedFieldsEntry, manager: string): boolean {
  return entry.manager === manager && entry.operation === "Apply";
}

/**
 * OwnedStep is one step of a field path from the root of an object: a
 * field or map key, the item of a list with these key values, the item of
 * a set list with this value, or the item at an index.
 */
export type OwnedStep =
  | { field: string }
  | { item: Record<string, unknown> }
  | { value: unknown }
  | { index: number };

/**
 * stepOf returns the step of one FieldsV1 key: f: for a field or a map key,
 * k: for a list item by key, v: for a set item, i: for a list index. It
 * returns undefined for "." and for a key that it cannot read.
 */
export function stepOf(key: string): OwnedStep | undefined {
  const rest = key.slice(2);
  try {
    switch (key.slice(0, 2)) {
      case "f:":
        return { field: rest };
      case "k:": {
        const item: unknown = JSON.parse(rest);
        return isRecord(item) ? { item } : undefined;
      }
      case "v:":
        return { value: JSON.parse(rest) };
      case "i:": {
        const index = Number(rest);
        return Number.isInteger(index) ? { index } : undefined;
      }
      default:
        return undefined;
    }
  } catch {
    return undefined;
  }
}

/** stepPath returns the path of step under path, built with the functions of path.ts. */
export function stepPath(path: FieldPath, step: OwnedStep): FieldPath {
  if ("field" in step) return child(path, step.field);
  if ("item" in step) return keyedItem(path, step.item);
  if ("value" in step) return valueItem(path, step.value);
  return indexItem(path, step.index);
}

/**
 * itemOwners finds the node of item of a list in owners: a k: node whose
 * key values item holds, or the v: node of its value. matches reports
 * whether another item is the same item, and step is the step to it. It
 * returns undefined when no manager owns item or anything in it.
 */
export function itemOwners(
  owners: OwnerTree,
  item: unknown,
):
  | {
      owners: OwnerTree;
      step: OwnedStep;
      matches: (candidate: unknown) => boolean;
    }
  | undefined {
  for (const [key, node] of owners.children) {
    if (!key.startsWith("k:")) continue;
    const step = stepOf(key);
    if (step === undefined || !("item" in step)) continue;
    const keys = step.item;
    const matches = (candidate: unknown) =>
      isRecord(candidate) &&
      Object.entries(keys).every(([k, v]) => sameJSON(candidate[k], v));
    if (matches(item)) return { owners: node, step, matches };
  }
  const node = owners.children.get(`v:${JSON.stringify(item)}`);
  if (node === undefined) return undefined;
  return {
    owners: node,
    step: { value: item },
    matches: (candidate) => sameJSON(candidate, item),
  };
}

function newOwners(): OwnerTree {
  return { owners: [], whole: [], children: new Map() };
}

function addOwner(
  at: OwnerTree,
  fields: Record<string, unknown>,
  owner: FieldOwner,
): void {
  for (const [key, sub] of Object.entries(fields)) {
    if (key === ".") {
      own(at.owners, owner);
      continue;
    }
    let next = at.children.get(key);
    if (next === undefined) {
      next = newOwners();
      at.children.set(key, next);
    }
    const children = isRecord(sub) ? sub : {};
    if (Object.keys(children).length === 0) {
      own(next.owners, owner);
      own(next.whole, owner);
    } else addOwner(next, children, owner);
  }
}

// inOrderOf puts the items of before first, in their order in before, as
// the API server keeps a list item that an apply leaves out in its place.
function inOrderOf(
  before: unknown[],
  items: unknown[],
  owners: OwnerTree,
): unknown[] {
  const keys = [...owners.children.keys()].find((k) => k.startsWith("k:"));
  let names: string[] | undefined;
  try {
    names = keys ? Object.keys(JSON.parse(keys.slice(2)) as object) : undefined;
  } catch {
    names = undefined;
  }
  const same = (a: unknown, b: unknown) =>
    names
      ? isRecord(a) && isRecord(b) && names.every((n) => sameJSON(a[n], b[n]))
      : sameJSON(a, b);
  const rank = (item: unknown) => {
    const i = before.findIndex((b) => same(b, item));
    return i < 0 ? before.length : i;
  };
  return items
    .map((item, i) => ({ item, i, r: rank(item) }))
    .sort((a, b) => a.r - b.r || a.i - b.i)
    .map(({ item }) => item);
}

function own(owners: FieldOwner[], owner: FieldOwner): void {
  if (
    !owners.some(
      (o) => o.manager === owner.manager && o.operation === owner.operation,
    )
  )
    owners.push(owner);
}

function addName(names: string[], manager: string): void {
  if (!names.includes(manager)) names.push(manager);
}

// restore returns after with each value of before that owners own and
// after leaves out put back, and adds it to kept, or its managers to into.
function restore(
  owners: OwnerTree,
  before: unknown,
  after: unknown,
  path: FieldPath,
  kept: KeptField[],
  into?: KeptField,
  seed?: unknown,
): unknown {
  if (after === undefined && owners.owners.length > 0) {
    let entry = into;
    if (entry)
      owners.owners.forEach((o) => addName(entry!.managers, o.manager));
    else {
      entry = { path, managers: [] };
      owners.owners.forEach((o) => addName(entry!.managers, o.manager));
      kept.push(entry);
    }
    // A node owned only by "." is owned for its presence: of its
    // children, only those that other managers own stay.
    if (owners.whole.length > 0) return before;
    const empty = Array.isArray(before) ? [] : (seed ?? {});
    return restore(owners, before, empty, path, kept, entry);
  }
  if (Array.isArray(before)) {
    let items = Array.isArray(after) ? after : undefined;
    let changed = false;
    for (const item of before) {
      const found = itemOwners(owners, item);
      if (found === undefined) continue;
      const index =
        items?.findIndex((candidate) => found.matches(candidate)) ?? -1;
      const current = index < 0 ? undefined : items![index];
      const next = restore(
        found.owners,
        item,
        current,
        stepPath(path, found.step),
        kept,
        into,
        "item" in found.step ? { ...found.step.item } : undefined,
      );
      if (next === current) continue;
      changed = true;
      items =
        index < 0
          ? [...(items ?? []), next]
          : items!.map((x, i) => (i === index ? next : x));
    }
    return changed ? inOrderOf(before, items!, owners) : after;
  }
  if (isRecord(before)) {
    let fields = isRecord(after) ? after : undefined;
    let changed = false;
    for (const [key, value] of Object.entries(before)) {
      const below = owners.children.get(`f:${key}`);
      if (below === undefined) continue;
      const current = fields?.[key];
      const next = restore(below, value, current, child(path, key), kept, into);
      if (next === current) continue;
      changed = true;
      fields = { ...(fields ?? {}), [key]: next };
    }
    return changed ? fields : after;
  }
  return after;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function sameJSON(a: unknown, b: unknown): boolean {
  if (a === b) return true;
  if (typeof a !== "object" || typeof b !== "object" || !a || !b) return false;
  if (Array.isArray(a) !== Array.isArray(b)) return false;
  const left = a as Record<string, unknown>;
  const right = b as Record<string, unknown>;
  const keys = Object.keys(left);
  return (
    keys.length === Object.keys(right).length &&
    keys.every((k) => Object.hasOwn(right, k) && sameJSON(left[k], right[k]))
  );
}

function owners(
  live: ManagedObject,
  keep: (whole: boolean) => boolean,
): Record<FieldPath, string> {
  const owners = new Map<FieldPath, { manager: string; time: number }>();
  for (const entry of live.metadata?.managedFields ?? []) {
    const manager = entry.manager;
    if (!manager || entry.fieldsV1 === undefined) continue;
    const parsed =
      entry.time === undefined ? 0 : new Date(entry.time).getTime();
    const time = Number.isNaN(parsed) ? 0 : parsed;
    walk(entry.fieldsV1 as Record<string, unknown>, "", (path, whole) => {
      if (!keep(whole)) return;
      const current = owners.get(path);
      if (current === undefined || time >= current.time)
        owners.set(path, { manager, time });
    });
  }
  return Object.fromEntries(
    [...owners].map(([path, owner]) => [path, owner.manager]),
  );
}

// walk visits each path that a FieldsV1 tree owns. A key "." owns the node of
// its parent. A node without children is owned as a whole.
function walk(
  fields: Record<string, unknown>,
  path: FieldPath,
  visit: (path: FieldPath, whole: boolean) => void,
): void {
  for (const [key, sub] of Object.entries(fields)) {
    if (key === ".") {
      if (path !== "") visit(path, false);
      continue;
    }
    const at = segment(path, key);
    if (at === undefined) continue;
    const children =
      typeof sub === "object" && sub !== null
        ? (sub as Record<string, unknown>)
        : {};
    if (Object.keys(children).length === 0) visit(at, true);
    else walk(children, at, visit);
  }
}

// segment turns one FieldsV1 key into the path of its node, or undefined
// for a key that stepOf cannot read.
function segment(path: FieldPath, key: string): FieldPath | undefined {
  const step = stepOf(key);
  return step === undefined ? undefined : stepPath(path, step);
}
