// The probes of the chart call this route. The server serves it only after
// the start checks of src/instrumentation.ts passed.
export const dynamic = "force-dynamic";

export function GET(): Response {
  return new Response("ok");
}
