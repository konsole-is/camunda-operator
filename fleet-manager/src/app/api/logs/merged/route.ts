import { MAX_LOG_STREAMS } from "@/lib/observe/logs";
import {
  PodLogNotFound,
  type PodLogRequest,
  PodLogUnavailable,
} from "@/lib/observe/pod-log";
import { followPodLog } from "@/lib/observe/readers";

// A page reads the logs of several containers over one EventSource, since
// Chromium opens at most 6 connections to one origin without TLS. Each
// log parameter is namespace/pod or namespace/pod/container, at most
// MAX_LOG_STREAMS. Each event names its log by its index among them: a
// line event has { source, line }, an end event { source }, and a failure
// event { source, message }. The answer ends when every log ended or broke.
export const dynamic = "force-dynamic";

class BadParameter extends Error {}

export async function GET(request: Request): Promise<Response> {
  let reqs: PodLogRequest[];
  try {
    reqs = parseRequests(new URL(request.url).searchParams);
  } catch (error) {
    if (error instanceof BadParameter) {
      return new Response(error.message, { status: 400 });
    }
    throw error;
  }
  return new Response(merged(reqs, request.signal), {
    headers: {
      "Content-Type": "text/event-stream; charset=utf-8",
      // no-transform keeps the compression of Next.js from buffering the stream.
      "Cache-Control": "no-cache, no-transform",
      "X-Accel-Buffering": "no",
    },
  });
}

function parseRequests(params: URLSearchParams): PodLogRequest[] {
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
  const logs = params.getAll("log");
  if (logs.length === 0 || logs.length > MAX_LOG_STREAMS) {
    throw new BadParameter(
      `The parameter log is required, at most ${MAX_LOG_STREAMS} times`,
    );
  }
  const sinceSeconds = positive("sinceSeconds");
  const tailLines = positive("tailLines");
  return logs.map((log) => {
    const [namespace, pod, container, ...rest] = log.split("/");
    if (!namespace || !pod || container === "" || rest.length > 0) {
      throw new BadParameter(
        "The parameter log must be namespace/pod or namespace/pod/container",
      );
    }
    return {
      namespace,
      pod,
      ...(container && { container }),
      ...(tailLines && { tailLines }),
      ...(sinceSeconds && { sinceSeconds }),
    };
  });
}

function merged(
  reqs: PodLogRequest[],
  signal: AbortSignal,
): ReadableStream<Uint8Array> {
  const encoder = new TextEncoder();
  const readers: ReadableStreamDefaultReader<string>[] = [];
  let cancelled = false;
  return new ReadableStream<Uint8Array>({
    start(controller) {
      const frame = (event: string, data: unknown) => {
        if (cancelled) return;
        controller.enqueue(
          encoder.encode(`event: ${event}\ndata: ${JSON.stringify(data)}\n\n`),
        );
      };
      const follow = async (req: PodLogRequest, source: number) => {
        // The error can quote the API server, so the page gets a fixed text,
        // except for the fixed texts of PodLogNotFound and PodLogUnavailable.
        const broke = `The log stream of pod ${req.pod} broke`;
        let lines: ReadableStream<string>;
        try {
          lines = await followPodLog(req, signal);
        } catch (error) {
          const known =
            error instanceof PodLogNotFound ||
            error instanceof PodLogUnavailable;
          frame("failure", { source, message: known ? error.message : broke });
          return;
        }
        try {
          const reader = lines.getReader();
          readers.push(reader);
          for (;;) {
            const { done, value } = await reader.read();
            if (done) break;
            frame("line", { source, line: value });
          }
          frame("end", { source });
        } catch {
          frame("failure", { source, message: broke });
        }
      };
      void Promise.all(reqs.map(follow)).then(() => {
        if (!cancelled) controller.close();
      });
    },
    async cancel() {
      cancelled = true;
      await Promise.all(readers.map((r) => r.cancel()));
    },
  });
}
