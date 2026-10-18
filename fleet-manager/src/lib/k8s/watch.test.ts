import type { KubernetesObject } from "@kubernetes/client-node";
import { describe, expect, it, vi } from "vitest";
import type { CacheListener, ResourceCache } from "./cache";
import { parseWatchRequest, parseWatchRequests, watchStream } from "./watch";

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

describe("parseWatchRequests", () => {
  it("reads each watch with the rules of one watch", () => {
    expect(
      parseWatchRequests(
        params(
          `watch=${encodeURIComponent("kind=Pod&namespace=team")}&watch=${encodeURIComponent("kind=Environment")}`,
        ),
      ),
    ).toEqual([
      {
        id: "kind=Pod&namespace=team",
        request: { kinds: ["Pod"], namespace: "team" },
      },
      { id: "kind=Environment", request: { kinds: ["Environment"] } },
    ]);
  });

  it.each([
    ["", "Name at least one watch with the parameter watch"],
    ["kind=Pod", "Name at least one watch with the parameter watch"],
    [
      "watch=kind%3DPod&watch=kind%3DPod",
      'The watch "kind=Pod" appears more than once',
    ],
  ])("refuses %j with a message that names the problem", (query, message) => {
    expect(() => parseWatchRequests(params(query))).toThrow(message);
  });

  it("keeps a watch that is not valid, with its problem", () => {
    expect(
      parseWatchRequests(params("watch=kind%3DWidget&watch=kind%3DPod")),
    ).toEqual([
      {
        id: "kind=Widget",
        error: 'The fleet manager does not read the kind "Widget"',
      },
      { id: "kind=Pod", request: { kinds: ["Pod"] } },
    ]);
  });
});

interface Frame {
  event: string;
  retry?: number;
  data: {
    watch: string;
    kind: string;
    type?: string;
    message?: string;
    object?: KubernetesObject;
    objects?: KubernetesObject[];
  };
}

// readFrames reads the SSE frames of a stream until it ends or until it has
// count frames.
async function readFrames(
  reader: ReadableStreamDefaultReader<Uint8Array>,
  count = Infinity,
): Promise<{ frames: Frame[]; done: boolean }> {
  const decoder = new TextDecoder();
  const frames: Frame[] = [];
  let buffer = "";
  while (frames.length < count) {
    const { value, done } = await reader.read();
    if (done) return { frames, done: true };
    buffer += decoder.decode(value);
    for (
      let end = buffer.indexOf("\n\n");
      end >= 0;
      end = buffer.indexOf("\n\n")
    ) {
      const text = buffer.slice(0, end);
      buffer = buffer.slice(end + 2);
      if (text.startsWith(":")) continue;
      const retry = /^retry: (\d+)$/m.exec(text)?.[1];
      frames.push({
        retry: retry === undefined ? undefined : Number(retry),
        event: /^event: (.*)$/m.exec(text)![1],
        data: JSON.parse(/^data: (.*)$/m.exec(text)![1]),
      });
    }
  }
  return { frames, done: false };
}

const object = (namespace: string, name: string) =>
  ({ metadata: { namespace, name } }) as KubernetesObject;

// fakeCache records the listeners of each kind and answers subscribe from
// answers, one promise per kind.
function fakeCache(answers: Partial<Record<string, () => Promise<unknown>>>) {
  const listeners = new Map<string, CacheListener<"Pod">[]>();
  const cache = {
    subscribe: vi.fn((kind: string, listener: CacheListener<"Pod">) => {
      listeners.set(kind, [...(listeners.get(kind) ?? []), listener]);
      return answers[kind]!();
    }),
  } as unknown as ResourceCache;
  return { cache, listeners };
}

