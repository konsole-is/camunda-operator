import http from "node:http";
import type { AddressInfo } from "node:net";
import { KubeConfig } from "@kubernetes/client-node";
import { afterAll, beforeAll, beforeEach, describe, expect, it } from "vitest";
import { requestJSON } from "./client";

type Seen = {
  method?: string;
  url?: string;
  contentType?: string;
  body?: unknown;
};

let server: http.Server;
let kc: KubeConfig;
let seen: Seen;
let answer: { status: number; body: string };

beforeAll(async () => {
  server = http.createServer((request, response) => {
    const chunks: Buffer[] = [];
    request.on("data", (chunk: Buffer) => chunks.push(chunk));
    request.on("end", () => {
      const text = Buffer.concat(chunks).toString("utf8");
      seen = {
        method: request.method,
        url: request.url,
        contentType: request.headers["content-type"],
        body: text === "" ? undefined : JSON.parse(text),
      };
      response.writeHead(answer.status, {
        "Content-Type": "application/json",
      });
      response.end(answer.body);
    });
  });
  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
  const { port } = server.address() as AddressInfo;
  kc = new KubeConfig();
  kc.loadFromOptions({
    clusters: [
      { name: "c", server: `http://127.0.0.1:${port}`, skipTLSVerify: true },
    ],
    users: [{ name: "u" }],
    contexts: [{ name: "x", cluster: "c", user: "u" }],
    currentContext: "x",
  });
});

afterAll(async () => {
  await new Promise((resolve) => server.close(resolve));
});

beforeEach(() => {
  seen = {};
  answer = { status: 200, body: '{"ok":true}' };
});

describe("requestJSON", () => {
  it("sends the method, the body as JSON, and the content type", async () => {
    const result = await requestJSON(kc, {
      method: "PATCH",
      path: "/apis/fleet.camunda.io/v1alpha1/environments/prod?fieldManager=fleet-manager",
      body: { kind: "Environment" },
      contentType: "application/apply-patch+yaml",
    });

    expect(result).toEqual({ ok: true });
    expect(seen).toEqual({
      method: "PATCH",
      url: "/apis/fleet.camunda.io/v1alpha1/environments/prod?fieldManager=fleet-manager",
      contentType: "application/apply-patch+yaml",
      body: { kind: "Environment" },
    });
  });

  it("sends a body as application/json by default and no body when none is given", async () => {
    await requestJSON(kc, { method: "POST", path: "/x", body: { a: 1 } });
    expect(seen.contentType).toBe("application/json");

    await requestJSON(kc, { method: "GET", path: "/x" });
    expect(seen).toMatchObject({ contentType: undefined, body: undefined });
  });

  it("sends the body of a DELETE", async () => {
    await requestJSON(kc, {
      method: "DELETE",
      path: "/x",
      body: { preconditions: { resourceVersion: "7" } },
    });
    expect(seen.body).toEqual({ preconditions: { resourceVersion: "7" } });
  });

  it("resolves an empty answer to undefined", async () => {
    answer = { status: 200, body: "" };
    await expect(
      requestJSON(kc, { method: "DELETE", path: "/x" }),
    ).resolves.toBeUndefined();
  });

  it("rejects with the Status of an answer outside 2xx", async () => {
    const status = {
      kind: "Status",
      code: 409,
      reason: "Conflict",
      message:
        'Apply failed with 1 conflict: conflict with "kubectl": .spec.displayName',
      details: {
        causes: [
          {
            reason: "FieldManagerConflict",
            message: 'conflict with "kubectl"',
            field: ".spec.displayName",
          },
        ],
      },
    };
    answer = { status: 409, body: JSON.stringify(status) };

    await expect(
      requestJSON(kc, { method: "PATCH", path: "/x", body: {} }),
    ).rejects.toMatchObject({
      statusCode: 409,
      message:
        'Apply failed with 1 conflict: conflict with "kubectl": .spec.displayName',
      status: {
        reason: "Conflict",
        details: {
          causes: [
            { reason: "FieldManagerConflict", field: ".spec.displayName" },
          ],
        },
      },
    });
  });

  it("rejects with the HTTP status when the answer holds no Status", async () => {
    answer = { status: 503, body: "upstream down" };
    await expect(
      requestJSON(kc, { method: "GET", path: "/x" }),
    ).rejects.toMatchObject({ statusCode: 503, message: "HTTP 503" });
  });
});
