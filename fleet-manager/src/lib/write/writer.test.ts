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
  CREATE_ONLY_RESOURCE_VERSION,
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

describe("directWriter when the object of a create appears after the dry run", () => {
  // The object is missing at the read and the dry run. Before the write,
  // another client creates it, so the real apply answers 409.
  let race: http.Server;
  let raceKc: KubeConfig;
  let bodies: unknown[];
  let conflict: object;

  beforeAll(async () => {
    race = http.createServer((request, response) => {
      const chunks: Buffer[] = [];
      request.on("data", (chunk: Buffer) => chunks.push(chunk));
      request.on("end", () => {
        const json = (status: number, body: unknown) => {
          response.writeHead(status, { "Content-Type": "application/json" });
          response.end(JSON.stringify(body));
        };
        const body = Buffer.concat(chunks).toString("utf8");
        if (request.method === "GET")
          return json(404, {
            kind: "Status",
            apiVersion: "v1",
            status: "Failure",
            reason: "NotFound",
            code: 404,
          });
        if (request.url?.includes("dryRun=All"))
          return json(200, JSON.parse(body));
        bodies.push(JSON.parse(body));
        return json(409, conflict);
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
    bodies = [];
    conflict = {
      kind: "Status",
      apiVersion: "v1",
      status: "Failure",
      reason: "Conflict",
      code: 409,
      message: "the object has been modified",
    };
  });

  const creates = [
    {
      apiVersion: "core.camunda.io/v1",
      kind: "ObjectStorageConfig",
      metadata: { name: "archive", namespace: "team-a" },
      spec: { type: "S3", s3: { bucketName: "acme" } },
    },
    {
      apiVersion: "core.camunda.io/v1",
      kind: "CamundaPlatformConfig",
      metadata: { name: "basic" },
      spec: { auth: { method: "basic" } },
    },
  ];

  it.each(creates)(
    "fails with Stale and sends the create-only resourceVersion when a $kind appears after the dry run",
    async (object) => {
      const ref = {
        kind: object.kind,
        namespace: object.metadata.namespace,
        name: object.metadata.name,
      };
      const result = await directWriter(raceKc).apply({
        intent: "Create",
        apply: [object],
        create: [refKey(ref)],
      });

      expect(result).toEqual({
        ok: false,
        errors: [
          expect.objectContaining({
            code: "Stale",
            message: `The ${object.kind} ${object.metadata.name} exists already. Pick another name.`,
          }),
        ],
      });
      expect(bodies).toEqual([
        expect.objectContaining({
          metadata: {
            ...object.metadata,
            resourceVersion: CREATE_ONLY_RESOURCE_VERSION,
          },
        }),
      ]);
    },
  );

  it("gives Stale for a field manager conflict of a create too", async () => {
    conflict = {
      ...conflict,
      details: {
        causes: [
          {
            reason: "FieldManagerConflict",
            field: ".spec.s3.bucketName",
            message: 'conflict with "kubectl"',
          },
        ],
      },
    };

    const result = await directWriter(raceKc).apply({
      intent: "Create",
      apply: [creates[0]],
      create: ["ObjectStorageConfig/team-a/archive"],
    });

    expect(result).toEqual({
      ok: false,
      errors: [expect.objectContaining({ code: "Stale" })],
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

  describe("a preview that a field of another tool stops", () => {
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
    const live = environment({ displayName: "Stage", color: "red" }, [
      kubectlOwns("displayName"),
    ]);

    it("shows the change in blocked, and not in objects", async () => {
      answer = (r) =>
        r.method === "GET"
          ? { status: 200, body: live }
          : { status: 409, body: fieldConflict };

      const preview = await directWriter(apiConfig).preview({
        intent: "Rename",
        apply: [environment({ displayName: "Prod", color: "red" })],
        preconditions: { [key]: "7" },
      });

      expect(preview.objects).toEqual([]);
      expect(preview.errors).toEqual([
        expect.objectContaining({ code: "Conflict", path: "spec.displayName" }),
      ]);
      expect(preview.blocked).toHaveLength(1);
      const [blocked] = preview.blocked!;
      expect(blocked.ref).toEqual({
        apiVersion: "fleet.camunda.io/v1alpha1",
        kind: "Environment",
        name: "staging",
      });
      expect(blocked.before?.spec).toEqual({
        displayName: "Stage",
        color: "red",
      });
      expect(blocked.after?.spec).toEqual({
        displayName: "Prod",
        color: "red",
      });
      expect(blocked.diff).toEqual(
        expect.arrayContaining([
          { op: "removed", text: "  displayName: Stage" },
          { op: "added", text: "  displayName: Prod" },
        ]),
      );
      expect(blocked.after?.metadata.managedFields).toBeUndefined();
    });

    it("keeps in blocked a field that another tool set and the change leaves out, and drops one that only the fleet manager set", async () => {
      const owned = environment(
        { displayName: "Stage", color: "red", owner: "team-a" },
        [kubectlOwns("displayName", "owner"), fleetOwns("Apply", "color")],
      );
      answer = (r) =>
        r.method === "GET"
          ? { status: 200, body: owned }
          : { status: 409, body: fieldConflict };

      const preview = await directWriter(apiConfig).preview({
        intent: "Rename",
        apply: [environment({ displayName: "Prod" })],
        preconditions: { [key]: "7" },
      });

      expect(preview.blocked?.[0].after?.spec).toEqual({
        displayName: "Prod",
        owner: "team-a",
      });
    });

    it("sends no forced request", async () => {
      answer = (r) =>
        r.method === "GET"
          ? { status: 200, body: live }
          : { status: 409, body: fieldConflict };

      await directWriter(apiConfig).preview({
        intent: "Rename",
        apply: [environment({ displayName: "Prod", color: "red" })],
        preconditions: { [key]: "7" },
      });

      expect(seen.some((r) => r.url.includes("force=true"))).toBe(false);
      expect(seen.filter((r) => r.method === "PATCH")).toHaveLength(1);
    });

    it("leaves blocked unset when the object changed since the read", async () => {
      let reads = 0;
      answer = (r) => {
        if (r.method !== "GET") return { status: 409, body: fieldConflict };
        reads += 1;
        return {
          status: 200,
          body:
            reads === 1
              ? live
              : environment(
                  { displayName: "Stage", color: "red" },
                  [kubectlOwns("displayName")],
                  "8",
                ),
        };
      };

      const preview = await directWriter(apiConfig).preview({
        intent: "Rename",
        apply: [environment({ displayName: "Prod", color: "red" })],
        preconditions: { [key]: "7" },
      });

      expect(preview.errors).toEqual([
        expect.objectContaining({ code: "Stale" }),
      ]);
      expect(preview.blocked).toBeUndefined();
    });

    it("leaves blocked unset without a precondition, which gives no ownership", async () => {
      answer = (r) =>
        r.method === "GET"
          ? { status: 200, body: live }
          : { status: 409, body: fieldConflict };

      const preview = await directWriter(apiConfig).preview({
        intent: "Rename",
        apply: [environment({ displayName: "Prod", color: "red" })],
      });

      expect(preview.ownership).toBeUndefined();
      expect(preview.blocked).toBeUndefined();
    });
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

  // removalAnswers answers a save that removes a field: the GET with live,
  // the dry run and the update of the JSON patch with updated at the
  // resourceVersions "7" and "8", the dry-run apply after it with probe, and
  // the write of managedFields with record.
  const removalAnswers =
    (
      live: object,
      updated: { metadata: object },
      probe: object,
      overrides: {
        update?: { status: number; body: unknown };
        record?: { status: number; body: unknown };
      } = {},
    ) =>
    (r: Seen) => {
      if (r.method === "GET") return { status: 200, body: live };
      const dry = r.url.includes("dryRun=All");
      if (r.contentType === "application/apply-patch+yaml")
        return { status: 200, body: probe };
      if (dry) return { status: 200, body: updated };
      const ops = r.body as { path: string }[];
      if (ops.some((op) => op.path === "/metadata/managedFields"))
        return overrides.record ?? { status: 200, body: probe };
      return (
        overrides.update ?? {
          status: 200,
          body: {
            ...updated,
            metadata: { ...updated.metadata, resourceVersion: "8" },
          },
        }
      );
    };
  const fleetOwns = (operation: string, ...fields: string[]) => ({
    manager: "fleet-manager",
    operation,
    fieldsV1: {
      "f:spec": Object.fromEntries(fields.map((f) => [`f:${f}`, {}])),
    },
  });
  const clearName = {
    intent: "Clear the name",
    apply: [environment({ color: "red" })],
    preconditions: { [key]: "7" },
    remove: { [key]: ["spec.displayName"] },
  };
  const named = environment({ displayName: "Stage", color: "red" }, [
    kubectlOwns("displayName"),
  ]);
  const probed = environment(
    { color: "red" },
    [fleetOwns("Apply", "color"), fleetOwns("Update")],
    "8",
  );

  it("a save that removes a field of another tool sends one update at the resourceVersion of the review, then records the fields", async () => {
    answer = removalAnswers(named, environment({ color: "red" }), probed);

    const result = await directWriter(apiConfig).apply(clearName);

    expect(result).toEqual({ ok: true, mode: "direct" });
    const sent = writes().filter((r) => r.method !== "GET");
    expect(sent).toEqual([
      {
        method: "PATCH",
        url: "/apis/fleet.camunda.io/v1alpha1/environments/staging?fieldManager=fleet-manager",
        contentType: "application/json-patch+json",
        body: [
          { op: "replace", path: "/metadata/resourceVersion", value: "7" },
          { op: "add", path: "/spec", value: { color: "red" } },
        ],
      },
      {
        method: "PATCH",
        url: "/apis/fleet.camunda.io/v1alpha1/environments/staging?fieldManager=fleet-manager",
        contentType: "application/json-patch+json",
        body: [
          { op: "replace", path: "/metadata/resourceVersion", value: "8" },
          {
            op: "replace",
            path: "/metadata/managedFields",
            value: [fleetOwns("Apply", "color")],
          },
        ],
      },
    ]);
    const applies = seen.filter(
      (r) => r.contentType === "application/apply-patch+yaml",
    );
    expect(applies).toHaveLength(1);
    expect(applies[0].url).toContain("force=false&dryRun=All");
    expect(applies[0].body).toMatchObject({
      metadata: { resourceVersion: "8" },
      spec: { color: "red" },
    });
  });

  it("the preview of a removal is the dry run of the update that the save sends", async () => {
    const updated = environment({ color: "red" });
    answer = removalAnswers(named, updated, probed);

    const preview = await directWriter(apiConfig).preview(clearName);

    const patches = seen.filter((r) => r.method === "PATCH");
    expect(patches).toHaveLength(1);
    expect(patches[0].contentType).toBe("application/json-patch+json");
    expect(patches[0].url).toContain("dryRun=All");
    expect(preview.errors).toEqual([]);
    expect(preview.objects[0].after).toEqual({
      apiVersion: "fleet.camunda.io/v1alpha1",
      kind: "Environment",
      metadata: { name: "staging" },
      spec: { color: "red" },
    });
    const shown = patches[0].body;
    seen = [];
    await directWriter(apiConfig).apply(clearName);
    expect(writes().find((r) => r.method === "PATCH")?.body).toEqual(shown);
  });

  it("refuses an update that would take a field that the lists do not name", async () => {
    // The walk sees no change of spec.color, but the API server takes it.
    const live = environment({ displayName: "Stage", color: "red" }, [
      kubectlOwns("displayName", "color"),
    ]);
    answer = removalAnswers(
      live,
      environment({ color: "red" }, [fleetOwns("Update", "color")]),
      probed,
    );

    const result = await directWriter(apiConfig).apply(clearName);

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
    expect(writes().filter((r) => r.method !== "GET")).toEqual([]);
  });

  it("a removed field that its default brings back is not a take", async () => {
    const defaulted = environment({ displayName: "Default", color: "red" }, [
      fleetOwns("Update", "displayName"),
    ]);
    answer = removalAnswers(named, defaulted, {
      ...defaulted,
      metadata: { ...defaulted.metadata, resourceVersion: "8" },
    });

    const preview = await directWriter(apiConfig).preview(clearName);

    expect(preview.errors).toEqual([]);
    expect(preview.objects[0].after?.spec).toEqual({
      displayName: "Default",
      color: "red",
    });
  });

  it("refuses an update that would remove a field that the lists do not name", async () => {
    const operator =
      "CamundaClusterClaim/cluster/0b5c7a2e-1f0e-4f43-9d0a-6a0c2f1e9b11";
    const live = environment(
      { displayName: "Stage", allowed: [{ name: "x" }, { name: "y" }] },
      [
        kubectlOwns("displayName"),
        {
          manager: operator,
          operation: "Apply",
          fieldsV1: {
            "f:spec": {
              "f:allowed": { 'k:{"name":"y"}': { ".": {}, "f:name": {} } },
            },
          },
        },
      ],
    );
    answer = removalAnswers(
      live,
      environment({ allowed: [{ name: "x" }] }),
      probed,
    );

    const result = await directWriter(apiConfig).apply({
      ...clearName,
      apply: [environment({ allowed: [{ name: "x" }] })],
    });

    expect(result).toEqual({
      ok: false,
      errors: [
        expect.objectContaining({
          code: "Conflict",
          path: "spec.allowed[name=y]",
          manager: operator,
          message: `The save would remove spec.allowed[name=y] of ${operator}, and the review did not list it.`,
        }),
      ],
    });
    expect(writes().filter((r) => r.method !== "GET")).toEqual([]);
  });

  it("a newer write before the update fails with Stale and writes nothing", async () => {
    answer = removalAnswers(named, environment({ color: "red" }), probed, {
      update: { status: 409, body: conflict },
    });

    const result = await directWriter(apiConfig).apply(clearName);

    expect(result).toMatchObject({ ok: false, errors: [{ code: "Stale" }] });
    const [error] = (result as { errors: { message: string }[] }).errors;
    expect(error.message).not.toContain("did not record");
    expect(
      seen.filter((r) => r.contentType === "application/apply-patch+yaml"),
    ).toEqual([]);
    expect(writes().filter((r) => r.method !== "GET")).toHaveLength(1);
  });

  it("a newer write after the update fails with Stale and says that the next save records the fields", async () => {
    answer = removalAnswers(named, environment({ color: "red" }), probed, {
      record: { status: 409, body: conflict },
    });

    const result = await directWriter(apiConfig).apply(clearName);

    expect(result).toMatchObject({ ok: false, errors: [{ code: "Stale" }] });
    const [error] = (result as { errors: { message: string }[] }).errors;
    expect(error.message).toMatch(
      /^The fleet manager saved the Environment staging, but did not record its fields\. The next save of the Environment records them\. /,
    );
  });

  // movingAnswers answers a removal save whose object a status write moves
  // after the update. reads gives the object of each GET after the update,
  // and record answers each write of managedFields.
  const movingAnswers = (
    reads: object[],
    record: (resourceVersion: string) => { status: number; body: unknown },
  ) => {
    let updated = false;
    let read = 0;
    return (r: Seen) => {
      if (r.method === "GET") {
        if (!updated) return { status: 200, body: named };
        return { status: 200, body: reads[Math.min(read++, reads.length - 1)] };
      }
      const body = r.body as {
        path: string;
        value?: unknown;
        metadata?: { resourceVersion: string };
      }[] & { metadata?: { resourceVersion: string } };
      if (r.contentType === "application/apply-patch+yaml") {
        const version = (r.body as { metadata: { resourceVersion: string } })
          .metadata.resourceVersion;
        return {
          status: 200,
          body: {
            ...probed,
            metadata: { ...probed.metadata, resourceVersion: version },
          },
        };
      }
      if (r.url.includes("dryRun=All"))
        return { status: 200, body: environment({ color: "red" }) };
      const version = body.find((op) => op.path === "/metadata/resourceVersion")
        ?.value as string;
      if (body.some((op) => op.path === "/metadata/managedFields"))
        return record(version);
      updated = true;
      return { status: 200, body: environment({ color: "red" }, [], "8") };
    };
  };
  const statusOnly = (resourceVersion: string) => ({
    ...environment({ color: "red" }, [], resourceVersion),
    status: { observedGeneration: 2 },
  });
  const records = () =>
    writes().filter(
      (r) =>
        r.method === "PATCH" &&
        (r.body as { path: string }[]).some(
          (op) => op.path === "/metadata/managedFields",
        ),
    );

  it("a status write between the update and the record does not stop the save", async () => {
    answer = movingAnswers([statusOnly("9")], (version) =>
      version === "8"
        ? { status: 409, body: conflict }
        : { status: 200, body: statusOnly(version) },
    );

    const result = await directWriter(apiConfig).apply(clearName);

    expect(result).toEqual({ ok: true, mode: "direct" });
    expect(
      records().map(
        (r) => (r.body as { path: string; value: unknown }[])[0].value,
      ),
    ).toEqual(["8", "9"]);
  });

  it("a spec write between the update and the record fails with Stale and records nothing again", async () => {
    answer = movingAnswers([environment({ color: "blue" }, [], "9")], () => ({
      status: 409,
      body: conflict,
    }));

    const result = await directWriter(apiConfig).apply(clearName);

    expect(result).toMatchObject({ ok: false, errors: [{ code: "Stale" }] });
    const [error] = (result as { errors: { message: string }[] }).errors;
    expect(error.message).toMatch(
      /^The fleet manager saved the Environment staging, but did not record its fields\. /,
    );
    expect(records()).toHaveLength(1);
  });

  it("a record that newer status writes keep refusing stops after three tries", async () => {
    answer = movingAnswers(
      [statusOnly("9"), statusOnly("10"), statusOnly("11"), statusOnly("12")],
      () => ({ status: 409, body: conflict }),
    );

    const result = await directWriter(apiConfig).apply(clearName);

    expect(result).toMatchObject({ ok: false, errors: [{ code: "Stale" }] });
    const [error] = (result as { errors: { message: string }[] }).errors;
    expect(error.message).toMatch(
      /^The fleet manager saved the Environment staging, but did not record its fields\. /,
    );
    expect(records()).toHaveLength(3);
  });

  it("the record keeps the entries of other managers as the object holds them", async () => {
    const kubectl = kubectlOwns("color");
    const written = environment(
      { color: "red" },
      [kubectl, fleetOwns("Update", "color")],
      "8",
    );
    answer = removalAnswers(
      named,
      written,
      {
        ...probed,
        metadata: {
          ...probed.metadata,
          managedFields: [
            kubectlOwns("other"),
            fleetOwns("Apply", "color"),
            fleetOwns("Update", "color"),
          ],
        },
      },
      { update: { status: 200, body: written } },
    );

    const result = await directWriter(apiConfig).apply(clearName);

    expect(result).toEqual({ ok: true, mode: "direct" });
    expect((records()[0].body as { value: unknown }[])[1].value).toEqual([
      kubectl,
      fleetOwns("Apply", "color"),
    ]);
  });

  it("stops before the record when the apply after the update sets other values", async () => {
    answer = removalAnswers(
      named,
      environment({ color: "red" }),
      environment({ color: "blue" }, [], "8"),
    );

    const result = await directWriter(apiConfig).apply(clearName);

    expect(result).toEqual({
      ok: false,
      errors: [
        expect.objectContaining({
          code: "Invalid",
          message:
            "The fleet manager saved the Environment staging, but did not record its fields. The next save of the Environment records them. An apply of the change sets other values than the update.",
        }),
      ],
    });
    const ops = writes()
      .filter((r) => r.method === "PATCH")
      .map((r) => r.body as { path: string }[]);
    expect(
      ops.some((o) => o.some((op) => op.path === "/metadata/managedFields")),
    ).toBe(false);
  });

  it("a save of an object with an Update entry of the fleet manager goes through the update and drops that entry", async () => {
    const live = environment({ color: "red" }, [fleetOwns("Update", "color")]);
    answer = removalAnswers(
      live,
      live,
      environment(
        { color: "red" },
        [fleetOwns("Update", "color"), fleetOwns("Apply", "color")],
        "8",
      ),
    );

    const result = await directWriter(apiConfig).apply({
      intent: "Save",
      apply: [environment({ color: "red" })],
      preconditions: { [key]: "7" },
    });

    expect(result).toEqual({ ok: true, mode: "direct" });
    const sent = writes().filter((r) => r.method !== "GET");
    expect(sent.map((r) => r.contentType)).toEqual([
      "application/json-patch+json",
      "application/json-patch+json",
    ]);
    expect(sent[1].body).toEqual([
      { op: "replace", path: "/metadata/resourceVersion", value: "8" },
      {
        op: "replace",
        path: "/metadata/managedFields",
        value: [fleetOwns("Apply", "color")],
      },
    ]);
  });

  it("a preview with the hints of the Git writer dry-runs an apply, also for an object with an Update entry of the fleet manager", async () => {
    const live = environment({ color: "red" }, [fleetOwns("Update", "color")]);
    answer = (r) => ({
      status: 200,
      body: r.method === "GET" ? live : environment({ color: "blue" }),
    });

    const preview = await directWriter(apiConfig).preview({
      intent: "Recolor",
      apply: [environment({ color: "blue" })],
      preconditions: { [key]: "7" },
      force: true,
      previewManagers: {},
      previewMergePatches: [],
    });

    expect(preview.errors).toEqual([]);
    const patches = seen.filter((r) => r.method === "PATCH");
    expect(patches.map((r) => r.contentType)).toEqual([
      "application/apply-patch+yaml",
    ]);
    expect(patches[0].url).toContain("force=true&dryRun=All");
  });

  it("a change without lists sends no forced apply and no update", async () => {
    const live = environment({ color: "red" }, [fleetOwns("Apply", "color")]);
    answer = (r) => ({
      status: 200,
      body: r.method === "GET" ? live : environment({ color: "blue" }),
    });

    const result = await directWriter(apiConfig).apply({
      intent: "Recolor",
      apply: [environment({ color: "blue" })],
      preconditions: { [key]: "7" },
    });

    expect(result).toEqual({ ok: true, mode: "direct" });
    const sent = writes().filter((r) => r.method !== "GET");
    expect(sent).toHaveLength(1);
    expect(sent[0].contentType).toBe("application/apply-patch+yaml");
    expect(sent[0].url).toContain("force=false");
    expect(
      seen.some((r) => r.contentType === "application/json-patch+json"),
    ).toBe(false);
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

  it.each([
    ["a take-over of", "takeOver", "spec.displayName", "spec.displayName"],
    ["a removal at", "remove", "spec.sections", "spec.sections"],
    ["a removal above", "remove", "spec.sections", "spec.sections.gone"],
  ])("refuses %s a kept path before any write", async (_, list, path, kept) => {
    const live = environment(
      { displayName: "Stage", sections: { gone: { required: true } } },
      [kubectlOwns("displayName", "sections")],
    );
    answer = () => ({ status: 200, body: live });

    const result = await directWriter(apiConfig).apply({
      intent: "Rename",
      // The take-over row sends sections, so only keep makes its list wrong.
      apply: [
        environment(
          list === "takeOver"
            ? { displayName: "Prod", sections: { gone: { required: true } } }
            : { displayName: "Stage" },
        ),
      ],
      preconditions: { [key]: "7" },
      [list]: { [key]: [path] },
      keep: { [key]: [kept] },
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
