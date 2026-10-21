import {
  PodLogNotFound,
  type PodLogRequest,
  followPodLog,
} from "@/lib/observe/pod-log";

// A page reads the log of one container with EventSource. Each line is a
// line event with the line as a JSON string. The end event says that the
// log ended, and the failure event that the stream broke.
export const dynamic = "force-dynamic";

class BadParameter extends Error {}

export async function GET(request: Request): Promise<Response> {
  let req: PodLogRequest;
  try {
    req = parseRequest(new URL(request.url).searchParams);
  } catch (error) {
    if (error instanceof BadParameter) {
      return new Response(error.message, { status: 400 });
    }
    throw error;
  }
  let lines: ReadableStream<string>;
  try {
    lines = await followPodLog(req, request.signal);
  } catch (error) {
    if (error instanceof PodLogNotFound) {
      return new Response(error.message, { status: 404 });
    }
    throw error;
  }
  return new Response(events(lines, req.pod), {
    headers: {
      "Content-Type": "text/event-stream; charset=utf-8",
      // no-transform keeps the compression of Next.js from buffering the stream.
      "Cache-Control": "no-cache, no-transform",
      "X-Accel-Buffering": "no",
    },
  });
}

function parseRequest(params: URLSearchParams): PodLogRequest {
  const required = (name: string) => {
    const value = params.get(name);
    if (!value) throw new BadParameter(`The parameter ${name} is required`);
    return value;
  };
  const positive = (name: string) => {
    const value = params.get(name);
    if (value === null) return undefined;
    if (!/^[1-9]\d*$/.test(value)) {
      throw new BadParameter(
        `The parameter ${name} must be a positive integer`,
      );
    }
    return Number(value);
  };
  const container = params.get("container") || undefined;
  const sinceSeconds = positive("sinceSeconds");
  const tailLines = positive("tailLines");
  return {
    namespace: required("namespace"),
    pod: required("pod"),
    ...(container && { container }),
    ...(sinceSeconds && { sinceSeconds }),
    ...(tailLines && { tailLines }),
  };
}

function events(
  lines: ReadableStream<string>,
  pod: string,
): ReadableStream<Uint8Array> {
  const encoder = new TextEncoder();
  const frame = (event: string, data: unknown) =>
    encoder.encode(`event: ${event}\ndata: ${JSON.stringify(data)}\n\n`);
  const reader = lines.getReader();
  return new ReadableStream<Uint8Array>({
    async pull(controller) {
      try {
        const { done, value } = await reader.read();
        if (!done) {
          controller.enqueue(frame("line", value));
          return;
        }
        controller.enqueue(frame("end", {}));
      } catch {
        // The error can quote the API server, so the page gets a fixed text.
        controller.enqueue(
          frame("failure", { message: `The log stream of pod ${pod} broke` }),
        );
      }
      controller.close();
    },
    cancel() {
      return reader.cancel();
    },
  });
}
