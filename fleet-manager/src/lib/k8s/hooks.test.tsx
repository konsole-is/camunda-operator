// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  act,
  cleanup,
  render,
  renderHook,
  waitFor,
} from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { FakeEventSource } from "../../../test/fakes/event-source";
import { useResource, useResources } from "./hooks";
import { watchId } from "./shared-watch";

// TanStack Query sends cache updates to the hooks on a later tick, so the
// tests wait for the result after each frame. The hooks open their one
// connection in a microtask, so the tests wait for it too.

const pod = (name: string, phase = "Pending", namespace = "team") =>
  ({
    apiVersion: "v1",
    kind: "Pod",
    metadata: { namespace, name },
    status: { phase },
  }) as never;

let wrapper: (props: { children: ReactNode }) => ReactNode;

beforeEach(() => {
  FakeEventSource.instances = [];
  vi.stubGlobal("EventSource", FakeEventSource);
  const client = new QueryClient();
  wrapper = ({ children }) => (
    <QueryClientProvider client={client}>{children}</QueryClientProvider>
  );
});
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

const flush = () => act(async () => {});

// source waits for the connection that the hooks opened last.
async function source(): Promise<FakeEventSource> {
  await waitFor(() =>
    expect(FakeEventSource.instances.length).toBeGreaterThan(0),
  );
  return FakeEventSource.instances.at(-1)!;
}

