import type { KubernetesObject } from "@kubernetes/client-node";
import { describe, expect, it, vi } from "vitest";
import type { CacheListener, ResourceCache } from "./cache";
import { parseWatchRequest, watchStream } from "./watch";

const params = (query: string) => new URLSearchParams(query);

describe("parseWatchRequest", () => {
  it("reads the kinds, the namespace, the selector, and the name", () => {
    expect(
      parseWatchRequest(
        params(
          "kind=Pod&kind=Secret&namespace=team&labelSelector=app%3Dweb&name=x",
        ),
      ),
    ).toEqual({
      kinds: ["Pod", "Secret"],
      namespace: "team",
      labelSelector: "app=web",
      name: "x",
    });
  });

  it.each([
    ["", "Name at least one kind with the parameter kind"],
    ["kind=Widget", 'The fleet manager does not read the kind "Widget"'],
    [
      "kind=Environment&namespace=team",
      "Environment is cluster-scoped, so a watch of it takes no namespace",
    ],
    ["kind=Pod&labelSelector=a%20b", 'The label selector "a b" is not valid'],
    [
      "kind=Pod&name=worker",
      "Pod is namespaced, so a watch of one name needs a namespace",
    ],
  ])("refuses %j with a message that names the problem", (query, message) => {
    expect(() => parseWatchRequest(params(query))).toThrow(message);
  });
});

describe("watchStream", () => {
  it("releases the subscription when the request aborts after the snapshot", async () => {
    const unsubscribe = vi.fn();
    const cache = {
      subscribe: async () => ({ snapshot: [], unsubscribe }),
    } as unknown as ResourceCache;
    const abort = new AbortController();

    const reader = watchStream(
      cache,
      { kinds: ["Pod"] },
      abort.signal,
    ).getReader();
    const first = new TextDecoder().decode((await reader.read()).value);
    expect(first).toContain("event: snapshot");
    abort.abort();

    expect(unsubscribe).toHaveBeenCalledOnce();
    expect((await reader.read()).done).toBe(true);
  });

  it("releases the subscription when the request aborts while subscribe waits", async () => {
    const unsubscribe = vi.fn();
    let answer: (s: { snapshot: never[]; unsubscribe: () => void }) => void;
    const cache = {
      subscribe: () => new Promise((resolve) => (answer = resolve)),
    } as unknown as ResourceCache;
    const abort = new AbortController();

    const reader = watchStream(
      cache,
      { kinds: ["Pod"] },
      abort.signal,
    ).getReader();
    await vi.waitFor(() => expect(answer).toBeDefined());
    abort.abort();
    answer!({ snapshot: [], unsubscribe });

    await vi.waitFor(() => expect(unsubscribe).toHaveBeenCalledOnce());
    expect((await reader.read()).done).toBe(true);
  });

  it("ends at once for a signal that has already aborted", async () => {
    const controller = new AbortController();
    controller.abort();

    const stream = watchStream(
      {} as ResourceCache,
      { kinds: ["Pod"] },
      controller.signal,
    );

    expect(await stream.getReader().read()).toEqual({
      done: true,
      value: undefined,
    });
  });

  it("ends the stream and its subscription when the client does not read", async () => {
    let listener: CacheListener<"Pod"> | undefined;
    const unsubscribe = vi.fn();
    const cache = {
      subscribe: async (_kind: string, l: CacheListener<"Pod">) => {
        listener = l;
        return { snapshot: [], unsubscribe };
      },
    } as unknown as ResourceCache;

    const stream = watchStream(
      cache,
      { kinds: ["Pod"] },
      new AbortController().signal,
      { maxPendingBytes: 1_000 },
    );
    await vi.waitFor(() => expect(listener).toBeDefined());
    for (let i = 0; i < 100; i++) {
      listener!("add", {
        metadata: { namespace: "team", name: `pod-${i}` },
      } as KubernetesObject as never);
    }

    expect(unsubscribe).toHaveBeenCalled();
    await stream.cancel();
  });
});
