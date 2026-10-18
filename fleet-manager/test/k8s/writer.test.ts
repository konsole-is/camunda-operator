import { randomUUID } from "node:crypto";
import { KubeConfig, KubernetesObjectApi } from "@kubernetes/client-node";
import { beforeAll, beforeEach, describe, expect, inject, it } from "vitest";
import type { KubeObject } from "@/lib/write/types";
import { directWriter, refKey } from "@/lib/write/writer";

const kc = new KubeConfig();
kc.loadFromFile(inject("kubeconfig"));
const api = KubernetesObjectApi.makeApiClient(kc);
const writer = directWriter(kc);
const namespace = `write-${randomUUID().slice(0, 8)}`;

const environment = (name: string, spec: object): KubeObject => ({
  apiVersion: "fleet.camunda.io/v1alpha1",
  kind: "Environment",
  metadata: { name },
  spec,
});
const environmentRef = (name: string) => ({
  apiVersion: "fleet.camunda.io/v1alpha1",
  kind: "Environment",
  name,
});
const live = (name: string) =>
  api.read({
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "Environment",
    metadata: { name },
  });

let name: string;

beforeAll(async () => {
  await api.create({
    apiVersion: "v1",
    kind: "Namespace",
    metadata: { name: namespace },
  });
});

beforeEach(() => {
  name = `env-${randomUUID().slice(0, 8)}`;
});

