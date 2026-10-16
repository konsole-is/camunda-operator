// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { objectDiff } from "@/lib/diff";
import { DiffView } from "./diff-view";

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

  it("takes the keyboard focus, so a keyboard user can scroll it", () => {
    render(<DiffView label="Claim orders" lines={lines} />);
    const diff = screen.getByRole("figure", { name: "Claim orders" });
    diff.focus();
    expect(document.activeElement).toBe(diff);
  });

  it("says so when nothing changed", () => {
    render(<DiffView label="Claim" lines={objectDiff({ a: 1 }, { a: 1 })} />);
    expect(screen.getByText("No changes")).toBeTruthy();
  });
});
