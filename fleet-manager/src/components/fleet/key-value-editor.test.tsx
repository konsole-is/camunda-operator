// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { KeyValueEditor } from "./key-value-editor";

afterEach(cleanup);

describe("KeyValueEditor", () => {
  it("returns the entries as a map", async () => {
    const onChange = vi.fn();
    render(
      <KeyValueEditor
        label="Pod labels"
        defaultValue={{ team: "payments" }}
        onChange={onChange}
      />,
    );
    await userEvent.click(screen.getByRole("button", { name: "Add an entry" }));
    const row = screen.getByRole("group", { name: "Entry 2" });
    await userEvent.type(
      within(row).getByRole("textbox", { name: "Key" }),
      "tier",
    );
    await userEvent.type(
      within(row).getByRole("textbox", { name: "Value" }),
      "gold",
    );
    expect(onChange).toHaveBeenLastCalledWith(
      { team: "payments", tier: "gold" },
      [],
    );
  });

  it("refuses a repeated key and, for labels, a bad key", async () => {
    const onChange = vi.fn();
    render(
      <KeyValueEditor
        label="Labels"
        format="label"
        defaultValue={{ team: "a" }}
        onChange={onChange}
      />,
    );
    await userEvent.click(screen.getByRole("button", { name: "Add an entry" }));
    const row = screen.getByRole("group", { name: "Entry 2" });
    await userEvent.type(
      within(row).getByRole("textbox", { name: "Key" }),
      "team",
    );
    expect(onChange.mock.lastCall![1]).toEqual([
      { path: "1.key", message: "team is set twice" },
    ]);
    await userEvent.clear(within(row).getByRole("textbox", { name: "Key" }));
    await userEvent.type(
      within(row).getByRole("textbox", { name: "Key" }),
      "-bad",
    );
    expect(onChange.mock.lastCall![1]).toEqual([
      {
        path: "1.key",
        message: "Use a valid label key, for example team or example.com/team",
      },
    ]);
    expect(
      within(row)
        .getByRole("textbox", { name: "Key" })
        .getAttribute("aria-invalid"),
    ).toBe("true");
  });

  it("removes an entry", async () => {
    const onChange = vi.fn();
    render(
      <KeyValueEditor
        label="Labels"
        defaultValue={{ a: "1", b: "2" }}
        onChange={onChange}
      />,
    );
    await userEvent.click(
      screen.getByRole("button", { name: "Remove entry 1" }),
    );
    expect(onChange).toHaveBeenLastCalledWith({ b: "2" }, []);
  });
});
