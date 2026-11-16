// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { FakeEventSource } from "../../../../test/fakes/event-source";
import { presetFixtures } from "./fixtures";
import { usePresetSources } from "./sources";

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

describe("usePresetSources", () => {
  it("does not show a failed server read again after its watch answered and the connection dropped", async () => {
    const initial = {
      ...presetFixtures(),
      readErrors: [{ kind: "PluginRequest", message: "forbidden at load" }],
    };
    const { result } = renderHook(() => usePresetSources(initial), {
      wrapper,
    });
    expect(result.current.errors).toEqual(initial.readErrors);

    await waitFor(() =>
      expect(FakeEventSource.instances.at(-1)?.ids()).toHaveLength(13),
    );
    const source = FakeEventSource.instances.at(-1)!;
    for (const watch of source.ids()) {
      const kind = new URLSearchParams(watch).get("kind");
      source.emit("snapshot", { watch, kind, objects: [] });
    }
    await waitFor(() => expect(result.current.pending).toEqual([]));
    expect(result.current.errors).toEqual([]);

    source.fail(0);
    await waitFor(() =>
      expect(result.current.pending).toContain("PluginRequest"),
    );
    expect(result.current.errors).toEqual([]);
  });
});