describe("useResources", () => {
  it("returns the server snapshot at once", () => {
    const { result } = renderHook(
      () => useResources("Pod", { namespace: "team" }, [pod("a")]),
      { wrapper },
    );
    expect(result.current).toEqual({ items: [pod("a")] });
  });

  it("watches the kind, the namespace, and the selector", async () => {
    renderHook(
      () =>
        useResources(
          "Pod",
          { namespace: "team", labelSelector: "app=web" },
          [],
        ),
      { wrapper },
    );
    const s = await source();
    expect(new URL(s.url, "http://localhost").pathname).toBe("/api/watch");
    expect(s.ids()).toHaveLength(1);
    expect(Object.fromEntries(new URLSearchParams(s.ids()[0]))).toEqual({
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
    const s = await source();
    s.emit("snapshot", {
      watch: s.ids()[0],
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
    const s = await source();
    const watch = s.ids()[0];
    s.emit("delta", {
      watch,
      type: "MODIFIED",
      kind: "Pod",
      object: pod("a", "Running"),
    });
    s.emit("delta", { watch, type: "DELETED", kind: "Pod", object: pod("b") });
    s.emit("delta", { watch, type: "ADDED", kind: "Pod", object: pod("c") });
    await waitFor(() =>
      expect(result.current.items).toEqual([pod("a", "Running"), pod("c")]),
    );
  });

  it("keeps the list and reports a failure frame until the next snapshot", async () => {
    const { result } = renderHook(
      () => useResources("Pod", { namespace: "team" }, [pod("a")]),
      { wrapper },
    );
    const s = await source();
    const watch = s.ids()[0];

    s.emit("failure", {
      watch,
      kind: "Pod",
      message: "The API server is down",
    });
    await waitFor(() =>
      expect(result.current).toEqual({
        items: [pod("a")],
        error: "The API server is down",
      }),
    );

    s.emit("snapshot", { watch, kind: "Pod", objects: [pod("b")] });
    await waitFor(() => expect(result.current).toEqual({ items: [pod("b")] }));
  });

  it("clears the error of the old stream when the filter changes", async () => {
    const { result, rerender } = renderHook(
      ({ namespace }) => useResources("Pod", { namespace }, []),
      { wrapper, initialProps: { namespace: "team" } },
    );
    const s = await source();
    s.emit("failure", {
      watch: s.ids()[0],
      kind: "Pod",
      message: "The API server is down",
    });
    await waitFor(() =>
      expect(result.current.error).toBe("The API server is down"),
    );

    rerender({ namespace: "other" });
    await flush();

    expect(result.current.error).toBeUndefined();

    rerender({ namespace: "team" });
    await flush();

    expect(result.current.error).toBeUndefined();
  });

  it("closes the stream on unmount", async () => {
    const { unmount } = renderHook(() => useResources("Pod", {}, []), {
      wrapper,
    });
    const s = await source();
    unmount();
    await flush();
    expect(s.closed).toBe(true);
  });

  it("a page with 8 hooks of different kinds opens one EventSource", async () => {
    renderHook(
      () => {
        useResources("Pod", {}, []);
        useResources("Service", {}, []);
        useResources("PersistentVolumeClaim", {}, []);
        useResources("Namespace", {}, []);
        useResources("Secret", {}, []);
        useResources("Event", {}, []);
        useResources("Environment", {}, []);
        useResource("CamundaCluster", { namespace: "team", name: "one" });
      },
      { wrapper },
    );
    const s = await source();
    await flush();

    expect(FakeEventSource.instances).toHaveLength(1);
    expect(s.ids()).toHaveLength(8);
  });

  it("a delta reaches only the hooks that match it", async () => {
    const { result } = renderHook(
      () => ({
        team: useResources("Pod", { namespace: "team" }, []),
        other: useResources("Pod", { namespace: "other" }, []),
        one: useResource("Pod", { namespace: "team", name: "a" }),
      }),
      { wrapper },
    );
    const s = await source();
    const team = watchId({ kind: "Pod", namespace: "team" });
    const one = watchId({ kind: "Pod", namespace: "team", name: "a" });
    for (const watch of s.ids()) {
      s.emit("snapshot", { watch, kind: "Pod", objects: [] });
    }

    for (const watch of [team, one]) {
      s.emit("delta", { watch, type: "ADDED", kind: "Pod", object: pod("a") });
    }

    await waitFor(() => expect(result.current.team.items).toEqual([pod("a")]));
    expect(result.current.one.item).toEqual(pod("a"));
    expect(result.current.other.items).toEqual([]);
  });

  it("two hooks with the same kind and filter share one subscription and one ordered entry", async () => {
    const results: { items: unknown[] }[] = [];
    function TeamPods({ slot }: { slot: number }) {
      results[slot] = useResources("Pod", { namespace: "team" }, []);
      return null;
    }
    function Services() {
      useResources("Service", { namespace: "team" }, []);
      return null;
    }
    const tree = (services: boolean) => (
      <>
        <TeamPods slot={0} />
        <TeamPods slot={1} />
        {services && <Services />}
      </>
    );
    const { rerender } = render(tree(false), { wrapper });
    const first = await source();
    expect(first.ids()).toHaveLength(1);
    const watch = first.ids()[0];
    first.emit("snapshot", { watch, kind: "Pod", objects: [pod("a")] });
    first.emit("delta", {
      watch,
      type: "MODIFIED",
      kind: "Pod",
      object: pod("a", "Running"),
    });

    rerender(tree(true));
    await waitFor(() => expect(FakeEventSource.instances).toHaveLength(2));
    const second = FakeEventSource.instances[1];
    second.emit("snapshot", {
      watch,
      kind: "Pod",
      objects: [pod("a", "Succeeded")],
    });
    first.emit("delta", {
      watch,
      type: "MODIFIED",
      kind: "Pod",
      object: pod("a", "Pending"),
    });

    await waitFor(() =>
      expect(results.map((r) => r.items)).toEqual([
        [pod("a", "Succeeded")],
        [pod("a", "Succeeded")],
      ]),
    );
  });

  it("drops the watch of the last hook and closes the connection with the last hook", async () => {
    function Pods() {
      useResources("Pod", {}, []);
      return null;
    }
    function Services() {
      useResources("Service", {}, []);
      return null;
    }
    const { rerender, unmount } = render(
      <>
        <Pods />
        <Services />
      </>,
      { wrapper },
    );
    const first = await source();
    expect(first.ids()).toHaveLength(2);

    rerender(
      <>
        <Pods />
      </>,
    );
    await waitFor(() => expect(FakeEventSource.instances).toHaveLength(2));
    const second = FakeEventSource.instances[1];
    expect(first.closed).toBe(true);
    expect(second.ids()).toEqual(["kind=Pod"]);

    unmount();
    await flush();
    expect(second.closed).toBe(true);
    expect(FakeEventSource.instances).toHaveLength(2);
  });

  it("shows the error after a failure and recovers on the next snapshot", async () => {
    const { result } = renderHook(
      () => useResources("Pod", { namespace: "team" }, [pod("a")]),
      { wrapper },
    );
    const s = await source();
    const watch = s.ids()[0];

    s.emit("failure", {
      watch,
      kind: "Pod",
      message: "The API server is down",
    });
    s.fail(0);
    await waitFor(() =>
      expect(result.current).toEqual({
        items: [pod("a")],
        error: "The API server is down",
      }),
    );

    s.emit("snapshot", { watch, kind: "Pod", objects: [pod("b")] });
    await waitFor(() => expect(result.current).toEqual({ items: [pod("b")] }));
  });
});

describe("useResource", () => {
  it("watches one name and follows its changes", async () => {
    const { result } = renderHook(
      () => useResource("Pod", { namespace: "team", name: "a" }, pod("a")),
      { wrapper },
    );
    const s = await source();
    const watch = s.ids()[0];
    expect(new URLSearchParams(watch).get("name")).toBe("a");

    s.emit("delta", {
      watch,
      type: "MODIFIED",
      kind: "Pod",
      object: pod("a", "Running"),
    });
    await waitFor(() =>
      expect(result.current.item).toEqual(pod("a", "Running")),
    );

    s.emit("delta", { watch, type: "DELETED", kind: "Pod", object: pod("a") });
    await waitFor(() => expect(result.current.item).toBeUndefined());

    s.emit("failure", {
      watch,
      kind: "Pod",
      message: "The API server is down",
    });
    await waitFor(() =>
      expect(result.current.error).toBe("The API server is down"),
    );
  });
});