describe("watchStream", () => {
  it("releases the subscription when the request aborts after the snapshot", async () => {
    const unsubscribe = vi.fn();
    const cache = {
      subscribe: async () => ({ snapshot: [], unsubscribe }),
    } as unknown as ResourceCache;
    const abort = new AbortController();

    const reader = watchStream(
      cache,
      [{ id: "kind=Pod", request: { kinds: ["Pod"] } }],
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
      [{ id: "kind=Pod", request: { kinds: ["Pod"] } }],
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
      [{ id: "kind=Pod", request: { kinds: ["Pod"] } }],
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
      [{ id: "kind=Pod", request: { kinds: ["Pod"] } }],
      new AbortController().signal,
      { maxPendingBytes: 1_000 },
    );
    await vi.waitFor(() => expect(listener).toBeDefined());
    for (let i = 0; i < 100; i++) {
      listener!("add", {
        metadata: { namespace: "team", name: `pod-${i}` },
      } as KubernetesObject as never);
    }

    await vi.waitFor(() => expect(unsubscribe).toHaveBeenCalled());
    await stream.cancel();
  });
  it("frames carry the id of their watch", async () => {
    const pods = [object("team", "x"), object("other", "y")];
    const { cache, listeners } = fakeCache({
      Pod: async () => ({ snapshot: pods, unsubscribe: vi.fn() }),
    });
    const abort = new AbortController();
    const reader = watchStream(
      cache,
      [
        { id: "a", request: { kinds: ["Pod"], namespace: "team" } },
        { id: "b", request: { kinds: ["Pod"], namespace: "other" } },
      ],
      abort.signal,
    ).getReader();

    const { frames: snapshots } = await readFrames(reader, 2);
    const snapshotOf = (id: string) =>
      snapshots.find((f) => f.event === "snapshot" && f.data.watch === id);
    expect(snapshotOf("a")?.data.objects).toEqual([object("team", "x")]);
    expect(snapshotOf("b")?.data.objects).toEqual([object("other", "y")]);

    listeners
      .get("Pod")!
      .forEach((l) => l("add", object("team", "z") as never));
    // The stream ends at the abort and keeps the frames it already sent.
    abort.abort();
    const { frames: deltas } = await readFrames(reader);
    expect(deltas).toEqual([
      {
        event: "delta",
        data: {
          watch: "a",
          type: "ADDED",
          kind: "Pod",
          object: object("team", "z"),
        },
      },
    ]);
  });

  it("a failing watch does not hold back the snapshot of another watch", async () => {
    const unsubscribe = vi.fn();
    let answer: (s: { snapshot: never[]; unsubscribe: () => void }) => void;
    const { cache } = fakeCache({
      Pod: async () => {
        throw new Error("pods are not served");
      },
      Service: () => new Promise((resolve) => (answer = resolve)),
    });
    const reader = watchStream(
      cache,
      [
        { id: "a", request: { kinds: ["Pod"] } },
        { id: "b", request: { kinds: ["Service"] } },
      ],
      new AbortController().signal,
    ).getReader();

    const { frames: first } = await readFrames(reader, 1);
    expect(first).toEqual([
      {
        event: "failure",
        retry: 5_000,
        data: { watch: "a", kind: "Pod", message: "pods are not served" },
      },
    ]);
    answer!({ snapshot: [], unsubscribe });

    const { frames: rest, done } = await readFrames(reader);
    expect(rest).toEqual([
      {
        event: "snapshot",
        data: { watch: "b", kind: "Service", objects: [] },
      },
    ]);
    expect(done).toBe(true);
    expect(unsubscribe).toHaveBeenCalledOnce();
  });

  it("starts every watch before the first snapshot", async () => {
    const never = () => new Promise(() => {});
    const { cache } = fakeCache({ Pod: never, Service: never });
    const stream = watchStream(
      cache,
      [
        { id: "a", request: { kinds: ["Pod"] } },
        { id: "b", request: { kinds: ["Service"] } },
      ],
      new AbortController().signal,
    );

    await vi.waitFor(() =>
      expect(
        vi.mocked(cache.subscribe).mock.calls.map(([kind]) => kind),
      ).toEqual(["Pod", "Service"]),
    );
    await stream.cancel();
  });
  it("a watch that is not valid gets a failure frame, and the other watches stay live", async () => {
    const { cache, listeners } = fakeCache({
      Pod: async () => ({ snapshot: [], unsubscribe: vi.fn() }),
    });
    const abort = new AbortController();
    const reader = watchStream(
      cache,
      [
        { id: "kind=Widget", error: "bad" },
        { id: "a", request: { kinds: ["Pod"] } },
      ],
      abort.signal,
    ).getReader();

    const { frames } = await readFrames(reader, 2);
    expect(frames).toEqual([
      {
        event: "failure",
        data: { watch: "kind=Widget", kind: "Widget", message: "bad" },
      },
      { event: "snapshot", data: { watch: "a", kind: "Pod", objects: [] } },
    ]);
    listeners.get("Pod")![0]("add", object("team", "x") as never);
    const { frames: deltas } = await readFrames(reader, 1);
    expect(deltas[0].data).toMatchObject({ watch: "a", type: "ADDED" });
    abort.abort();
  });

  it("holds a snapshot until the queue has room, so a slow reader gets every snapshot", async () => {
    const big = (name: string) =>
      ({
        metadata: { namespace: "team", name },
        data: "x".repeat(500),
      }) as KubernetesObject;
    const answer = (name: string) => async () => ({
      snapshot: [big(name)],
      unsubscribe: vi.fn(),
    });
    const { cache } = fakeCache({
      Pod: answer("p"),
      Service: answer("s"),
      Secret: answer("c"),
    });
    const abort = new AbortController();
    const reader = watchStream(
      cache,
      [
        { id: "a", request: { kinds: ["Pod"] } },
        { id: "b", request: { kinds: ["Service"] } },
        { id: "c", request: { kinds: ["Secret"] } },
      ],
      abort.signal,
      { maxPendingBytes: 1_000 },
    ).getReader();
    await vi.waitFor(() =>
      expect(vi.mocked(cache.subscribe)).toHaveBeenCalledTimes(3),
    );
    await new Promise((wake) => setTimeout(wake, 10));

    const { frames, done } = await readFrames(reader, 3);
    expect(done).toBe(false);
    expect(frames.map((f) => f.data.watch).sort()).toEqual(["a", "b", "c"]);
    abort.abort();
  });
  it("ends the stream when the changes behind a waiting snapshot pass maxPendingBytes", async () => {
    const padded = (name: string, size: number) =>
      ({
        metadata: { namespace: "team", name },
        data: "x".repeat(size),
      }) as KubernetesObject;
    const unsubscribe = vi.fn();
    const { cache, listeners } = fakeCache({
      Pod: async () => ({
        snapshot: [padded("first", 800)],
        unsubscribe: vi.fn(),
      }),
      Service: async () => ({
        snapshot: [padded("waiting", 500)],
        unsubscribe,
      }),
    });
    const stream = watchStream(
      cache,
      [
        { id: "a", request: { kinds: ["Pod"] } },
        { id: "b", request: { kinds: ["Service"] } },
      ],
      new AbortController().signal,
      { maxPendingBytes: 1_000 },
    );
    await vi.waitFor(() => expect(listeners.get("Service")).toBeDefined());
    await new Promise((wake) => setTimeout(wake, 10));

    for (let i = 0; i < 20; i++) {
      listeners.get("Service")![0]("add", padded(`change-${i}`, 100) as never);
    }

    await vi.waitFor(() => expect(unsubscribe).toHaveBeenCalled());
    await stream.cancel();
  });
  it("a failure frame waits for room as a snapshot does, so a large snapshot does not drop it", async () => {
    const padded = (name: string, size: number) =>
      ({
        metadata: { namespace: "team", name },
        data: "x".repeat(size),
      }) as KubernetesObject;
    const { cache } = fakeCache({
      Pod: async () => ({
        snapshot: [padded("large", 1_500)],
        unsubscribe: vi.fn(),
      }),
      Service: () =>
        new Promise((_, reject) =>
          setTimeout(() => reject(new Error("services are not served")), 5),
        ),
      Secret: async () => ({
        snapshot: [padded("small", 10)],
        unsubscribe: vi.fn(),
      }),
    });
    const reader = watchStream(
      cache,
      [
        { id: "a", request: { kinds: ["Pod"] } },
        { id: "b", request: { kinds: ["Service"] } },
        { id: "c", request: { kinds: ["Secret"] } },
      ],
      new AbortController().signal,
      { maxPendingBytes: 1_000 },
    ).getReader();
    await new Promise((wake) => setTimeout(wake, 20));

    const { frames, done } = await readFrames(reader);
    expect(done).toBe(true);
    expect(frames.map((f) => `${f.event} ${f.data.watch}`).sort()).toEqual([
      "failure b",
      "snapshot a",
      "snapshot c",
    ]);
  });
});
