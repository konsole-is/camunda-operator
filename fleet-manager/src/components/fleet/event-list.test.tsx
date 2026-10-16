// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { EventList } from "./event-list";

afterEach(cleanup);

const now = "2026-10-15T12:00:00Z";

describe("EventList", () => {
  it("lists the newest event first and marks a warning", () => {
    render(
      <EventList
        now={now}
        events={[
          {
            type: "Normal",
            reason: "Created",
            message: "Created the StatefulSet",
            lastTimestamp: "2026-10-15T10:00:00Z",
          },
          {
            type: "Warning",
            reason: "BackOff",
            message: "Back-off restarting the container",
            count: 3,
            lastTimestamp: "2026-10-15T11:59:00Z",
          },
        ]}
      />,
    );
    const items = screen
      .getAllByRole("listitem")
      .map((li) => li.textContent ?? "");
    expect(items[0]).toContain("BackOff");
    expect(items[0]).toContain("Warning");
    expect(items[0]).toContain("3 times");
    expect(items[1]).toContain("Created");
  });

  it("orders, ages, and counts a recurring event by its series", () => {
    render(
      <EventList
        now={now}
        events={[
          {
            type: "Normal",
            reason: "Scheduled",
            message: "Scheduled the pod",
            eventTime: "2026-10-15T11:00:00Z",
          },
          {
            type: "Warning",
            reason: "Unhealthy",
            message: "Readiness probe failed",
            eventTime: "2026-10-15T10:00:00Z",
            series: { count: 4, lastObservedTime: "2026-10-15T11:59:00Z" },
          },
        ]}
      />,
    );
    const items = screen
      .getAllByRole("listitem")
      .map((li) => li.textContent ?? "");
    expect(items[0]).toContain("Unhealthy");
    expect(items[0]).toContain("1m");
    expect(items[0]).toContain("4 times");
  });

  it("says so when there is no event", () => {
    render(<EventList now={now} events={[]} />);
    expect(screen.getByText("No events")).toBeTruthy();
  });
});
