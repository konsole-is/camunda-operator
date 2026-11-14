// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { KeyValueEditor, keyValueErrors } from "./key-value-editor";

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

  it("gives undefined once no entry has a key", async () => {
    const onChange = vi.fn();
    render(
      <KeyValueEditor
        label="Labels"
        defaultValue={{ a: "1" }}
        onChange={onChange}
      />,
    );
    await userEvent.click(
      screen.getByRole("button", { name: "Remove entry 1" }),
    );
    expect(onChange).toHaveBeenLastCalledWith(undefined, []);
    await userEvent.click(screen.getByRole("button", { name: "Add an entry" }));
    expect(onChange).toHaveBeenLastCalledWith(undefined, []);
  });

  it("offers no empty map without emptyMap", () => {
    render(<KeyValueEditor label="Labels" defaultValue={{}} />);
    expect(
      screen.queryByRole("button", { name: "Set to an empty map" }),
    ).toBeNull();
    expect(screen.queryByText(/Empty map/)).toBeNull();
  });

  describe("with an inherited value", () => {
    it("says that no entries give the inherited value", () => {
      render(<KeyValueEditor label="Labels" emptyMap="inherited" />);
      expect(
        screen.getByText("No entries. The inherited value applies."),
      ).toBeTruthy();
    });

    it("sets the map to {} and back to the inherited value", async () => {
      const onChange = vi.fn();
      render(
        <KeyValueEditor
          label="Labels"
          emptyMap="inherited"
          defaultValue={{ team: "a" }}
          onChange={onChange}
        />,
      );
      expect(
        screen.queryByRole("button", { name: "Set to an empty map" }),
      ).toBeNull();
      await userEvent.click(
        screen.getByRole("button", { name: "Remove entry 1" }),
      );
      expect(onChange).toHaveBeenLastCalledWith(undefined, []);

      await userEvent.click(
        screen.getByRole("button", { name: "Set to an empty map" }),
      );
      expect(onChange).toHaveBeenLastCalledWith({}, []);
      const undo = screen.getByRole("button", {
        name: "Use the inherited value",
        description: "Empty map, overrides the inherited value",
      });
      expect(document.activeElement).toBe(undo);
      expect(
        screen.queryByText("No entries. The inherited value applies."),
      ).toBeNull();

      await userEvent.click(undo);
      expect(onChange).toHaveBeenLastCalledWith(undefined, []);
      expect(document.activeElement).toBe(
        screen.getByRole("button", { name: "Set to an empty map" }),
      );
      expect(screen.queryByText(/Empty map/)).toBeNull();
    });

    it("starts as the empty map for a defaultValue of {}", () => {
      render(
        <KeyValueEditor
          label="Labels"
          emptyMap="inherited"
          defaultValue={{}}
        />,
      );
      expect(
        screen.getByText("Empty map, overrides the inherited value"),
      ).toBeTruthy();
      expect(
        screen.queryByRole("button", { name: "Set to an empty map" }),
      ).toBeNull();
    });

    it("leaves the empty map when an entry is added", async () => {
      const onChange = vi.fn();
      render(
        <KeyValueEditor
          label="Labels"
          emptyMap="inherited"
          defaultValue={{}}
          onChange={onChange}
        />,
      );
      await userEvent.click(
        screen.getByRole("button", { name: "Add an entry" }),
      );
      expect(screen.queryByText(/Empty map/)).toBeNull();
      await userEvent.type(screen.getByRole("textbox", { name: "Key" }), "a");
      expect(onChange).toHaveBeenLastCalledWith({ a: "" }, []);
      await userEvent.click(
        screen.getByRole("button", { name: "Remove entry 1" }),
      );
      expect(onChange).toHaveBeenLastCalledWith(undefined, []);
    });
  });

  it("sets an unset map to {} and clears it", async () => {
    const onChange = vi.fn();
    render(
      <KeyValueEditor label="Value" emptyMap="unset" onChange={onChange} />,
    );
    expect(screen.queryByText(/inherited/)).toBeNull();
    await userEvent.click(
      screen.getByRole("button", { name: "Set to an empty map" }),
    );
    expect(onChange).toHaveBeenLastCalledWith({}, []);
    await userEvent.click(
      screen.getByRole("button", { name: "Clear", description: "Empty map" }),
    );
    expect(onChange).toHaveBeenLastCalledWith(undefined, []);
  });

  it("describes each input by the hint of the field and the error of its row", async () => {
    render(
      <>
        <KeyValueEditor
          label="Labels"
          format="label"
          defaultValue={{ team: "a" }}
          describedBy="labels-hint"
        />
        <p id="labels-hint">Leave empty to match every cluster.</p>
      </>,
    );
    const row = () => screen.getByRole("group", { name: "Entry 1" });
    expect(
      within(row()).getByRole("textbox", {
        name: "Key",
        description: "Leave empty to match every cluster.",
      }),
    ).toBeTruthy();
    await userEvent.type(
      within(row()).getByRole("textbox", { name: "Value" }),
      "-",
    );
    for (const name of ["Key", "Value"])
      expect(
        within(row()).getByRole("textbox", {
          name,
          description:
            "Use at most 63 letters, digits, '-', '_', or '.', starting and ending with a letter or digit Leave empty to match every cluster.",
        }),
      ).toBeTruthy();
  });
});

describe("keyValueErrors", () => {
  it("gives the errors that the editor shows, by row", () => {
    expect(
      keyValueErrors(
        [
          { key: "tier", value: "gold" },
          { key: "-x", value: "-y" },
          { key: "", value: "" },
          { key: "", value: "v" },
        ],
        "label",
      ),
    ).toEqual([
      {
        path: "1.key",
        message: "Use a valid label key, for example team or example.com/team",
      },
      {
        path: "1.value",
        message:
          "Use at most 63 letters, digits, '-', '_', or '.', starting and ending with a letter or digit",
      },
      { path: "3.key", message: "Enter a key" },
    ]);
  });

  it("checks no label rule without the label format", () => {
    expect(keyValueErrors([{ key: "-x", value: "-y" }])).toEqual([]);
  });
});
