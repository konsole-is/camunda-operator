// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { objectDiff } from "@/lib/diff";
import { ReviewPanel } from "./review-panel";

afterEach(cleanup);

const lines = objectDiff({ spec: { replicas: 1 } }, { spec: { replicas: 3 } });

describe("ReviewPanel", () => {
  const panel = (props: Partial<Parameters<typeof ReviewPanel>[0]> = {}) => (
    <ReviewPanel
      lines={lines}
      effects={["The Zeebe pods restart one by one."]}
      mode="direct"
      confirmLabel="Change the claim"
      {...props}
    />
  );

  it("shows the direct label in direct mode", () => {
    render(panel());
    expect(
      screen.getByRole("button", { name: "Change the claim" }),
    ).toBeTruthy();
  });

  it("shows the Git label in Git mode", () => {
    render(panel({ mode: "git" }));
    expect(screen.getByRole("button", { name: "Push to Git" })).toBeTruthy();
    expect(
      screen.queryByRole("button", { name: "Change the claim" }),
    ).toBeNull();
  });

  it("uses the Git label that the page gives", () => {
    render(panel({ mode: "git", gitConfirmLabel: "Push anyway" }));
    expect(screen.getByRole("button", { name: "Push anyway" })).toBeTruthy();
  });

  it("renders the caution slot when given, and no caution otherwise", () => {
    const { rerender } = render(panel());
    expect(screen.queryByRole("note", { name: "Caution" })).toBeNull();
    rerender(
      panel({ caution: "A backup runs now. The change makes it fail." }),
    );
    expect(screen.getByRole("note", { name: "Caution" }).textContent).toContain(
      "A backup runs now.",
    );
  });

  it("lists the effects and calls onConfirm once per click", async () => {
    const onConfirm = vi.fn();
    render(panel({ onConfirm }));
    expect(screen.getByText("The Zeebe pods restart one by one.")).toBeTruthy();
    await userEvent.click(
      screen.getByRole("button", { name: "Change the claim" }),
    );
    expect(onConfirm).toHaveBeenCalledTimes(1);
  });

  it("goes back inside a form without a submit", async () => {
    const onSubmit = vi.fn((event: Event) => event.preventDefault());
    const onCancel = vi.fn();
    render(
      <form onSubmit={(event) => onSubmit(event.nativeEvent)}>
        {panel({ onCancel })}
      </form>,
    );
    await userEvent.click(screen.getByRole("button", { name: "Back" }));
    expect(onCancel).toHaveBeenCalledTimes(1);
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("enables the confirm only for the exact typed word", async () => {
    render(panel({ typed: "orders", confirmLabel: "Change anyway" }));
    const button = screen.getByRole<HTMLButtonElement>("button", {
      name: "Change anyway",
    });
    const field = screen.getByLabelText("Type orders to confirm");
    expect(button.disabled).toBe(true);
    for (const text of ["Orders", " orders", "orders ", "order"]) {
      await userEvent.clear(field);
      await userEvent.type(field, text);
      expect(button.disabled).toBe(true);
    }
    await userEvent.clear(field);
    await userEvent.type(field, "orders");
    expect(button.disabled).toBe(false);
  });

  it("keeps the confirm disabled while pending, also with the typed word", async () => {
    render(panel({ typed: "orders", pending: true }));
    await userEvent.type(
      screen.getByLabelText("Type orders to confirm"),
      "orders",
    );
    expect(
      screen.getByRole<HTMLButtonElement>("button", {
        name: "Change the claim",
      }).disabled,
    ).toBe(true);
  });

  it("shows no typed field without typed", () => {
    render(panel());
    expect(screen.queryByLabelText(/to confirm$/)).toBeNull();
  });

  it("blocks the confirm while the write runs", () => {
    render(panel({ pending: true }));
    expect(
      (
        screen.getByRole("button", {
          name: "Change the claim",
        }) as HTMLButtonElement
      ).disabled,
    ).toBe(true);
  });
});
