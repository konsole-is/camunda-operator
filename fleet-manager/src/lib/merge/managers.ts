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

/** ManagedFieldsEntry holds the fields of a managedFields entry that fieldManagers reads. */
export type ManagedFieldsEntry = {
  manager?: string;
  operation?: string;
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
 * manager other than applier owns it in live.metadata.managedFields, with
 * every such manager. An apply of after without force keeps these fields,
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
  const tree = newOwners();
  for (const entry of live.metadata?.managedFields ?? []) {
    if (!entry.manager || entry.manager === applier || !entry.fieldsV1)
      continue;
    addOwner(tree, entry.fieldsV1 as Record<string, unknown>, entry.manager);
  }
  const kept: KeptField[] = [];
  const result = restore(tree, before, after, "", kept) as object;
  return { kept, result };
}

// Owners is the merged FieldsV1 tree of several managers: the managers
// that own each node, by the FieldsV1 key of each child. whole says that a
// manager owns the node with no child keys, not only by ".".
type Owners = {
  managers: string[];
  whole: boolean;
  children: Map<string, Owners>;
};

function newOwners(): Owners {
  return { managers: [], whole: false, children: new Map() };
}

function addOwner(
  at: Owners,
  fields: Record<string, unknown>,
  manager: string,
): void {
  for (const [key, sub] of Object.entries(fields)) {
    if (key === ".") {
      own(at, manager);
      continue;
    }
    let next = at.children.get(key);
    if (next === undefined) {
      next = newOwners();
      at.children.set(key, next);
    }
    const children = isRecord(sub) ? sub : {};
    if (Object.keys(children).length === 0) {
      own(next, manager);
      next.whole = true;
    } else addOwner(next, children, manager);
  }
}

function own(at: Owners, manager: string): void {
  if (!at.managers.includes(manager)) at.managers.push(manager);
}

// restore returns after with each value of before that owners own and
// after leaves out put back, and adds each such value to kept unless quiet.
function restore(
  owners: Owners,
  before: unknown,
  after: unknown,
  path: FieldPath,
  kept: KeptField[],
  quiet = false,
  seed?: unknown,
): unknown {
  if (after === undefined && owners.managers.length > 0) {
    if (!quiet) kept.push({ path, managers: [...owners.managers] });
    // A node owned only by "." is owned for its presence: of its
    // children, only those that other managers own stay.
    if (owners.whole) return before;
    const empty = Array.isArray(before) ? [] : (seed ?? {});
    return restore(owners, before, empty, path, kept, true);
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
        found.path(path),
        kept,
        quiet,
        found.seed,
      );
      if (next === current) continue;
      changed = true;
      items =
        index < 0
          ? [...(items ?? []), next]
          : items!.map((x, i) => (i === index ? next : x));
    }
    return changed ? items : after;
  }
  if (isRecord(before)) {
    let fields = isRecord(after) ? after : undefined;
    let changed = false;
    for (const [key, value] of Object.entries(before)) {
      const below = owners.children.get(`f:${key}`);
      if (below === undefined) continue;
      const current = fields?.[key];
      const next = restore(
        below,
        value,
        current,
        child(path, key),
        kept,
        quiet,
      );
      if (next === current) continue;
      changed = true;
      fields = { ...(fields ?? {}), [key]: next };
    }
    return changed ? fields : after;
  }
  return after;
}

// itemOwners finds the node of item of a list: a k: node whose keys item
// holds, or the v: node of its value.
function itemOwners(
  owners: Owners,
  item: unknown,
):
  | {
      owners: Owners;
      matches: (candidate: unknown) => boolean;
      path: (list: FieldPath) => FieldPath;
      seed?: unknown;
    }
  | undefined {
  for (const [key, node] of owners.children) {
    if (!key.startsWith("k:")) continue;
    let keys: Record<string, unknown>;
    try {
      keys = JSON.parse(key.slice(2)) as Record<string, unknown>;
    } catch {
      continue;
    }
    const matches = (candidate: unknown) =>
      isRecord(candidate) &&
      Object.entries(keys).every(([k, v]) => sameJSON(candidate[k], v));
    if (matches(item))
      return {
        owners: node,
        matches,
        path: (list) => keyedItem(list, keys),
        seed: { ...keys },
      };
  }
  const node = owners.children.get(`v:${JSON.stringify(item)}`);
  if (node === undefined) return undefined;
  return {
    owners: node,
    matches: (candidate) => sameJSON(candidate, item),
    path: (list) => valueItem(list, item),
  };
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

// segment turns one FieldsV1 key into the path of its node: f: for a field
// or a map key, k: for a list item by key, v: for a set item, i: for a list
// index. It returns undefined for a key it cannot read.
function segment(path: FieldPath, key: string): FieldPath | undefined {
  const rest = key.slice(2);
  try {
    switch (key.slice(0, 2)) {
      case "f:":
        return child(path, rest);
      case "k:":
        return keyedItem(path, JSON.parse(rest) as Record<string, unknown>);
      case "v:":
        return valueItem(path, JSON.parse(rest));
      case "i:":
        return indexItem(path, Number(rest));
      default:
        return undefined;
    }
  } catch {
    return undefined;
  }
}
