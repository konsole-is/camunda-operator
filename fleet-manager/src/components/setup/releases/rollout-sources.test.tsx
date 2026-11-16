// @vitest-environment jsdom
import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { PluginSection } from "@/generated/crds/pluginsection";
import { rolloutListsFixture } from "./fixtures";
import { useRolloutSources } from "./rollout-sources";
import { ROLLOUT_SOURCE_KINDS } from "./source-kinds";

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

const KINDS = Object.values(ROLLOUT_SOURCE_KINDS);

beforeEach(() => {
  watch.fresh = [];
  watch.failed = {};
});

describe("useRolloutSources", () => {
  it("marks every list stale while no watch has answered", () => {
    const { result } = renderHook(() =>
      useRolloutSources(rolloutListsFixture()),
    );
    expect(result.current.stale).toEqual(KINDS);
    expect(result.current.errors).toEqual([]);
  });

  it("marks no list stale and gives no error once every watch has a snapshot", () => {
    watch.fresh = KINDS;
    const { result } = renderHook(() =>
      useRolloutSources(rolloutListsFixture()),
    );
    expect(result.current.stale).toEqual([]);
    expect(result.current.errors).toEqual([]);
  });

  it("gives the error of a list whose server read failed and whose watch fails", () => {
    watch.fresh = KINDS.filter((k) => k !== "LogicalBackupRDBMS");
    watch.failed = { LogicalBackupRDBMS: "forbidden" };
    const { result } = renderHook(() =>
      useRolloutSources({ ...rolloutListsFixture(), rdbmsBackups: [] }),
    );
    expect(result.current.rdbmsBackups).toEqual([]);
    expect(result.current.errors).toEqual([
      { kind: "LogicalBackupRDBMS", message: "forbidden" },
    ]);
    expect(result.current.stale).toEqual(["LogicalBackupRDBMS"]);
  });

  it("keeps the items of a good read and gives the error when the watch fails later", () => {
    const initial = {
      ...rolloutListsFixture(),
      sections: [
        {
          apiVersion: "plugins.camunda.io/v1alpha1",
          kind: "PluginSection",
          metadata: { name: "access" },
        } as PluginSection,
      ],
    };
    watch.fresh = KINDS;
    const { result, rerender } = renderHook(() => useRolloutSources(initial));
    watch.fresh = KINDS.filter((k) => k !== "PluginSection");
    watch.failed = { PluginSection: "connection lost" };
    rerender();
    expect(result.current.sections).toEqual(initial.sections);
    expect(result.current.sections).toHaveLength(1);
    expect(result.current.errors).toEqual([
      { kind: "PluginSection", message: "connection lost" },
    ]);
  });

  it("marks a list stale again when its watch reconnects without a snapshot", () => {
    watch.fresh = KINDS;
    const { result, rerender } = renderHook(() =>
      useRolloutSources(rolloutListsFixture()),
    );
    expect(result.current.stale).toEqual([]);
    watch.fresh = KINDS.filter((k) => k !== "LogicalBackupElasticsearch");
    rerender();
    expect(result.current.stale).toEqual(["LogicalBackupElasticsearch"]);
    expect(result.current.errors).toEqual([]);
  });
});
