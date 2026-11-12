import { fleetPodWarnings } from "@/lib/observe/warnings";
import type { PodWarnings } from "@/lib/warnings";

// A page polls the pod and metric warnings of the whole fleet. The route
// takes no parameter: the server picks the pods and renders the queries.
// The answer is { warnings }.
export const dynamic = "force-dynamic";

// One read serves the polls of every open page for 10 seconds.
const SHARE_MS = 10_000;

let shared: { at: number; answer: Promise<PodWarnings> } | undefined;

export async function GET(): Promise<Response> {
  const now = Date.now();
  if (!shared || now - shared.at >= SHARE_MS) {
    const answer = fleetPodWarnings(new Date(now));
    shared = { at: now, answer };
    // A failed read is not shared, so the next poll reads again.
    answer.catch(() => {
      if (shared?.answer === answer) shared = undefined;
    });
  }
  const { answer } = shared;
  try {
    return Response.json(
      { warnings: await answer },
      { headers: { "Cache-Control": "no-store" } },
    );
  } catch {
    // The error can quote the API server, so the page gets a fixed text.
    return new Response("Could not read the warnings of the pods", {
      status: 502,
    });
  }
}
