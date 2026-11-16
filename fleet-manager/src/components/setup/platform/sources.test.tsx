// @vitest-environment jsdom
import { renderHook } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { platformFixtures } from "./fixtures";
import { usePlatformSources } from "./sources";

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

const KINDS = [
  "CamundaPlatformConfig",
  "Environment",
  "Namespace",
  "CamundaCluster",
  "CamundaManagementCluster",
  "DatabaseServer",
  "CamundaClusterPreset",
  "DatabaseServerPreset",
  "CamundaRelease",
  "CamundaClusterClaim",
  "CamundaManagementClusterClaim",
];

describe("usePlatformSources", () => {
  it("marks each list stale until its watch has a fresh snapshot", () => {
    watch.fresh = [];
    watch.failed = {};
    const { result, rerender } = renderHook(() =>
      usePlatformSources(platformFixtures()),
    );
    expect(result.current.stale).toEqual(KINDS);

    watch.fresh = KINDS.filter((k) => k !== "DatabaseServerPreset");
    rerender();
    expect(result.current.stale).toEqual(["DatabaseServerPreset"]);
  });

  it("gives the error of each failed watch with its kind", () => {
    watch.fresh = KINDS;
    watch.failed = { CamundaManagementClusterClaim: "forbidden" };
    const { result } = renderHook(() => usePlatformSources(platformFixtures()));
    expect(result.current.errors).toEqual([
      { kind: "CamundaManagementClusterClaim", message: "forbidden" },
    ]);
  });
});
