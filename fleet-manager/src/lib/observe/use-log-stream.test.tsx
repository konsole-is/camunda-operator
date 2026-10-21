// @vitest-environment jsdom
import { act, cleanup, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { FakeEventSource } from "../../../test/fakes/event-source";
import type { LogFilter } from "./logs";
import { type LogSource, useLogStream } from "./use-log-stream";

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
