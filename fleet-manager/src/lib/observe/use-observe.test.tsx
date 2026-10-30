// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { usePartitions, usePodUsage } from "./use-observe";

const wrapper = ({ children }: { children: ReactNode }) => (
  <QueryClientProvider
    client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
  >
    {children}
  </QueryClientProvider>
);

function stubFetch(answer: () => Promise<Response>) {
  const fetchMock = vi.fn((_url: string) => answer());
  vi.stubGlobal("fetch", fetchMock);
  return fetchMock;
}

const usage = [
  { pod: "orders-zeebe-0", cpu: { usedCores: 1 }, memory: { usedBytes: 2 } },
];
const partitions = [{ partition: 1, followers: [], health: "unknown" }];

afterEach(() => vi.unstubAllGlobals());

describe("usePodUsage", () => {
  it("asks /api/usage for the namespace and the selector", async () => {
    const fetchMock = stubFetch(async () => Response.json({ usage }));

    const { result } = renderHook(
      () => usePodUsage("team-a", "camunda.io/cluster=orders"),
      { wrapper },
    );

    await waitFor(() => expect(result.current).toEqual(usage));
    const url = new URL(String(fetchMock.mock.calls[0][0]), "http://x");
    expect(url.pathname).toBe("/api/usage");
    expect(Object.fromEntries(url.searchParams)).toEqual({
      namespace: "team-a",
      selector: "camunda.io/cluster=orders",
    });
  });

  it("is undefined before the first answer", () => {
    stubFetch(() => new Promise(() => {}));

    const { result } = renderHook(() => usePodUsage("team-a", "a=b"), {
      wrapper,
    });

    expect(result.current).toBeUndefined();
  });

  it("gives the unavailable of the answer", async () => {
    stubFetch(async () => Response.json({ usage: "unavailable" }));

    const { result } = renderHook(() => usePodUsage("team-a", "a=b"), {
      wrapper,
    });

    await waitFor(() => expect(result.current).toBe("unavailable"));
  });

  it("is unavailable on an answer other than 200", async () => {
    stubFetch(async () => new Response("no", { status: 502 }));

    const { result } = renderHook(() => usePodUsage("team-a", "a=b"), {
      wrapper,
    });

    await waitFor(() => expect(result.current).toBe("unavailable"));
  });

  it("is unavailable when the fetch fails", async () => {
    stubFetch(async () => {
      throw new TypeError("network");
    });

    const { result } = renderHook(() => usePodUsage("team-a", "a=b"), {
      wrapper,
    });

    await waitFor(() => expect(result.current).toBe("unavailable"));
  });
});

describe("usePartitions", () => {
  it("asks /api/partitions for the namespace and the name", async () => {
    const fetchMock = stubFetch(async () => Response.json({ partitions }));

    const { result } = renderHook(() => usePartitions("team-a", "orders"), {
      wrapper,
    });

    await waitFor(() => expect(result.current).toEqual(partitions));
    const url = new URL(String(fetchMock.mock.calls[0][0]), "http://x");
    expect(url.pathname).toBe("/api/partitions");
    expect(Object.fromEntries(url.searchParams)).toEqual({
      namespace: "team-a",
      name: "orders",
    });
  });

  it("is unavailable on a 404", async () => {
    stubFetch(async () => new Response("gone", { status: 404 }));

    const { result } = renderHook(() => usePartitions("team-a", "orders"), {
      wrapper,
    });

    await waitFor(() => expect(result.current).toBe("unavailable"));
  });

  it("is unavailable when the fetch fails", async () => {
    stubFetch(async () => {
      throw new TypeError("network");
    });

    const { result } = renderHook(() => usePartitions("team-a", "orders"), {
      wrapper,
    });

    await waitFor(() => expect(result.current).toBe("unavailable"));
  });
});
