import "server-only";
import type { KubernetesObject } from "@kubernetes/client-node";
import { defaultCache, type ResourceCache } from "./cache";
import { KindMap, type KindName, type Resource } from "./kinds";
import { parseSelector } from "./selector";

/**
 * ListOptions select the objects that list and useResources return.
 * labelSelector is a Kubernetes label selector.
 */
export interface ListOptions {
  namespace?: string;
  labelSelector?: string;
}

/**
 * list returns the objects of kind from the cache of the server, in the
 * namespace and with the labels that opts name, sorted by namespace and
 * name. It waits for the first sync of the kind, as ResourceCache.objects
 * does. The objects are copies. A list right after a write can miss the
 * write until its watch event arrives. It throws for a namespace of a
 * cluster-scoped kind and for a selector that is not valid.
 */
export function list<K extends KindName>(
  kind: K,
  opts: ListOptions = {},
): Promise<Resource<K>[]> {
  return listFrom(defaultCache(), kind, opts);
}

/**
 * get returns a copy of one object of kind from the cache of the server, or
 * undefined. namespace is required for a namespaced kind and refused for a
 * cluster-scoped one. It waits for the first sync of the kind, as list does.
 */
export function get<K extends KindName>(
  kind: K,
  namespace: string | undefined,
  name: string,
): Promise<Resource<K> | undefined> {
  return getFrom(defaultCache(), kind, namespace, name);
}

/** listFrom is list on the given cache. */
export async function listFrom<K extends KindName>(
  cache: ResourceCache,
  kind: K,
  opts: ListOptions = {},
): Promise<Resource<K>[]> {
  if (opts.namespace && KindMap[kind].scope === "Cluster") {
    throw new Error(
      `${kind} is cluster-scoped, so a list of it takes no namespace`,
    );
  }
  const matches = parseSelector(opts.labelSelector ?? "");
  const objects = (await cache.objects(kind)) as readonly KubernetesObject[];
  return objects
    .filter(
      (o) =>
        (!opts.namespace || o.metadata?.namespace === opts.namespace) &&
        matches(o.metadata?.labels),
    )
    .sort(byNamespaceAndName)
    .map((o) => structuredClone(o) as Resource<K>);
}

/** getFrom is get on the given cache. */
export async function getFrom<K extends KindName>(
  cache: ResourceCache,
  kind: K,
  namespace: string | undefined,
  name: string,
): Promise<Resource<K> | undefined> {
  const scope = KindMap[kind].scope;
  if (scope === "Cluster" && namespace) {
    throw new Error(
      `${kind} is cluster-scoped, so a get of it takes no namespace`,
    );
  }
  if (scope === "Namespaced" && !namespace) {
    throw new Error(`${kind} is namespaced, so a get of it needs a namespace`);
  }
  const objects = (await cache.objects(kind)) as readonly KubernetesObject[];
  const found = objects.find(
    (o) =>
      o.metadata?.name === name &&
      (o.metadata?.namespace ?? undefined) === namespace,
  );
  return found && (structuredClone(found) as Resource<K>);
}

function byNamespaceAndName(a: KubernetesObject, b: KubernetesObject): number {
  return (
    (a.metadata?.namespace ?? "").localeCompare(b.metadata?.namespace ?? "") ||
    (a.metadata?.name ?? "").localeCompare(b.metadata?.name ?? "")
  );
}
