// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { objectDiff } from "@/lib/diff";
import { DiffView } from "./diff-view";
import { ReviewPanel } from "./review-panel";

afterEach(cleanup);

const lines = objectDiff({ spec: { replicas: 1 } }, { spec: { replicas: 3 } });

describe("DiffView", () => {
  it("shows added lines as insertions and removed lines as deletions", () => {
    const { container } = render(
      <DiffView label="Claim orders" lines={lines} />,
    );
    expect(
      [...container.querySelectorAll("ins")].map((e) => e.textContent),
    ).toEqual(["  replicas: 3"]);
    expect(
      [...container.querySelectorAll("del")].map((e) => e.textContent),
    ).toEqual(["  replicas: 1"]);
  });

  it("folds a long run of unchanged lines and says how many it holds", () => {
    const before = Object.fromEntries(
      Array.from({ length: 20 }, (_, i) => [
        `k${String(i).padStart(2, "0")}`,
        i,
      ]),
    );
    const after = { ...before, k19: 99 };
    render(<DiffView label="Claim" lines={objectDiff(before, after)} />);
    expect(screen.getByText("16 unchanged lines")).toBeTruthy();
  });

  it("says so when nothing changed", () => {
    render(<DiffView label="Claim" lines={objectDiff({ a: 1 }, { a: 1 })} />);
    expect(screen.getByText("No changes")).toBeTruthy();
  });
});

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
