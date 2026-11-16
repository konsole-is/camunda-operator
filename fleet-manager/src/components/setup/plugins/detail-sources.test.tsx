// @vitest-environment jsdom
import { renderHook } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { Resource } from "@/lib/k8s/kinds";
import { useRegistrationDetail } from "./detail-sources";
import { detailData, pluginGrant, registrationEvent } from "./fixtures";

const watch = vi.hoisted(() => ({
  fresh: [] as string[],
  failed: {} as Record<string, string>,
  opts: {} as Record<string, unknown>,
}));
vi.mock("@/lib/k8s/hooks", () => ({
  useResources: (kind: string, opts: unknown, initial: unknown[]) => {
    watch.opts[kind] = opts;
    return {
      items: initial,
      ...(watch.failed[kind] && { error: watch.failed[kind] }),
    };
  },
  useWatchesFresh: (lists: { kind: string }[]) =>
    lists.map(({ kind }) => watch.fresh.includes(kind)),
}));

afterEach(() => {
  watch.fresh = [];
  watch.failed = {};
  watch.opts = {};
});

const withUid = (uid: string) => {
  const data = detailData("elasticsearch");
  data.sources.registrations = data.sources.registrations.map((r) =>
    r.metadata.name === "elasticsearch"
      ? { ...r, metadata: { ...r.metadata, uid } }
      : r,
  );
  return data;
};

const eventOf = (uid: string): Resource<"Event"> => {
  const event = registrationEvent("elasticsearch", "Normal", "Accepted", uid);
  return {
    ...event,
    metadata: { ...event.metadata, uid: `event-${uid}` },
    regarding: { ...event.regarding, uid },
  };
};

describe("useRegistrationDetail", () => {
  it("follows the Grants that carry the registration label, in every namespace", () => {
    const grant = pluginGrant(
      "team-orders",
      "orders-access",
      {},
      {
        "plugins.camunda.io/registration": "elasticsearch",
      },
    );
    const { result } = renderHook(() =>
      useRegistrationDetail({
        ...detailData("elasticsearch"),
        grants: [grant],
      }),
    );
    expect(watch.opts.Grant).toEqual({
      labelSelector: "plugins.camunda.io/registration=elasticsearch",
    });
    expect(result.current.grants).toEqual([grant]);
  });

  it("keeps only the events of the live registration", () => {
    const { result } = renderHook(() =>
      useRegistrationDetail({
        ...withUid("new"),
        events: [eventOf("old"), eventOf("new")],
      }),
    );
    expect(result.current.events.map((e) => e.note)).toEqual(["new"]);
  });

  it("marks Grant and Event stale until their watches have a fresh snapshot", () => {
    watch.fresh = [
      "PluginSection",
      "PluginRegistration",
      "PluginRequest",
      "Namespace",
      "Environment",
    ];
    const { result, rerender } = renderHook(() =>
      useRegistrationDetail(detailData("elasticsearch")),
    );
    expect(result.current.stale).toEqual(["Grant", "Event"]);
    watch.fresh.push("Grant", "Event");
    rerender();
    expect(result.current.stale).toEqual([]);
  });

  it("gives the error of a failed watch of Grant or Event with its kind", () => {
    watch.failed = { Grant: "forbidden", Event: "gone" };
    const { result } = renderHook(() =>
      useRegistrationDetail(detailData("elasticsearch")),
    );
    expect(result.current.errors).toEqual([
      { kind: "Grant", message: "forbidden" },
      { kind: "Event", message: "gone" },
    ]);
  });
});
