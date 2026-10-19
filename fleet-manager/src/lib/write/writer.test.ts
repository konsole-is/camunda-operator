import http from "node:http";
import type { AddressInfo } from "node:net";
import { KubeConfig } from "@kubernetes/client-node";
import { afterAll, beforeAll, beforeEach, describe, expect, it } from "vitest";
import { objectPath } from "@/lib/k8s/kinds";
import {
  WriteOnlyKindMap,
  applyBody,
  directWriter,
  getWriter,
  refKey,
} from "./writer";

function configFor(server: string): KubeConfig {
  const kc = new KubeConfig();
  kc.loadFromOptions({
    clusters: [{ name: "c", server, skipTLSVerify: true }],
    users: [{ name: "u" }],
    contexts: [{ name: "x", cluster: "c", user: "u" }],
    currentContext: "x",
  });
  return kc;
}

let server: http.Server;
let kc: KubeConfig;
let closed: KubeConfig;
let requests: number;

beforeAll(async () => {
  server = http.createServer((_request, response) => {
    requests++;
    response.writeHead(500);
    response.end();
  });
  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
  kc = configFor(`http://127.0.0.1:${(server.address() as AddressInfo).port}`);

  const spare = http.createServer();
  await new Promise<void>((resolve) => spare.listen(0, "127.0.0.1", resolve));
  const port = (spare.address() as AddressInfo).port;
  await new Promise((resolve) => spare.close(resolve));
  closed = configFor(`http://127.0.0.1:${port}`);
});

afterAll(async () => {
  await new Promise((resolve) => server.close(resolve));
});

beforeEach(() => {
  requests = 0;
});

describe("refKey", () => {
  it("keeps an empty namespace for a cluster-scoped kind", () => {
    expect(refKey({ kind: "Environment", name: "prod" })).toBe(
      "Environment//prod",
    );
    expect(
      refKey({
        kind: "CamundaClusterClaim",
        namespace: "team-a",
        name: "orders",
      }),
    ).toBe("CamundaClusterClaim/team-a/orders");
  });
});

describe("applyBody", () => {
  const live = {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "Environment",
    metadata: {
      name: "prod",
      uid: "u",
      resourceVersion: "41",
      generation: 3,
      creationTimestamp: "2026-10-18T10:00:00Z",
      managedFields: [{ manager: "x" }],
      labels: { a: "b" },
    },
    spec: { displayName: "Prod" },
    status: { conditions: [] },
  };

  it("drops server fields and sets the resourceVersion of the precondition", () => {
    expect(applyBody(live, "42")).toEqual({
      apiVersion: "fleet.camunda.io/v1alpha1",
      kind: "Environment",
      metadata: { name: "prod", resourceVersion: "42", labels: { a: "b" } },
      spec: { displayName: "Prod" },
    });
  });

  it("sends no resourceVersion without a precondition", () => {
    expect(applyBody(live).metadata).not.toHaveProperty("resourceVersion");
  });
});

describe("directWriter", () => {
  it("refuses a kind that the fleet manager does not write, without a request", async () => {
    const result = await directWriter(kc).apply({
      intent: "Write a Deployment",
      apply: [
        {
          apiVersion: "apps/v1",
          kind: "Deployment",
          metadata: { name: "x", namespace: "a" },
        },
      ],
    });

    expect(result).toMatchObject({ ok: false, errors: [{ code: "Invalid" }] });
    expect(requests).toBe(0);
  });

  it("refuses a wrong apiVersion and a wrong scope, without a request", async () => {
    const result = await directWriter(kc).apply({
      intent: "Write wrong objects",
      apply: [
        {
          apiVersion: "fleet.camunda.io/v1",
          kind: "Environment",
          metadata: { name: "prod" },
        },
        {
          apiVersion: "fleet.camunda.io/v1alpha1",
          kind: "Environment",
          metadata: { name: "prod", namespace: "team-a" },
        },
      ],
      delete: [{ apiVersion: "v1", kind: "ConfigMap", name: "settings" }],
    });

    expect(result).toMatchObject({
      ok: false,
      errors: [{ code: "Invalid" }, { code: "Invalid" }, { code: "Invalid" }],
    });
    expect(requests).toBe(0);
  });

  it.each([".", "..", ""])(
    "refuses the name and the namespace %j without a request",
    async (segment) => {
      const result = await directWriter(kc).apply({
        intent: "Write objects with path segments as names",
        apply: [
          {
            apiVersion: "v1",
            kind: "Secret",
            metadata: { name: segment, namespace: "team-a" },
          },
          {
            apiVersion: "v1",
            kind: "Secret",
            metadata: { name: "git", namespace: segment },
          },
        ],
        delete: [
          {
            apiVersion: "v1",
            kind: "Secret",
            namespace: "team-a",
            name: segment,
          },
        ],
      });

      expect(result).toMatchObject({
        ok: false,
        errors: [{ code: "Invalid" }, { code: "Invalid" }, { code: "Invalid" }],
      });
      expect(requests).toBe(0);
    },
  );

  it("returns Unavailable when the API server is not reachable", async () => {
    const result = await directWriter(closed).apply({
      intent: "Create the environment prod",
      apply: [
        {
          apiVersion: "fleet.camunda.io/v1alpha1",
          kind: "Environment",
          metadata: { name: "prod" },
          spec: {},
        },
      ],
    });

    expect(result).toMatchObject({
      ok: false,
      errors: [{ code: "Unavailable" }],
    });
  });
});

