// @vitest-environment jsdom
import { renderHook } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { releaseFixtures } from "./fixtures";
import { useReleaseSources } from "./sources";

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

describe("useReleaseSources", () => {
  it("marks each list stale until its watch has a fresh snapshot", () => {
    watch.fresh = [];
    watch.failed = {};
    const { result, rerender } = renderHook(() =>
      useReleaseSources(releaseFixtures()),
    );
    expect(result.current.stale).toEqual([
      "CamundaRelease",
      "Environment",
      "Namespace",
      "CamundaCluster",
      "ElasticsearchCluster",
      "DatabaseServer",
      "CamundaClusterClaim",
    ]);
    watch.fresh = [
      "CamundaRelease",
      "Environment",
      "Namespace",
      "CamundaCluster",
      "ElasticsearchCluster",
      "DatabaseServer",
    ];
    rerender();
    expect(result.current.stale).toEqual(["CamundaClusterClaim"]);
  });

  it("gives the error of each failed watch with its kind", () => {
    watch.fresh = [];
    watch.failed = { CamundaClusterClaim: "forbidden" };
    const { result } = renderHook(() => useReleaseSources(releaseFixtures()));
    expect(result.current.errors).toEqual([
      { kind: "CamundaClusterClaim", message: "forbidden" },
    ]);
  });
});
