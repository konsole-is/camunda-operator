// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { ActionSlot } from "./action-slot";
import { ACTION_SLOTS, type ClusterSlotContext } from "./slots";

afterEach(cleanup);

describe("ActionSlot", () => {
  it.each(ACTION_SLOTS)(
    "renders an empty mount point for %s of a managed cluster",
    (name) => {
      const { container } = render(<ActionSlot name={name} managed />);
      const mount = container.querySelector(`[data-action-slot="${name}"]`);
      expect(mount).not.toBeNull();
      expect(mount?.childNodes.length).toBe(0);
    },
  );

  it("renders the node that fills the slot", () => {
    render(
      <ActionSlot
        name="cluster.header.actions"
        managed
        slots={{ "cluster.header.actions": <button>Delete</button> }}
      />,
    );
    expect(screen.getByRole("button", { name: "Delete" })).toBeTruthy();
  });

  it("renders nothing for an unmanaged cluster, also when the slot is filled", () => {
    const { container } = render(
      <ActionSlot
        name="cluster.header.actions"
        managed={false}
        slots={{ "cluster.header.actions": <button>Delete</button> }}
      />,
    );
    expect(container.innerHTML).toBe("");
  });

  it("calls a render function with the slot context", () => {
    const context = { now: "2026-11-10T12:00:00Z" } as ClusterSlotContext;
    render(
      <ActionSlot
        name="cluster.header.actions"
        managed
        context={context}
        slots={{ "cluster.header.actions": (c) => <p>Now {c.now}</p> }}
      />,
    );
    expect(screen.getByText("Now 2026-11-10T12:00:00Z")).toBeTruthy();
  });

  it("renders nothing for a render function without a context", () => {
    const { container } = render(
      <ActionSlot
        name="cluster.header.actions"
        managed
        slots={{ "cluster.header.actions": () => <p>x</p> }}
      />,
    );
    expect(
      container.querySelector("[data-action-slot]")?.childNodes.length,
    ).toBe(0);
  });
});
