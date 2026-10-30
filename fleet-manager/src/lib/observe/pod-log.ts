import "server-only";
import http from "node:http";
import https from "node:https";
import { kubeConfig } from "@/lib/k8s/client";
import { pathSegment } from "@/lib/k8s/kinds";
import { get } from "@/lib/k8s/read";

/** PodLogRequest names the log to follow. sinceSeconds and tailLines are positive integers. */
export type PodLogRequest = {
  namespace: string;
  pod: string;
  container?: string;
  sinceSeconds?: number;
  tailLines?: number;
};

/** PodLogNotFound is a pod or a container that does not exist. */
export class PodLogNotFound extends Error {
  constructor(message: string) {
    super(message);
    this.name = "PodLogNotFound";
  }
}

/**
 * PodLogUnavailable is a log that the API server did not give. status is
 * 409 for a container that is not running, 502 otherwise. The message is
 * fixed text that names the pod.
 */
export class PodLogUnavailable extends Error {
  constructor(
    readonly status: 409 | 502,
    message: string,
  ) {
    super(message);
    this.name = "PodLogUnavailable";
  }
}

/**
 * followPodLog follows the log of a container of a pod, by default its
 * first container, and returns a stream with one chunk for each line,
 * without the line end. A line longer than 64 KiB arrives in more than one
 * chunk. The stream closes when the log ends, and errors when the
 * connection breaks. An abort of signal stops the request. It rejects with
 * PodLogNotFound for a pod or a container that does not exist, and with
 * PodLogUnavailable for another answer outside 200.
 */
export async function followPodLog(
  req: PodLogRequest,
  signal: AbortSignal,
): Promise<ReadableStream<string>> {
  const { namespace, pod: name } = req;
  const pod = await get("Pod", namespace, name);
  const names = [
    ...(pod?.spec?.containers ?? []),
    ...(pod?.spec?.initContainers ?? []),
    ...(pod?.spec?.ephemeralContainers ?? []),
  ].map((c) => c.name);
  const container = req.container ?? names[0];
  if (!pod) {
    throw new PodLogNotFound(`The pod ${namespace}/${name} does not exist`);
  }
  if (!container || !names.includes(container)) {
    throw new PodLogNotFound(
      `The pod ${namespace}/${name} has no container ${container ?? ""}`.trim(),
    );
  }

  const kc = kubeConfig();
  const cluster = kc.getCurrentCluster();
  if (!cluster) {
    throw new Error(
      "The kubeconfig of the fleet manager has no current cluster",
    );
  }
  const url = new URL(
    `${cluster.server}/api/v1/namespaces/${pathSegment(namespace)}/pods/${pathSegment(name)}/log`,
  );
  url.searchParams.set("container", container);
  url.searchParams.set("follow", "true");
  if (req.sinceSeconds) {
    url.searchParams.set("sinceSeconds", String(req.sinceSeconds));
  }
  if (req.tailLines) url.searchParams.set("tailLines", String(req.tailLines));
  const options: https.RequestOptions = { method: "GET", headers: {} };
  await kc.applyToHTTPSOptions(options);
  const transport = url.protocol === "http:" ? http : https;

  // The Log class of @kubernetes/client-node pipes the answer into a
  // Writable, and a pipe does not pass on an error of its source. So a
  // broken connection would look like a log that never ends.
  const response = await new Promise<http.IncomingMessage>(
    (resolve, reject) => {
      const request = transport.request(url, { ...options, signal }, resolve);
      request.on("error", reject);
      request.end();
    },
  );
  if (response.statusCode !== 200) {
    response.destroy();
    if (response.statusCode === 404) {
      throw new PodLogNotFound(`The pod ${namespace}/${name} does not exist`);
    }
    // The API server answers 400 for a container that is waiting to start.
    if (response.statusCode === 400) {
      throw new PodLogUnavailable(
        409,
        `The container ${container} of pod ${namespace}/${name} is not running`,
      );
    }
    throw new PodLogUnavailable(
      502,
      `The API server did not give the log of pod ${namespace}/${name}`,
    );
  }
  return lineStream(response);
}

// The log pauses while QUEUED_LINES lines wait for the reader.
const QUEUED_LINES = 100;
const MAX_LINE = 64 * 1024;

function lineStream(response: http.IncomingMessage): ReadableStream<string> {
  let carry = "";
  let done = false;
  return new ReadableStream<string>(
    {
      start(controller) {
        response.setEncoding("utf8");
        response.on("data", (chunk: string) => {
          const parts = (carry + chunk).split("\n");
          carry = parts.pop()!;
          if (carry.length > MAX_LINE) {
            parts.push(carry);
            carry = "";
          }
          for (const line of parts) {
            controller.enqueue(line.replace(/\r$/, ""));
          }
          if ((controller.desiredSize ?? 0) <= 0) response.pause();
        });
        response.on("end", () => {
          done = true;
          if (carry) controller.enqueue(carry.replace(/\r$/, ""));
          controller.close();
        });
        // An error always comes with a close, which reports it.
        response.on("error", () => {});
        response.on("close", () => {
          if (done) return;
          done = true;
          controller.error(new Error("The log stream broke"));
        });
      },
      pull() {
        response.resume();
      },
      cancel() {
        done = true;
        response.destroy();
      },
    },
    new CountQueuingStrategy({ highWaterMark: QUEUED_LINES }),
  );
}
