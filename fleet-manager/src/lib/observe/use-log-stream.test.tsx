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
  const line = (source: number, text: string) =>
    last().emit("line", { source, line: text });

  it("follows every pod over one connection to /api/logs/merged", () => {
    renderHook(() =>
      useLogStreams([a, { namespace: "plugins", pod: "c", container: "m" }], {
        tailLines: 200,
      }),
    );

    expect(FakeEventSource.instances.map((e) => e.url)).toEqual([
      "/api/logs/merged?log=n%2Fa&log=plugins%2Fc%2Fm&tailLines=200",
    ]);
  });

  it("keeps the lines in the order they arrive, each with its pod", () => {
    const { result } = renderHook(() => useLogStreams([a, b]));

    line(0, "one");
    line(1, "two");
    line(0, "three");

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

    line(
      0,
      '{"msg":"mine","request":{"name":"orders-backup-storage","namespace":"team-a"}}',
    );
    line(0, '{"msg":"other","request":{"name":"x","namespace":"team-a"}}');
    line(1, "request pod line");

    expect(result.current.lines.map((l) => l.message)).toEqual([
      "mine",
      "request pod line",
    ]);
  });

  it("names the pod of a log that broke and keeps the others", () => {
    const { result } = renderHook(() => useLogStreams([a, b]));

    last().emit("failure", {
      source: 0,
      message: "The pod n/a does not exist",
    });
    line(1, "still here");

    expect(result.current.errors).toEqual([
      { pod: "a", error: "The pod n/a does not exist" },
    ]);
    expect(result.current.lines.map((l) => l.message)).toEqual(["still here"]);
    expect(result.current.ended).toBe(false);
    expect(last().closed).toBe(false);
  });

  it("is ended and closes the connection when every log ended or broke", () => {
    const { result } = renderHook(() => useLogStreams([a, b]));

    last().emit("end", { source: 0 });
    expect(result.current.ended).toBe(false);
    last().emit("failure", { source: 1, message: "broke" });

    expect(result.current.ended).toBe(true);
    expect(last().closed).toBe(true);
  });

  it("names each open pod when the connection breaks, and does not connect again", () => {
    const { result } = renderHook(() => useLogStreams([a, b]));
    last().emit("open", {});
    last().emit("end", { source: 0 });

    last().fail(0);

    expect(result.current.errors).toEqual([
      { pod: "b", error: "The connection to the log stream broke" },
    ]);
    expect(result.current.ended).toBe(true);
    expect(last().closed).toBe(true);
    expect(FakeEventSource.instances).toHaveLength(1);
  });

  it("names each pod with the text of /api/logs/merged when it refuses the connection", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(
        async () =>
          new Response("The parameter log is required and must be valid", {
            status: 400,
          }),
      ),
    );
    const { result } = renderHook(() => useLogStreams([a, b]));

    last().fail(2);

    await waitFor(() =>
      expect(result.current.errors).toEqual([
        { pod: "a", error: "The parameter log is required and must be valid" },
        { pod: "b", error: "The parameter log is required and must be valid" },
      ]),
    );
  });

  it("opens no connection and is not ended without a source", () => {
    const { result } = renderHook(() => useLogStreams([]));

    expect(FakeEventSource.instances).toEqual([]);
    expect(result.current).toEqual({ lines: [], errors: [], ended: false });
  });

  it("keeps the newest 1000 lines over all logs", () => {
    const { result } = renderHook(() => useLogStreams([a, b]));

    act(() => {
      for (let i = 0; i < 1005; i++) line(i % 2, `line ${i}`);
    });

    expect(result.current.lines).toHaveLength(1000);
    expect(result.current.lines[0].message).toBe("line 5");
    expect(result.current.lines.at(-1)!.message).toBe("line 1004");
  });

  it("closes the connection on unmount", () => {
    const { unmount } = renderHook(() => useLogStreams([a, b]));

    unmount();

    expect(last().closed).toBe(true);
  });

  it("starts with no lines and a new connection when the pods change", () => {
    const { result, rerender } = renderHook(
      ({ pods }: { pods: LogSource[] }) => useLogStreams(pods),
      { initialProps: { pods: [a, b] } },
    );
    const first = last();
    line(0, "old");
    first.emit("failure", { source: 1, message: "broke" });

    rerender({ pods: [a] });

    expect(result.current).toEqual({ lines: [], errors: [], ended: false });
    expect(first.closed).toBe(true);
    expect(FakeEventSource.instances).toHaveLength(2);
  });

  it("keeps its connection when it gets an equal list of sources", () => {
    const { rerender } = renderHook(
      ({ pods }: { pods: LogSource[] }) => useLogStreams(pods),
      { initialProps: { pods: [a, b] } },
    );

    rerender({ pods: [{ ...a }, { ...b }] });

    expect(FakeEventSource.instances).toHaveLength(1);
  });

  it("applies the level filter to the lines of every log", () => {
    const { result } = renderHook(() =>
      useLogStreams([a, b], {}, { level: "error" }),
    );

    line(0, '{"level":"info","msg":"a1"}');
    line(0, '{"level":"error","msg":"a2"}');
    line(1, '{"level":"ERROR","msg":"b1"}');

    expect(result.current.lines.map((l) => l.message)).toEqual(["a2", "b1"]);
  });
});
