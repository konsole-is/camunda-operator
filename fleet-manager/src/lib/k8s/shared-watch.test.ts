// @vitest-environment jsdom
import { QueryClient } from "@tanstack/react-query";
import { act } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { FakeEventSource } from "../../../test/fakes/event-source";
import type { KindName } from "./kinds";
import {
  failureKey,
  freshKey,
  REOPEN_DELAY_MS,
  sharedWatch,
  watchId,
  watchKey,
  type WatchSpec,
} from "./shared-watch";

const flush = () => act(async () => {});

const pod = (namespace: string, name: string, phase = "Pending") => ({
  metadata: { namespace, name },
  status: { phase },
});

const team: WatchSpec = { kind: "Pod", namespace: "team" };
const other: WatchSpec = { kind: "Pod", namespace: "other" };

let client: QueryClient;

beforeEach(() => {
  FakeEventSource.instances = [];
  vi.stubGlobal("EventSource", FakeEventSource);
  client = new QueryClient();
});
afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

const snapshot = (spec: WatchSpec, objects: unknown[]) => ({
  watch: watchId(spec),
  kind: spec.kind,
  objects,
});
const delta = (spec: WatchSpec, type: string, object: unknown) => ({
  watch: watchId(spec),
  type,
  kind: spec.kind,
  object,
});

