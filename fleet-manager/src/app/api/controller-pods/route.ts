import { get } from "@/lib/k8s/read";
import { isDNSSubdomain } from "@/lib/labels";
import { controllerPods } from "@/lib/observe/kube";

// A page polls the controller pods of one registration, to follow their
// logs. The answer is { controller: { namespace, pods } }, where pods are
// names.
export const dynamic = "force-dynamic";

export async function GET(request: Request): Promise<Response> {
  const name = new URL(request.url).searchParams.get("registration");
  if (!name || !isDNSSubdomain(name)) {
    return new Response(
      "The parameter registration is required and must be valid",
      { status: 400 },
    );
  }
  try {
    const registration = await get("PluginRegistration", undefined, name);
    if (!registration) {
      return new Response(`The registration ${name} does not exist`, {
        status: 404,
      });
    }
    const pods = await controllerPods(registration);
    return Response.json(
      {
        controller: {
          namespace: registration.spec.serviceAccount.namespace,
          pods: pods.map((pod) => pod.metadata?.name ?? ""),
        },
      },
      { headers: { "Cache-Control": "no-store" } },
    );
  } catch {
    // The error can quote the API server, so the page gets a fixed text.
    return new Response("Could not read the controller pods", { status: 502 });
  }
}