describe("directWriter when an object is deleted after the preview", () => {
  // The object has the uid "old" at the preview. Before the write, another
  // client deletes it, so the write creates a new object, with the uid "new".
  let race: http.Server;
  let raceKc: KubeConfig;
  let deletes: unknown[];
  let deleteStatus: number;

  beforeAll(async () => {
    race = http.createServer((request, response) => {
      const chunks: Buffer[] = [];
      request.on("data", (chunk: Buffer) => chunks.push(chunk));
      request.on("end", () => {
        const dryRun = request.url?.includes("dryRun=All") ?? false;
        const uid = request.method === "PATCH" && !dryRun ? "new" : "old";
        if (request.method === "DELETE") {
          deletes.push(JSON.parse(Buffer.concat(chunks).toString("utf8")));
          if (deleteStatus !== 200) {
            response.writeHead(deleteStatus, {
              "Content-Type": "application/json",
            });
            response.end(JSON.stringify({ message: "delete is forbidden" }));
            return;
          }
        }
        response.writeHead(200, { "Content-Type": "application/json" });
        response.end(
          JSON.stringify({
            apiVersion: "fleet.camunda.io/v1alpha1",
            kind: "Environment",
            metadata: { name: "prod", uid, resourceVersion: "9" },
            spec: {},
          }),
        );
      });
    });
    await new Promise<void>((resolve) => race.listen(0, "127.0.0.1", resolve));
    raceKc = configFor(
      `http://127.0.0.1:${(race.address() as AddressInfo).port}`,
    );
  });

  afterAll(async () => {
    await new Promise((resolve) => race.close(resolve));
  });

  beforeEach(() => {
    deletes = [];
    deleteStatus = 200;
  });

  it("an apply with a precondition fails with Stale and deletes the object that it created", async () => {
    const result = await directWriter(raceKc).apply({
      intent: "Rename from the old form",
      apply: [
        {
          apiVersion: "fleet.camunda.io/v1alpha1",
          kind: "Environment",
          metadata: { name: "prod" },
          spec: {},
        },
      ],
      preconditions: { "Environment//prod": "7" },
    });

    expect(result).toEqual({
      ok: false,
      errors: [expect.objectContaining({ code: "Stale" })],
    });
    expect(deletes).toEqual([
      expect.objectContaining({ preconditions: { uid: "new" } }),
    ]);
  });

  it("fails with Stale and the error of the delete when the app may not delete the object that it created", async () => {
    deleteStatus = 403;

    const result = await directWriter(raceKc).apply({
      intent: "Rename from the old form",
      apply: [
        {
          apiVersion: "fleet.camunda.io/v1alpha1",
          kind: "Environment",
          metadata: { name: "prod" },
          spec: {},
        },
      ],
      preconditions: { "Environment//prod": "7" },
    });

    expect(result).toEqual({
      ok: false,
      errors: [
        expect.objectContaining({ code: "Stale" }),
        expect.objectContaining({ code: "Forbidden" }),
      ],
    });
  });
});

describe("directWriter when a target of a delete is gone before the dry run", () => {
  // The GET still finds the object, but another client deletes it before
  // the dry-run DELETE, so every DELETE answers 404.
  let gone: http.Server;
  let goneKc: KubeConfig;

  beforeAll(async () => {
    gone = http.createServer((request, response) => {
      request.resume();
      request.on("end", () => {
        const found = request.method === "GET";
        response.writeHead(found ? 200 : 404, {
          "Content-Type": "application/json",
        });
        response.end(
          JSON.stringify(
            found
              ? {
                  apiVersion: "fleet.camunda.io/v1alpha1",
                  kind: "Environment",
                  metadata: { name: "prod", uid: "old" },
                }
              : { reason: "NotFound", message: "not found" },
          ),
        );
      });
    });
    await new Promise<void>((resolve) => gone.listen(0, "127.0.0.1", resolve));
    goneKc = configFor(
      `http://127.0.0.1:${(gone.address() as AddressInfo).port}`,
    );
  });

  afterAll(async () => {
    await new Promise((resolve) => gone.close(resolve));
  });

  it("the delete succeeds, also with a precondition", async () => {
    const ref = {
      apiVersion: "fleet.camunda.io/v1alpha1",
      kind: "Environment",
      name: "prod",
    };
    const writer = directWriter(goneKc);

    expect(
      await writer.apply({ intent: "Delete", apply: [], delete: [ref] }),
    ).toEqual({ ok: true, mode: "direct" });
    expect(
      await writer.apply({
        intent: "Delete",
        apply: [],
        delete: [ref],
        preconditions: { "Environment//prod": "7" },
      }),
    ).toEqual({ ok: true, mode: "direct" });
    expect(
      (await writer.preview({ intent: "Delete", apply: [], delete: [ref] }))
        .objects,
    ).toEqual([]);
  });
});

describe("WriteOnlyKindMap", () => {
  it("holds the namespaced core ConfigMap, and objectPath names one", () => {
    expect(WriteOnlyKindMap.ConfigMap).toEqual({
      group: "",
      version: "v1",
      plural: "configmaps",
      scope: "Namespaced",
    });
    expect(
      objectPath(WriteOnlyKindMap.ConfigMap, "fleet", "fleet-manager-settings"),
    ).toBe("/api/v1/namespaces/fleet/configmaps/fleet-manager-settings");
  });
});

describe("getWriter", () => {
  it("returns the direct writer", async () => {
    expect((await getWriter()).mode).toBe("direct");
  });
});
