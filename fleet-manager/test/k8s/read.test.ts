import { randomUUID } from "node:crypto";
import { KubeConfig, KubernetesObjectApi } from "@kubernetes/client-node";
import { afterAll, beforeAll, describe, expect, inject, it } from "vitest";
import { defaultCache } from "@/lib/k8s/cache";
import { get, list } from "@/lib/k8s/read";

// The data layer reads KUBECONFIG through kubeConfig, on its first call.
process.env.KUBECONFIG = inject("kubeconfig");

const kc = new KubeConfig();
kc.loadFromFile(inject("kubeconfig"));
const api = KubernetesObjectApi.makeApiClient(kc);
const namespace = `read-${randomUUID().slice(0, 8)}`;

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

describe("list and get against envtest", () => {
  it("a list after a create returns the object", async () => {
    const name = `env-${randomUUID().slice(0, 8)}`;
    await api.create({
      apiVersion: "fleet.camunda.io/v1alpha1",
      kind: "Environment",
      metadata: { name },
      spec: { displayName: "Read test" },
    });

    const environments = await list("Environment");

    expect(environments).toContainEqual(
      expect.objectContaining({
        apiVersion: "fleet.camunda.io/v1alpha1",
        kind: "Environment",
        metadata: expect.objectContaining({ name }),
      }),
    );
  });

  it("a Secret in the cache has no data and no stringData", async () => {
    const value = "s3cret-value";
    await api.create({
      apiVersion: "v1",
      kind: "Secret",
      metadata: {
        name: "git",
        namespace,
        annotations: {
          "kubectl.kubernetes.io/last-applied-configuration": JSON.stringify({
            stringData: { token: value },
          }),
        },
      },
      stringData: { token: value },
    });

    const secret = await get("Secret", namespace, "git");

    expect(secret).toBeDefined();
    expect(secret).not.toHaveProperty("data");
    expect(secret).not.toHaveProperty("stringData");
    expect(secret!.keys).toEqual(["token"]);
    expect(JSON.stringify(secret)).not.toContain(value);
    expect(JSON.stringify(secret)).not.toContain(
      Buffer.from(value).toString("base64"),
    );
  });

  it("a list with a label selector returns only matching objects", async () => {
    for (const [name, labels] of [
      ["web", { tier: "web" }],
      ["db", { tier: "db" }],
      ["plain", undefined],
    ] as const) {
      await api.create({
        apiVersion: "v1",
        kind: "Secret",
        metadata: { name, namespace, labels },
      });
    }

    const names = async (labelSelector: string) =>
      (await list("Secret", { namespace, labelSelector })).map(
        (s) => s.metadata.name,
      );

    // The Secret informer is warm from the spec before, so the creates can
    // take a moment to reach it. plain is the last create.
    await expect
      .poll(() => names("!tier"), { timeout: 5_000 })
      .toEqual(["git", "plain"]);
    expect(await names("tier in (web,db)")).toEqual(["db", "web"]);
    expect(await names("tier=web")).toEqual(["web"]);
  });

  it("a get of a kind whose CRD is not installed fails with a message that names the kind", async () => {
    const started = Date.now();

    await expect(get("HTTPRoute", namespace, "web")).rejects.toThrow(
      /does not serve HTTPRoute \(gateway\.networking\.k8s\.io\/v1/,
    );
    expect(Date.now() - started).toBeLessThan(5_000);
  });
});
