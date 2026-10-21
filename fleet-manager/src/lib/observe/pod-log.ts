import "server-only";
import http from "node:http";
import https from "node:https";
import { KubernetesRequestError, kubeConfig } from "@/lib/k8s/client";
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
 * followPodLog follows the log of a container of a pod, by default its
 * first container, and returns a stream with one chunk for each line,
 * without the line end. The stream closes when the log ends, and errors
 * when the connection breaks. An abort of signal stops the request. It
 * rejects with PodLogNotFound for a pod or a container that does not
 * exist, and with KubernetesRequestError for another answer outside 200.
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
    `${cluster.server}/api/v1/namespaces/${encodeURIComponent(namespace)}/pods/${encodeURIComponent(name)}/log`,
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
      const request = transport.request(url, options, resolve);
      request.on("error", reject);
      signal.addEventListener("abort", () => request.destroy(), { once: true });
      request.end();
    },
  );
  if (response.statusCode !== 200) {
    const code = response.statusCode ?? 0;
    const text = await readText(response);
    if (code === 404) {
      throw new PodLogNotFound(`The pod ${namespace}/${name} does not exist`);
    }
    throw new KubernetesRequestError(
      code,
      statusMessage(text) ?? `HTTP ${code}`,
    );
  }
  return lineStream(response);
}

function lineStream(response: http.IncomingMessage): ReadableStream<string> {
  let carry = "";
  let done = false;
  return new ReadableStream<string>({
    start(controller) {
      response.setEncoding("utf8");
      response.on("data", (chunk: string) => {
        const parts = (carry + chunk).split("\n");
        carry = parts.pop()!;
        for (const line of parts) controller.enqueue(line.replace(/\r$/, ""));
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
    cancel() {
      done = true;
      response.destroy();
    },
  });
}

async function readText(response: http.IncomingMessage): Promise<string> {
  const chunks: Buffer[] = [];
  for await (const chunk of response) chunks.push(chunk as Buffer);
  return Buffer.concat(chunks).toString("utf8");
}

function statusMessage(text: string): string | undefined {
  try {
    const message = (JSON.parse(text) as { message?: unknown }).message;
    return typeof message === "string" ? message : undefined;
  } catch {
    return undefined;
  }
}
