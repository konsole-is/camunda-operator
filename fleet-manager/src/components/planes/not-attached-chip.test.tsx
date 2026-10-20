// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { NotAttachedChip } from "./not-attached-chip";

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

const clusters = [
  {
    namespace: "team-b",
    name: "billing",
    reason: "ClaimedElsewhere",
    message: "plane-b holds it",
  },
  { namespace: "team-c", name: "search", reason: "NotReady" },
];

describe("NotAttachedChip", () => {
  it("renders nothing when every cluster is attached", () => {
    const { container } = render(<NotAttachedChip clusters={[]} />);
    expect(container.firstChild).toBeNull();
  });

  it("shows the count of clusters that are not attached", () => {
    render(<NotAttachedChip clusters={clusters} />);
    expect(screen.getByRole("button", { name: "2 not attached" })).toBeTruthy();
    expect(screen.queryByRole("note")).toBeNull();
  });

  it("lists each cluster with its reason and message on hover", async () => {
    render(<NotAttachedChip clusters={clusters} />);
    await userEvent.hover(
      screen.getByRole("button", { name: "2 not attached" }),
    );
    const note = screen.getByRole("note");
    expect(note.textContent).toContain("team-b/billing");
    expect(note.textContent).toContain("ClaimedElsewhere: plane-b holds it");
    expect(note.textContent).toContain("team-c/search");
    expect(note.textContent).toContain("NotReady");
    await userEvent.unhover(
      screen.getByRole("button", { name: "2 not attached" }),
    );
    expect(screen.queryByRole("note")).toBeNull();
  });

  it("opens on keyboard focus and closes on Escape", async () => {
    render(<NotAttachedChip clusters={clusters} />);
    await userEvent.tab();
    const button = screen.getByRole("button", { name: "2 not attached" });
    expect(document.activeElement).toBe(button);
    expect(button.getAttribute("aria-expanded")).toBe("true");
    expect(screen.getByRole("note").id).toBe(
      button.getAttribute("aria-controls"),
    );
    await userEvent.keyboard("{Escape}");
    expect(screen.queryByRole("note")).toBeNull();
  });

  it("shows the list outside a box that scrolls, so the box does not clip it", () => {
    const { container } = render(
      <div style={{ overflowX: "auto" }}>
        <NotAttachedChip clusters={clusters} defaultOpen />
      </div>,
    );
    const note = screen.getByRole("note");
    expect(container.contains(note)).toBe(false);
    expect(document.body.contains(note)).toBe(true);
  });

  it("closes on Escape while only the pointer is on it", async () => {
    render(<NotAttachedChip clusters={clusters} />);
    await userEvent.hover(
      screen.getByRole("button", { name: "2 not attached" }),
    );
    expect(document.activeElement).toBe(document.body);
    await userEvent.keyboard("{Escape}");
    expect(screen.queryByRole("note")).toBeNull();
  });

  it("fits a long list into the room of the viewport and lets the keyboard scroll it", async () => {
    vi.spyOn(window, "innerHeight", "get").mockReturnValue(800);
    vi.spyOn(Element.prototype, "getBoundingClientRect").mockImplementation(
      function (this: Element) {
        const chip = this.querySelector("button") !== null;
        return {
          top: chip ? 380 : 0,
          bottom: chip ? 400 : 1000,
          left: 0,
          right: chip ? 120 : 320,
          width: chip ? 120 : 320,
          height: chip ? 20 : 1000,
        } as DOMRect;
      },
    );
    render(<NotAttachedChip clusters={clusters} />);
    await userEvent.tab();
    const note = screen.getByRole("note");
    expect(note.parentElement?.style.maxHeight).toBe("392px");
    await userEvent.keyboard("{ArrowDown}");
    expect(document.activeElement).toBe(note);
    expect(screen.getByRole("note")).toBe(note);
    await userEvent.keyboard("{Escape}");
    expect(screen.queryByRole("note")).toBeNull();
    expect(document.activeElement).toBe(
      screen.getByRole("button", { name: "2 not attached" }),
    );
  });

  it("starts open with defaultOpen", () => {
    render(<NotAttachedChip clusters={clusters} defaultOpen />);
    expect(screen.getByRole("note")).toBeTruthy();
  });
});
