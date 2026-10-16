// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { TypedConfirm } from "./typed-confirm";

afterEach(cleanup);

const button = (name: string) =>
  screen.getByRole("button", { name }) as HTMLButtonElement;

describe("TypedConfirm", () => {
  it("stays disabled until the typed name matches", async () => {
    const onConfirm = vi.fn();
    render(
      <TypedConfirm
        name="orders"
        confirmLabel="Delete cluster"
        onConfirm={onConfirm}
      />,
    );
    const input = screen.getByRole("textbox", {
      name: "Type orders to confirm",
    });

    expect(button("Delete cluster").disabled).toBe(true);
    await userEvent.type(input, "order");
    expect(button("Delete cluster").disabled).toBe(true);
    await userEvent.type(input, "s");
    expect(button("Delete cluster").disabled).toBe(false);
    await userEvent.click(button("Delete cluster"));
    expect(onConfirm).toHaveBeenCalledTimes(1);
  });

  it.each([["Orders"], [" orders"], ["orders "]])(
    "does not accept %j for orders",
    async (typed) => {
      render(<TypedConfirm name="orders" confirmLabel="Delete cluster" />);
      await userEvent.type(screen.getByRole("textbox"), typed);
      expect(button("Delete cluster").disabled).toBe(true);
    },
  );
});
