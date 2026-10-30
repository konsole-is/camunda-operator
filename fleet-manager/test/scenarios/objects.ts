// The object model of the page scenarios that make fleet-scenarios writes
// into e2e/scenarios. Playwright loads it too, so it has no import.meta.
import fs from "node:fs/promises";
import { isDeepStrictEqual } from "node:util";
import { parseAllDocuments } from "yaml";

/** KubeObject is one object of a scenario file. */
export interface KubeObject {
  apiVersion: string;
  kind: string;
  metadata: {
    name: string;
    namespace?: string;
    uid?: string;
    generation?: number;
    ownerReferences?: { uid: string }[];
    finalizers?: string[];
    deletionTimestamp?: string;
    managedFields?: unknown[];
    [key: string]: unknown;
  };
  status?: unknown;
  [key: string]: unknown;
}

/** parseObjects reads every object of a multi-document YAML text. */
export function parseObjects(text: string): KubeObject[] {
  return parseAllDocuments(text)
    .map((doc) => doc.toJS() as KubeObject | null)
    .filter((o): o is KubeObject => o !== null && o !== undefined);
}

/** readObjects reads every object of a scenario file. */
export async function readObjects(file: string): Promise<KubeObject[]> {
  return parseObjects(await fs.readFile(file, "utf8"));
}

/** objectKey names an object by its API version, kind, namespace, and name. */
export function objectKey(o: KubeObject): string {
  return `${o.apiVersion}/${o.kind}/${o.metadata.namespace ?? ""}/${o.metadata.name}`;
}

/** describeObject names an object as an error message does: `<Kind> <namespace>/<name>`. */
export function describeObject(o: KubeObject): string {
  const { namespace, name } = o.metadata;
  return `${o.kind} ${namespace ? `${namespace}/` : ""}${name}`;
}

/**
 * createOrder returns objs with each object after the objects of objs that
 * it names as owners, and after its Namespace when that Namespace has a
 * uid. Apart from that, Namespaces come first and events.k8s.io Events
 * last. An owner that is not in objs does not hold an object back. It
 * throws on an owner cycle and names each object that it could not place.
 */
export function createOrder(objs: readonly KubeObject[]): KubeObject[] {
  const rank = (o: KubeObject) =>
    o.apiVersion === "v1" && o.kind === "Namespace"
      ? 0
      : o.apiVersion === "events.k8s.io/v1" && o.kind === "Event"
        ? 2
        : 1;
  const remaining = [0, 1, 2].flatMap((r) => objs.filter((o) => rank(o) === r));
  const uids = new Set(objs.flatMap((o) => o.metadata.uid ?? []));
  const namespaces = new Map(
    objs.flatMap((o) =>
      rank(o) === 0 && o.metadata.uid
        ? [[o.metadata.name, o.metadata.uid]]
        : [],
    ),
  );
  const needs = (o: KubeObject) => {
    const owners = (o.metadata.ownerReferences ?? []).map((ref) => ref.uid);
    const namespace = namespaces.get(o.metadata.namespace ?? "");
    return namespace ? [...owners, namespace] : owners;
  };
  const placed = new Set<string>();
  const order: KubeObject[] = [];
  while (remaining.length > 0) {
    const next = remaining.findIndex((o) =>
      needs(o).every((uid) => !uids.has(uid) || placed.has(uid)),
    );
    if (next < 0) {
      throw new Error(
        `The owner references of these objects form a cycle: ${remaining.map(describeObject).join(", ")}`,
      );
    }
    const [o] = remaining.splice(next, 1);
    if (o.metadata.uid) placed.add(o.metadata.uid);
    order.push(o);
  }
  return order;
}

/**
 * replaceUIDs returns a copy of value in which each occurrence of a key of
 * uids, in any string, is its value. It reaches keys and values alike, such
 * as a UID in a field manager name or in a managedFields key.
 */
export function replaceUIDs<T>(value: T, uids: ReadonlyMap<string, string>): T {
  let text = JSON.stringify(value);
  for (const [from, to] of uids) text = text.split(from).join(to);
  return JSON.parse(text) as T;
}

/**
 * alignObservedGeneration returns a copy of status in which each
 * observedGeneration, at any depth, that equals recorded is live. Other
 * values stay: the generator writes 0 for a status that was behind.
 */
export function alignObservedGeneration<T>(
  status: T,
  recorded: number,
  live: number,
): T {
  const walk = (v: unknown): unknown => {
    if (Array.isArray(v)) return v.map(walk);
    if (v === null || typeof v !== "object") return v;
    return Object.fromEntries(
      Object.entries(v).map(([k, e]) => [
        k,
        k === "observedGeneration" && e === recorded ? live : walk(e),
      ]),
    );
  };
  return walk(status) as T;
}

/**
 * diffStates compares two snapshots of a scenario by objectKey. create
 * holds the objects that only to has, in createOrder. update holds the
 * objects of to that differ from their copy in from. remove holds the
 * objects that only from has, dependents before their owners.
 */
export function diffStates(
  from: readonly KubeObject[],
  to: readonly KubeObject[],
): { create: KubeObject[]; update: KubeObject[]; remove: KubeObject[] } {
  const before = new Map(from.map((o) => [objectKey(o), o]));
  const after = new Set(to.map(objectKey));
  return {
    create: createOrder(to).filter((o) => !before.has(objectKey(o))),
    update: to.filter((o) => {
      const old = before.get(objectKey(o));
      return old !== undefined && !isDeepStrictEqual(old, o);
    }),
    remove: createOrder(from)
      .reverse()
      .filter((o) => !after.has(objectKey(o))),
  };
}
