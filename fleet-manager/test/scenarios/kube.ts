// Playwright loads this module, so it must not import src/lib/k8s/client.ts,
// which imports server-only. That package throws outside the react-server
// condition of Next.js.
import http from "node:http";
import https from "node:https";
import type { KubeConfig } from "@kubernetes/client-node";

/** KubeError is a request that the API server answered outside 2xx. */
export class KubeError extends Error {
  constructor(
    readonly status: number,
    message: string,
  ) {
    super(message);
    this.name = "KubeError";
  }
}

export type KubeMethod = "GET" | "POST" | "PUT" | "PATCH" | "DELETE";

/**
 * kubeRequest sends one request to the API server of kc and returns the body
 * as plain JSON, or undefined for an empty body. It rejects with KubeError
 * outside 2xx, with the message of the Status that the API server sent. It
 * is a small copy of requestJSON of src/lib/k8s/client.ts, which the
 * Playwright tests cannot load.
 */
export async function kubeRequest<T>(
  kc: KubeConfig,
  method: KubeMethod,
  path: string,
  body?: unknown,
  contentType?: string,
): Promise<T> {
  const cluster = kc.getCurrentCluster();
  if (!cluster) throw new Error("The kubeconfig has no current cluster");
  const url = new URL(cluster.server + path);
  const payload = body === undefined ? undefined : JSON.stringify(body);
  const headers: Record<string, string> = { Accept: "application/json" };
  if (payload !== undefined) {
    headers["Content-Type"] = contentType ?? "application/json";
    // Node sends no body for a DELETE without a Content-Length.
    headers["Content-Length"] = String(Buffer.byteLength(payload));
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
          reject(new KubeError(code, statusMessage(text) ?? `HTTP ${code}`));
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
    request.end(payload);
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
