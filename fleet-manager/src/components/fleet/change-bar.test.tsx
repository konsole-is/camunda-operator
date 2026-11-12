// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ChangeBar } from "./change-bar";

afterEach(cleanup);

const review = () =>
  screen.getByRole<HTMLButtonElement>("button", { name: "Review" });

describe("ChangeBar", () => {
  it("renders nothing without a pending change", () => {
    const { container } = render(<ChangeBar count={0} />);
    expect(container.innerHTML).toBe("");
  });

  it("counts the pending changes and offers Discard and Review", async () => {
    const onDiscard = vi.fn();
    const onReview = vi.fn();
    render(<ChangeBar count={2} onDiscard={onDiscard} onReview={onReview} />);
    const bar = screen.getByRole("region", { name: "Unsaved changes" });
    expect(within(bar).getByText("2 unsaved changes")).toBeTruthy();
    await userEvent.click(within(bar).getByRole("button", { name: "Review" }));
    await userEvent.click(within(bar).getByRole("button", { name: "Discard" }));
    expect(onReview).toHaveBeenCalledOnce();
    expect(onDiscard).toHaveBeenCalledOnce();
  });

  it("says 1 unsaved change for one change", () => {
    render(<ChangeBar count={1} />);
    expect(screen.getByText("1 unsaved change")).toBeTruthy();
    expect(review().disabled).toBe(false);
  });

  it("blocks Review while a field has an error and shows the errors without a path", () => {
    render(
      <ChangeBar
        count={1}
        errors={[
          { path: "spec.releaseRef", message: "Release x does not exist." },
          { message: "The claim is refused." },
        ]}
      />,
    );
    expect(review().disabled).toBe(true);
    expect(screen.getByText("Fix 2 fields before review.")).toBeTruthy();
    expect(screen.getByText("The claim is refused.")).toBeTruthy();
    // An error with a path shows next to its field.
    expect(screen.queryByText("Release x does not exist.")).toBeNull();
  });

  it("counts one field in the singular", () => {
    render(
      <ChangeBar
        count={3}
        errors={[{ path: "spec.presetRef", message: "Not allowed." }]}
      />,
    );
    expect(screen.getByText("Fix 1 field before review.")).toBeTruthy();
  });

  it("blocks Review with the blocked reason", () => {
    render(
      <ChangeBar count={1} blocked="Changes are off while the page loads." />,
    );
    expect(review().disabled).toBe(true);
    expect(
      screen.getByText("Changes are off while the page loads."),
    ).toBeTruthy();
  });
});
