import { podUsage } from "@/lib/observe/kube";

// A page polls the live CPU and memory of the pods that a label selector
// selects. The answer is { usage }, where usage is "unavailable" when the
// cluster has no metrics server.
export const dynamic = "force-dynamic";

export async function GET(request: Request): Promise<Response> {
  const params = new URL(request.url).searchParams;
  const namespace = params.get("namespace");
  if (!namespace) return badParameter("namespace");
  // An empty selector selects every pod of the namespace.
  const selector = params.get("selector");
  if (!selector) return badParameter("selector");
  try {
    return Response.json(
      { usage: await podUsage(namespace, selector) },
      { headers: { "Cache-Control": "no-store" } },
    );
  } catch {
    // The error can quote the API server, so the page gets a fixed text.
    return new Response("Could not read the usage of the pods", {
      status: 502,
    });
  }
}

function badParameter(name: string): Response {
  return new Response(`The parameter ${name} is required`, { status: 400 });
}
