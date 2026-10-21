import "server-only";
import { KubernetesRequestError, getJSON, kubeConfig } from "@/lib/k8s/client";
import type { Resource } from "@/lib/k8s/kinds";
import { list } from "@/lib/k8s/read";
import { labelSelectorString } from "@/lib/labels";
import { parseQuantity } from "./quantity";
import type { PodUsage, Unavailable } from "./types";

const TIMEOUT_MS = 10_000;

/** EventRef names the object of events. Without namespace it is a cluster-scoped object. */
export type EventRef = {
  kind: string;
  name: string;
  namespace?: string;
  uid?: string;
};

type PodMetrics = {
  items?: {
    metadata?: { name?: string };
    containers?: { usage?: { cpu?: string; memory?: string } }[];
  }[];
};

type Resources = { requests?: Record<string, string>; limits?: Record<string, string> };

/**
 * podUsage returns the live CPU and memory of the pods in namespace that
 * selector selects, from metrics.k8s.io, with the requests and limits of
 * their spec. A request is set when a container of the pod has one, and a
 * limit only when every container has one. It returns "unavailable" when the API server answers 404 or
 * 503, which means that no metrics server serves metrics.k8s.io. It
 * rejects for another failure.
 */
export async function podUsage(
  namespace: string,
  selector: string,
): Promise<PodUsage[] | Unavailable> {
  const query = selector
    ? `?${new URLSearchParams({ labelSelector: selector })}`
    : "";
  let metrics: PodMetrics;
  try {
    metrics = await getJSON<PodMetrics>(
      kubeConfig(),
      `/apis/metrics.k8s.io/v1beta1/namespaces/${namespace}/pods${query}`,
      TIMEOUT_MS,
    );
  } catch (error) {
    if (
      error instanceof KubernetesRequestError &&
      (error.statusCode === 404 || error.statusCode === 503)
    ) {
      return "unavailable";
    }
    throw error;
  }
  const pods = await list("Pod", {
    namespace,
    ...(selector && { labelSelector: selector }),
  });
  const specs = new Map(
    pods.map((pod) => [
      pod.metadata?.name,
      (pod.spec?.containers ?? []).map((c) => c.resources ?? {}),
    ]),
  );
  return (metrics.items ?? []).map((item) => {
    const name = item.metadata?.name ?? "";
    const containers = item.containers ?? [];
    const resources = specs.get(name);
    const used = (key: "cpu" | "memory") =>
      containers.reduce((sum, c) => sum + parseQuantity(c.usage?.[key] ?? "0"), 0);
    const cpu = sums(resources, "cpu");
    const memory = sums(resources, "memory");
    return {
      pod: name,
      cpu: {
        usedCores: used("cpu"),
        ...defined({ requestCores: cpu.request, limitCores: cpu.limit }),
      },
      memory: {
        usedBytes: used("memory"),
        ...defined({ requestBytes: memory.request, limitBytes: memory.limit }),
      },
    };
  });
}

/**
 * requestPods returns the names of the pods in the namespace of request
 * that its status.podSelector selects. No selector and an empty one give no
 * pods, never every pod.
 */
export async function requestPods(
  request: Resource<"PluginRequest">,
): Promise<string[]> {
  const selector = labelSelectorString(request.status?.podSelector);
  if (!selector) return [];
  const pods = await list("Pod", {
    namespace: request.metadata?.namespace,
    labelSelector: selector,
  });
  return pods.map((pod) => pod.metadata?.name ?? "");
}

/**
 * events returns the events.k8s.io events whose regarding object has the
 * kind, name, and namespace of ref, and its uid when ref has one, so that
 * the events of an older object of the same name can be left out.
 */
export async function events(ref: EventRef): Promise<Resource<"Event">[]> {
  const all = await list("Event", ref.namespace ? { namespace: ref.namespace } : {});
  return all.filter(({ regarding }) =>
    regarding?.kind === ref.kind &&
    regarding.name === ref.name &&
    (regarding.namespace ?? undefined) === ref.namespace &&
    (!ref.uid || regarding.uid === ref.uid),
  );
}

// sums returns the request and the limit of key for a pod. A container
// without a request asks for nothing, so the request sums the containers
// that have one. A container without a limit can use the whole node, so the
// pod has a limit only when each container has one.
function sums(
  containers: Resources[] | undefined,
  key: "cpu" | "memory",
): { request?: number; limit?: number } {
  const values = (part: "requests" | "limits") =>
    (containers ?? []).map((c) => c[part]?.[key]);
  const total = (set: string[]) =>
    set.reduce((sum, v) => sum + parseQuantity(v), 0);
  const requests = values("requests").filter((v) => v !== undefined);
  const limits = values("limits");
  return {
    request: requests.length ? total(requests) : undefined,
    limit:
      limits.length && limits.every((v) => v !== undefined)
        ? total(limits as string[])
        : undefined,
  };
}

function defined<T extends Record<string, number | undefined>>(
  values: T,
): Partial<Record<keyof T, number>> {
  return Object.fromEntries(
    Object.entries(values).filter(([, v]) => v !== undefined),
  ) as Partial<Record<keyof T, number>>;
}
