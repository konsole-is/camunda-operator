// This module runs in the browser and on the server, so it imports types only.
import type { Resource } from "@/lib/k8s/kinds";

/** EventRef names the object of events. Without namespace it is a cluster-scoped object. */
export type EventRef = {
  kind: string;
  name: string;
  namespace?: string;
  uid?: string;
};

/**
 * EventScope names the objects of one component in namespace: its pods,
 * the names of the Services and the PersistentVolumeClaims that its pod
 * selector selects, and refs, the objects whose events belong to it in any
 * namespace, such as its owner resource.
 */
export type EventScope = {
  namespace: string;
  pods: readonly Resource<"Pod">[];
  services: readonly string[];
  claims: readonly string[];
  refs: readonly EventRef[];
};

/**
 * regards is true when the regarding object of event has the kind, name,
 * and namespace of ref, and its uid when ref has one, so that the events
 * of an older object of the same name can be left out.
 */
export function regards(event: Resource<"Event">, ref: EventRef): boolean {
  const { regarding } = event;
  return (
    regarding?.kind === ref.kind &&
    regarding.name === ref.name &&
    (regarding.namespace ?? undefined) === ref.namespace &&
    (!ref.uid || regarding.uid === ref.uid)
  );
}

/**
 * workloadNames returns the names of the workloads of pods, unique and
 * sorted: the Service names, the StatefulSet that owns a pod, and the
 * Deployment of a pod whose ReplicaSet is named after its
 * pod-template-hash label. It reads no workload object.
 */
export function workloadNames(
  pods: readonly Resource<"Pod">[],
  services: readonly string[],
): string[] {
  const names = new Set(services);
  for (const pod of pods) {
    const hash = pod.metadata?.labels?.["pod-template-hash"];
    for (const owner of pod.metadata?.ownerReferences ?? []) {
      if (owner.kind === "StatefulSet") names.add(owner.name);
      if (owner.kind === "ReplicaSet" && hash && owner.name.endsWith(`-${hash}`))
        names.add(owner.name.slice(0, -hash.length - 1));
    }
  }
  return [...names].sort();
}

/**
 * componentEvents returns the events of scope, in their order. An event in
 * the namespace of scope belongs to it when its object is a pod of scope,
 * one of its Services or PersistentVolumeClaims, a StatefulSet or a
 * Deployment with a workload name, or a ReplicaSet named
 * "<workload name>-<hash>". An event in any namespace belongs to it when
 * it regards one of the refs.
 */
export function componentEvents(
  events: readonly Resource<"Event">[],
  scope: EventScope,
): Resource<"Event">[] {
  const pods = new Set(scope.pods.map((p) => p.metadata?.name));
  const services = new Set(scope.services);
  const claims = new Set(scope.claims);
  const workloads = workloadNames(scope.pods, scope.services);
  const ownObject = ({ kind, name }: { kind?: string; name?: string }) => {
    if (name === undefined) return false;
    switch (kind) {
      case "Pod":
        return pods.has(name);
      case "Service":
        return services.has(name);
      case "PersistentVolumeClaim":
        return claims.has(name);
      case "StatefulSet":
      case "Deployment":
        return workloads.includes(name);
      case "ReplicaSet":
        return workloads.some(
          (w) =>
            name.startsWith(`${w}-`) &&
            /^[a-z0-9]+$/.test(name.slice(w.length + 1)),
        );
      default:
        return false;
    }
  };
  return events.filter(
    (event) =>
      scope.refs.some((ref) => regards(event, ref)) ||
      (event.regarding?.namespace === scope.namespace &&
        ownObject(event.regarding)),
  );
}
