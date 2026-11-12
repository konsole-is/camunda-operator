import { isDNSLabel, isDNSSubdomain } from "@/lib/labels";
import {
  METRICS_RANGES,
  componentMetrics,
  requestMetrics,
} from "@/lib/observe/metrics";
import { coreQueries } from "@/lib/observe/queries";
import type { MetricsComponent, MetricsRange } from "@/lib/observe/types";

// A page polls the charts of a component of a cluster, or of a request. It
// sends a ref, never a query: the server renders every query from its own
// table or from the registration on the cluster. The answer is { metrics }.
export const dynamic = "force-dynamic";

export async function GET(request: Request): Promise<Response> {
  const params = new URL(request.url).searchParams;
  const namespace = params.get("namespace");
  if (!namespace || !isDNSLabel(namespace)) return badParameter("namespace");
  const name = params.get("request");
  const cluster = params.get("cluster");
  const component = params.get("component");
  if (name !== null && (cluster !== null || component !== null)) {
    return new Response(
      "The parameter request does not go with cluster or component",
      { status: 400 },
    );
  }
  if (name !== null) {
    if (!isDNSSubdomain(name)) return badParameter("request");
  } else {
    if (!cluster || !isDNSSubdomain(cluster)) return badParameter("cluster");
    if (!component || !Object.hasOwn(coreQueries, component)) {
      return badParameter("component");
    }
  }
  const range = params.get("range") ?? "1h";
  if (!Object.hasOwn(METRICS_RANGES, range)) return badParameter("range");

  try {
    const metrics =
      name !== null
        ? await requestMetrics(
            namespace,
            name,
            range as MetricsRange,
            Date.now(),
          )
        : await componentMetrics(
            namespace,
            cluster!,
            component as MetricsComponent,
            range as MetricsRange,
            Date.now(),
          );
    if (!metrics) {
      return new Response(
        name !== null
          ? `The request ${namespace}/${name} does not exist`
          : `The cluster ${namespace}/${cluster} does not exist`,
        { status: 404 },
      );
    }
    return Response.json(
      { metrics },
      { headers: { "Cache-Control": "no-store" } },
    );
  } catch {
    // The error can quote the API server, so the page gets a fixed text.
    return new Response("Could not read the metrics", { status: 502 });
  }
}

function badParameter(name: string): Response {
  return new Response(`The parameter ${name} is required and must be valid`, {
    status: 400,
  });
}
