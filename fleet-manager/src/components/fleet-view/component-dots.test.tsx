// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it } from "vitest";
import { ComponentDots } from "./component-dots";
import type { ComponentDot } from "./model";

afterEach(cleanup);

const dots: ComponentDot[] = [
  {
    id: "zeebe",
    label: "Brokers",
    used: true,
    condition: {
      status: "False",
      reason: "PodsNotReady",
      message: "2 of 3 brokers are ready",
    },
  },
  {
    id: "gateway",
    label: "Gateway",
    used: true,
    condition: { status: "True", reason: "Healthy" },
  },
  { id: "connectors", label: "Connectors", used: false },
];

const trigger = () => screen.getByRole("button", { name: /Components/ });

describe("ComponentDots", () => {
  it("shows a dot for each component and a grey dot for a component that is not used", () => {
    render(<ComponentDots dots={dots} />);
    expect(
      screen.getByRole("img", { name: "Brokers: Not ready (PodsNotReady)" }),
    ).toBeTruthy();
    expect(screen.getByRole("img", { name: "Gateway: Ready" })).toBeTruthy();
    expect(
      screen.getByRole("img", { name: "Connectors: Not used" }),
    ).toBeTruthy();
  });

  it("is one stop of the keyboard for all its dots", async () => {
    render(
      <>
        <ComponentDots dots={dots} />
        <ComponentDots dots={dots.slice(0, 1)} />
      </>,
    );
    const [first, second] = screen.getAllByRole("button", {
      name: /Components/,
    });
    await userEvent.tab();
    expect(document.activeElement).toBe(first);
    await userEvent.tab();
    expect(document.activeElement).toBe(second);
  });

  it("lists each component with its state and message on keyboard focus", async () => {
    render(<ComponentDots dots={dots} />);
    expect(screen.queryByRole("note")).toBeNull();
    await userEvent.tab();
    const note = screen.getByRole("note");
    expect(note.id).toBe(trigger().getAttribute("aria-controls"));
    expect(note.textContent).toContain(
      "Brokers: Not ready (PodsNotReady). 2 of 3 brokers are ready",
    );
    expect(note.textContent).toContain("Gateway: Ready");
    expect(note.textContent).toContain("Connectors: Not used");
    await userEvent.keyboard("{Escape}");
    expect(screen.queryByRole("note")).toBeNull();
    expect(document.activeElement).toBe(trigger());
  });

  it("does not announce a dialog for its list", () => {
    render(<ComponentDots dots={dots} />);
    expect(trigger().getAttribute("aria-haspopup")).toBeNull();
    expect(trigger().getAttribute("aria-expanded")).toBe("false");
  });

  it("lists the components on hover and closes when the pointer leaves", async () => {
    render(<ComponentDots dots={dots} />);
    await userEvent.hover(trigger());
    expect(screen.getByRole("note")).toBeTruthy();
    await userEvent.unhover(trigger());
    expect(screen.queryByRole("note")).toBeNull();
  });

  it("keeps the list open while the pointer moves from the dots into the list", async () => {
    render(<ComponentDots dots={dots} />);
    await userEvent.hover(trigger());
    await userEvent.hover(screen.getByRole("note"));
    expect(screen.getByRole("note")).toBeTruthy();
  });

  it("keeps the focus where it is when Escape closes a list that hover opened", async () => {
    render(
      <>
        <input aria-label="Search clusters" />
        <ComponentDots dots={dots} />
      </>,
    );
    const search = screen.getByRole("textbox", { name: "Search clusters" });
    search.focus();
    await userEvent.hover(trigger());
    expect(screen.getByRole("note")).toBeTruthy();
    await userEvent.keyboard("{Escape}");
    expect(screen.queryByRole("note")).toBeNull();
    expect(document.activeElement).toBe(search);
  });

  it("moves the focus into the list on Arrow Down, and back to the dots on Escape", async () => {
    render(<ComponentDots dots={dots} />);
    await userEvent.tab();
    const note = screen.getByRole("note");
    await userEvent.keyboard("{ArrowDown}");
    expect(document.activeElement).toBe(note);
    await userEvent.keyboard("{Escape}");
    expect(screen.queryByRole("note")).toBeNull();
    expect(document.activeElement).toBe(trigger());
  });

  it("moves the focus on from the dots when Tab leaves the list", async () => {
    render(
      <>
        <ComponentDots dots={dots} />
        <ComponentDots dots={dots.slice(0, 1)} />
      </>,
    );
    const [first, second] = screen.getAllByRole("button", {
      name: /Components/,
    });
    await userEvent.tab();
    await userEvent.keyboard("{ArrowDown}");
    expect(document.activeElement).toBe(screen.getByRole("note"));
    await userEvent.tab();
    expect(document.activeElement).toBe(second);
    expect(first).toBeTruthy();
  });

  it("moves the focus back before the dots when Shift+Tab leaves the list", async () => {
    render(
      <>
        <input aria-label="Before" />
        <ComponentDots dots={dots} />
      </>,
    );
    await userEvent.tab();
    await userEvent.tab();
    expect(document.activeElement).toBe(trigger());
    await userEvent.keyboard("{ArrowDown}");
    await userEvent.tab({ shift: true });
    expect(document.activeElement).toBe(
      screen.getByRole("textbox", { name: "Before" }),
    );
  });

  it("starts open with defaultOpen", () => {
    render(<ComponentDots dots={dots} defaultOpen />);
    expect(screen.getByRole("note")).toBeTruthy();
  });

  it("shows a dash without components", () => {
    render(<ComponentDots dots={[]} />);
    expect(screen.queryByRole("button")).toBeNull();
    expect(screen.getByText("-")).toBeTruthy();
  });
});
