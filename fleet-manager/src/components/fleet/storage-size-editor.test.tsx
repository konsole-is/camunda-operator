// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { StorageSizeEditor } from "./storage-size-editor";

afterEach(cleanup);

const SHRINK = "Volumes cannot shrink. The operator keeps the current size.";

function amount() {
  return screen.getByRole("textbox", { name: "Amount" });
}

describe("StorageSizeEditor", () => {
  it("writes the amount with its unit", async () => {
    const onChange = vi.fn();
    render(
      <StorageSizeEditor
        label="storageSize"
        defaultValue="32Gi"
        onChange={onChange}
      />,
    );
    await userEvent.clear(amount());
    await userEvent.type(amount(), "64");
    expect(onChange).toHaveBeenLastCalledWith("64Gi", []);
    await userEvent.selectOptions(
      screen.getByRole("combobox", { name: "Unit" }),
      "Ti",
    );
    expect(onChange).toHaveBeenLastCalledWith("64Ti", []);
  });

  it("keeps a unit that the list does not hold", () => {
    render(<StorageSizeEditor label="storageSize" defaultValue="100G" />);
    expect(amount()).toHaveProperty("value", "100");
    expect(screen.getByRole("combobox", { name: "Unit" })).toHaveProperty(
      "value",
      "G",
    );
  });

  it("keeps an exponent as its unit", async () => {
    const onChange = vi.fn();
    render(
      <StorageSizeEditor
        label="storageSize"
        defaultValue="1e9"
        current="2e9"
        onChange={onChange}
      />,
    );
    expect(amount()).toHaveProperty("value", "1");
    expect(screen.getByRole("combobox", { name: "Unit" })).toHaveProperty(
      "value",
      "e9",
    );
    expect(screen.queryByText("Enter a size above 0")).toBeNull();
    expect(screen.getByText(SHRINK)).toBeDefined();
    await userEvent.clear(amount());
    await userEvent.type(amount(), "3");
    expect(onChange).toHaveBeenLastCalledWith("3e9", []);
  });

  it("says that volumes cannot shrink below the current size", async () => {
    render(
      <StorageSizeEditor
        label="storageSize"
        defaultValue="32Gi"
        current="32Gi"
      />,
    );
    expect(screen.queryByText(SHRINK)).toBeNull();
    await userEvent.clear(amount());
    await userEvent.type(amount(), "16");
    expect(screen.getByText(SHRINK)).toBeDefined();
    await userEvent.clear(amount());
    await userEvent.type(amount(), "64");
    expect(screen.queryByText(SHRINK)).toBeNull();
  });

  it("compares sizes in different units", async () => {
    render(
      <StorageSizeEditor
        label="storageSize"
        defaultValue="1Ti"
        current="2Ti"
      />,
    );
    expect(screen.getByText(SHRINK)).toBeDefined();
    await userEvent.selectOptions(
      screen.getByRole("combobox", { name: "Unit" }),
      "Gi",
    );
    await userEvent.clear(amount());
    await userEvent.type(amount(), "4096");
    expect(screen.queryByText(SHRINK)).toBeNull();
  });

  it("refuses an amount that is not a positive number", async () => {
    const onChange = vi.fn();
    render(<StorageSizeEditor label="storageSize" onChange={onChange} />);
    await userEvent.type(amount(), "-1");
    const [, errors] = onChange.mock.lastCall!;
    expect(errors).toHaveLength(1);
    expect(errors[0].path).toBe("");
    expect(amount().getAttribute("aria-invalid")).toBe("true");
    expect(screen.getByText(errors[0].message)).toBeDefined();
    await userEvent.clear(amount());
    await userEvent.type(amount(), "0");
    expect(onChange.mock.lastCall![1]).toHaveLength(1);
  });

  it("empty is not set", async () => {
    const onChange = vi.fn();
    render(
      <StorageSizeEditor
        label="storageSize"
        defaultValue="32Gi"
        onChange={onChange}
      />,
    );
    await userEvent.clear(amount());
    expect(onChange).toHaveBeenLastCalledWith(undefined, []);
  });
});
