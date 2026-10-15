import type {
  KubernetesListObject,
  KubernetesObject,
  Watch,
} from "@kubernetes/client-node";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ResourceCache, type CacheDeps } from "./cache";
import { KubernetesRequestError } from "./client";

type Callback = (phase: string, object: unknown) => void;

// FakeWatch records each watch that the informer opens, so a test can send
// events on it and end it.
class FakeWatch {
  calls: {
    params: Record<string, unknown>;
    callback: Callback;
    done: (err: unknown) => void;
  }[] = [];
  async watch(
    _path: string,
    params: Record<string, unknown>,
    callback: Callback,
    done: (err: unknown) => void,
  ) {
    this.calls.push({ params, callback, done });
    return new AbortController();
  }
}

function listOf(
  items: KubernetesObject[],
  resourceVersion: string,
): KubernetesListObject<KubernetesObject> {
  return {
    apiVersion: "v1",
    kind: "List",
    metadata: { resourceVersion },
    items,
  };
}

// item is an object as a list returns it: without apiVersion and kind.
function item(
  name: string,
  namespace?: string,
  extra: object = {},
): KubernetesObject {
  return {
    metadata: { name, namespace, resourceVersion: "1" },
    ...extra,
  } as KubernetesObject;
}

const caches: ResourceCache[] = [];
function newCache(
  deps: Partial<CacheDeps> & Pick<CacheDeps, "list">,
  watch = new FakeWatch(),
) {
  const cache = new ResourceCache({
    watch: watch as unknown as Watch,
    retryDelayMs: 1,
    ...deps,
  });
  caches.push(cache);
  return { cache, watch };
}

afterEach(async () => {
  await Promise.all(caches.splice(0).map((c) => c.stop()));
});

const names = (objects: readonly { metadata?: { name?: string } }[]) =>
  objects.map((o) => o.metadata?.name).sort();

