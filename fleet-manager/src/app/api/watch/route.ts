import { defaultCache } from "@/lib/k8s/cache";
import {
  parseWatchRequest,
  watchStream,
  WatchRequestError,
} from "@/lib/k8s/watch";

// Pages call this route with EventSource to get live changes of the kinds
// they show. Each connection reads the shared informer cache of the server.
export const dynamic = "force-dynamic";

export function GET(request: Request): Response {
  let watchRequest;
  try {
    watchRequest = parseWatchRequest(new URL(request.url).searchParams);
  } catch (error) {
    if (error instanceof WatchRequestError) {
      return new Response(error.message, { status: 400 });
    }
    throw error;
  }
  return new Response(
    watchStream(defaultCache(), watchRequest, request.signal),
    {
      headers: {
        "Content-Type": "text/event-stream; charset=utf-8",
        // no-transform keeps the compression of Next.js from buffering the stream.
        "Cache-Control": "no-cache, no-transform",
        "X-Accel-Buffering": "no",
      },
    },
  );
}