describe("sharedWatch", () => {
  it("opens one source for all watches of one tick", async () => {
    const kinds: KindName[] = [
      "Pod",
      "Service",
      "PersistentVolumeClaim",
      "Namespace",
      "Secret",
      "Event",
      "Environment",
      "CamundaCluster",
    ];
    const specs = kinds.map((kind) => ({ kind }));
    specs.forEach((spec) => sharedWatch(client).subscribe(spec));
    await flush();

    expect(FakeEventSource.instances).toHaveLength(1);
    const source = FakeEventSource.instances[0];
    expect(new URL(source.url, "http://localhost").pathname).toBe("/api/watch");
    expect(source.ids()).toEqual(specs.map(watchId).sort());
  });

  it("two subscriptions of one spec share one watch", async () => {
    sharedWatch(client).subscribe(team);
    sharedWatch(client).subscribe({ ...team });
    await flush();

    expect(FakeEventSource.instances[0].ids()).toEqual([watchId(team)]);
  });

  it("a snapshot and its deltas reach only the entry of their watch", async () => {
    const watch = sharedWatch(client);
    watch.subscribe(team);
    watch.subscribe(other);
    await flush();
    const source = FakeEventSource.instances[0];

    source.emit("snapshot", snapshot(team, []));
    source.emit("snapshot", snapshot(other, [pod("other", "b")]));
    source.emit("delta", delta(team, "ADDED", pod("team", "a")));

    expect(client.getQueryData(watchKey(team))).toEqual([pod("team", "a")]);
    expect(client.getQueryData(watchKey(other))).toEqual([pod("other", "b")]);
  });

  it("ignores the frames of a source that it closed", async () => {
    const watch = sharedWatch(client);
    watch.subscribe(team);
    await flush();
    const first = FakeEventSource.instances[0];
    first.emit("snapshot", snapshot(team, [pod("team", "a", "Running")]));

    watch.subscribe(other);
    await flush();
    const second = FakeEventSource.instances[1];
    expect(first.closed).toBe(true);
    second.emit("snapshot", snapshot(team, [pod("team", "a", "Succeeded")]));

    first.emit("delta", delta(team, "MODIFIED", pod("team", "a", "Pending")));
    first.emit("snapshot", snapshot(team, []));

    expect(client.getQueryData(watchKey(team))).toEqual([
      pod("team", "a", "Succeeded"),
    ]);
  });

  it("ignores a frame of a watch that it does not hold", async () => {
    sharedWatch(client).subscribe(team);
    await flush();
    const secrets: WatchSpec = { kind: "Secret" };

    FakeEventSource.instances[0].emit("snapshot", snapshot(secrets, []));

    expect(client.getQueryData(watchKey(secrets))).toBeUndefined();
  });

  it("keeps one count per subscribe", async () => {
    const watch = sharedWatch(client);
    const leave = watch.subscribe(team);
    watch.subscribe(team);
    leave();
    leave();
    await flush();

    expect(FakeEventSource.instances).toHaveLength(1);
    expect(FakeEventSource.instances[0].closed).toBe(false);
  });

  it("drops a watch when its last subscriber leaves and closes the source with the last watch", async () => {
    const watch = sharedWatch(client);
    const leaveTeam = watch.subscribe(team);
    const leaveOther = [watch.subscribe(other), watch.subscribe(other)];
    await flush();
    const first = FakeEventSource.instances[0];

    leaveOther.forEach((leave) => leave());
    await flush();
    expect(first.closed).toBe(true);
    const second = FakeEventSource.instances[1];
    expect(second.ids()).toEqual([watchId(team)]);

    leaveTeam();
    await flush();
    expect(second.closed).toBe(true);
    expect(FakeEventSource.instances).toHaveLength(2);
  });

  it("keeps the source when a watch is dropped and added in one tick", async () => {
    const watch = sharedWatch(client);
    const leave = watch.subscribe(team);
    await flush();
    const source = FakeEventSource.instances[0];
    source.emit("failure", {
      watch: watchId(team),
      kind: "Pod",
      message: "The API server is down",
    });

    leave();
    watch.subscribe(team);
    await flush();

    expect(FakeEventSource.instances).toHaveLength(1);
    expect(source.closed).toBe(false);
    expect(client.getQueryData(failureKey(team))).toBe(
      "The API server is down",
    );
  });

  it("a failure keeps the list, and the next snapshot clears it", async () => {
    sharedWatch(client).subscribe(team);
    await flush();
    const source = FakeEventSource.instances[0];
    source.emit("snapshot", snapshot(team, [pod("team", "a")]));

    source.emit("failure", {
      watch: watchId(team),
      kind: "Pod",
      message: "The API server is down",
    });
    expect(client.getQueryData(watchKey(team))).toEqual([pod("team", "a")]);
    expect(client.getQueryData(failureKey(team))).toBe(
      "The API server is down",
    );

    source.fail(0);
    source.emit("snapshot", snapshot(team, [pod("team", "b")]));
    expect(client.getQueryData(watchKey(team))).toEqual([pod("team", "b")]);
    expect(client.getQueryData(failureKey(team))).toBeNull();
    expect(FakeEventSource.instances).toHaveLength(1);
  });

  it("opens a new source after the browser closed the old one", async () => {
    vi.useFakeTimers({ toFake: ["setTimeout", "clearTimeout"] });
    sharedWatch(client).subscribe(team);
    await flush();

    FakeEventSource.instances[0].fail(2);
    vi.advanceTimersByTime(REOPEN_DELAY_MS - 1);
    expect(FakeEventSource.instances).toHaveLength(1);
    vi.advanceTimersByTime(1);
    expect(FakeEventSource.instances).toHaveLength(2);
    const second = FakeEventSource.instances[1];
    expect(second.ids()).toEqual([watchId(team)]);

    second.emit("snapshot", snapshot(team, [pod("team", "a")]));
    expect(client.getQueryData(watchKey(team))).toEqual([pod("team", "a")]);
  });

  it("does not reopen after its last watch left", async () => {
    vi.useFakeTimers({ toFake: ["setTimeout", "clearTimeout"] });
    const leave = sharedWatch(client).subscribe(team);
    await flush();

    FakeEventSource.instances[0].fail(2);
    leave();
    await flush();
    vi.advanceTimersByTime(REOPEN_DELAY_MS);

    expect(FakeEventSource.instances).toHaveLength(1);
  });

  it("marks a watch not fresh after a resubscribe, and fresh after its first snapshot", async () => {
    const watch = sharedWatch(client);
    const leave = watch.subscribe(team);
    await flush();
    FakeEventSource.instances[0].emit(
      "snapshot",
      snapshot(team, [pod("team", "a")]),
    );
    expect(client.getQueryData(freshKey(team))).toBe(true);

    leave();
    await flush();
    watch.subscribe(team);
    await flush();
    expect(client.getQueryData(watchKey(team))).toEqual([pod("team", "a")]);
    expect(client.getQueryData(freshKey(team))).toBe(false);

    FakeEventSource.instances
      .at(-1)!
      .emit("snapshot", snapshot(team, [pod("team", "a", "Running")]));
    expect(client.getQueryData(freshKey(team))).toBe(true);
  });

  it("marks its watches not fresh on a new connection until their snapshot", async () => {
    vi.useFakeTimers({ toFake: ["setTimeout", "clearTimeout"] });
    sharedWatch(client).subscribe(team);
    await flush();
    FakeEventSource.instances[0].emit("snapshot", snapshot(team, []));

    FakeEventSource.instances[0].fail(2);
    expect(client.getQueryData(freshKey(team))).toBe(false);
    vi.advanceTimersByTime(REOPEN_DELAY_MS);
    expect(client.getQueryData(freshKey(team))).toBe(false);

    FakeEventSource.instances[1].emit("snapshot", snapshot(team, []));
    expect(client.getQueryData(freshKey(team))).toBe(true);
  });

  it("marks its watches not fresh when the browser connects again by itself, until their snapshot", async () => {
    sharedWatch(client).subscribe(team);
    await flush();
    const source = FakeEventSource.instances[0];
    source.emit("snapshot", snapshot(team, []));

    source.fail(0);
    expect(client.getQueryData(freshKey(team))).toBe(false);

    source.emit("snapshot", snapshot(team, []));
    expect(client.getQueryData(freshKey(team))).toBe(true);
  });
});
