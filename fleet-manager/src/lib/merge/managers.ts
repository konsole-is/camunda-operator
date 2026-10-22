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
