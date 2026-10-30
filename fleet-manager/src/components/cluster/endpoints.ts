import type { CamundaClusterSpec } from "@/generated/crds/camundacluster";
import type { Resource } from "@/lib/k8s/kinds";
import { PROCESS_LABEL, type Process, topologyOf } from "./topology";

/** GRPC_HOST_OUTPUT is the output of the access section that holds the public gRPC host. It is OutputGRPCHost of internal/pluginstartup. */
export const GRPC_HOST_OUTPUT = "grpcHost";

/**
 * EndpointRow is one route of a cluster or of its Optimize. key is unique in
 * one result. url is undefined when the route has no public URL. servedBy is
 * undefined for a route name that the app does not know.
 */
export type EndpointRow = {
  key: string;
  owner: "CamundaCluster" | "CamundaOptimize";
  name: string;
  url?: string;
  service: string;
  port: number;
  protocol: "HTTP" | "GRPC";
  servedBy?: { label: string; embedded: boolean };
};

// The route names of api/v1 Endpoint, with the process that serves each.
const ROUTE_PROCESS: Record<string, Process> = {
  default: "gateway",
  rest: "gateway",
  grpc: "gateway",
  operate: "operate",
  tasklist: "tasklist",
  admin: "admin",
  connectors: "connectors",
};

/**
 * endpointRows returns the routes of the cluster in the order of its
 * status.endpoints, then those of each CamundaOptimize whose clusterRef
 * names the cluster. spec is the merged spec of the cluster, and
 * accessOutputs the status.outputs of its access request.
 *
 * The URL of an HTTP cluster route is the external URL of the cluster with
 * the path of the route. The URL of an Optimize route is the origin of its
 * external URL with the path of the route. A gRPC route has the grpcHost
 * output as its URL. servedBy names the process that serves the route, and
 * for an embedded process the box that it runs in, with embedded true.
 */
export function endpointRows(input: {
  cluster: Resource<"CamundaCluster">;
  spec: CamundaClusterSpec;
  optimizes: readonly Resource<"CamundaOptimize">[];
  accessOutputs?: Record<string, string>;
}): EndpointRow[] {
  const { cluster, spec, optimizes, accessOutputs } = input;
  const boxes = topologyOf(spec, []);
  const servedBy = (name: string): EndpointRow["servedBy"] => {
    const process = ROUTE_PROCESS[name];
    if (!process) return undefined;
    const own = boxes.find((b) => b.process === process);
    if (own) return { label: own.label, embedded: false };
    const host = boxes.find((b) =>
      b.embedded.some((e) => e.process === process),
    );
    if (host) return { label: host.label, embedded: true };
    return { label: PROCESS_LABEL[process], embedded: false };
  };

  const base = cluster.spec.externalUrl?.replace(/\/+$/, "") || undefined;
  const clusterRows = (cluster.status?.endpoints ?? []).map(
    (e): EndpointRow => ({
      key: `CamundaCluster/${e.name}`,
      owner: "CamundaCluster",
      name: e.name,
      url:
        e.protocol === "GRPC"
          ? accessOutputs?.[GRPC_HOST_OUTPUT]
          : base === undefined
            ? undefined
            : `${base}${e.pathPrefix ?? ""}`,
      service: e.service,
      port: e.port,
      protocol: e.protocol,
      servedBy: servedBy(e.name),
    }),
  );

  const clusterName = cluster.metadata?.name;
  const optimizeRows = optimizes
    .filter((o) => o.spec.clusterRef.name === clusterName)
    .flatMap((o) => {
      const origin = originOf(o.spec.externalUrl);
      return (o.status?.endpoints ?? []).map((e): EndpointRow => ({
        key: `CamundaOptimize/${o.metadata?.name}/${e.name}`,
        owner: "CamundaOptimize",
        name: e.name,
        url:
          origin === undefined ? undefined : `${origin}${e.pathPrefix ?? ""}`,
        service: e.service,
        port: e.port,
        protocol: e.protocol,
        servedBy: { label: "Optimize", embedded: false },
      }));
    });

  return [...clusterRows, ...optimizeRows];
}

function originOf(url: string | undefined): string | undefined {
  if (url === undefined) return undefined;
  try {
    return new URL(url).origin;
  } catch {
    return undefined;
  }
}
