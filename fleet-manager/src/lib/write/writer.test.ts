import http from "node:http";
import type { AddressInfo } from "node:net";
import { KubeConfig } from "@kubernetes/client-node";
import {
  afterAll,
  beforeAll,
  beforeEach,
  describe,
  expect,
  it,
  vi,
} from "vitest";
import { objectPath } from "@/lib/k8s/kinds";
import { SETTING_KEYS, type Settings } from "@/lib/settings/keys";
import { getSettings } from "@/lib/settings/store";
import {
  WriteOnlyKindMap,
  applyBody,
  directWriter,
  getWriter,
  inNewNamespaces,
  refKey,
  writeMode,
} from "./writer";
import type { ObjectRef } from "./types";

vi.mock("@/lib/settings/store", () => ({ getSettings: vi.fn() }));

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
      preconditions: { "Environment//prod": "9" },
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
      preconditions: { "Environment//prod": "9" },
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

describe("inNewNamespaces", () => {
  const ns = (name: string) => ({
    apiVersion: "v1",
    kind: "Namespace",
    metadata: { name },
  });
  const claim = (namespace: string) => ({
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "CamundaManagementClusterClaim",
    metadata: { name: "plane-a", namespace },
  });
  const reader = (existing: string[]) => async (ref: ObjectRef) =>
    ref.kind === "Namespace" && existing.includes(ref.name)
      ? ns(ref.name)
      : undefined;

  it("holds an object in a namespace that the change makes", async () => {
    const apply = [ns("platform"), claim("platform")];

    expect([...(await inNewNamespaces(apply, reader([])))]).toEqual([apply[1]]);
  });

  it("holds nothing when that namespace exists", async () => {
    expect(
      (
        await inNewNamespaces(
          [ns("platform"), claim("platform")],
          reader(["platform"]),
        )
      ).size,
    ).toBe(0);
  });

  it("holds nothing for an object in a namespace that the change does not list", async () => {
    expect((await inNewNamespaces([claim("platform")], reader([]))).size).toBe(
      0,
    );
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
  const settings = (values: Partial<Settings>): Settings => ({
    ...(Object.fromEntries(SETTING_KEYS.map((k) => [k, ""])) as Settings),
    ...values,
  });

  it("getWriter is the Git writer when git.repository is set, else the direct writer", async () => {
    vi.mocked(getSettings).mockResolvedValueOnce(
      settings({ "git.repository": "https://git.example.com/fleet.git" }),
    );
    expect((await getWriter()).mode).toBe("git");

    vi.mocked(getSettings).mockResolvedValueOnce(settings({}));
    expect((await getWriter()).mode).toBe("direct");
  });

  it("throws when the settings cannot be read", async () => {
    vi.mocked(getSettings).mockRejectedValueOnce(new Error("no settings"));
    await expect(getWriter()).rejects.toThrow("no settings");
  });

  it("writeMode is git when git.repository is set, else direct", async () => {
    vi.mocked(getSettings).mockResolvedValueOnce(
      settings({ "git.repository": "https://git.example.com/fleet.git" }),
    );
    expect(await writeMode()).toBe("git");

    vi.mocked(getSettings).mockResolvedValueOnce(settings({}));
    expect(await writeMode()).toBe("direct");
  });

  it("writeMode throws when the settings cannot be read", async () => {
    vi.mocked(getSettings).mockRejectedValueOnce(new Error("no settings"));
    await expect(writeMode()).rejects.toThrow("no settings");
  });
});

describe("directWriter with a take-over", () => {
  type Seen = {
    method: string;
    url: string;
    contentType?: string;
    body: unknown;
  };
  const uid = "0b5c7a2e-1f0e-4f43-9d0a-6a0c2f1e9b11";
  const key = "Environment//staging";
  let api: http.Server;
  let apiConfig: KubeConfig;
  let seen: Seen[];
  let answer: (request: Seen) => { status: number; body: unknown };

  const environment = (
    spec: object,
    managedFields: object[] = [],
    resourceVersion = "7",
  ) => ({
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "Environment",
    metadata: { name: "staging", uid: "u-1", resourceVersion, managedFields },
    spec,
  });
  const kubectlOwns = (...fields: string[]) => ({
    manager: "kubectl-client-side-apply",
    operation: "Update",
    fieldsV1: {
      "f:spec": Object.fromEntries(fields.map((f) => [`f:${f}`, {}])),
    },
  });
  const conflict = {
    kind: "Status",
    apiVersion: "v1",
    status: "Failure",
    reason: "Conflict",
    code: 409,
    message: "the object has been modified",
  };
  const writes = () => seen.filter((r) => !r.url.includes("dryRun=All"));

  beforeAll(async () => {
    api = http.createServer((request, response) => {
      const chunks: Buffer[] = [];
      request.on("data", (c: Buffer) => chunks.push(c));
      request.on("end", () => {
        const text = Buffer.concat(chunks).toString();
        const record: Seen = {
          method: request.method ?? "",
          url: request.url ?? "",
          contentType: request.headers["content-type"],
          body: text === "" ? undefined : JSON.parse(text),
        };
        seen.push(record);
        const { status, body } = answer(record);
        response.writeHead(status, { "Content-Type": "application/json" });
        response.end(JSON.stringify(body));
      });
    });
    await new Promise<void>((resolve) => api.listen(0, "127.0.0.1", resolve));
    apiConfig = configFor(
      `http://127.0.0.1:${(api.address() as AddressInfo).port}`,
    );
  });

  afterAll(async () => {
    await new Promise((resolve) => api.close(resolve));
  });

  beforeEach(() => {
    seen = [];
  });

  it("names the fields of other managers in the preview", async () => {
    const live = environment({ displayName: "Stage" }, [
      kubectlOwns("displayName"),
    ]);
    answer = (r) =>
      r.method === "GET"
        ? { status: 200, body: live }
        : {
            status: 409,
            body: {
              ...conflict,
              message: "Apply failed with 1 conflict",
              details: {
                causes: [
                  {
                    reason: "FieldManagerConflict",
                    message: 'conflict with "kubectl-client-side-apply"',
                    field: ".spec.displayName",
                  },
                ],
              },
            },
          };

    const preview = await directWriter(apiConfig).preview({
      intent: "Rename",
      apply: [environment({ displayName: "Prod" })],
      preconditions: { [key]: "7" },
    });

    expect(preview.ownership).toEqual([
      {
        ref: {
          apiVersion: "fleet.camunda.io/v1alpha1",
          kind: "Environment",
          name: "staging",
        },
        changed: [
          {
            path: "spec.displayName",
            steps: [{ field: "spec" }, { field: "displayName" }],
            managers: [
              {
                name: "kubectl-client-side-apply",
                operation: "Update",
                kind: "other",
                known: true,
              },
            ],
          },
        ],
        cleared: [],
      },
    ]);
    expect(preview.errors).toEqual([
      expect.objectContaining({ code: "Conflict", path: "spec.displayName" }),
    ]);
  });

  it("refuses a forced apply that would take a field the list does not name", async () => {
    // The walk sees no change of spec.color, but the API server takes it.
    const live = environment({ displayName: "Stage", color: "red" }, [
      kubectlOwns("displayName", "color"),
    ]);
    answer = (r) =>
      r.method === "GET"
        ? { status: 200, body: live }
        : {
            status: 200,
            body: environment({ displayName: "Prod", color: "red" }, [
              {
                manager: "fleet-manager",
                operation: "Apply",
                fieldsV1: { "f:spec": { "f:displayName": {}, "f:color": {} } },
              },
            ]),
          };

    const result = await directWriter(apiConfig).apply({
      intent: "Rename",
      apply: [environment({ displayName: "Prod", color: "red" })],
      preconditions: { [key]: "7" },
      takeOver: { [key]: ["spec.displayName"] },
    });

    expect(result).toEqual({
      ok: false,
      errors: [
        expect.objectContaining({
          code: "Conflict",
          path: "spec.color",
          manager: "kubectl-client-side-apply",
          message:
            "The save would take spec.color from kubectl-client-side-apply, and the review did not list it.",
        }),
      ],
    });
    expect(seen.find((r) => r.method === "PATCH")?.url).toContain(
      "force=true&dryRun=All",
    );
    expect(writes().filter((r) => r.method !== "GET")).toEqual([]);
  });

  it("a removal after a newer write fails with Stale and names the fields it did not remove", async () => {
    const live = environment({ displayName: "Stage", color: "red" }, [
      kubectlOwns("displayName"),
    ]);
    const kept = environment({ displayName: "Stage", color: "blue" }, [
      kubectlOwns("displayName"),
    ]);
    answer = (r) => {
      if (r.method === "GET") return { status: 200, body: live };
      const dry = r.url.includes("dryRun=All");
      if (r.contentType === "application/apply-patch+yaml")
        return {
          status: 200,
          body: dry
            ? kept
            : { ...kept, metadata: { ...kept.metadata, resourceVersion: "8" } },
        };
      return dry
        ? { status: 200, body: environment({ color: "blue" }) }
        : { status: 409, body: conflict };
    };

    const result = await directWriter(apiConfig).apply({
      intent: "Clear the name",
      apply: [environment({ color: "blue" })],
      preconditions: { [key]: "7" },
      remove: { [key]: ["spec.displayName"] },
    });

    expect(result).toMatchObject({ ok: false, errors: [{ code: "Stale" }] });
    const [error] = (result as { errors: { message: string }[] }).errors;
    expect(error.message).toMatch(
      /^The fleet manager saved the Environment staging, but did not remove spec\.displayName\. /,
    );
    const applied = writes().find(
      (r) => r.contentType === "application/apply-patch+yaml",
    )!;
    expect(applied.url).toContain("force=false");
    expect(applied.body).toMatchObject({ metadata: { resourceVersion: "7" } });
    const removal = writes().at(-1)!;
    expect(removal).toMatchObject({
      method: "PATCH",
      contentType: "application/json-patch+json",
      body: [
        { op: "replace", path: "/metadata/resourceVersion", value: "8" },
        { op: "add", path: "/spec", value: { color: "blue" } },
      ],
    });
    expect(removal.url).toContain("fieldManager=fleet-manager");
  });

  it("applies a take-over once with force at the resourceVersion of the review", async () => {
    const live = environment({ displayName: "Stage" }, [
      kubectlOwns("displayName"),
    ]);
    const taken = environment({ displayName: "Prod" }, [
      {
        manager: "fleet-manager",
        operation: "Apply",
        fieldsV1: { "f:spec": { "f:displayName": {} } },
      },
    ]);
    answer = (r) => ({ status: 200, body: r.method === "GET" ? live : taken });

    const result = await directWriter(apiConfig).apply({
      intent: "Rename",
      apply: [environment({ displayName: "Prod" })],
      preconditions: { [key]: "7" },
      takeOver: { [key]: ["spec.displayName"] },
    });

    expect(result).toEqual({ ok: true, mode: "direct" });
    const applies = writes().filter((r) => r.method === "PATCH");
    expect(applies).toHaveLength(1);
    expect(applies[0].url).toContain("force=true");
    expect(applies[0].body).toMatchObject({
      metadata: { resourceVersion: "7" },
    });
  });

  it("refuses a listed path that names two fields", async () => {
    const live = environment({ parameters: { "a.b": 1, a: { b: 2 } } }, [
      {
        manager: "kubectl",
        operation: "Apply",
        fieldsV1: {
          "f:spec": { "f:parameters": { "f:a.b": {}, "f:a": { "f:b": {} } } },
        },
      },
    ]);
    answer = () => ({ status: 200, body: live });

    const result = await directWriter(apiConfig).apply({
      intent: "Change both",
      apply: [environment({ parameters: { "a.b": 3, a: { b: 4 } } })],
      preconditions: { [key]: "7" },
      takeOver: { [key]: ["spec.parameters.a.b"] },
    });

    expect(result).toEqual({
      ok: false,
      errors: [
        expect.objectContaining({
          code: "Invalid",
          path: "spec.parameters.a.b",
          message:
            "spec.parameters.a.b names more than one field of the Environment staging. The fleet manager does not take over a field that it cannot name exactly.",
        }),
      ],
    });
    expect(seen.filter((r) => r.method !== "GET")).toEqual([]);
  });

  it("refuses lists that differ from the fields of other managers", async () => {
    const live = environment({ displayName: "Stage" }, [
      kubectlOwns("displayName"),
    ]);
    answer = () => ({ status: 200, body: live });

    const result = await directWriter(apiConfig).apply({
      intent: "Rename",
      apply: [environment({ displayName: "Prod" })],
      preconditions: { [key]: "7" },
      takeOver: { [key]: ["spec.color"] },
    });

    expect(result).toEqual({
      ok: false,
      errors: [
        expect.objectContaining({
          code: "Invalid",
          message:
            "The fields of other tools are not the ones that the review showed. Review the change again.",
        }),
      ],
    });
    expect(seen.filter((r) => r.method !== "GET")).toEqual([]);
  });

  it("refuses a listed field of an operator controller before any write", async () => {
    const manager = `CamundaClusterClaim/cluster/${uid}`;
    const live = environment({ displayName: "Stage" }, [
      { ...kubectlOwns("displayName"), manager, operation: "Apply" },
    ]);
    answer = () => ({ status: 200, body: live });

    const result = await directWriter(apiConfig).apply({
      intent: "Rename",
      apply: [environment({ displayName: "Prod" })],
      preconditions: { [key]: "7" },
      takeOver: { [key]: ["spec.displayName"] },
    });

    expect(result).toEqual({
      ok: false,
      errors: [
        expect.objectContaining({
          code: "Conflict",
          path: "spec.displayName",
          manager,
          message: `spec.displayName belongs to ${manager}, a controller of the operator. The fleet manager does not take it over.`,
        }),
      ],
    });
    expect(seen.filter((r) => r.method !== "GET")).toEqual([]);
  });

  it("refuses a take-over of an object that changed since the review with Stale", async () => {
    const live = environment(
      { displayName: "Stage" },
      [kubectlOwns("displayName")],
      "9",
    );
    answer = () => ({ status: 200, body: live });

    const result = await directWriter(apiConfig).preview({
      intent: "Rename",
      apply: [environment({ displayName: "Prod" })],
      preconditions: { [key]: "7" },
      takeOver: { [key]: ["spec.displayName"] },
    });

    expect(result.errors).toEqual([expect.objectContaining({ code: "Stale" })]);
    expect(seen.filter((r) => r.method !== "GET")).toEqual([]);
  });

  it("refuses a listed path that also names a field of an operator controller", async () => {
    // spec.a.b names both the field b of a and the field "a.b".
    const manager = `CamundaClusterClaim/cluster/${uid}`;
    const live = environment({ a: { b: 1 }, "a.b": 1 }, [
      {
        manager: "kubectl-client-side-apply",
        operation: "Update",
        fieldsV1: { "f:spec": { "f:a": { "f:b": {} } } },
      },
      {
        manager,
        operation: "Apply",
        fieldsV1: { "f:spec": { "f:a.b": {} } },
      },
    ]);
    answer = () => ({ status: 200, body: live });

    const result = await directWriter(apiConfig).apply({
      intent: "Change both",
      apply: [environment({ a: { b: 2 }, "a.b": 2 })],
      preconditions: { [key]: "7" },
      takeOver: { [key]: ["spec.a.b"] },
    });

    expect(result).toEqual({
      ok: false,
      errors: [
        expect.objectContaining({
          code: "Conflict",
          path: "spec.a.b",
          manager,
        }),
      ],
    });
    expect(seen.filter((r) => r.method !== "GET")).toEqual([]);
  });

  describe("when another write lands after the read", () => {
    const fieldConflict = {
      ...conflict,
      message: "Apply failed with 1 conflict",
      details: {
        causes: [
          {
            reason: "FieldManagerConflict",
            message: 'conflict with "kubectl-client-side-apply"',
            field: ".spec.displayName",
          },
        ],
      },
    };
    const live = environment({ displayName: "Stage" }, [
      kubectlOwns("displayName"),
    ]);
    const newer = environment(
      { displayName: "Stage" },
      [kubectlOwns("displayName")],
      "8",
    );

    it("a preview fails with Stale instead of Conflict", async () => {
      let reads = 0;
      answer = (r) => {
        if (r.method !== "GET") return { status: 409, body: fieldConflict };
        reads += 1;
        return { status: 200, body: reads === 1 ? live : newer };
      };

      const preview = await directWriter(apiConfig).preview({
        intent: "Rename",
        apply: [environment({ displayName: "Prod" })],
        preconditions: { [key]: "7" },
      });

      expect(preview.errors).toEqual([
        expect.objectContaining({ code: "Stale" }),
      ]);
    });

    it("a save fails with Stale instead of Conflict", async () => {
      let written = false;
      answer = (r) => {
        if (r.method === "GET")
          return { status: 200, body: written ? newer : live };
        if (r.url.includes("dryRun=All")) return { status: 200, body: live };
        written = true;
        return { status: 409, body: fieldConflict };
      };

      const result = await directWriter(apiConfig).apply({
        intent: "Rename",
        apply: [environment({ displayName: "Prod" })],
        preconditions: { [key]: "7" },
      });

      expect(result).toEqual({
        ok: false,
        errors: [expect.objectContaining({ code: "Stale" })],
      });
    });

    it("keeps the Conflict and names a failed read of the object after it", async () => {
      let reads = 0;
      answer = (r) => {
        if (r.method !== "GET") return { status: 409, body: fieldConflict };
        reads += 1;
        return reads === 1
          ? { status: 200, body: live }
          : {
              status: 403,
              body: {
                ...conflict,
                reason: "Forbidden",
                code: 403,
                message: "environments is forbidden",
              },
            };
      };

      const preview = await directWriter(apiConfig).preview({
        intent: "Rename",
        apply: [environment({ displayName: "Prod" })],
        preconditions: { [key]: "7" },
      });

      expect(preview.errors).toEqual([
        expect.objectContaining({
          code: "Conflict",
          path: "spec.displayName",
          manager: "kubectl-client-side-apply",
        }),
        expect.objectContaining({
          code: "Forbidden",
          message: expect.stringMatching(
            /^The fleet manager could not read the Environment staging again, so it cannot tell if another write changed it\. /,
          ),
        }),
      ]);
    });
  });
});
