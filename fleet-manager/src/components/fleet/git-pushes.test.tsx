// @vitest-environment jsdom
import { act, cleanup, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useGitGone, useGitSync } from "@/lib/k8s/hooks";
import type { WriteResult } from "@/lib/write/types";
import { GitPushesProvider, useGitPushes } from "./git-pushes";

vi.mock("@/lib/k8s/hooks", async (importOriginal) => {
  const hooks = await importOriginal<typeof import("@/lib/k8s/hooks")>();
  return { ...hooks, useGitSync: vi.fn(), useGitGone: vi.fn() };
});

beforeEach(() => {
  vi.mocked(useGitSync).mockReturnValue("waiting");
  vi.mocked(useGitGone).mockReturnValue("waiting");
});
afterEach(cleanup);

const pushedAt = "2026-10-21T12:00:00Z";
const gitResult = (revision: string): WriteResult => ({
  ok: true,
  mode: "git",
  commit: "c0ffee",
  revisions: {
    "CamundaClusterClaim/team-a/orders": revision,
    "Namespace//team-a": "ns-rev",
  },
  paths: {
    "CamundaClusterClaim/team-a/orders":
      "fleet/team-a/camundaclusterclaims/orders.yaml",
    "Namespace//team-a": "fleet/_cluster/namespaces/team-a.yaml",
  },
  deleted: {},
});
const deleteResult: WriteResult = {
  ok: true,
  mode: "git",
  commit: "d00d",
  revisions: {},
  paths: {},
  deleted: {
    "CamundaClusterClaim/team-a/orders":
      "fleet/team-a/camundaclusterclaims/orders.yaml",
  },
};

const render = () =>
  renderHook(() => useGitPushes(), { wrapper: GitPushesProvider });

describe("useGitPushes", () => {
  it("record adds one pending push for each revision of a Git result, with its path and ref", () => {
    const { result } = render();

    act(() => result.current.record(gitResult("abc"), pushedAt));

    expect(result.current.pushes).toEqual([
      {
        key: "CamundaClusterClaim/team-a/orders",
        ref: {
          kind: "CamundaClusterClaim",
          namespace: "team-a",
          name: "orders",
        },
        revision: "abc",
        path: "fleet/team-a/camundaclusterclaims/orders.yaml",
        pushedAt,
      },
      {
        key: "Namespace//team-a",
        ref: { kind: "Namespace", name: "team-a" },
        revision: "ns-rev",
        path: "fleet/_cluster/namespaces/team-a.yaml",
        pushedAt,
      },
    ]);
  });

  it("record ignores a direct result and a failure", () => {
    const { result } = render();

    act(() => {
      result.current.record({ ok: true, mode: "direct" }, pushedAt);
      result.current.record(
        { ok: false, errors: [{ code: "Stale", message: "changed" }] },
        pushedAt,
      );
    });

    expect(result.current.pushes).toEqual([]);
  });

  it("settle removes the push", () => {
    const { result } = render();
    act(() => result.current.record(gitResult("abc"), pushedAt));

    act(() => result.current.settle("Namespace//team-a"));

    expect(result.current.pushes.map((p) => p.key)).toEqual([
      "CamundaClusterClaim/team-a/orders",
    ]);
  });

  it("a second push of the same object replaces the first", () => {
    const { result } = render();
    act(() => result.current.record(gitResult("abc"), pushedAt));

    act(() => result.current.record(gitResult("def"), "2026-10-21T12:05:00Z"));

    const orders = result.current.pushes.filter(
      (p) => p.key === "CamundaClusterClaim/team-a/orders",
    );
    expect(orders).toHaveLength(1);
    expect(orders[0]).toEqual(
      expect.objectContaining({
        revision: "def",
        pushedAt: "2026-10-21T12:05:00Z",
      }),
    );
  });

  it("record adds one pending push for each deleted object, with its path and ref", () => {
    const { result } = render();

    act(() => result.current.record(deleteResult, pushedAt));

    expect(result.current.pushes).toEqual([
      {
        key: "CamundaClusterClaim/team-a/orders",
        ref: {
          kind: "CamundaClusterClaim",
          namespace: "team-a",
          name: "orders",
        },
        path: "fleet/team-a/camundaclusterclaims/orders.yaml",
        pushedAt,
        deleted: true,
      },
    ]);
  });

  it("a delete of an object replaces its pending write", () => {
    const { result } = render();
    act(() => result.current.record(gitResult("abc"), pushedAt));

    act(() => result.current.record(deleteResult, "2026-10-21T12:05:00Z"));

    expect(result.current.pushes).toEqual([
      expect.objectContaining({ key: "Namespace//team-a", revision: "ns-rev" }),
      {
        key: "CamundaClusterClaim/team-a/orders",
        ref: {
          kind: "CamundaClusterClaim",
          namespace: "team-a",
          name: "orders",
        },
        path: "fleet/team-a/camundaclusterclaims/orders.yaml",
        pushedAt: "2026-10-21T12:05:00Z",
        deleted: true,
      },
    ]);
  });

  it("the provider settles a write when the object carries its revision", async () => {
    const { result, rerender } = render();
    act(() => result.current.record(gitResult("abc"), pushedAt));

    vi.mocked(useGitSync).mockImplementation((ref) =>
      ref.kind === "Namespace" ? "applied" : "waiting",
    );
    await act(async () => rerender());

    expect(result.current.pushes.map((p) => p.key)).toEqual([
      "CamundaClusterClaim/team-a/orders",
    ]);
  });

  it("the provider settles a delete when the object is gone", async () => {
    const { result, rerender } = render();
    act(() => result.current.record(deleteResult, pushedAt));
    expect(useGitGone).toHaveBeenCalledWith({
      kind: "CamundaClusterClaim",
      namespace: "team-a",
      name: "orders",
    });

    vi.mocked(useGitGone).mockReturnValue("applied");
    await act(async () => rerender());

    expect(result.current.pushes).toEqual([]);
  });

  it("throws without a GitPushesProvider", () => {
    expect(() => renderHook(() => useGitPushes())).toThrow("GitPushesProvider");
  });
});
