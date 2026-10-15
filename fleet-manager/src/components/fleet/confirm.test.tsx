// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { InlineConfirm } from "./inline-confirm";
import { Toast } from "./toast";
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

describe("InlineConfirm", () => {
  const confirm = (onConfirm = vi.fn()) => (
    <InlineConfirm
      triggerLabel="Delete backup"
      confirmLabel="Delete"
      goes={["The stored artifacts of the backup"]}
      stays={["The other backups of the cluster"]}
      onConfirm={onConfirm}
    />
  );

  it("opens in place with what goes and what stays", async () => {
    render(confirm());
    expect(screen.queryByText("The stored artifacts of the backup")).toBeNull();
    await userEvent.click(button("Delete backup"));
    const group = screen.getByRole("group", { name: "Delete backup" });
    expect(group.textContent).toContain("The stored artifacts of the backup");
    expect(group.textContent).toContain("The other backups of the cluster");
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("closes on Cancel without a confirm", async () => {
    const onConfirm = vi.fn();
    render(confirm(onConfirm));
    await userEvent.click(button("Delete backup"));
    await userEvent.click(button("Cancel"));
    expect(onConfirm).not.toHaveBeenCalled();
    expect(document.activeElement).toBe(button("Delete backup"));
  });

  it("calls onConfirm on Delete", async () => {
    const onConfirm = vi.fn();
    render(confirm(onConfirm));
    await userEvent.click(button("Delete backup"));
    await userEvent.click(button("Delete"));
    expect(onConfirm).toHaveBeenCalledTimes(1);
  });
});

describe("Toast", () => {
  it("shows the Git waiting state with the short commit and no tool name", () => {
    render(
      <Toast result={{ kind: "waiting", commit: "4f1c3a7e9b2d8c6f0a5e" }} />,
    );
    const status = screen.getByRole("status");
    expect(status.textContent).toContain("Waiting for GitOps to apply");
    expect(status.textContent).toContain("4f1c3a7");
    expect(status.textContent).not.toMatch(/argo|flux/i);
  });

  it("announces an error as an alert", () => {
    render(
      <Toast
        result={{
          kind: "error",
          message: "The claim changed since you opened it.",
        }}
      />,
    );
    expect(screen.getByRole("alert").textContent).toContain(
      "The claim changed since you opened it.",
    );
  });

  it("shows a saved result as a status", () => {
    render(
      <Toast result={{ kind: "saved", message: "Saved the claim orders" }} />,
    );
    expect(screen.getByRole("status").textContent).toContain(
      "Saved the claim orders",
    );
  });
});