describe("ResourceCache", () => {
  it("lists a kind on first use and stamps apiVersion and kind on each item", async () => {
    const list = vi.fn(async () => listOf([item("prod")], "5"));
    const { cache } = newCache({ list });

    const objects = await cache.objects("Environment");

    expect(list).toHaveBeenCalledWith(
      "/apis/fleet.camunda.io/v1alpha1/environments",
    );
    expect(objects).toEqual([
      expect.objectContaining({
        apiVersion: "fleet.camunda.io/v1alpha1",
        kind: "Environment",
      }),
    ]);
  });

  it("starts one informer for a kind, however many readers wait", async () => {
    const list = vi.fn(async () => listOf([], "1"));
    const { cache } = newCache({ list });

    await Promise.all([
      cache.objects("Environment"),
      cache.objects("Environment"),
    ]);
    await cache.objects("Environment");

    expect(list).toHaveBeenCalledTimes(1);
  });

  it("drops Secret values from a list and from watch events", async () => {
    const secret = { stringData: { token: "hunter2" }, type: "Opaque" };
    const { cache, watch } = newCache({
      list: async () => listOf([item("git", "team", secret)], "1"),
    });

    const [listed] = await cache.objects("Secret");
    watch.calls[0].callback("MODIFIED", {
      apiVersion: "v1",
      kind: "Secret",
      metadata: { name: "git", namespace: "team", resourceVersion: "2" },
      data: { password: "c2VjcmV0" },
    });
    const [watched] = await cache.objects("Secret");

    expect(listed).toMatchObject({ keys: ["token"] });
    expect(watched).toMatchObject({ keys: ["password"] });
    expect(JSON.stringify([listed, watched])).not.toMatch(
      /hunter2|c2VjcmV0|"data"|"stringData"/,
    );
  });

  it("lists again after 410 Gone and drops what the new list no longer has", async () => {
    const lists = [
      listOf([item("a"), item("b")], "10"),
      listOf([item("a")], "20"),
    ];
    const list = vi.fn(async () => lists.shift()!);
    const { cache, watch } = newCache({ list });
    await cache.objects("Environment");

    watch.calls[0].callback("ERROR", { code: 410 });
    watch.calls[0].done(null);

    await vi.waitFor(() => expect(list).toHaveBeenCalledTimes(2));
    await vi.waitFor(() => expect(watch.calls).toHaveLength(2));
    expect(names(await cache.objects("Environment"))).toEqual(["a"]);
    expect(watch.calls[1].params.resourceVersion).toBe("20");
  });

  it("keeps reconnecting quickly after each timeout of the watch", async () => {
    const list = vi.fn(async () => listOf([item("a")], "10"));
    const { cache, watch } = newCache({ list, timeoutReconnectMs: 1 });
    await cache.objects("Environment");

    // The client ends each watch after a request timeout, also when nothing
    // changed. The default delay of ListWatch doubles up to 30 seconds.
    for (let closes = 1; closes <= 6; closes++) {
      const timeout = Object.assign(new Error("timed out"), {
        name: "TimeoutError",
      });
      watch.calls[closes - 1].done(timeout);
      await vi.waitFor(() => expect(watch.calls).toHaveLength(closes + 1), {
        timeout: 500,
      });
    }
    expect(watch.calls.at(-1)!.params.resourceVersion).toBe("10");
    expect(list).toHaveBeenCalledTimes(1);
  });

  it("reads the kind on the next read after the first watch could not start", async () => {
    const list = vi.fn(async () => listOf([item("a")], "10"));
    const watch = new FakeWatch();
    const watchOnce = watch.watch.bind(watch);
    let starts = 0;
    watch.watch = async (...args: Parameters<FakeWatch["watch"]>) => {
      starts++;
      if (starts === 1) throw new Error("Unauthorized");
      return watchOnce(...args);
    };
    const { cache } = newCache({ list }, watch);

    await expect(cache.objects("Environment")).rejects.toThrow(/Unauthorized/);

    expect(names(await cache.objects("Environment"))).toEqual(["a"]);
    expect(list).toHaveBeenCalledTimes(1);
  });

  it("keeps retrying when a watch cannot start again after a timeout", async () => {
    const list = vi.fn(async () => listOf([item("a")], "10"));
    const watch = new FakeWatch();
    const watchOnce = watch.watch.bind(watch);
    let starts = 0;
    watch.watch = async (...args: Parameters<FakeWatch["watch"]>) => {
      starts++;
      if (starts === 2) throw new Error("Unauthorized");
      return watchOnce(...args);
    };
    const { cache } = newCache({ list, timeoutReconnectMs: 1 }, watch);
    await cache.objects("Environment");

    watch.calls[0].done(
      Object.assign(new Error("timed out"), { name: "TimeoutError" }),
    );

    await vi.waitFor(() => expect(watch.calls).toHaveLength(2));
    expect(starts).toBe(3);
    expect(watch.calls[1].params.resourceVersion).toBe("10");
  });

  it("watches again from the last resourceVersion after a watch error", async () => {
    const list = vi.fn(async () => listOf([item("a")], "10"));
    const { cache, watch } = newCache({ list });
    await cache.objects("Environment");

    watch.calls[0].done(new Error("connection reset"));

    await vi.waitFor(() => expect(watch.calls).toHaveLength(2));
    expect(watch.calls[1].params.resourceVersion).toBe("10");
    expect(list).toHaveBeenCalledTimes(1);
  });

  it("fails at once when the API server does not serve the kind, and tries again on the next read", async () => {
    const list = vi.fn(async () => {
      throw new KubernetesRequestError(
        404,
        "the server could not find the requested resource",
      );
    });
    const { cache } = newCache({ list });

    await expect(cache.objects("HTTPRoute")).rejects.toThrow(
      /does not serve HTTPRoute \(gateway\.networking\.k8s\.io\/v1/,
    );
    await expect(cache.objects("HTTPRoute")).rejects.toThrow(/HTTPRoute/);
    expect(list).toHaveBeenCalledTimes(2);
  });

  it("fails with a message that names the kind when the first list does not answer in time", async () => {
    const { cache } = newCache({
      list: () => new Promise(() => {}),
      syncTimeoutMs: 20,
    });

    await expect(cache.objects("Environment")).rejects.toThrow(
      /waited 0\.02 seconds for the first list of Environment/,
    );
  });

  it("starts a new first list on the next read after a list that does not answer", async () => {
    const answers = [
      new Promise<never>(() => {}),
      Promise.resolve(listOf([item("prod")], "1")),
    ];
    const list = vi.fn(() => answers.shift()!);
    const { cache } = newCache({ list, syncTimeoutMs: 20 });

    await expect(cache.objects("Environment")).rejects.toThrow(
      /waited 0\.02 seconds/,
    );

    await vi.waitFor(async () =>
      expect(names(await cache.objects("Environment"))).toEqual(["prod"]),
    );
    expect(list).toHaveBeenCalledTimes(2);
  });

  it("gives a subscriber the objects at subscribe time, then each change", async () => {
    const { cache, watch } = newCache({
      list: async () => listOf([item("a")], "1"),
    });
    const listener = vi.fn();

    const { snapshot, unsubscribe } = await cache.subscribe(
      "Environment",
      listener,
    );
    watch.calls[0].callback("ADDED", {
      metadata: { name: "b", resourceVersion: "2" },
    });
    unsubscribe();
    watch.calls[0].callback("ADDED", {
      metadata: { name: "c", resourceVersion: "3" },
    });

    expect(names(snapshot)).toEqual(["a"]);
    expect(listener).toHaveBeenCalledTimes(1);
    expect(listener).toHaveBeenCalledWith(
      "add",
      expect.objectContaining({
        metadata: expect.objectContaining({ name: "b" }),
      }),
    );
  });
});
