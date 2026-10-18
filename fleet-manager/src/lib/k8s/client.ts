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

/** KubeStatus is the Status object of an API server answer outside 2xx. */
export interface KubeStatus {
  code?: number;
  reason?: string;
  message?: string;
  details?: {
    causes?: { reason?: string; message?: string; field?: string }[];
  };
}

/**
 * KubernetesRequestError is an answer of the API server outside 2xx.
 * status is unset when the answer holds no Status.
 */
export class KubernetesRequestError extends Error {
  constructor(
    readonly statusCode: number,
    message: string,
    readonly status?: KubeStatus,
  ) {
    super(message);
    this.name = "KubernetesRequestError";
  }
}

/** RequestOptions is one request to the API server. path holds the query string. */
export interface RequestOptions {
  method: "GET" | "PATCH" | "DELETE" | "POST" | "PUT";
  path: string;
  /** body is sent as JSON. */
  body?: unknown;
  /** contentType is the Content-Type of the body, "application/json" by default. */
  contentType?: string;
  timeoutMs?: number;
}

/**
 * getJSON sends a GET of path to the API server of kc and returns the body
 * as plain JSON. It fails as requestJSON does.
 */
export async function getJSON<T>(
  kc: KubeConfig,
  path: string,
  timeoutMs?: number,
): Promise<T> {
  return requestJSON<T>(kc, { method: "GET", path, timeoutMs });
}

/**
 * requestJSON sends one request to the API server of kc and returns the
 * body as plain JSON, or undefined for an empty body. It rejects with
 * KubernetesRequestError, which carries the Status of the API server, when
 * the status is not 2xx. It rejects when the answer does not end within
 * timeoutMs, if given.
 */
export async function requestJSON<T>(
  kc: KubeConfig,
  { method, path, body, contentType, timeoutMs }: RequestOptions,
): Promise<T> {
  const cluster = kc.getCurrentCluster();
  if (!cluster) {
    throw new Error(
      "The kubeconfig of the fleet manager has no current cluster",
    );
  }
  const url = new URL(cluster.server + path);
  const headers: Record<string, string> = { Accept: "application/json" };
  if (body !== undefined) {
    headers["Content-Type"] = contentType ?? "application/json";
  }
  const options: https.RequestOptions = { method, headers };
  await kc.applyToHTTPSOptions(options);
  const transport = url.protocol === "http:" ? http : https;

  return new Promise<T>((resolve, reject) => {
    const request = transport.request(url, options, (response) => {
      const chunks: Buffer[] = [];
      response.on("data", (chunk: Buffer) => chunks.push(chunk));
      response.on("error", reject);
      response.on("end", () => {
        const text = Buffer.concat(chunks).toString("utf8");
        const code = response.statusCode ?? 0;
        if (code < 200 || code > 299) {
          const status = parseStatus(text);
          reject(
            new KubernetesRequestError(
              code,
              status?.message ?? `HTTP ${code}`,
              status,
            ),
          );
          return;
        }
        if (text === "") {
          resolve(undefined as T);
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
    request.end(body === undefined ? undefined : JSON.stringify(body));
  });
}

function parseStatus(text: string): KubeStatus | undefined {
  try {
    const status = JSON.parse(text) as unknown;
    if (typeof status !== "object" || status === null) return undefined;
    const message = (status as { message?: unknown }).message;
    return typeof message === "string" ? (status as KubeStatus) : undefined;
  } catch {
    return undefined;
  }
}
