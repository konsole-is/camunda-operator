// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { InlineConfirm } from "./inline-confirm";

afterEach(cleanup);

const button = (name: string) =>
  screen.getByRole("button", { name }) as HTMLButtonElement;

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

  it("shows children, keeps the confirm off while confirmDisabled, and calls onCancel on Cancel", async () => {
    const onConfirm = vi.fn();
    const onCancel = vi.fn();
    render(
      <InlineConfirm
        triggerLabel="Resume"
        confirmLabel="Resume the plane"
        goes={[]}
        defaultOpen
        confirmDisabled
        onConfirm={onConfirm}
        onCancel={onCancel}
      >
        <p>A choice first</p>
      </InlineConfirm>,
    );
    const group = screen.getByRole("group", { name: "Resume" });
    expect(group.textContent).toContain("A choice first");
    expect(group.textContent).not.toContain("What goes");
    expect(button("Resume the plane").disabled).toBe(true);
    await userEvent.click(button("Cancel"));
    expect(onCancel).toHaveBeenCalledTimes(1);
    expect(onConfirm).not.toHaveBeenCalled();
  });
});