describe("the direct writer against envtest", () => {
  it("an apply sets the field manager fleet-manager on the object", async () => {
    expect(
      await writer.apply({
        intent: "Create",
        apply: [environment(name, { displayName: "One" })],
      }),
    ).toEqual({ ok: true, mode: "direct" });

    const object = await live(name);
    expect(object.metadata?.managedFields).toContainEqual(
      expect.objectContaining({ manager: "fleet-manager", operation: "Apply" }),
    );
  });

  it("a preview of a change that a CEL rule refuses returns the API message and leaves the object unchanged", async () => {
    await writer.apply({
      intent: "Create",
      apply: [environment(name, { displayName: "One" })],
    });
    const before = await live(name);

    const preview = await writer.preview({
      intent: "Change the releases",
      apply: [
        environment(name, {
          displayName: "One",
          releases: { allowed: ["r1"], default: "r2" },
        }),
      ],
    });

    expect(preview.objects).toEqual([]);
    expect(preview.errors).toEqual([
      expect.objectContaining({
        code: "Invalid",
        message: expect.stringContaining(
          "releases.default must be one of releases.allowed",
        ),
      }),
    ]);
    const after = await live(name);
    expect(after.metadata?.resourceVersion).toBe(
      before.metadata?.resourceVersion,
    );
    expect(after).toMatchObject({ spec: { displayName: "One" } });
  });

  it("a preview returns the live object, the result, and the diff, and writes nothing", async () => {
    await writer.apply({
      intent: "Create",
      apply: [environment(name, { displayName: "One" })],
    });

    const preview = await writer.preview({
      intent: "Rename",
      apply: [environment(name, { displayName: "Two" })],
    });

    expect(preview.errors).toEqual([]);
    expect(preview.objects[0]).toMatchObject({
      ref: environmentRef(name),
      before: { spec: { displayName: "One" } },
      after: { spec: { displayName: "Two" } },
    });
    expect(preview.objects[0].diff.filter((l) => l.op !== "same")).toEqual([
      { op: "removed", text: "  displayName: One" },
      { op: "added", text: "  displayName: Two" },
    ]);
    expect(await live(name)).toMatchObject({
      spec: { displayName: "One" },
    });
  });

  it("an apply of a field that another manager owns fails with Conflict and the name of that manager", async () => {
    await api.create(
      environment(name, { displayName: "Theirs" }),
      undefined,
      undefined,
      "other-writer",
    );

    const result = await writer.apply({
      intent: "Rename",
      apply: [environment(name, { displayName: "Ours" })],
    });

    expect(result).toEqual({
      ok: false,
      errors: [
        expect.objectContaining({
          code: "Conflict",
          manager: "other-writer",
          path: "spec.displayName",
        }),
      ],
    });
    expect(await live(name)).toMatchObject({
      spec: { displayName: "Theirs" },
    });
  });

  it("a forced apply takes a field that another manager owns", async () => {
    await api.create(
      environment(name, { displayName: "Theirs" }),
      undefined,
      undefined,
      "other-writer",
    );

    const result = await writer.apply({
      intent: "Rename",
      apply: [environment(name, { displayName: "Ours" })],
      force: true,
    });

    expect(result).toEqual({ ok: true, mode: "direct" });
    expect(await live(name)).toMatchObject({
      spec: { displayName: "Ours" },
    });
  });

  it("a forced apply with the resourceVersion of an older copy still fails with Stale", async () => {
    await writer.apply({
      intent: "Create",
      apply: [environment(name, { displayName: "One" })],
    });
    const old = (await live(name)).metadata!.resourceVersion!;
    await writer.apply({
      intent: "Rename",
      apply: [environment(name, { displayName: "Two" })],
    });

    const result = await writer.apply({
      intent: "Rename from the old form",
      apply: [environment(name, { displayName: "Three" })],
      preconditions: { [refKey({ kind: "Environment", name })]: old },
      force: true,
    });

    expect(result).toEqual({
      ok: false,
      errors: [expect.objectContaining({ code: "Stale" })],
    });
    expect(await live(name)).toMatchObject({
      spec: { displayName: "Two" },
    });
  });

  it("a forced apply that the API server refuses fails with Invalid", async () => {
    await writer.apply({
      intent: "Create",
      apply: [environment(name, { displayName: "One" })],
    });

    const result = await writer.apply({
      intent: "Change the releases",
      apply: [
        environment(name, {
          displayName: "One",
          releases: { allowed: ["r1"], default: "r2" },
        }),
      ],
      force: true,
    });

    expect(result).toMatchObject({ ok: false, errors: [{ code: "Invalid" }] });
  });

  it("an apply with the resourceVersion of an older copy fails with Stale and leaves the object unchanged", async () => {
    await writer.apply({
      intent: "Create",
      apply: [environment(name, { displayName: "One" })],
    });
    const old = (await live(name)).metadata!.resourceVersion!;
    await writer.apply({
      intent: "Rename",
      apply: [environment(name, { displayName: "Two" })],
    });

    const result = await writer.apply({
      intent: "Rename from the old form",
      apply: [environment(name, { displayName: "Three" })],
      preconditions: { [refKey({ kind: "Environment", name })]: old },
    });

    expect(result).toEqual({
      ok: false,
      errors: [expect.objectContaining({ code: "Stale" })],
    });
    expect(await live(name)).toMatchObject({
      spec: { displayName: "Two" },
    });
  });

  it("an apply with the current resourceVersion succeeds", async () => {
    await writer.apply({
      intent: "Create",
      apply: [environment(name, { displayName: "One" })],
    });
    const current = (await live(name)).metadata!.resourceVersion!;

    const result = await writer.apply({
      intent: "Rename",
      apply: [environment(name, { displayName: "Two" })],
      preconditions: { [refKey({ kind: "Environment", name })]: current },
    });

    expect(result).toEqual({ ok: true, mode: "direct" });
    expect(await live(name)).toMatchObject({
      spec: { displayName: "Two" },
    });
  });

  it("an apply with a precondition for a deleted object fails with Stale and creates nothing", async () => {
    await writer.apply({
      intent: "Create",
      apply: [environment(name, { displayName: "One" })],
    });
    const old = (await live(name)).metadata!.resourceVersion!;
    await api.delete(environment(name, {}));

    const result = await writer.apply({
      intent: "Rename from the old form",
      apply: [environment(name, { displayName: "Two" })],
      preconditions: { [refKey({ kind: "Environment", name })]: old },
    });

    expect(result).toEqual({
      ok: false,
      errors: [expect.objectContaining({ code: "Stale" })],
    });
    await expect(live(name)).rejects.toMatchObject({ code: 404 });
  });

  it("writes nothing when one object of the change is refused", async () => {
    const other = `env-${randomUUID().slice(0, 8)}`;

    const result = await writer.apply({
      intent: "Create two environments",
      apply: [
        environment(name, { displayName: "Valid" }),
        environment(other, {
          displayName: "Refused",
          releases: { allowed: ["r1"], default: "r2" },
        }),
      ],
    });

    expect(result).toMatchObject({
      ok: false,
      errors: [{ code: "Invalid", ref: environmentRef(other) }],
    });
    await expect(live(name)).rejects.toMatchObject({ code: 404 });
  });

  it("a delete with the current resourceVersion removes the object", async () => {
    await writer.apply({
      intent: "Create",
      apply: [environment(name, { displayName: "One" })],
    });
    const current = (await live(name)).metadata!.resourceVersion!;

    const result = await writer.apply({
      intent: "Delete",
      apply: [],
      delete: [environmentRef(name)],
      preconditions: { [refKey({ kind: "Environment", name })]: current },
    });

    expect(result).toEqual({ ok: true, mode: "direct" });
    await expect(live(name)).rejects.toMatchObject({ code: 404 });
  });

  it("a delete with the resourceVersion of an older copy fails with Stale and keeps the object", async () => {
    await writer.apply({
      intent: "Create",
      apply: [environment(name, { displayName: "One" })],
    });
    const old = (await live(name)).metadata!.resourceVersion!;
    await writer.apply({
      intent: "Rename",
      apply: [environment(name, { displayName: "Two" })],
    });

    const result = await writer.apply({
      intent: "Delete",
      apply: [],
      delete: [environmentRef(name)],
      preconditions: { [refKey({ kind: "Environment", name })]: old },
    });

    expect(result).toEqual({
      ok: false,
      errors: [expect.objectContaining({ code: "Stale" })],
    });
    expect(await live(name)).toMatchObject({
      spec: { displayName: "Two" },
    });
  });

  it("a delete of a missing object succeeds", async () => {
    expect(
      await writer.apply({
        intent: "Delete",
        apply: [],
        delete: [environmentRef(name)],
      }),
    ).toEqual({ ok: true, mode: "direct" });
  });

  it("applies a ConfigMap with the field manager fleet-manager, and a stale resourceVersion fails with Stale", async () => {
    const configMap = (value: string): KubeObject => ({
      apiVersion: "v1",
      kind: "ConfigMap",
      metadata: { name, namespace },
      data: { key: value },
    });
    const read = () =>
      api.read({
        apiVersion: "v1",
        kind: "ConfigMap",
        metadata: { name, namespace },
      });
    const key = refKey({ kind: "ConfigMap", namespace, name });

    expect(
      await writer.apply({ intent: "Create", apply: [configMap("one")] }),
    ).toEqual({ ok: true, mode: "direct" });
    const old = (await read()).metadata!.resourceVersion!;
    expect((await read()).metadata?.managedFields).toContainEqual(
      expect.objectContaining({ manager: "fleet-manager", operation: "Apply" }),
    );
    await writer.apply({ intent: "Change", apply: [configMap("two")] });

    const result = await writer.apply({
      intent: "Change from the old form",
      apply: [configMap("three")],
      preconditions: { [key]: old },
    });

    expect(result).toEqual({
      ok: false,
      errors: [expect.objectContaining({ code: "Stale" })],
    });
    expect(await read()).toMatchObject({ data: { key: "two" } });
  });

  it("applies a Secret, and its preview holds no value, also not in an annotation", async () => {
    const secret = (value: string): KubeObject => ({
      apiVersion: "v1",
      kind: "Secret",
      metadata: {
        name,
        namespace,
        annotations: { "example.com/copy": `token=${value}` },
      },
      stringData: { token: value },
    });
    expect(
      await writer.apply({ intent: "Create", apply: [secret("first-value")] }),
    ).toEqual({ ok: true, mode: "direct" });

    const preview = await writer.preview({
      intent: "Change",
      apply: [secret("second-value")],
    });

    expect(preview.errors).toEqual([]);
    expect(preview.objects).toHaveLength(1);
    const shown = JSON.stringify(preview.objects);
    for (const value of ["first-value", "second-value"]) {
      expect(shown).not.toContain(value);
      expect(shown).not.toContain(Buffer.from(value).toString("base64"));
    }
    expect(preview.objects[0].diff.some((l) => l.op !== "same")).toBe(true);
  });
});
