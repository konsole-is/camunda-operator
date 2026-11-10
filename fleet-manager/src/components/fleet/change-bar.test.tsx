// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ChangeBar } from "./change-bar";

afterEach(cleanup);

describe("ChangeBar", () => {
  it("renders nothing for no change", () => {
    const { container } = render(<ChangeBar count={0} />);
    expect(container.innerHTML).toBe("");
  });

  it("shows the count, Discard, and Review", () => {
    const onDiscard = vi.fn();
    const onReview = vi.fn();
    const { rerender } = render(
      <ChangeBar count={2} onDiscard={onDiscard} onReview={onReview} />,
    );
    expect(screen.getByText("2 changes")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Discard" }));
    fireEvent.click(screen.getByRole("button", { name: "Review" }));
    expect(onDiscard).toHaveBeenCalledTimes(1);
    expect(onReview).toHaveBeenCalledTimes(1);
    rerender(<ChangeBar count={1} />);
    expect(screen.getByText("1 change")).toBeTruthy();
  });

  it("disables Review and shows the reason while blocked", () => {
    render(<ChangeBar count={1} blocked="Give the environment a name." />);
    expect(screen.getByRole("button", { name: "Review" })).toHaveProperty(
      "disabled",
      true,
    );
    expect(screen.getByText("Give the environment a name.")).toBeTruthy();
  });

  it("disables Review while there are errors, and shows each error without a path", () => {
    render(
      <ChangeBar
        count={1}
        errors={[
          { message: "The claim changed since you opened it." },
          { path: "spec.cluster.zeebe.replicas", message: "At least 1." },
        ]}
      />,
    );
    expect(screen.getByRole("button", { name: "Review" })).toHaveProperty(
      "disabled",
      true,
    );
    expect(
      screen.getByText("The claim changed since you opened it."),
    ).toBeTruthy();
    expect(screen.queryByText("At least 1.")).toBeNull();
  });
});
