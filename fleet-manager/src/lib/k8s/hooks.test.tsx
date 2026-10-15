// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useResource, useResources } from "./hooks";

// TanStack Query sends cache updates to the hooks on a later tick, so the
// tests wait for the result after each frame.

// FakeEventSource stands in for the browser EventSource, so a test can send
// the frames of /api/watch.
class FakeEventSource {
  static instances: FakeEventSource[] = [];
  closed = false;
  private readonly listeners = new Map<
    string,
    ((event: MessageEvent) => void)[]
  >();
  constructor(readonly url: string) {
    FakeEventSource.instances.push(this);
  }
  addEventListener(type: string, listener: (event: MessageEvent) => void) {
    this.listeners.set(type, [...(this.listeners.get(type) ?? []), listener]);
  }
  close() {
    this.closed = true;
  }
  emit(type: string, data: unknown) {
    act(() =>
      this.listeners
        .get(type)
        ?.forEach((l) =>
          l(new MessageEvent(type, { data: JSON.stringify(data) })),
        ),
    );
  }
}

const pod = (name: string, phase = "Pending") =>
  ({
    apiVersion: "v1",
    kind: "Pod",
    metadata: { namespace: "team", name },
    status: { phase },
  }) as never;

function wrapper({ children }: { children: ReactNode }) {
  return (
    <QueryClientProvider client={new QueryClient()}>
      {children}
    </QueryClientProvider>
  );
}

beforeEach(() => {
  FakeEventSource.instances = [];
  vi.stubGlobal("EventSource", FakeEventSource);
});
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("useResources", () => {
  it("returns the server snapshot at once", () => {
    const { result } = renderHook(
      () => useResources("Pod", { namespace: "team" }, [pod("a")]),
      { wrapper },
    );
    expect(result.current).toEqual({ items: [pod("a")] });
  });

  it("watches the kind, the namespace, and the selector", () => {
    renderHook(
      () =>
        useResources(
          "Pod",
          { namespace: "team", labelSelector: "app=web" },
          [],
        ),
      { wrapper },
    );
    const url = new URL(FakeEventSource.instances[0].url, "http://localhost");
    expect(url.pathname).toBe("/api/watch");
    expect(Object.fromEntries(url.searchParams)).toEqual({
      kind: "Pod",
      namespace: "team",
      labelSelector: "app=web",
    });
  });

  it("a snapshot replaces the list", async () => {
    const { result } = renderHook(
      () => useResources("Pod", { namespace: "team" }, [pod("gone")]),
      { wrapper },
    );
    FakeEventSource.instances[0].emit("snapshot", {
      kind: "Pod",
      objects: [pod("a"), pod("b")],
    });
    await waitFor(() =>
      expect(result.current.items).toEqual([pod("a"), pod("b")]),
    );
  });

  it("applies each delta", async () => {
    const { result } = renderHook(
      () => useResources("Pod", { namespace: "team" }, [pod("a"), pod("b")]),
      { wrapper },
    );
    const source = FakeEventSource.instances[0];
    source.emit("delta", {
      type: "MODIFIED",
      kind: "Pod",
      object: pod("a", "Running"),
    });
    source.emit("delta", { type: "DELETED", kind: "Pod", object: pod("b") });
    source.emit("delta", { type: "ADDED", kind: "Pod", object: pod("c") });
    await waitFor(() =>
      expect(result.current.items).toEqual([pod("a", "Running"), pod("c")]),
    );
  });

  it("keeps the list and reports a failure frame until the next snapshot", async () => {
    const { result } = renderHook(
      () => useResources("Pod", { namespace: "team" }, [pod("a")]),
      { wrapper },
    );
    const source = FakeEventSource.instances[0];

    source.emit("failure", { kind: "Pod", message: "The API server is down" });
    await waitFor(() =>
      expect(result.current).toEqual({
        items: [pod("a")],
        error: "The API server is down",
      }),
    );

    source.emit("snapshot", { kind: "Pod", objects: [pod("b")] });
    await waitFor(() => expect(result.current).toEqual({ items: [pod("b")] }));
  });

  it("closes the stream on unmount", () => {
    const { unmount } = renderHook(() => useResources("Pod", {}, []), {
      wrapper,
    });
    unmount();
    expect(FakeEventSource.instances[0].closed).toBe(true);
  });
});

describe("useResource", () => {
  it("watches one name and follows its changes", async () => {
    const { result } = renderHook(
      () => useResource("Pod", { namespace: "team", name: "a" }, pod("a")),
      { wrapper },
    );
    const source = FakeEventSource.instances[0];
    expect(
      new URL(source.url, "http://localhost").searchParams.get("name"),
    ).toBe("a");

    source.emit("delta", {
      type: "MODIFIED",
      kind: "Pod",
      object: pod("a", "Running"),
    });
    await waitFor(() =>
      expect(result.current.item).toEqual(pod("a", "Running")),
    );

    source.emit("delta", { type: "DELETED", kind: "Pod", object: pod("a") });
    await waitFor(() => expect(result.current.item).toBeUndefined());

    source.emit("failure", { kind: "Pod", message: "The API server is down" });
    await waitFor(() =>
      expect(result.current.error).toBe("The API server is down"),
    );
  });
});
