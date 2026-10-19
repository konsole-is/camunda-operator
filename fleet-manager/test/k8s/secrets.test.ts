import { randomUUID } from "node:crypto";
import {
  KubeConfig,
  KubernetesObjectApi,
  type V1Secret,
} from "@kubernetes/client-node";
import { afterAll, beforeAll, describe, expect, inject, it } from "vitest";
import { defaultCache } from "@/lib/k8s/cache";
import { listSecretRefs, writeSecretValue } from "@/lib/secrets";

// The data layer reads KUBECONFIG through kubeConfig, on its first call.
process.env.KUBECONFIG = inject("kubeconfig");

const kc = new KubeConfig();
kc.loadFromFile(inject("kubeconfig"));
const api = KubernetesObjectApi.makeApiClient(kc);
const namespace = `secrets-${randomUUID().slice(0, 8)}`;
const id = randomUUID().slice(0, 8);

const secret = (
  name: string,
  values: Record<string, string>,
  extra: Partial<V1Secret> = {},
): V1Secret => ({
  apiVersion: "v1",
  kind: "Secret",
  metadata: { name, namespace },
  stringData: values,
  ...extra,
});

// readSecretByHand decodes the Secret through the API client, so the tests
// check what the API server stores and not what the module reads back.
async function readSecretByHand(
  name: string,
): Promise<Record<string, string> | undefined> {
  try {
    const live = await api.read<V1Secret>({
      apiVersion: "v1",
      kind: "Secret",
      metadata: { name, namespace },
    });
    return Object.fromEntries(
      Object.entries(live.data ?? {}).map(([k, v]) => [
        k,
        Buffer.from(v, "base64").toString("utf8"),
      ]),
    );
  } catch {
    return undefined;
  }
}

beforeAll(async () => {
  await api.create({
    apiVersion: "v1",
    kind: "Namespace",
    metadata: { name: namespace },
  });
});
afterAll(async () => {
  await defaultCache().stop();
});

describe("listSecretRefs against envtest", () => {
  it("returns key names and no values", async () => {
    await api.create(secret("db", { password: "pw-" + id, user: "u-" + id }));

    await expect
      .poll(() => listSecretRefs(namespace))
      .toContainEqual({ name: "db", keys: ["password", "user"] });
    const text = JSON.stringify(await listSecretRefs(namespace));
    expect(text).not.toContain("pw-" + id);
    expect(text).not.toContain(Buffer.from("pw-" + id).toString("base64"));
  });

  it("is empty for a namespace without Secrets", async () => {
    const empty = `secrets-${randomUUID().slice(0, 8)}`;
    await api.create({
      apiVersion: "v1",
      kind: "Namespace",
      metadata: { name: empty },
    });

    expect(await listSecretRefs(empty)).toEqual([]);
  });
});

describe("writeSecretValue against envtest", () => {
  it("creates a missing Secret", async () => {
    expect(
      await writeSecretValue(
        { namespace, name: "git", key: "token" },
        "t-" + id,
      ),
    ).toEqual({ ok: true, mode: "direct" });

    expect(await readSecretByHand("git")).toEqual({ token: "t-" + id });
  });

  it("writing one key keeps the other keys", async () => {
    await api.create(secret("prom", { username: "admin", password: "old" }));

    expect(
      await writeSecretValue(
        { namespace, name: "prom", key: "password" },
        "new-" + id,
      ),
    ).toEqual({ ok: true, mode: "direct" });

    expect(await readSecretByHand("prom")).toEqual({
      username: "admin",
      password: "new-" + id,
    });
  });

  it("a second key keeps the key that the fleet manager wrote first", async () => {
    await writeSecretValue({ namespace, name: "pair", key: "username" }, "me");
    await writeSecretValue({ namespace, name: "pair", key: "token" }, "t");

    expect(await readSecretByHand("pair")).toEqual({
      username: "me",
      token: "t",
    });
  });

  it("writes of different keys at once to a missing Secret keep every key", async () => {
    const keys = ["a", "b", "c", "d"];

    const results = await Promise.all(
      keys.map((key) =>
        writeSecretValue({ namespace, name: "burst", key }, `v-${key}`),
      ),
    );

    expect(results).toEqual(keys.map(() => ({ ok: true, mode: "direct" })));
    expect(await readSecretByHand("burst")).toEqual({
      a: "v-a",
      b: "v-b",
      c: "v-c",
      d: "v-d",
    });
  });

  it("the result of a write holds no value", async () => {
    const result = await writeSecretValue(
      { namespace, name: "git2", key: "token" },
      "t2-" + id,
    );

    expect(JSON.stringify(result)).not.toContain("t2-" + id);
  });

  it("an immutable Secret gives Invalid and keeps its value", async () => {
    await api.create(secret("frozen", { token: "old" }, { immutable: true }));

    const result = await writeSecretValue(
      { namespace, name: "frozen", key: "token" },
      "new-" + id,
    );

    expect(result).toEqual({
      ok: false,
      errors: [expect.objectContaining({ code: "Invalid" })],
    });
    expect(JSON.stringify(result)).not.toContain("new-" + id);
    expect(await readSecretByHand("frozen")).toEqual({ token: "old" });
  });

  it("refuses a key name that a Secret cannot hold", async () => {
    expect(
      await writeSecretValue({ namespace, name: "bad", key: "a b" }, "x"),
    ).toEqual({
      ok: false,
      errors: [expect.objectContaining({ code: "Invalid", path: "key" })],
    });
    expect(await readSecretByHand("bad")).toBeUndefined();
  });
});
