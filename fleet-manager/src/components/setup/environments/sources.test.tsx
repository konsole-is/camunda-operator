// @vitest-environment jsdom
import { renderHook } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { environmentFixtures } from "./fixtures";
import { useEnvironmentSources } from "./sources";

const watch = vi.hoisted(() => ({ fresh: [] as string[] }));
vi.mock("@/lib/k8s/hooks", () => ({
  useResources: (_kind: string, _opts: unknown, initial: unknown[]) => ({
    items: initial,
  }),
  useWatchesFresh: (lists: { kind: string }[]) =>
    lists.map(({ kind }) => watch.fresh.includes(kind)),
}));

describe("useEnvironmentSources", () => {
  it("marks the namespaces stale until their watch has a fresh snapshot", () => {
    watch.fresh = [];
    const { result, rerender } = renderHook(() =>
      useEnvironmentSources(environmentFixtures()),
    );
    expect(result.current.namespacesStale).toBe(true);
    watch.fresh = ["Namespace"];
    rerender();
    expect(result.current.namespacesStale).toBe(false);
  });
});
