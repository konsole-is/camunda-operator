import type {
  KubernetesListObject,
  KubernetesObject,
  Watch,
} from "@kubernetes/client-node";
import { afterEach, describe, expect, it } from "vitest";
import { ResourceCache } from "./cache";
import { getFrom, listFrom } from "./read";

const caches: ResourceCache[] = [];
function cacheWith(items: KubernetesObject[]): ResourceCache {
  const list: KubernetesListObject<KubernetesObject> = {
    apiVersion: "v1",
    kind: "List",
    metadata: { resourceVersion: "1" },
    items,
  };
  const watch = {
    watch: async () => new AbortController(),
  } as unknown as Watch;
  const cache = new ResourceCache({
    list: async () => structuredClone(list),
    watch,
  });
  caches.push(cache);
  return cache;
}
afterEach(async () => {
  await Promise.all(caches.splice(0).map((c) => c.stop()));
});

const secret = (
  namespace: string,
  name: string,
  labels?: Record<string, string>,
) =>
  ({
    metadata: { namespace, name, labels, resourceVersion: "1" },
  }) as KubernetesObject;

describe("listFrom", () => {
  it("returns the objects of a namespace, sorted by namespace and name", async () => {
    const cache = cacheWith([
      secret("b", "x"),
      secret("a", "z"),
      secret("a", "y"),
    ]);

    expect(
      (await listFrom(cache, "Secret")).map(
        (s) => `${s.metadata.namespace}/${s.metadata.name}`,
      ),
    ).toEqual(["a/y", "a/z", "b/x"]);
    expect(
      (await listFrom(cache, "Secret", { namespace: "a" })).map(
        (s) => s.metadata.name,
      ),
    ).toEqual(["y", "z"]);
  });

  it("a change to a returned object does not reach the cache", async () => {
    const cache = cacheWith([secret("a", "x", { tier: "web" })]);

    const [first] = await listFrom(cache, "Secret");
    first.metadata.labels!.tier = "changed";
    const [again] = await listFrom(cache, "Secret");

    expect(again.metadata.labels).toEqual({ tier: "web" });
  });

  it("refuses a namespace for a cluster-scoped kind", async () => {
    await expect(
      listFrom(cacheWith([]), "Environment", { namespace: "a" }),
    ).rejects.toThrow(
      "Environment is cluster-scoped, so a list of it takes no namespace",
    );
  });

  it("refuses a selector that is not valid, and names it", async () => {
    await expect(
      listFrom(cacheWith([]), "Secret", { labelSelector: "a b" }),
    ).rejects.toThrow('The label selector "a b" is not valid');
  });
});

describe("getFrom", () => {
  it("returns undefined for a name that is not there", async () => {
    expect(
      await getFrom(cacheWith([secret("a", "x")]), "Secret", "a", "y"),
    ).toBeUndefined();
  });

  it("returns a copy of the object with that name", async () => {
    const cache = cacheWith([secret("a", "x", { tier: "web" })]);

    const found = await getFrom(cache, "Secret", "a", "x");
    found!.metadata.labels!.tier = "changed";

    expect(
      (await getFrom(cache, "Secret", "a", "x"))!.metadata.labels,
    ).toEqual({ tier: "web" });
  });

  it("needs a namespace for a namespaced kind", async () => {
    await expect(
      getFrom(cacheWith([]), "Secret", undefined, "x"),
    ).rejects.toThrow("Secret is namespaced, so a get of it needs a namespace");
  });

  it("refuses a namespace for a cluster-scoped kind", async () => {
    await expect(
      getFrom(cacheWith([]), "Environment", "a", "x"),
    ).rejects.toThrow(
      "Environment is cluster-scoped, so a get of it takes no namespace",
    );
  });
});
