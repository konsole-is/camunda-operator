// This module runs in the browser and on the server.
import type { Resource } from "@/lib/k8s/kinds";
import { environmentNameOf } from "@/lib/policy";

/**
 * ServedCluster is one row of status.clusters of a plane. environment is
 * undefined when the namespace of the cluster belongs to no environment or
 * is not in the namespace list. environmentUnknown is true when the page
 * could not read the namespaces or the environments, so it cannot tell.
 */
export type ServedCluster = {
  namespace: string;
  name: string;
  environment?: string;
  environmentUnknown?: true;
  attached: boolean;
  reason?: string;
  message?: string;
  href: string;
};

/**
 * servedClusters returns the rows of status.clusters of plane in their
 * order. Set environmentsUnread while the namespaces or the environments
 * are not read. A row without a string namespace and name is left out. A
 * row is attached only when attached is true.
 */
export function servedClusters(input: {
  plane: Resource<"CamundaManagementCluster"> | undefined;
  namespaces: readonly Resource<"Namespace">[];
  environments: readonly Resource<"Environment">[];
  environmentsUnread?: boolean;
}): ServedCluster[] {
  const { plane, namespaces, environments, environmentsUnread } = input;
  const rows: unknown = plane?.status?.clusters;
  if (!Array.isArray(rows)) return [];
  return rows.flatMap((row: unknown): ServedCluster[] => {
    if (typeof row !== "object" || row === null) return [];
    const r = row as Record<string, unknown>;
    const namespace = text(r.namespace);
    const name = text(r.name);
    if (namespace === undefined || name === undefined) return [];
    const reason = text(r.reason);
    const message = text(r.message);
    const namespaceObject = namespaces.find(
      (n) => n.metadata?.name === namespace,
    );
    const environment =
      environmentsUnread || !namespaceObject
        ? undefined
        : environmentNameOf(namespaceObject, environments);
    return [
      {
        namespace,
        name,
        ...(environment !== undefined && { environment }),
        ...(environmentsUnread && { environmentUnknown: true as const }),
        attached: r.attached === true,
        ...(reason !== undefined && { reason }),
        ...(message !== undefined && { message }),
        href: `/fleet/clusters/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}`,
      },
    ];
  });
}

function text(value: unknown): string | undefined {
  return typeof value === "string" && value !== "" ? value : undefined;
}
