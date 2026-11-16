// @vitest-environment jsdom
import { renderHook } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { registrationSources } from "./fixtures";
import { useRegistrationSources } from "./sources";

const watch = vi.hoisted(() => ({
  fresh: [] as string[],
  failed: {} as Record<string, string>,
}));
vi.mock("@/lib/k8s/hooks", () => ({
  useResources: (kind: string, _opts: unknown, initial: unknown[]) => ({
    items: initial,
    ...(watch.failed[kind] && { error: watch.failed[kind] }),
  }),
  useWatchesFresh: (lists: { kind: string }[]) =>
    lists.map(({ kind }) => watch.fresh.includes(kind)),
}));

describe("useRegistrationSources", () => {
  it("marks each list stale until its watch has a fresh snapshot", () => {
    watch.fresh = [];
    watch.failed = {};
    const { result, rerender } = renderHook(() =>
      useRegistrationSources(registrationSources()),
    );
    expect(result.current.stale).toEqual([
      "PluginSection",
      "PluginRegistration",
      "PluginRequest",
      "Namespace",
      "Environment",
    ]);
    watch.fresh = [
      "PluginSection",
      "PluginRequest",
      "Namespace",
      "Environment",
    ];
    rerender();
    expect(result.current.stale).toEqual(["PluginRegistration"]);
  });

  it("gives the error of each failed watch with its kind", () => {
    watch.fresh = [];
    watch.failed = { PluginRequest: "forbidden" };
    const { result } = renderHook(() =>
      useRegistrationSources(registrationSources()),
    );
    expect(result.current.errors).toEqual([
      { kind: "PluginRequest", message: "forbidden" },
    ]);
  });
});
