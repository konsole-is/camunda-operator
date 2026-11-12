// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { Resource } from "@/lib/k8s/kinds";
import { DetailEvents } from "./detail-events";

const live = vi.hoisted(() => ({
  calls: [] as { kind: string; opts: unknown }[],
  items: {} as Record<string, unknown[]>,
  errors: {} as Record<string, string>,
}));
vi.mock("@/lib/k8s/hooks", () => ({
  useResources: (kind: string, opts: unknown) => {
    live.calls.push({ kind, opts });
    const error = live.errors[kind];
    return { items: live.items[kind] ?? [], ...(error && { error }) };
  },
}));

beforeEach(() => {
  live.calls = [];
  live.items = {};
  live.errors = {};
});
afterEach(cleanup);

const SELECTOR = "camunda.io/cluster=orders,camunda.io/component=zeebe";
const NOW = "2026-11-10T12:00:00Z";
const brokers = [
  {
    metadata: {
      name: "orders-zeebe-0",
      namespace: "team-a",
      ownerReferences: [
        {
          apiVersion: "apps/v1",
          kind: "StatefulSet",
          name: "orders-zeebe",
          uid: "s",
        },
      ],
    },
  },
] as Resource<"Pod">[];
const event = (reason: string, kind: string, name: string, uid?: string) => ({
  metadata: { name: `${name}.${reason}`, namespace: "team-a", uid: reason },
  reason,
  eventTime: "2026-11-10T11:00:00Z",
  regarding: { kind, name, namespace: "team-a", ...(uid && { uid }) },
});

function show() {
  render(
    <DetailEvents
      namespace="team-a"
      selector={SELECTOR}
      pods={brokers}
      refs={[
        {
          kind: "CamundaCluster",
          name: "orders",
          namespace: "team-a",
          uid: "u-1",
        },
      ]}
      now={NOW}
    />,
  );
}

describe("DetailEvents", () => {
  it("watches the events of the namespace and the Services and PVCs of the selector", () => {
    show();

    expect(live.calls).toEqual(
      expect.arrayContaining([
        { kind: "Event", opts: { namespace: "team-a" } },
        {
          kind: "Service",
          opts: { namespace: "team-a", labelSelector: SELECTOR },
        },
        {
          kind: "PersistentVolumeClaim",
          opts: { namespace: "team-a", labelSelector: SELECTOR },
        },
      ]),
    );
  });

  it("lists only the events of the component, each with its object", () => {
    live.items = {
      Event: [
        event("Started", "Pod", "orders-zeebe-0"),
        event("Resized", "PersistentVolumeClaim", "data-orders-zeebe-0"),
        event("Synced", "Service", "orders-zeebe"),
        event("CreatedStatefulSet", "CamundaCluster", "orders", "u-1"),
        event("Pulled", "Pod", "orders-gateway-0"),
      ],
      Service: [{ metadata: { name: "orders-zeebe" } }],
      PersistentVolumeClaim: [{ metadata: { name: "data-orders-zeebe-0" } }],
    };

    show();

    const items = screen.getAllByRole("listitem").map((li) => li.textContent);
    expect(items).toHaveLength(4);
    expect(items.join("\n")).toContain("Pod/orders-zeebe-0");
    expect(items.join("\n")).toContain("CamundaCluster/orders");
    expect(items.join("\n")).not.toContain("Pulled");
  });

  it("names a read that failed and does not say that there are no events", () => {
    live.errors = { Event: "The API server refused the list of Event" };

    show();

    expect(
      screen.getByText(
        "Could not read Event: The API server refused the list of Event",
      ),
    ).toBeTruthy();
    expect(screen.queryByText("No events")).toBeNull();
  });

  it("lists the events it has and says that the list can be incomplete when a Service read failed", () => {
    live.items = { Event: [event("Started", "Pod", "orders-zeebe-0")] };
    live.errors = { Service: "watch broke" };

    show();

    expect(
      screen.getByText("Could not read Service: watch broke"),
    ).toBeTruthy();
    expect(
      screen.getByText("The list can miss events, because a read failed."),
    ).toBeTruthy();
    expect(
      screen
        .getAllByRole("listitem")
        .map((li) => li.textContent)
        .join(),
    ).toContain("Pod/orders-zeebe-0");
  });

  it("says so when the component has no events", () => {
    show();

    expect(screen.getByText("No events")).toBeTruthy();
  });
});
