// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it } from "vitest";
import { LockedField } from "./locked-field";

afterEach(cleanup);

const field = (
  <LockedField label="Version" value="8.9.23" reason="Set by the release" />
);

describe("LockedField", () => {
  it("shows its value and its reason", () => {
    render(field);
    const group = screen.getByRole("group", { name: "Version" });
    expect(within(group).getByText("8.9.23")).toBeTruthy();
    expect(within(group).getByText("Set by the release")).toBeTruthy();
  });

  it("is not an input", () => {
    render(field);
    const group = screen.getByRole("group", { name: "Version" });
    for (const role of [
      "textbox",
      "combobox",
      "spinbutton",
      "checkbox",
      "button",
    ]) {
      expect(within(group).queryAllByRole(role)).toHaveLength(0);
    }
    expect(
      group.querySelectorAll(
        "input, select, textarea, [contenteditable], [tabindex]",
      ),
    ).toHaveLength(0);
  });

  it("takes no focus when the user tabs through the form", async () => {
    const user = userEvent.setup();
    render(
      <>
        <button type="button">Before</button>
        {field}
        <button type="button">After</button>
      </>,
    );
    await user.tab();
    expect(document.activeElement?.textContent).toBe("Before");
    await user.tab();
    expect(document.activeElement?.textContent).toBe("After");
  });
});
