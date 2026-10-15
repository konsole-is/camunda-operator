import "server-only";
import http from "node:http";
import https from "node:https";
import { KubeConfig } from "@kubernetes/client-node";

let loaded: KubeConfig | undefined;

/**
 * kubeConfig returns the configuration of the app. It reads KUBECONFIG,
 * then ~/.kube/config, then the ServiceAccount of the pod, once per process.
 */
export function kubeConfig(): KubeConfig {
  if (!loaded) {
    loaded = new KubeConfig();
    loaded.loadFromDefault();
  }
  return loaded;
}

/** KubernetesRequestError is an answer of the API server outside 2xx. */
export class KubernetesRequestError extends Error {
  constructor(
    readonly statusCode: number,
    message: string,
  ) {
    super(message);
    this.name = "KubernetesRequestError";
  }
}

/**
 * getJSON sends a GET of path to the API server of kc and returns the body
 * as plain JSON. It rejects with KubernetesRequestError, which carries the
 * message of the Status of the API server, when the status is not 2xx. It
 * rejects when the answer does not end within timeoutMs, if given.
 */
export async function getJSON<T>(
  kc: KubeConfig,
  path: string,
  timeoutMs?: number,
): Promise<T> {
  const cluster = kc.getCurrentCluster();
  if (!cluster) {
    throw new Error(
      "The kubeconfig of the fleet manager has no current cluster",
    );
  }
  const url = new URL(cluster.server + path);
  const options: https.RequestOptions = {
    method: "GET",
    headers: { Accept: "application/json" },
  };
  await kc.applyToHTTPSOptions(options);
  const transport = url.protocol === "http:" ? http : https;

  return new Promise<T>((resolve, reject) => {
    const request = transport.request(url, options, (response) => {
      const chunks: Buffer[] = [];
      response.on("data", (chunk: Buffer) => chunks.push(chunk));
      response.on("error", reject);
      response.on("end", () => {
        const text = Buffer.concat(chunks).toString("utf8");
        const status = response.statusCode ?? 0;
        if (status < 200 || status > 299) {
          reject(
            new KubernetesRequestError(
              status,
              statusMessage(text) ?? `HTTP ${status}`,
            ),
          );
          return;
        }
        try {
          resolve(JSON.parse(text) as T);
        } catch (error) {
          reject(error);
        }
      });
    });
    request.on("error", reject);
    if (timeoutMs !== undefined) {
      const timer = setTimeout(
        () =>
          request.destroy(
            new Error(`The API server did not answer ${path} in time`),
          ),
        timeoutMs,
      );
      request.on("close", () => clearTimeout(timer));
    }
    request.end();
  });
}

function statusMessage(text: string): string | undefined {
  try {
    const status = JSON.parse(text) as { message?: unknown };
    return typeof status.message === "string" ? status.message : undefined;
  } catch {
    return undefined;
  }
}
