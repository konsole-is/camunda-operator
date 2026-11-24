import { randomUUID } from "node:crypto";
import { KubeConfig } from "@kubernetes/client-node";
import { describe, expect, inject, it } from "vitest";
import { KubernetesRequestError, requestJSON } from "@/lib/k8s/client";
import { APP_FIELD_MANAGER, type ManagedFieldsEntry } from "@/lib/merge";
import type { KubeObject } from "@/lib/write/types";

// Each case pins one fact of the API server that the direct writer relies
// on when a save removes a field of another tool.

const kc = new KubeConfig();
kc.loadFromFile(inject("kubeconfig"));

const JSON_PATCH = "application/json-patch+json";
const APPLY_PATCH = "application/apply-patch+yaml";
const KUBECTL = "kubectl-client-side-apply";
const collection = "/apis/core.camunda.io/v1/camundaplatformconfigs";

const oidc = {
  issuerUrl: "https://login.example.com/realms/camunda",
  clientId: "orchestration",
  clientSecretRef: {
    name: "oidc-credentials",
    namespace: "camunda-system",
    key: "client-secret",
  },
};

const send = (
  method: "GET" | "POST" | "PATCH",
  path: string,
  body?: unknown,
  contentType?: string,
) =>
  requestJSON<KubeObject>(kc, {
    method,
    path,
    body,
    contentType,
    timeoutMs: 15_000,
  });

const path = (object: KubeObject) => `${collection}/${object.metadata.name}`;

// make creates a platform config with OIDC sign-in as kubectl apply without
// --server-side does: every field in an Update entry of kubectl.
const make = (prefix: string) =>
  send("POST", `${collection}?fieldManager=${KUBECTL}`, {
    apiVersion: "core.camunda.io/v1",
    kind: "CamundaPlatformConfig",
    metadata: { name: `${prefix}-${randomUUID().slice(0, 8)}` },
    spec: { auth: { method: "oidc", oidc } },
  });

const toBasic = (object: KubeObject, resourceVersion: string) =>
  send(
    "PATCH",
    `${path(object)}?fieldManager=${APP_FIELD_MANAGER}`,
    [
      {
        op: "replace",
        path: "/metadata/resourceVersion",
        value: resourceVersion,
      },
      { op: "add", path: "/spec", value: { auth: { method: "basic" } } },
    ],
    JSON_PATCH,
  );

const applyBasic = (
  object: KubeObject,
  query: string,
  auth: object = { method: "basic" },
) =>
  send(
    "PATCH",
    `${path(object)}?fieldManager=${APP_FIELD_MANAGER}&${query}`,
    {
      apiVersion: "core.camunda.io/v1",
      kind: "CamundaPlatformConfig",
      metadata: {
        name: object.metadata.name,
        resourceVersion: object.metadata.resourceVersion,
      },
      spec: { auth },
    },
    APPLY_PATCH,
  );

const entries = (object: KubeObject) =>
  (object.metadata.managedFields ?? []) as ManagedFieldsEntry[];

const isUpdateOfApp = (entry: ManagedFieldsEntry) =>
  entry.manager === APP_FIELD_MANAGER && entry.operation === "Update";

// owns reports whether an entry of manager owns the field at keys of
// FieldsV1, such as ["f:spec", "f:auth"].
const owns = (
  object: KubeObject,
  manager: string,
  keys: string[],
  operation?: string,
) =>
  entries(object).some((entry) => {
    if (entry.manager !== manager) return false;
    if (operation !== undefined && entry.operation !== operation) return false;
    let at: unknown = entry.fieldsV1;
    for (const key of keys) {
      if (typeof at !== "object" || at === null || !(key in at)) return false;
      at = (at as Record<string, unknown>)[key];
    }
    return true;
  });

const method = ["f:spec", "f:auth", "f:method"];

