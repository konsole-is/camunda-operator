// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { fieldDocFor } from "@/lib/fielddocs";
import { FieldHelp } from "./field-help";

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

const partitions = fieldDocFor(
  "core.camunda.io",
  "CamundaCluster",
  "spec.zeebe.partitions",
);

describe("FieldHelp", () => {
  it("shows the description and the API reference link for a known path on hover", async () => {
    render(
      <FieldHelp label="Partitions" doc={partitions} operatorVersion="0.2.3" />,
    );
    expect(screen.queryByRole("note")).toBeNull();
    await userEvent.hover(
      screen.getByRole("button", { name: "About Partitions" }),
    );
    const note = screen.getByRole("note");
    expect(note.textContent).toContain(
      "Partitions is the number of partitions.",
    );
    expect(
      within(note)
        .getByRole("link", { name: "API reference" })
        .getAttribute("href"),
    ).toBe(
      "https://konsole-is.github.io/camunda-operator/0.2/crds/api-reference/#zeebespec",
    );
  });

  it("opens on keyboard focus and closes on Escape", async () => {
    render(
      <FieldHelp label="Partitions" doc={partitions} operatorVersion="0.2.3" />,
    );
    await userEvent.tab();
    expect(document.activeElement).toBe(
      screen.getByRole("button", { name: "About Partitions" }),
    );
    expect(screen.getByRole("note")).toBeTruthy();
    await userEvent.tab();
    expect(document.activeElement).toBe(
      screen.getByRole("link", { name: "API reference" }),
    );
    await userEvent.keyboard("{Escape}");
    expect(screen.queryByRole("note")).toBeNull();
    expect(document.activeElement).toBe(
      screen.getByRole("button", { name: "About Partitions" }),
    );
  });

  it("stays open when the pointer leaves while the focus is inside", async () => {
    render(
      <FieldHelp label="Partitions" doc={partitions} operatorVersion="0.2.3" />,
    );
    const button = screen.getByRole("button", { name: "About Partitions" });
    await userEvent.hover(button);
    await userEvent.tab();
    await userEvent.tab();
    expect(document.activeElement).toBe(
      screen.getByRole("link", { name: "API reference" }),
    );
    await userEvent.unhover(button);
    expect(screen.getByRole("note")).toBeTruthy();
  });

  // jsdom has no layout, so these cases give the box its position and the
  // window its width.
  it.each([
    ["near the middle of a phone window", 180, "-78px"],
    ["so wide that it can only start at the left edge", 20, "-20px"],
  ])("keeps the box inside the window %s", (_, left, shift) => {
    const width = left === 20 ? 400 : 288;
    vi.spyOn(document.documentElement, "clientWidth", "get").mockReturnValue(
      390,
    );
    vi.spyOn(HTMLElement.prototype, "getBoundingClientRect").mockReturnValue(
      new DOMRect(left, 0, width, 100),
    );
    render(
      <FieldHelp
        label="Partitions"
        doc={partitions}
        operatorVersion="0.2.3"
        defaultOpen
      />,
    );
    const box = screen.getByRole("note").parentElement!;
    expect(box.style.left).toBe(shift);
  });

  it("renders nothing for a path without a description", () => {
    const { container } = render(
      <FieldHelp
        label="Constructor"
        doc={fieldDocFor("core.camunda.io", "CamundaCluster", "constructor")}
        operatorVersion="0.2.3"
      />,
    );
    expect(container.innerHTML).toBe("");
  });
});
