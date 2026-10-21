// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { ActionSlot } from "./action-slot";
import { ACTION_SLOTS } from "./slots";

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
});
