// @vitest-environment jsdom
import { act, cleanup, renderHook, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { FakeEventSource } from "../../../test/fakes/event-source";
import type { LogFilter } from "./logs";
import { type LogSource, useLogStream, useLogStreams } from "./use-log-stream";

beforeEach(() => {
  FakeEventSource.instances = [];
  vi.stubGlobal("EventSource", FakeEventSource);
});
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

const source: LogSource = { namespace: "n", pod: "p" };
const last = () => FakeEventSource.instances.at(-1)!;

describe("useLogStream", () => {
  it("opens /api/logs with the source as parameters", () => {
    renderHook(() =>
      useLogStream({
        namespace: "n",
        pod: "p",
        container: "c",
        sinceSeconds: 60,
      }),
    );

    expect(FakeEventSource.instances.map((s) => s.url)).toEqual([
      "/api/logs?namespace=n&pod=p&container=c&sinceSeconds=60",
    ]);
  });

  it("parses each line", () => {
    const { result } = renderHook(() => useLogStream(source));

    last().emit("line", '{"level":"info","msg":"hi"}');

    expect(result.current.lines[0]).toMatchObject({
      level: "info",
      message: "hi",
    });
  });

  it("keeps only the lines of the request in the filter", () => {
    const { result } = renderHook(() =>
      useLogStream(source, { request: "ns/one" }),
    );

    last().emit("line", '{"msg":"a","request":"ns/one"}');
    last().emit("line", '{"msg":"b","request":"ns/two"}');

    expect(result.current.lines.map((l) => l.message)).toEqual(["a"]);
  });

  it("applies a new filter to the lines it has", () => {
    const { result, rerender } = renderHook(
      ({ filter }: { filter: LogFilter }) => useLogStream(source, filter),
      { initialProps: { filter: {} } },
    );
    last().emit("line", '{"level":"info","msg":"a"}');
    last().emit("line", '{"level":"error","msg":"b"}');

    rerender({ filter: { level: "error" } });

    expect(result.current.lines.map((l) => l.message)).toEqual(["b"]);
    expect(FakeEventSource.instances).toHaveLength(1);
  });

  it("keeps the newest 1000 lines", () => {
    const { result } = renderHook(() => useLogStream(source));

    act(() => {
      for (let i = 0; i < 1005; i++) last().emit("line", `line ${i}`);
    });

    expect(result.current.lines).toHaveLength(1000);
    expect(result.current.lines[0].message).toBe("line 5");
    expect(result.current.lines.at(-1)!.message).toBe("line 1004");
  });

  it("sets ended on an end event and closes the connection", () => {
    const { result } = renderHook(() => useLogStream(source));

    last().emit("end", {});

    expect(result.current.ended).toBe(true);
    expect(last().closed).toBe(true);
  });

  it("sets error on a failure event", () => {
    const { result } = renderHook(() => useLogStream(source));

    last().emit("failure", { message: "The log stream of pod p broke" });

    expect(result.current.error).toBe("The log stream of pod p broke");
    expect(last().closed).toBe(true);
  });

  it("sets error and does not connect again when the connection fails", () => {
    const { result } = renderHook(() => useLogStream(source));

    last().fail(0);

    expect(result.current.error).toBeDefined();
    expect(last().closed).toBe(true);
  });

  it("sets error to the text of /api/logs when it refuses the stream", async () => {
    const fetch = vi.fn(
      async () =>
        new Response("The container c of pod n/p is not running", {
          status: 409,
        }),
    );
    vi.stubGlobal("fetch", fetch);
    const { result } = renderHook(() => useLogStream(source));

    last().fail(2);

    await waitFor(() =>
      expect(result.current.error).toBe(
        "The container c of pod n/p is not running",
      ),
    );
    expect(fetch).toHaveBeenCalledWith(last().url, expect.anything());
  });

  it("asks /api/logs for no text when the stream broke after it opened", () => {
    const fetch = vi.fn();
    vi.stubGlobal("fetch", fetch);
    const { result } = renderHook(() => useLogStream(source));

    last().emit("open", {});
    last().fail(0);

    expect(result.current.error).toBe("The connection to the log stream broke");
    expect(fetch).not.toHaveBeenCalled();
  });

  it("opens no connection without a source", () => {
    const { result } = renderHook(() => useLogStream(undefined));

    expect(FakeEventSource.instances).toEqual([]);
    expect(result.current).toEqual({ lines: [], ended: false });
  });

  it("closes the connection on unmount", () => {
    const { unmount } = renderHook(() => useLogStream(source));

    unmount();

    expect(last().closed).toBe(true);
  });

  it.each([
    ["no source", undefined],
    ["another source", { namespace: "n", pod: "q" }],
  ])("starts empty when it opens a source again after %s", (_, between) => {
    const { result, rerender } = renderHook(
      ({ from }: { from: LogSource | undefined }) => useLogStream(from),
      { initialProps: { from: source as LogSource | undefined } },
    );
    last().emit("line", "old");
    last().emit("end", {});

    rerender({ from: between });
    rerender({ from: source });

    expect(result.current).toEqual({ lines: [], ended: false });
  });

  it("starts empty with a new connection for a new source", () => {
    const { result, rerender } = renderHook(
      ({ pod }: { pod: string }) => useLogStream({ namespace: "n", pod }),
      { initialProps: { pod: "p" } },
    );
    const first = last();
    first.emit("line", "old");

    rerender({ pod: "q" });

    expect(first.closed).toBe(true);
    expect(FakeEventSource.instances).toHaveLength(2);
    expect(result.current.lines).toEqual([]);
  });
});

describe("useLogStreams", () => {
  const a = { namespace: "n", pod: "a" };
  const b = { namespace: "n", pod: "b" };
  const of = (pod: string) =>
    FakeEventSource.instances.find((e) => e.url.includes(`pod=${pod}`))!;

  it("opens one stream for each pod and keeps the lines in the order they arrive, with their pod", () => {
    const { result } = renderHook(() => useLogStreams([a, b]));

    expect(FakeEventSource.instances).toHaveLength(2);
    of("a").emit("line", "one");
    of("b").emit("line", "two");
    of("a").emit("line", "three");

    expect(result.current.lines.map((l) => [l.pod, l.message])).toEqual([
      ["a", "one"],
      ["b", "two"],
      ["a", "three"],
    ]);
  });

  it("keeps only the lines of the request of a controller source", () => {
    const { result } = renderHook(() =>
      useLogStreams([{ ...a, request: "team-a/orders-backup-storage" }, b]),
    );

    of("a").emit(
      "line",
      '{"msg":"mine","request":{"name":"orders-backup-storage","namespace":"team-a"}}',
    );
    of("a").emit(
      "line",
      '{"msg":"other","request":{"name":"x","namespace":"team-a"}}',
    );
    of("b").emit("line", "request pod line");

    expect(result.current.lines.map((l) => l.message)).toEqual([
      "mine",
      "request pod line",
    ]);
  });

  it("names the pod of a stream that broke and keeps the others", () => {
    const { result } = renderHook(() => useLogStreams([a, b]));

    of("a").emit("failure", { message: "The log stream of pod a broke" });
    of("b").emit("line", "still here");

    expect(result.current.errors).toEqual([
      { pod: "a", error: "The log stream of pod a broke" },
    ]);
    expect(result.current.lines.map((l) => l.message)).toEqual(["still here"]);
    expect(result.current.ended).toBe(false);
  });

  it("names the refusal of /api/logs for a stream that did not open", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => new Response("The pod n/a does not exist", { status: 404 })),
    );
    const { result } = renderHook(() => useLogStreams([a, b]));

    of("a").fail(2);

    await waitFor(() =>
      expect(result.current.errors).toEqual([
        { pod: "a", error: "The pod n/a does not exist" },
      ]),
    );
  });

  it("is ended when every stream ended or broke", () => {
    const { result } = renderHook(() => useLogStreams([a, b]));

    of("a").emit("end", {});
    expect(result.current.ended).toBe(false);
    of("b").emit("failure", { message: "broke" });

    expect(result.current.ended).toBe(true);
  });

  it("is not ended without a source", () => {
    const { result } = renderHook(() => useLogStreams([]));

    expect(FakeEventSource.instances).toEqual([]);
    expect(result.current).toEqual({ lines: [], errors: [], ended: false });
  });

  it("keeps the newest 1000 lines over all streams", () => {
    const { result } = renderHook(() => useLogStreams([a, b]));

    act(() => {
      for (let i = 0; i < 1005; i++) of(i % 2 ? "b" : "a").emit("line", `line ${i}`);
    });

    expect(result.current.lines).toHaveLength(1000);
    expect(result.current.lines[0].message).toBe("line 5");
    expect(result.current.lines.at(-1)!.message).toBe("line 1004");
  });

  it("closes every stream on unmount", () => {
    const { unmount } = renderHook(() => useLogStreams([a, b]));

    unmount();

    expect(FakeEventSource.instances.map((e) => e.closed)).toEqual([true, true]);
  });

  it("starts with no lines when the pods change", () => {
    const { result, rerender } = renderHook(
      ({ pods }: { pods: LogSource[] }) => useLogStreams(pods),
      { initialProps: { pods: [a, b] } },
    );
    of("a").emit("line", "old");
    of("b").emit("failure", { message: "broke" });

    rerender({ pods: [a] });

    expect(result.current).toEqual({ lines: [], errors: [], ended: false });
    expect(FakeEventSource.instances).toHaveLength(3);
    expect(FakeEventSource.instances.slice(0, 2).every((e) => e.closed)).toBe(true);
  });

  it("keeps its streams when it gets an equal list of sources", () => {
    const { rerender } = renderHook(
      ({ pods }: { pods: LogSource[] }) => useLogStreams(pods),
      { initialProps: { pods: [a, b] } },
    );

    rerender({ pods: [{ ...a }, { ...b }] });

    expect(FakeEventSource.instances).toHaveLength(2);
  });

  it("applies the level filter to the lines of every stream", () => {
    const { result } = renderHook(() =>
      useLogStreams([a, b], { level: "error" }),
    );

    of("a").emit("line", '{"level":"info","msg":"a1"}');
    of("a").emit("line", '{"level":"error","msg":"a2"}');
    of("b").emit("line", '{"level":"ERROR","msg":"b1"}');

    expect(result.current.lines.map((l) => l.message)).toEqual(["a2", "b1"]);
  });
});
