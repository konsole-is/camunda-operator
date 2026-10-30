import { get } from "@/lib/k8s/read";
import { isDNSLabel, isDNSSubdomain } from "@/lib/labels";
import { brokerPartitions } from "@/lib/observe/partitions";

// A page polls the partitions of one cluster. The answer is { partitions },
// where partitions is "unavailable" when the management API of the brokers
// does not answer.
export const dynamic = "force-dynamic";

export async function GET(request: Request): Promise<Response> {
  const params = new URL(request.url).searchParams;
  const namespace = params.get("namespace");
  if (!namespace || !isDNSLabel(namespace)) return badParameter("namespace");
  const name = params.get("name");
  if (!name || !isDNSSubdomain(name)) return badParameter("name");
  try {
    const cluster = await get("CamundaCluster", namespace, name);
    if (!cluster) {
      return new Response(`The cluster ${namespace}/${name} does not exist`, {
        status: 404,
      });
    }
    return Response.json(
      { partitions: await brokerPartitions(cluster, Date.now()) },
      { headers: { "Cache-Control": "no-store" } },
    );
  } catch {
    // The error can quote the API server, so the page gets a fixed text.
    return new Response(
      `Could not read the partitions of cluster ${namespace}/${name}`,
      { status: 502 },
    );
  }
}

function badParameter(name: string): Response {
  return new Response(`The parameter ${name} is required and must be valid`, {
    status: 400,
  });
}