describe("the API server and field managers", () => {
  it("an update gives each field that it changes to its field manager, and a removed field leaves every manager", async () => {
    const made = await make("update");

    const saved = await toBasic(made, made.metadata.resourceVersion!);

    expect(saved.spec).toEqual({ auth: { method: "basic" } });
    expect(owns(saved, APP_FIELD_MANAGER, method, "Update")).toBe(true);
    expect(owns(saved, KUBECTL, method)).toBe(false);
    expect(owns(saved, KUBECTL, ["f:spec", "f:auth", "f:oidc"])).toBe(false);
    expect(owns(saved, KUBECTL, ["f:spec", "f:auth"])).toBe(true);
  });

  it("an update at the resourceVersion of an older read fails with 409 and changes nothing", async () => {
    const made = await make("stale");
    const first = await toBasic(made, made.metadata.resourceVersion!);

    const second = toBasic(made, made.metadata.resourceVersion!);

    await expect(second).rejects.toBeInstanceOf(KubernetesRequestError);
    await expect(second).rejects.toMatchObject({ statusCode: 409 });
    expect((await send("GET", path(made))).metadata.resourceVersion).toBe(
      first.metadata.resourceVersion,
    );
  });

  it("an apply of values that the object holds changes no value, keeps the other managers, and gives the Apply entry the applied fields", async () => {
    const made = await make("apply");
    const saved = await toBasic(made, made.metadata.resourceVersion!);

    const probe = await applyBasic(saved, "force=false&dryRun=All");

    expect(probe.spec).toEqual(saved.spec);
    expect(owns(probe, APP_FIELD_MANAGER, method, "Apply")).toBe(true);
    expect(owns(probe, APP_FIELD_MANAGER, method, "Update")).toBe(true);
    expect(owns(probe, KUBECTL, ["f:spec", "f:auth"])).toBe(true);
  });

  it("an update that sends only managedFields changes no value and keeps the entries as sent", async () => {
    const made = await make("record");
    const saved = await toBasic(made, made.metadata.resourceVersion!);
    const probe = await applyBasic(saved, "force=false&dryRun=All");
    const kept = entries(probe).filter((e) => !isUpdateOfApp(e));

    const recorded = await send(
      "PATCH",
      `${path(saved)}?fieldManager=${APP_FIELD_MANAGER}`,
      [
        {
          op: "replace",
          path: "/metadata/resourceVersion",
          value: saved.metadata.resourceVersion,
        },
        { op: "replace", path: "/metadata/managedFields", value: kept },
      ],
      JSON_PATCH,
    );

    expect(recorded.spec).toEqual(saved.spec);
    expect(entries(recorded).some(isUpdateOfApp)).toBe(false);
    expect(
      entries(recorded).map((e) => [e.manager, e.operation, e.fieldsV1]),
    ).toEqual(kept.map((e) => [e.manager, e.operation, e.fieldsV1]));
  });

  it("an update that changes a value gives the field to its own manager, also when it sends managedFields", async () => {
    const made = await make("both");
    const handed = {
      manager: APP_FIELD_MANAGER,
      operation: "Apply",
      apiVersion: "core.camunda.io/v1",
      fieldsType: "FieldsV1",
      fieldsV1: { "f:spec": { "f:auth": { "f:method": {} } } },
    } as ManagedFieldsEntry;

    const answer = await send(
      "PATCH",
      `${path(made)}?fieldManager=${APP_FIELD_MANAGER}&dryRun=All`,
      [
        {
          op: "replace",
          path: "/metadata/resourceVersion",
          value: made.metadata.resourceVersion,
        },
        { op: "add", path: "/spec", value: { auth: { method: "basic" } } },
        {
          op: "replace",
          path: "/metadata/managedFields",
          value: [...entries(made), handed],
        },
      ],
      JSON_PATCH,
    );

    expect(owns(answer, APP_FIELD_MANAGER, method, "Update")).toBe(true);
    expect(owns(answer, APP_FIELD_MANAGER, method, "Apply")).toBe(false);
  });

  it("an apply with a null value does not remove a field that another manager set", async () => {
    const made = await make("null");

    const answer = applyBasic(made, "force=true&dryRun=All", {
      method: "basic",
      oidc: null,
    });

    await expect(answer).rejects.toMatchObject({
      statusCode: 422,
      message: expect.stringContaining("must not be set when method is basic"),
    });
  });
});
