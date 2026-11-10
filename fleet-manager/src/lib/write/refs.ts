import type { Change, KubeObject, ObjectRef, RefKey } from "./types";

/** refKey returns the key of an object in Change.preconditions. */
export function refKey(ref: {
  kind: string;
  namespace?: string;
  name: string;
}): RefKey {
  return `${ref.kind}/${ref.namespace ?? ""}/${ref.name}`;
}

/** refOf returns the reference of an object. */
export function refOf(object: KubeObject): ObjectRef {
  const { namespace, name } = object.metadata;
  return {
    apiVersion: object.apiVersion,
    kind: object.kind,
    ...(namespace === undefined ? {} : { namespace }),
    name,
  };
}

/** createdObjects returns the objects of change.apply whose key change.create lists, in apply order. */
export function createdObjects(change: Change): KubeObject[] {
  const create = change.create ?? [];
  return change.apply.filter((o) => create.includes(refKey(refOf(o))));
}
