// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import type { Resource } from "@/lib/k8s/kinds";
import { EventList } from "./event-list";

afterEach(cleanup);

const now = "2026-10-15T12:00:00Z";

const items = () =>
  screen.getAllByRole("listitem").map((li) => li.textContent ?? "");

describe("EventList", () => {
  it("lists the newest event first and marks a warning", () => {
    render(
      <EventList
        now={now}
        events={[
          {
            type: "Normal",
            reason: "Created",
            note: "Created the StatefulSet",
            eventTime: "2026-10-15T10:00:00Z",
          },
          {
            type: "Warning",
            reason: "BackOff",
            note: "Back-off restarting the container",
            deprecatedCount: 3,
            eventTime: "2026-10-15T11:00:00Z",
            deprecatedLastTimestamp: "2026-10-15T11:59:00Z",
          },
        ]}
      />,
    );
    const [first, second] = items();
    expect(first).toContain("BackOff");
    expect(first).toContain("Warning");
    expect(first).toContain("Back-off restarting the container");
    expect(first).toContain("3 times");
    expect(second).toContain("Created");
    expect(second).toContain("Created the StatefulSet");
  });

  it("orders, ages, and counts a recurring event by its series", () => {
    render(
      <EventList
        now={now}
        events={[
          {
            type: "Normal",
            reason: "Scheduled",
            note: "Scheduled the pod",
            eventTime: "2026-10-15T11:00:00Z",
          },
          {
            type: "Warning",
            reason: "Unhealthy",
            note: "Readiness probe failed",
            eventTime: "2026-10-15T10:00:00Z",
            deprecatedCount: 2,
            series: { count: 4, lastObservedTime: "2026-10-15T11:59:00Z" },
          },
        ]}
      />,
    );
    const [first] = items();
    expect(first).toContain("Unhealthy");
    expect(first).toContain("1m");
    expect(first).toContain("4 times");
  });

  it("shows the note and the count of an Event as the events.k8s.io API serves it", () => {
    // An events.k8s.io/v1 response for an Event that the kubelet wrote
    // through core/v1: eventTime is null and the times are deprecated.
    const fromCore = JSON.parse(`{
      "apiVersion": "events.k8s.io/v1",
      "kind": "Event",
      "metadata": {
        "name": "orders-zeebe-0.18a2b3c4d5e6f708",
        "namespace": "orders",
        "uid": "5d2f8c1e-0b7a-4f3e-9c6d-2a1b0e9f8d7c",
        "creationTimestamp": "2026-10-15T11:40:00Z"
      },
      "eventTime": null,
      "reportingController": "kubelet",
      "reportingInstance": "node-a",
      "action": "",
      "reason": "BackOff",
      "regarding": {
        "kind": "Pod",
        "namespace": "orders",
        "name": "orders-zeebe-0",
        "apiVersion": "v1"
      },
      "note": "Back-off restarting failed container zeebe in pod orders-zeebe-0",
      "type": "Warning",
      "deprecatedSource": { "component": "kubelet", "host": "node-a" },
      "deprecatedFirstTimestamp": "2026-10-15T11:40:00Z",
      "deprecatedLastTimestamp": "2026-10-15T11:58:00Z",
      "deprecatedCount": 7
    }`) as Resource<"Event">;

    render(<EventList now={now} events={[fromCore]} />);

    const [item] = items();
    expect(item).toContain("BackOff");
    expect(item).toContain(
      "Back-off restarting failed container zeebe in pod orders-zeebe-0",
    );
    expect(item).toContain("7 times");
    expect(item).toContain("2m");
  });

  it("takes the events.k8s.io fields, not the core/v1 ones", () => {
    const event = {
      eventTime: now,
      // @ts-expect-error message is a core/v1 field. The note holds it.
      message: "Created the StatefulSet",
    } satisfies Resource<"Event">;
    const counted = {
      eventTime: now,
      // @ts-expect-error count is a core/v1 field. deprecatedCount holds it.
      count: 3,
    } satisfies Resource<"Event">;

    expect([event, counted]).toHaveLength(2);
  });

  it("says so when there is no event", () => {
    render(<EventList now={now} events={[]} />);
    expect(screen.getByText("No events")).toBeTruthy();
  });
});
